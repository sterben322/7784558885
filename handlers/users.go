package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetUsers(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	current := currentUserID(c)
	search := strings.TrimSpace(c.Query("q"))

	rows, err := database.DB.Query(`
		SELECT id,
		       COALESCE(first_name, '') AS first_name,
		       COALESCE(last_name, '') AS last_name,
		       full_name,
		       email,
		       COALESCE(company_name, '') AS company_name,
		       COALESCE(phone, '') AS phone,
		       COALESCE(position, '') AS position,
		       COALESCE(avatar_url, '') AS avatar_url,
		       is_private_profile,
		       created_at
		FROM users
		WHERE id <> $1
		  AND (
		    $2 = ''
		    OR full_name ILIKE '%' || $2 || '%'
		    OR COALESCE(first_name, '') ILIKE '%' || $2 || '%'
		    OR COALESCE(last_name, '') ILIKE '%' || $2 || '%'
		    OR COALESCE(name, '') ILIKE '%' || $2 || '%'
		    OR email ILIKE '%' || $2 || '%'
		  )
		ORDER BY full_name ASC
		LIMIT 100
	`, current, search)
	if err != nil {
		log.Printf("users list query failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load users")
		return
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.FullName, &u.Email, &u.CompanyName, &u.Phone, &u.Position, &u.AvatarURL, &u.IsPrivateProfile, &u.CreatedAt); err != nil {
			log.Printf("users list scan failed: %v", err)
			jsonError(c, http.StatusInternalServerError, "Failed to parse users")
			return
		}
		users = append(users, u)
	}

	c.JSON(http.StatusOK, gin.H{"users": users})
}

func GetUserFriends(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid user id")
		return
	}

	viewerID := currentUserID(c)
	isSelf := viewerID == targetID

	var isPrivate bool
	err = database.DB.QueryRow(`SELECT is_private_profile FROM users WHERE id = $1`, targetID).Scan(&isPrivate)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		log.Printf("user privacy query failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load user")
		return
	}

	if isPrivate && !isSelf {
		c.JSON(http.StatusForbidden, gin.H{"error": "Профиль закрыт"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT u.id, u.full_name, u.email, u.avatar_url, uf.updated_at
		FROM user_friends uf
		JOIN users u ON u.id = CASE WHEN uf.user_id = $1 THEN uf.friend_id ELSE uf.user_id END
		WHERE $1 IN (uf.user_id, uf.friend_id)
		  AND uf.status = 'accepted'
		ORDER BY uf.updated_at DESC
	`, targetID)
	if err != nil {
		log.Printf("user friends query failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load friends")
		return
	}
	defer rows.Close()

	friends := make([]models.Friend, 0)
	for rows.Next() {
		var friend models.Friend
		var avatar sql.NullString
		if err := rows.Scan(&friend.FriendID, &friend.FriendName, &friend.FriendEmail, &avatar, &friend.CreatedAt); err != nil {
			log.Printf("user friends scan failed: %v", err)
			jsonError(c, http.StatusInternalServerError, "Failed to parse friends")
			return
		}
		if avatar.Valid {
			friend.FriendAvatar = &avatar.String
		}
		friend.Status = "accepted"
		friends = append(friends, friend)
	}

	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

func GetUserProfile(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	targetID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid user id")
		return
	}

	viewerID := currentUserID(c)

	var user models.User
	err = database.DB.QueryRow(`
		SELECT id,
		       COALESCE(first_name, '') AS first_name,
		       COALESCE(last_name, '') AS last_name,
		       full_name,
		       email,
		       COALESCE(company_name, '') AS company_name,
		       COALESCE(phone, '') AS phone,
		       COALESCE(position, '') AS position,
		       COALESCE(avatar_url, '') AS avatar_url,
		       is_private_profile,
		       created_at
		FROM users
		WHERE id = $1
	`, targetID).Scan(&user.ID, &user.FirstName, &user.LastName, &user.FullName, &user.Email, &user.CompanyName, &user.Phone, &user.Position, &user.AvatarURL, &user.IsPrivateProfile, &user.CreatedAt)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		log.Printf("profile query failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to load profile")
		return
	}

	isSelf := viewerID == targetID
	if user.IsPrivateProfile && !isSelf {
		c.JSON(http.StatusForbidden, gin.H{"error": "Профиль закрыт"})
		return
	}

	if !isSelf {
		user.Email = ""
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "is_self": isSelf})
}

func UpdateProfileSettings(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	var req struct {
		IsPrivateProfile bool `json:"is_private_profile"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid settings payload")
		return
	}

	if _, err := database.DB.Exec(`UPDATE users SET is_private_profile = $1, updated_at = NOW() WHERE id = $2`, req.IsPrivateProfile, userID); err != nil {
		log.Printf("profile settings update failed: %v", err)
		jsonError(c, http.StatusInternalServerError, "Failed to save profile settings")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated", "is_private_profile": req.IsPrivateProfile})
}
