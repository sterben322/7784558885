package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"lastop/database"
	"lastop/utils"

	"github.com/gin-gonic/gin"
)

func abortUnauthorized(c *gin.Context, message string) {
	acceptHeader := c.GetHeader("Accept")
	if c.Request.Method == http.MethodGet && strings.Contains(acceptHeader, "text/html") {
		if c.Writer != nil {
			c.Writer.Header().Set("Location", "/login.html")
			c.Writer.WriteHeader(http.StatusFound)
		}
		c.Abort()
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"error": message})
	c.Abort()
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortUnauthorized(c, "Authorization header required")
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			abortUnauthorized(c, "Bearer token required")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := utils.ValidateJWT(tokenString)
		if err != nil {
			abortUnauthorized(c, "Invalid token")
			return
		}

		if !database.IsReady() {
			if !database.IsConfigured() {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "DATABASE_URL is not configured"})
				c.Abort()
				return
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database is initializing, retry shortly"})
			c.Abort()
			return
		}

		pingCtx, pingCancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		if err := database.Ping(pingCtx); err != nil {
			pingCancel()
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database is unavailable"})
			c.Abort()
			return
		}
		pingCancel()

		var sessionUserID string
		var expiresAt time.Time
		err = database.DB.QueryRow(`
            SELECT user_id::text, expires_at FROM sessions
            WHERE token = $1
		`, tokenString).Scan(&sessionUserID, &expiresAt)
		if err == sql.ErrNoRows {
			abortUnauthorized(c, "Session not found")
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate session"})
			c.Abort()
			return
		}
		if expiresAt.Before(time.Now()) || sessionUserID != userID.String() {
			abortUnauthorized(c, "Session expired")
			return
		}

		c.Set("user_id", userID)
		c.Set("token", tokenString)
		c.Next()
	}
}
