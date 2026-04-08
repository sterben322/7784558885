package handlers

import (
	"database/sql"
	"net/http"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetChats(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT c.id, c.name, c.type,
               (SELECT content FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1) AS last_message,
               (SELECT created_at FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1) AS last_time,
               cp.unread_count
        FROM chats c
        JOIN chat_participants cp ON c.id = cp.chat_id
        WHERE cp.user_id = $1
        ORDER BY last_time DESC NULLS LAST, c.created_at DESC
    `, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	chats := make([]models.Chat, 0)
	for rows.Next() {
		var chat models.Chat
		var lastMessage sql.NullString
		var lastTime sql.NullTime
		if err := rows.Scan(&chat.ID, &chat.Name, &chat.Type, &lastMessage, &lastTime, &chat.UnreadCount); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if lastMessage.Valid {
			chat.LastMessage = &lastMessage.String
		}
		if lastTime.Valid {
			chat.LastTime = &lastTime.Time
		}
		chats = append(chats, chat)
	}
	c.JSON(http.StatusOK, gin.H{"chats": chats})
}

func GetMessages(c *gin.Context) {
	chatID := c.Param("id")
	userID := currentUserID(c)
	if err := requireChatParticipant(chatID, userID); err != nil {
		if err.Error() == "forbidden" {
			jsonError(c, http.StatusForbidden, "Access denied")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, err := database.DB.Query(`
        SELECT m.id, m.chat_id, m.content, m.sender_id, u.full_name, m.read, m.created_at
        FROM messages m
        JOIN users u ON m.sender_id = u.id
        WHERE m.chat_id = $1
        ORDER BY m.created_at ASC
    `, chatID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	messages := make([]models.Message, 0)
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.Content, &msg.SenderID, &msg.SenderName, &msg.Read, &msg.CreatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		messages = append(messages, msg)
	}

	_, _ = database.DB.Exec(`UPDATE messages SET read = true WHERE chat_id = $1 AND sender_id != $2`, chatID, userID)
	_, _ = database.DB.Exec(`UPDATE chat_participants SET unread_count = 0 WHERE chat_id = $1 AND user_id = $2`, chatID, userID)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func SendMessage(c *gin.Context) {
	chatID := c.Param("id")
	senderID := currentUserID(c)
	if err := requireChatParticipant(chatID, senderID); err != nil {
		if err.Error() == "forbidden" {
			jsonError(c, http.StatusForbidden, "Access denied")
			return
		}
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	messageID := uuid.New()
	if _, err := database.DB.Exec(`INSERT INTO messages (id, chat_id, sender_id, content) VALUES ($1, $2, $3, $4)`, messageID, chatID, senderID, req.Content); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to send message")
		return
	}
	_, _ = database.DB.Exec(`UPDATE chat_participants SET unread_count = unread_count + 1 WHERE chat_id = $1 AND user_id != $2`, chatID, senderID)
	c.JSON(http.StatusCreated, gin.H{"message": "Message sent"})
}
