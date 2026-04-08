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

func scanCommunityRows(rows *sql.Rows, communities *[]models.Community) error {
	for rows.Next() {
		var comm models.Community
		var logoURL sql.NullString
		var tags pq.StringArray
		if err := rows.Scan(&comm.ID, &comm.Name, &comm.Description, &logoURL, &comm.Icon, &comm.Color, &tags, &comm.IsPrivate, &comm.OwnerID, &comm.OwnerName, &comm.MembersCount, &comm.PostsCount, &comm.Joined, &comm.CreatedAt); err != nil {
			return err
		}
		if logoURL.Valid {
			comm.LogoURL = &logoURL.String
		}
		comm.SearchTags = tags
		*communities = append(*communities, comm)
	}
	return nil
}

func GetCommunities(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT c.id, c.name, c.description, c.logo_url, c.icon, c.color, c.search_tags, c.is_private,
               c.owner_id, u.full_name, c.members_count, c.posts_count,
               EXISTS(SELECT 1 FROM community_members WHERE community_id = c.id AND user_id = $1) AS joined,
               c.created_at
        FROM communities c
        JOIN users u ON c.owner_id = u.id
        ORDER BY c.members_count DESC, c.created_at DESC
    `, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	communities := make([]models.Community, 0)
	if err := scanCommunityRows(rows, &communities); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"communities": communities})
}

func GetCommunity(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)

	var comm models.Community
	var logoURL sql.NullString
	var tags pq.StringArray
	err := database.DB.QueryRow(`
        SELECT c.id, c.name, c.description, c.logo_url, c.icon, c.color, c.search_tags, c.is_private,
               c.owner_id, u.full_name, c.members_count, c.posts_count,
               EXISTS(SELECT 1 FROM community_members WHERE community_id = c.id AND user_id = $1) AS joined,
               c.created_at
        FROM communities c
        JOIN users u ON c.owner_id = u.id
        WHERE c.id = $2
    `, userID, communityID).Scan(&comm.ID, &comm.Name, &comm.Description, &logoURL, &comm.Icon, &comm.Color, &tags, &comm.IsPrivate, &comm.OwnerID, &comm.OwnerName, &comm.MembersCount, &comm.PostsCount, &comm.Joined, &comm.CreatedAt)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "Community not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if logoURL.Valid {
		comm.LogoURL = &logoURL.String
	}
	comm.SearchTags = tags
	c.JSON(http.StatusOK, gin.H{"community": comm})
}

func CreateCommunity(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	var exists bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM communities WHERE name = $1)`, req.Name).Scan(&exists)
	if exists {
		jsonError(c, http.StatusConflict, "Community with this name already exists")
		return
	}

	communityID := uuid.New()
	if req.Icon == "" {
		req.Icon = "fa-users"
	}
	if req.Color == "" {
		req.Color = "blue"
	}

	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
        INSERT INTO communities (id, name, description, logo_url, icon, color, search_tags, is_private, owner_id, members_count)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1)
    `, communityID, req.Name, req.Description, req.LogoURL, req.Icon, req.Color, pq.Array(req.SearchTags), req.IsPrivate, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create community")
		return
	}

	if _, err = tx.Exec(`INSERT INTO community_members (community_id, user_id) VALUES ($1, $2)`, communityID, userID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to add owner as member")
		return
	}

	if _, err = tx.Exec(`INSERT INTO community_user_roles (community_id, user_id, role_name, assigned_by) VALUES ($1, $2, 'admin', $2)`, communityID, userID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to assign admin role")
		return
	}

	if err = tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	if err = database.EnsureDefaultCommunityRoles(communityID.String()); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create default roles")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Community created successfully", "community_id": communityID})
}

func UpdateCommunity(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)
	allowed, err := requireCommunityOwner(communityID, userID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Community not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only community owner can update")
		return
	}

	var req models.CreateCommunityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Icon == "" {
		req.Icon = "fa-users"
	}
	if req.Color == "" {
		req.Color = "blue"
	}

	_, err = database.DB.Exec(`
        UPDATE communities
        SET name = $1, description = $2, logo_url = $3, icon = $4, color = $5, search_tags = $6, is_private = $7, updated_at = NOW()
        WHERE id = $8
    `, req.Name, req.Description, req.LogoURL, req.Icon, req.Color, pq.Array(req.SearchTags), req.IsPrivate, communityID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update community")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Community updated successfully"})
}

func DeleteCommunity(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)
	allowed, err := requireCommunityOwner(communityID, userID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Community not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only community owner can delete")
		return
	}
	if _, err := database.DB.Exec(`DELETE FROM communities WHERE id = $1`, communityID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to delete community")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Community deleted successfully"})
}

func JoinCommunity(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)

	var isPrivate bool
	err := database.DB.QueryRow(`SELECT is_private FROM communities WHERE id = $1`, communityID).Scan(&isPrivate)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "Community not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if isPrivate {
		jsonError(c, http.StatusBadRequest, "This is a private community. Send a join request instead.")
		return
	}

	result, err := database.DB.Exec(`INSERT INTO community_members (community_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, communityID, userID)
	rows, err := rowsAffectedOrError(result, err)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to join community")
		return
	}
	if rows > 0 {
		_, _ = database.DB.Exec(`INSERT INTO community_user_roles (community_id, user_id, role_name) VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING`, communityID, userID)
		recalcCommunityMembersCount(communityID)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Joined community successfully"})
}

func LeaveCommunity(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)

	allowed, err := requireCommunityOwner(communityID, userID)
	if err == nil && allowed {
		jsonError(c, http.StatusBadRequest, "Community owner cannot leave")
		return
	}

	_, err = database.DB.Exec(`DELETE FROM community_members WHERE community_id = $1 AND user_id = $2`, communityID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to leave community")
		return
	}
	_, _ = database.DB.Exec(`DELETE FROM community_user_roles WHERE community_id = $1 AND user_id = $2`, communityID, userID)
	recalcCommunityMembersCount(communityID)

	c.JSON(http.StatusOK, gin.H{"message": "Left community successfully"})
}

func GetMyCommunities(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT c.id, c.name, c.description, c.logo_url, c.icon, c.color, c.search_tags, c.is_private,
               c.owner_id, u.full_name, c.members_count, c.posts_count, true AS joined, c.created_at
        FROM communities c
        JOIN users u ON c.owner_id = u.id
        JOIN community_members cm ON c.id = cm.community_id
        WHERE cm.user_id = $1
        ORDER BY c.created_at DESC
    `, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	communities := make([]models.Community, 0)
	if err := scanCommunityRows(rows, &communities); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"communities": communities})
}

func RequestJoinCommunity(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Message *string `json:"message"`
	}
	_ = c.ShouldBindJSON(&req)

	var isMember bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`, communityID, userID).Scan(&isMember)
	if isMember {
		jsonError(c, http.StatusBadRequest, "Already a member")
		return
	}

	var requestExists bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_join_requests WHERE community_id = $1 AND user_id = $2 AND status = 'pending')`, communityID, userID).Scan(&requestExists)
	if requestExists {
		jsonError(c, http.StatusBadRequest, "Request already sent")
		return
	}

	requestID := uuid.New()
	if _, err := database.DB.Exec(`INSERT INTO community_join_requests (id, community_id, user_id, message) VALUES ($1, $2, $3, $4) ON CONFLICT (community_id, user_id) DO UPDATE SET status = 'pending', message = EXCLUDED.message, created_at = NOW()`, requestID, communityID, userID, req.Message); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create join request")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Join request sent"})
}

func GetJoinRequests(c *gin.Context) {
	communityID := c.Param("id")
	userID := currentUserID(c)
	allowed, err := requireCommunityOwner(communityID, userID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Community not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only community owner can view requests")
		return
	}

	rows, err := database.DB.Query(`
        SELECT r.id, r.community_id, cm.name, r.user_id, u.full_name, r.status, r.message, r.created_at
        FROM community_join_requests r
        JOIN communities cm ON r.community_id = cm.id
        JOIN users u ON r.user_id = u.id
        WHERE r.community_id = $1 AND r.status = 'pending'
        ORDER BY r.created_at ASC
    `, communityID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	requests := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, commID, requesterID uuid.UUID
		var communityName, userName, status string
		var message sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&id, &commID, &communityName, &requesterID, &userName, &status, &message, &createdAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		item := map[string]interface{}{
			"id": id, "community_id": commID, "community_name": communityName,
			"user_id": requesterID, "user_name": userName, "status": status, "created_at": createdAt,
		}
		if message.Valid {
			item["message"] = message.String
		}
		requests = append(requests, item)
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func ApproveJoinRequest(c *gin.Context) {
	requestID := c.Param("request_id")
	userID := currentUserID(c)

	var communityID, requestUserID uuid.UUID
	err := database.DB.QueryRow(`SELECT community_id, user_id FROM community_join_requests WHERE id = $1`, requestID).Scan(&communityID, &requestUserID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Join request not found")
		return
	}

	allowed, err := requireCommunityOwner(communityID.String(), userID)
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Only community owner can approve requests")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	_, _ = tx.Exec(`INSERT INTO community_members (community_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, communityID, requestUserID)
	_, _ = tx.Exec(`INSERT INTO community_user_roles (community_id, user_id, role_name) VALUES ($1, $2, 'member') ON CONFLICT DO NOTHING`, communityID, requestUserID)
	if _, err = tx.Exec(`UPDATE community_join_requests SET status = 'approved' WHERE id = $1`, requestID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to approve request")
		return
	}
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}
	recalcCommunityMembersCount(communityID.String())
	c.JSON(http.StatusOK, gin.H{"message": "Join request approved"})
}

func RejectJoinRequest(c *gin.Context) {
	requestID := c.Param("request_id")
	userID := currentUserID(c)

	var communityID uuid.UUID
	err := database.DB.QueryRow(`SELECT community_id FROM community_join_requests WHERE id = $1`, requestID).Scan(&communityID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Join request not found")
		return
	}
	allowed, err := requireCommunityOwner(communityID.String(), userID)
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Only community owner can reject requests")
		return
	}

	if _, err := database.DB.Exec(`UPDATE community_join_requests SET status = 'rejected' WHERE id = $1`, requestID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to reject request")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Join request rejected"})
}

func SearchCommunities(c *gin.Context) {
	userID := currentUserID(c)
	query := c.Query("q")
	rows, err := database.DB.Query(`
        SELECT c.id, c.name, c.description, c.logo_url, c.icon, c.color, c.search_tags, c.is_private,
               c.owner_id, u.full_name, c.members_count, c.posts_count,
               EXISTS(SELECT 1 FROM community_members WHERE community_id = c.id AND user_id = $1) AS joined,
               c.created_at
        FROM communities c
        JOIN users u ON c.owner_id = u.id
        WHERE c.name ILIKE '%' || $2 || '%' OR EXISTS (SELECT 1 FROM unnest(c.search_tags) tag WHERE tag ILIKE '%' || $2 || '%')
        ORDER BY c.members_count DESC, c.created_at DESC
        LIMIT 50
    `, userID, query)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	communities := make([]models.Community, 0)
	if err := scanCommunityRows(rows, &communities); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"communities": communities})
}
