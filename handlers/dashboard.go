package handlers

import (
	"net/http"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
)

func GetDashboardStats(c *gin.Context) {
	userID := currentUserID(c)
	var stats models.DashboardStats

	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM community_members WHERE user_id = $1`, userID).Scan(&stats.CommunitiesJoined)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM forum_topics WHERE author_id = $1`, userID).Scan(&stats.TopicsCount)
	_ = database.DB.QueryRow(`
		SELECT
			COALESCE((
				SELECT COUNT(*)
				FROM post_likes pl
				JOIN posts p ON p.id = pl.post_id
				WHERE p.author_type = 'user' AND p.author_id = $1 AND pl.user_id <> $1
			), 0)
			+
			COALESCE((
				SELECT COUNT(*)
				FROM post_comments pc
				JOIN posts p ON p.id = pc.post_id
				WHERE p.author_type = 'user' AND p.author_id = $1 AND pc.author_id <> $1
			), 0)
	`, userID).Scan(&stats.UnreadNotifications)
	_ = database.DB.QueryRow(`SELECT COALESCE(SUM(unread_count), 0) FROM chat_participants WHERE user_id = $1`, userID).Scan(&stats.UnreadMessages)
	_ = database.DB.QueryRow(`SELECT COUNT(*) FROM company_employees WHERE user_id = $1 AND is_active = true`, userID).Scan(&stats.ProjectsCount)

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}
