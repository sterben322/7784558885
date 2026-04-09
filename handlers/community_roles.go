package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func GetCommunityRoles(c *gin.Context) {
	communityID := c.Param("id")
	_ = database.EnsureDefaultCommunityRoles(communityID)
	rows, err := database.DB.Query(`
        SELECT name, display_name, permissions
        FROM community_roles
        WHERE community_id = $1
        ORDER BY CASE name WHEN 'admin' THEN 1 WHEN 'moderator' THEN 2 WHEN 'editor' THEN 3 ELSE 4 END
    `, communityID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	roles := make([]models.CommunityRole, 0)
	for rows.Next() {
		var role models.CommunityRole
		var permissions pq.StringArray
		if err := rows.Scan(&role.Name, &role.DisplayName, &permissions); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		role.Permissions = permissions
		roles = append(roles, role)
	}
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func GetCommunityMembersWithRoles(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)
	var isPrivate bool
	if err := database.DB.QueryRow(`SELECT is_private FROM communities WHERE id = $1`, communityID).Scan(&isPrivate); err == nil && isPrivate && !isCommunityMember(communityID, userID) {
		jsonError(c, http.StatusForbidden, "Members list is hidden for private community")
		return
	}
	rows, err := database.DB.Query(`
        SELECT u.id, u.full_name, u.email, u.avatar_url,
               COALESCE(cur.role_name, 'member') AS role_name,
               COALESCE(cr.display_name, 'Участник') AS role_display,
               cur.assigned_at
        FROM community_members cm
        JOIN users u ON cm.user_id = u.id
        LEFT JOIN community_user_roles cur ON cur.community_id = cm.community_id AND cur.user_id = cm.user_id
        LEFT JOIN community_roles cr ON cr.community_id = cm.community_id AND cr.name = COALESCE(cur.role_name, 'member')
        WHERE cm.community_id = $1
        ORDER BY CASE COALESCE(cur.role_name, 'member') WHEN 'admin' THEN 1 WHEN 'moderator' THEN 2 WHEN 'editor' THEN 3 ELSE 4 END, u.full_name
    `, communityID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	members := make([]map[string]interface{}, 0)
	for rows.Next() {
		var userID uuid.UUID
		var userName, userEmail string
		var avatarURL sql.NullString
		var roleName, roleDisplay string
		var assignedAt sql.NullTime
		if err := rows.Scan(&userID, &userName, &userEmail, &avatarURL, &roleName, &roleDisplay, &assignedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		member := map[string]interface{}{
			"user_id":      userID,
			"user_name":    userName,
			"user_email":   userEmail,
			"role_name":    roleName,
			"role_display": roleDisplay,
		}
		if avatarURL.Valid {
			member["user_avatar"] = avatarURL.String
		}
		if assignedAt.Valid {
			member["assigned_at"] = assignedAt.Time
		}
		members = append(members, member)
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func AssignCommunityRole(c *gin.Context) {
	communityID := c.Param("id")
	adminID := currentUserID(c)
	var req models.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	var isAdmin bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_user_roles WHERE community_id = $1 AND user_id = $2 AND role_name = 'admin')`, communityID, adminID).Scan(&isAdmin)
	if !isAdmin {
		allowed, err := requireCommunityOwner(communityID, adminID)
		if err != nil || !allowed {
			jsonError(c, http.StatusForbidden, "Only admin can assign roles")
			return
		}
	}

	var isMember bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`, communityID, req.UserID).Scan(&isMember)
	if !isMember {
		jsonError(c, http.StatusBadRequest, "User is not a member")
		return
	}

	if _, err := database.DB.Exec(`
        INSERT INTO community_user_roles (community_id, user_id, role_name, assigned_by, assigned_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (community_id, user_id) DO UPDATE SET role_name = EXCLUDED.role_name, assigned_by = EXCLUDED.assigned_by, assigned_at = NOW()
    `, communityID, req.UserID, req.RoleName, adminID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to assign role")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

func RemoveCommunityRole(c *gin.Context) {
	communityID := c.Param("id")
	targetUserID := c.Param("user_id")
	adminID := currentUserID(c)

	var isAdmin bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_user_roles WHERE community_id = $1 AND user_id = $2 AND role_name = 'admin')`, communityID, adminID).Scan(&isAdmin)
	if !isAdmin {
		allowed, err := requireCommunityOwner(communityID, adminID)
		if err != nil || !allowed {
			jsonError(c, http.StatusForbidden, "Only admin can remove roles")
			return
		}
	}

	var ownerID uuid.UUID
	if err := database.DB.QueryRow(`SELECT owner_id FROM communities WHERE id = $1`, communityID).Scan(&ownerID); err != nil {
		jsonError(c, http.StatusNotFound, "Community not found")
		return
	}
	if ownerID.String() == targetUserID {
		jsonError(c, http.StatusBadRequest, "Cannot remove role from owner")
		return
	}

	if _, err := database.DB.Exec(`DELETE FROM community_user_roles WHERE community_id = $1 AND user_id = $2`, communityID, targetUserID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to remove role")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Role removed successfully"})
}

func InviteToCommunity(c *gin.Context) {
	communityID := c.Param("id")
	inviterID := currentUserID(c)
	var req models.InviteToCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	var isMember bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`, communityID, req.UserID).Scan(&isMember)
	if isMember {
		jsonError(c, http.StatusBadRequest, "User is already a member")
		return
	}
	if req.RoleName == "" {
		req.RoleName = "member"
	}

	inviteID := uuid.New()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err := database.DB.Exec(`
        INSERT INTO community_invites (id, community_id, inviter_id, invitee_id, role_name, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, inviteID, communityID, inviterID, req.UserID, req.RoleName, expiresAt)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to send invite")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Invite sent successfully"})
}

func AcceptCommunityInvite(c *gin.Context) {
	inviteID := c.Param("invite_id")
	userID := currentUserID(c)

	var communityID uuid.UUID
	var roleName string
	var expiresAt time.Time
	err := database.DB.QueryRow(`
        SELECT community_id, role_name, expires_at
        FROM community_invites
        WHERE id = $1 AND invitee_id = $2 AND status = 'pending'
    `, inviteID, userID).Scan(&communityID, &roleName, &expiresAt)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Invite not found")
		return
	}
	if expiresAt.Before(time.Now()) {
		_, _ = database.DB.Exec(`UPDATE community_invites SET status = 'expired' WHERE id = $1`, inviteID)
		jsonError(c, http.StatusBadRequest, "Invite has expired")
		return
	}
	if roleName == "" {
		roleName = "member"
	}

	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	_, _ = tx.Exec(`INSERT INTO community_members (community_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, communityID, userID)
	_, _ = tx.Exec(`INSERT INTO community_user_roles (community_id, user_id, role_name) VALUES ($1, $2, $3) ON CONFLICT (community_id, user_id) DO UPDATE SET role_name = EXCLUDED.role_name, assigned_at = NOW()`, communityID, userID, roleName)
	if _, err = tx.Exec(`UPDATE community_invites SET status = 'accepted' WHERE id = $1`, inviteID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to accept invite")
		return
	}
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to commit invite")
		return
	}
	recalcCommunityMembersCount(communityID.String())
	c.JSON(http.StatusOK, gin.H{"message": "Invite accepted"})
}
