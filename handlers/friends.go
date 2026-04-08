package handlers

import (
	"database/sql"
	"net/http"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
)

func SendFriendRequest(c *gin.Context) {
	userID := currentUserID(c)
	friendID := c.Param("id")

	if userID.String() == friendID {
		jsonError(c, http.StatusBadRequest, "Cannot add yourself as friend")
		return
	}

	var isFriend bool
	err := database.DB.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM user_friends
            WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1))
              AND status = 'accepted'
        )
    `, userID, friendID).Scan(&isFriend)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Database error")
		return
	}
	if isFriend {
		jsonError(c, http.StatusBadRequest, "Already friends")
		return
	}

	var pendingExists bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_friends WHERE user_id = $1 AND friend_id = $2 AND status = 'pending')`, userID, friendID).Scan(&pendingExists)
	if pendingExists {
		jsonError(c, http.StatusBadRequest, "Friend request already sent")
		return
	}

	var receivedRequest bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_friends WHERE user_id = $2 AND friend_id = $1 AND status = 'pending')`, userID, friendID).Scan(&receivedRequest)
	if receivedRequest {
		if _, err := database.DB.Exec(`UPDATE user_friends SET status = 'accepted', updated_at = NOW() WHERE user_id = $1 AND friend_id = $2`, friendID, userID); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to accept request")
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Friend request accepted automatically"})
		return
	}

	if _, err := database.DB.Exec(`INSERT INTO user_friends (user_id, friend_id, status) VALUES ($1, $2, 'pending')`, userID, friendID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to send friend request")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Friend request sent"})
}

func AcceptFriendRequest(c *gin.Context) {
	userID := currentUserID(c)
	friendID := c.Param("id")

	result, err := database.DB.Exec(`UPDATE user_friends SET status = 'accepted', updated_at = NOW() WHERE user_id = $1 AND friend_id = $2 AND status = 'pending'`, friendID, userID)
	rows, err := rowsAffectedOrError(result, err)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to accept request")
		return
	}
	if rows == 0 {
		jsonError(c, http.StatusNotFound, "Friend request not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Friend request accepted"})
}

func RejectFriendRequest(c *gin.Context) {
	userID := currentUserID(c)
	friendID := c.Param("id")
	if _, err := database.DB.Exec(`DELETE FROM user_friends WHERE user_id = $1 AND friend_id = $2 AND status = 'pending'`, friendID, userID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to reject request")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Friend request rejected"})
}

func RemoveFriend(c *gin.Context) {
	userID := currentUserID(c)
	friendID := c.Param("id")
	if _, err := database.DB.Exec(`DELETE FROM user_friends WHERE ((user_id = $1 AND friend_id = $2) OR (user_id = $2 AND friend_id = $1)) AND status = 'accepted'`, userID, friendID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to remove friend")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Friend removed"})
}

func GetFriends(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT u.id, u.full_name, u.email, u.avatar_url, uf.created_at
        FROM users u
        JOIN user_friends uf ON ((uf.user_id = u.id AND uf.friend_id = $1) OR (uf.friend_id = u.id AND uf.user_id = $1))
        WHERE uf.status = 'accepted' AND u.id != $1
        ORDER BY uf.created_at DESC
    `, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	friends := make([]models.Friend, 0)
	for rows.Next() {
		var friend models.Friend
		var avatar sql.NullString
		if err := rows.Scan(&friend.FriendID, &friend.FriendName, &friend.FriendEmail, &avatar, &friend.CreatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if avatar.Valid {
			friend.FriendAvatar = &avatar.String
		}
		friends = append(friends, friend)
	}
	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

func GetFriendRequests(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT u.id, u.full_name, u.email, u.avatar_url, uf.created_at
        FROM users u
        JOIN user_friends uf ON uf.user_id = u.id
        WHERE uf.friend_id = $1 AND uf.status = 'pending'
        ORDER BY uf.created_at DESC
    `, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	requests := make([]models.Friend, 0)
	for rows.Next() {
		var req models.Friend
		var avatar sql.NullString
		if err := rows.Scan(&req.UserID, &req.FriendName, &req.FriendEmail, &avatar, &req.CreatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if avatar.Valid {
			req.FriendAvatar = &avatar.String
		}
		requests = append(requests, req)
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}
