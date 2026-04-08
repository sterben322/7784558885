package middleware

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"lastop/database"
	"lastop/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := utils.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		var sessionUserID string
		var expiresAt time.Time
		err = database.DB.QueryRow(`
            SELECT user_id::text, expires_at FROM sessions
            WHERE token = $1
        `, tokenString).Scan(&sessionUserID, &expiresAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
			c.Abort()
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate session"})
			c.Abort()
			return
		}
		if expiresAt.Before(time.Now()) || sessionUserID != userID.String() {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("token", tokenString)
		c.Next()
	}
}
