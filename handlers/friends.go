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

type friendAPIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func friendError(c *gin.Context, status int, message string) {
	c.JSON(status, friendAPIResponse{Success: false, Message: message})
}

func friendSuccess(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, friendAPIResponse{Success: true, Message: message, Data: data})
}

func friendDBError(c *gin.Context, status int, publicMessage string, err error) {
	log.Printf("friends db error: %v", err)
	friendError(c, status, publicMessage)
}

func parseFriendID(c *gin.Context) (uuid.UUID, bool) {
	friendID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		friendError(c, http.StatusBadRequest, "Invalid user id")
		return uuid.Nil, false
	}
	return friendID, true
}

func resolveFriendIDByName(currentUserID uuid.UUID, rawName string) (uuid.UUID, string, error) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return uuid.Nil, "", nil
	}

	exactRows, err := database.DB.Query(`
		SELECT id, COALESCE(NULLIF(full_name, ''), NULLIF(name, ''), email) AS display_name
		FROM users
		WHERE id <> $1::uuid
		  AND (
			LOWER(COALESCE(full_name, '')) = LOWER($2)
			OR LOWER(COALESCE(name, '')) = LOWER($2)
			OR LOWER(email) = LOWER($2)
		  )
		ORDER BY full_name ASC
		LIMIT 2
	`, currentUserID, name)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer exactRows.Close()

	var exactMatches []struct {
		ID   uuid.UUID
		Name string
	}
	for exactRows.Next() {
		var match struct {
			ID   uuid.UUID
			Name string
		}
		if scanErr := exactRows.Scan(&match.ID, &match.Name); scanErr != nil {
			return uuid.Nil, "", scanErr
		}
		exactMatches = append(exactMatches, match)
	}
	if err = exactRows.Err(); err != nil {
		return uuid.Nil, "", err
	}
	if len(exactMatches) == 1 {
		return exactMatches[0].ID, exactMatches[0].Name, nil
	}
	if len(exactMatches) > 1 {
		return uuid.Nil, "", sql.ErrNoRows
	}

	likeRows, err := database.DB.Query(`
		SELECT id, COALESCE(NULLIF(full_name, ''), NULLIF(name, ''), email) AS display_name
		FROM users
		WHERE id <> $1::uuid
		  AND (
			COALESCE(full_name, '') ILIKE '%' || $2 || '%'
			OR COALESCE(name, '') ILIKE '%' || $2 || '%'
			OR email ILIKE '%' || $2 || '%'
		  )
		ORDER BY full_name ASC
		LIMIT 2
	`, currentUserID, name)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer likeRows.Close()

	var likeMatches []struct {
		ID   uuid.UUID
		Name string
	}
	for likeRows.Next() {
		var match struct {
			ID   uuid.UUID
			Name string
		}
		if scanErr := likeRows.Scan(&match.ID, &match.Name); scanErr != nil {
			return uuid.Nil, "", scanErr
		}
		likeMatches = append(likeMatches, match)
	}
	if err = likeRows.Err(); err != nil {
		return uuid.Nil, "", err
	}
	if len(likeMatches) == 1 {
		return likeMatches[0].ID, likeMatches[0].Name, nil
	}
	if len(likeMatches) > 1 {
		return uuid.Nil, "", sql.ErrNoRows
	}

	return uuid.Nil, "", nil
}

func userExists(userID uuid.UUID) (bool, error) {
	var exists bool
	err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	return exists, err
}

func relationshipStatus(currentUserID, targetUserID uuid.UUID) (string, error) {
	var status string
	var requesterID sql.NullString
	err := database.DB.QueryRow(`
		SELECT status, requester_id
		FROM user_friends
		WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
		  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
	`, currentUserID, targetUserID).Scan(&status, &requesterID)
	if err == sql.ErrNoRows {
		return "none", nil
	}
	if err != nil {
		return "", err
	}

	if status == "pending" {
		if requesterID.Valid && requesterID.String == currentUserID.String() {
			return "outgoing_pending", nil
		}
		return "incoming_pending", nil
	}
	return status, nil
}

func SendFriendRequest(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	requesterID := currentUserID(c)
	targetID, ok := parseFriendID(c)
	if !ok {
		return
	}
	sendFriendRequestByID(c, requesterID, targetID)
}

func sendFriendRequestByID(c *gin.Context, requesterID, targetID uuid.UUID) {
	if requesterID == targetID {
		friendError(c, http.StatusBadRequest, "Нельзя отправить заявку самому себе")
		return
	}

	exists, err := userExists(targetID)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Database error", err)
		return
	}
	if !exists {
		friendError(c, http.StatusNotFound, "Пользователь не найден")
		return
	}

	var status string
	var existingRequester sql.NullString
	err = database.DB.QueryRow(`
		SELECT status, requester_id
		FROM user_friends
		WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
		  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
	`, requesterID, targetID).Scan(&status, &existingRequester)

	switch {
	case err == sql.ErrNoRows:
		_, err = database.DB.Exec(`
			INSERT INTO user_friends (user_id, friend_id, requester_id, status)
			VALUES ($1::uuid, $2::uuid, $1::uuid, 'pending')
		`, requesterID, targetID)
		if err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to send friend request", err)
			return
		}
		friendSuccess(c, http.StatusCreated, "Заявка в друзья отправлена", gin.H{"status": "pending"})
		return
	case err != nil:
		friendDBError(c, http.StatusInternalServerError, "Database error", err)
		return
	}

	switch status {
	case "accepted":
		friendError(c, http.StatusBadRequest, "Уже в друзьях")
	case "pending":
		if existingRequester.Valid && existingRequester.String == requesterID.String() {
			friendError(c, http.StatusConflict, "Заявка уже отправлена")
			return
		}
		_, err = database.DB.Exec(`
			UPDATE user_friends
			SET status = 'accepted', requester_id = $1, updated_at = NOW()
			WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
			  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
		`, requesterID, targetID)
		if err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to accept reciprocal request", err)
			return
		}
		friendSuccess(c, http.StatusOK, "Встречная заявка найдена, пользователи стали друзьями", gin.H{"status": "accepted"})
	case "rejected", "cancelled":
		_, err = database.DB.Exec(`
			UPDATE user_friends
			SET user_id = $1, friend_id = $2, requester_id = $1, status = 'pending', updated_at = NOW()
			WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
			  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
		`, requesterID, targetID)
		if err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to send friend request", err)
			return
		}
		friendSuccess(c, http.StatusOK, "Заявка в друзья отправлена", gin.H{"status": "pending"})
	default:
		friendError(c, http.StatusBadRequest, "Недопустимый статус связи")
	}
}

func SendFriendRequestByName(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		friendError(c, http.StatusBadRequest, "Укажите имя пользователя")
		return
	}

	currentUser := currentUserID(c)
	targetID, _, err := resolveFriendIDByName(currentUser, req.Name)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Database error", err)
		return
	}
	if targetID == uuid.Nil {
		friendError(c, http.StatusNotFound, "Пользователь не найден или найдено несколько совпадений, уточните имя")
		return
	}

	sendFriendRequestByID(c, currentUser, targetID)
}

func AcceptFriendRequest(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	currentUser := currentUserID(c)
	requesterID, ok := parseFriendID(c)
	if !ok {
		return
	}

	res, err := database.DB.Exec(`
		UPDATE user_friends
		SET status = 'accepted', updated_at = NOW()
		WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
		  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
		  AND status = 'pending'
			  AND requester_id = $2::uuid
	`, currentUser, requesterID)
	rows, err := rowsAffectedOrError(res, err)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to accept friend request", err)
		return
	}
	if rows == 0 {
		friendError(c, http.StatusNotFound, "Входящая заявка не найдена")
		return
	}

	friendSuccess(c, http.StatusOK, "Заявка принята", gin.H{"status": "accepted"})
}

func RejectFriendRequest(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	currentUser := currentUserID(c)
	requesterID, ok := parseFriendID(c)
	if !ok {
		return
	}

	res, err := database.DB.Exec(`
		UPDATE user_friends
		SET status = 'rejected', updated_at = NOW()
		WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
		  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
		  AND status = 'pending'
			  AND requester_id = $2::uuid
	`, currentUser, requesterID)
	rows, err := rowsAffectedOrError(res, err)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to reject friend request", err)
		return
	}
	if rows == 0 {
		friendError(c, http.StatusNotFound, "Входящая заявка не найдена")
		return
	}

	friendSuccess(c, http.StatusOK, "Заявка отклонена", gin.H{"status": "rejected"})
}

func CancelFriendRequest(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	currentUser := currentUserID(c)
	targetID, ok := parseFriendID(c)
	if !ok {
		return
	}

	res, err := database.DB.Exec(`
		UPDATE user_friends
		SET status = 'cancelled', updated_at = NOW()
		WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
		  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
		  AND status = 'pending'
			  AND requester_id = $1::uuid
	`, currentUser, targetID)
	rows, err := rowsAffectedOrError(res, err)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to cancel friend request", err)
		return
	}
	if rows == 0 {
		friendError(c, http.StatusNotFound, "Исходящая заявка не найдена")
		return
	}

	friendSuccess(c, http.StatusOK, "Заявка отменена", gin.H{"status": "cancelled"})
}

func RemoveFriend(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	currentUser := currentUserID(c)
	targetID, ok := parseFriendID(c)
	if !ok {
		return
	}

	res, err := database.DB.Exec(`
		DELETE FROM user_friends
		WHERE LEAST(user_id, friend_id) = LEAST($1, $2)
		  AND GREATEST(user_id, friend_id) = GREATEST($1, $2)
		  AND status = 'accepted'
	`, currentUser, targetID)
	rows, err := rowsAffectedOrError(res, err)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to remove friend", err)
		return
	}
	if rows == 0 {
		friendError(c, http.StatusNotFound, "Пользователь не находится в списке друзей")
		return
	}

	friendSuccess(c, http.StatusOK, "Пользователь удалён из друзей", nil)
}

func GetFriends(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT u.id, u.full_name, u.email, u.avatar_url, uf.updated_at
		FROM user_friends uf
		JOIN users u ON u.id = CASE WHEN uf.user_id = $1 THEN uf.friend_id ELSE uf.user_id END
		WHERE $1 IN (uf.user_id, uf.friend_id)
		  AND uf.status = 'accepted'
		ORDER BY uf.updated_at DESC
	`, userID)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to load friends", err)
		return
	}
	defer rows.Close()

	friends := make([]models.Friend, 0)
	for rows.Next() {
		var friend models.Friend
		var avatar sql.NullString
		if err := rows.Scan(&friend.FriendID, &friend.FriendName, &friend.FriendEmail, &avatar, &friend.CreatedAt); err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to read friends", err)
			return
		}
		if avatar.Valid {
			friend.FriendAvatar = &avatar.String
		}
		friend.Status = "accepted"
		friends = append(friends, friend)
	}

	friendSuccess(c, http.StatusOK, "Список друзей загружен", gin.H{"friends": friends})
}

func GetIncomingFriendRequests(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT u.id, u.full_name, u.email, u.avatar_url, uf.created_at
		FROM user_friends uf
		JOIN users u ON u.id = uf.requester_id
		WHERE uf.status = 'pending'
		  AND uf.requester_id <> $1
		  AND $1 IN (uf.user_id, uf.friend_id)
		ORDER BY uf.created_at DESC
	`, userID)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to load incoming requests", err)
		return
	}
	defer rows.Close()

	requests := make([]models.Friend, 0)
	for rows.Next() {
		var req models.Friend
		var avatar sql.NullString
		if err := rows.Scan(&req.UserID, &req.FriendName, &req.FriendEmail, &avatar, &req.CreatedAt); err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to read incoming requests", err)
			return
		}
		if avatar.Valid {
			req.FriendAvatar = &avatar.String
		}
		req.Status = "pending"
		requests = append(requests, req)
	}
	friendSuccess(c, http.StatusOK, "Входящие заявки загружены", gin.H{"requests": requests})
}

func GetOutgoingFriendRequests(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT u.id, u.full_name, u.email, u.avatar_url, uf.created_at
		FROM user_friends uf
		JOIN users u ON u.id = CASE WHEN uf.user_id = $1 THEN uf.friend_id ELSE uf.user_id END
		WHERE uf.status = 'pending'
		  AND uf.requester_id = $1
		  AND $1 IN (uf.user_id, uf.friend_id)
		ORDER BY uf.created_at DESC
	`, userID)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to load outgoing requests", err)
		return
	}
	defer rows.Close()

	requests := make([]models.Friend, 0)
	for rows.Next() {
		var req models.Friend
		var avatar sql.NullString
		if err := rows.Scan(&req.FriendID, &req.FriendName, &req.FriendEmail, &avatar, &req.CreatedAt); err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to read outgoing requests", err)
			return
		}
		if avatar.Valid {
			req.FriendAvatar = &avatar.String
		}
		req.Status = "pending"
		requests = append(requests, req)
	}
	friendSuccess(c, http.StatusOK, "Исходящие заявки загружены", gin.H{"requests": requests})
}

func GetAddableUsers(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	search := strings.TrimSpace(c.Query("q"))

	rows, err := database.DB.Query(`
		SELECT u.id, u.full_name, u.email, u.avatar_url, u.created_at
		FROM users u
		WHERE u.id <> $1
		  AND ($2 = '' OR u.full_name ILIKE '%' || $2 || '%' OR u.email ILIKE '%' || $2 || '%')
		  AND NOT EXISTS (
			SELECT 1
			FROM user_friends uf
			WHERE LEAST(uf.user_id, uf.friend_id) = LEAST($1, u.id)
			  AND GREATEST(uf.user_id, uf.friend_id) = GREATEST($1, u.id)
			  AND uf.status IN ('accepted', 'pending')
		)
		ORDER BY u.full_name ASC
		LIMIT 50
	`, userID, search)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to load users", err)
		return
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.FullName, &u.Email, &u.AvatarURL, &u.CreatedAt); err != nil {
			friendDBError(c, http.StatusInternalServerError, "Failed to read users", err)
			return
		}
		users = append(users, u)
	}

	friendSuccess(c, http.StatusOK, "Пользователи для добавления загружены", gin.H{"users": users})
}

func GetFriendStatus(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	currentUser := currentUserID(c)
	targetID, ok := parseFriendID(c)
	if !ok {
		return
	}

	if currentUser == targetID {
		friendSuccess(c, http.StatusOK, "Статус связи получен", gin.H{"status": "self"})
		return
	}

	exists, err := userExists(targetID)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Database error", err)
		return
	}
	if !exists {
		friendError(c, http.StatusNotFound, "Пользователь не найден")
		return
	}

	status, err := relationshipStatus(currentUser, targetID)
	if err != nil {
		friendDBError(c, http.StatusInternalServerError, "Failed to load friendship status", err)
		return
	}
	friendSuccess(c, http.StatusOK, "Статус связи получен", gin.H{"status": status})
}
