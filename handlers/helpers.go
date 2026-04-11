package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"lastop/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func currentUserID(c *gin.Context) uuid.UUID {
	v, _ := c.Get("user_id")
	id, _ := v.(uuid.UUID)
	return id
}

func currentToken(c *gin.Context) string {
	v, _ := c.Get("token")
	s, _ := v.(string)
	return s
}

func jsonError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func ensureDatabase(c *gin.Context) bool {
	if database.DB != nil {
		return true
	}
	jsonError(c, http.StatusServiceUnavailable, "Database is unavailable. Please configure DB connection and try again.")
	return false
}

func requireChatParticipant(chatID string, userID uuid.UUID) error {
	var exists bool
	err := database.DB.QueryRow(`
        SELECT EXISTS(SELECT 1 FROM chat_participants WHERE chat_id = $1 AND user_id = $2)
    `, chatID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("forbidden")
	}
	return nil
}

func requireCommunityOwner(communityID string, userID uuid.UUID) (bool, error) {
	var ownerID uuid.UUID
	err := database.DB.QueryRow(`SELECT owner_id FROM communities WHERE id = $1`, communityID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

func requireCompanyOwner(companyID string, userID uuid.UUID) (bool, error) {
	var ownerID uuid.UUID
	err := database.DB.QueryRow(`SELECT owner_id FROM companies WHERE id = $1`, companyID).Scan(&ownerID)
	if err != nil {
		return false, err
	}
	return ownerID == userID, nil
}

func recalcCommunityMembersCount(communityID string) {
	_, _ = database.DB.Exec(`
        UPDATE communities
        SET members_count = (SELECT COUNT(*) FROM community_members WHERE community_id = $1),
            updated_at = NOW()
        WHERE id = $1
    `, communityID)
}

func recalcCompanyFollowersCount(companyID string) {
	_, _ = database.DB.Exec(`
        UPDATE companies
        SET followers_count = (SELECT COUNT(*) FROM company_followers WHERE company_id = $1),
            updated_at = NOW()
        WHERE id = $1
    `, companyID)
}

func recalcCompanyEmployeesCount(companyID string) {
	_, _ = database.DB.Exec(`
        UPDATE companies
        SET employee_count = (SELECT COUNT(*) FROM company_employees WHERE company_id = $1 AND is_active = true),
            updated_at = NOW()
        WHERE id = $1
    `, companyID)
}

func recalcPostLikes(postID string) {
	_, _ = database.DB.Exec(`UPDATE posts SET likes_count = (SELECT COUNT(*) FROM post_likes WHERE post_id = $1) WHERE id = $1`, postID)
}

func recalcPostComments(postID string) {
	_, _ = database.DB.Exec(`UPDATE posts SET comments_count = (SELECT COUNT(*) FROM post_comments WHERE post_id = $1) WHERE id = $1`, postID)
}

func rowsAffectedOrError(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func authRequiredPage(c *gin.Context) {
	if currentToken(c) == "" {
		jsonError(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
}

func isCommunityMember(communityID string, userID uuid.UUID) bool {
	var exists bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`, communityID, userID).Scan(&exists)
	return exists
}

func isCompanyMember(companyID string, userID uuid.UUID) bool {
	var exists bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_members WHERE company_id = $1 AND user_id = $2)`, companyID, userID).Scan(&exists)
	if exists {
		return true
	}
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_employees WHERE company_id = $1 AND user_id = $2 AND is_active = true)`, companyID, userID).Scan(&exists)
	return exists
}
