package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errForbidden = errors.New("forbidden")

func parseUUIDParam(c *gin.Context, key string) (uuid.UUID, bool) {
	value := strings.TrimSpace(c.Param(key))
	id, err := uuid.Parse(value)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func getDirectConversation(userID, friendID uuid.UUID) (*models.Chat, error) {
	row := database.DB.QueryRow(`
		SELECT c.id,
			c.name,
			COALESCE(c.type, 'direct') AS type,
			c.last_message_at,
			(SELECT content FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message
		FROM chats c
		WHERE c.type = 'direct'
		  AND c.direct_user_low = LEAST($1, $2)
		  AND c.direct_user_high = GREATEST($1, $2)
	`, userID, friendID)

	var chat models.Chat
	var name sql.NullString
	var lastMessage sql.NullString
	var lastTime sql.NullTime
	if err := row.Scan(&chat.ID, &name, &chat.Type, &lastTime, &lastMessage); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if name.Valid {
		chat.Name = &name.String
	}
	if lastMessage.Valid {
		chat.LastMessage = &lastMessage.String
	}
	if lastTime.Valid {
		chat.LastTime = &lastTime.Time
	}
	return &chat, nil
}

func createDirectConversation(userID, friendID uuid.UUID) (*models.Chat, error) {
	chatID := uuid.New()
	_, err := database.DB.Exec(`
		INSERT INTO chats (id, type, direct_user_low, direct_user_high)
		VALUES ($1, 'direct', LEAST($2, $3), GREATEST($2, $3))
		ON CONFLICT (direct_user_low, direct_user_high) WHERE type = 'direct' DO NOTHING
	`, chatID, userID, friendID)
	if err != nil {
		return nil, err
	}

	_, _ = database.DB.Exec(`
		INSERT INTO chat_participants (chat_id, user_id)
		SELECT c.id, p.user_id
		FROM chats c
		CROSS JOIN (VALUES ($1::uuid), ($2::uuid)) AS p(user_id)
		WHERE c.type = 'direct'
		  AND c.direct_user_low = LEAST($1, $2)
		  AND c.direct_user_high = GREATEST($1, $2)
		ON CONFLICT (chat_id, user_id) DO NOTHING
	`, userID, friendID)

	return getDirectConversation(userID, friendID)
}

func GetChatConversations(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT c.id,
			COALESCE(c.name, u.full_name, 'Диалог') AS chat_name,
			c.type,
			(SELECT content FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message,
			c.last_message_at,
			COALESCE(cp.unread_count, 0) AS unread_count,
			u.id,
			u.avatar_url
		FROM chats c
		JOIN chat_participants cp ON cp.chat_id = c.id AND cp.user_id = $1
		LEFT JOIN users u ON c.type = 'direct' AND u.id = CASE
			WHEN c.direct_user_low = $1 THEN c.direct_user_high
			WHEN c.direct_user_high = $1 THEN c.direct_user_low
			ELSE NULL
		END
		ORDER BY c.last_message_at DESC NULLS LAST, c.updated_at DESC, c.created_at DESC
	`, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load conversations")
		return
	}
	defer rows.Close()

	chats := make([]models.Chat, 0)
	for rows.Next() {
		var chat models.Chat
		var chatName string
		var lastMessage sql.NullString
		var lastTime sql.NullTime
		var peerID sql.NullString
		var peerAvatar sql.NullString
		if err := rows.Scan(&chat.ID, &chatName, &chat.Type, &lastMessage, &lastTime, &chat.UnreadCount, &peerID, &peerAvatar); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse conversations")
			return
		}
		chat.Name = &chatName
		if lastMessage.Valid {
			chat.LastMessage = &lastMessage.String
		}
		if lastTime.Valid {
			chat.LastTime = &lastTime.Time
		}
		if peerID.Valid {
			chat.PeerUserID = &peerID.String
		}
		if peerAvatar.Valid {
			chat.PeerAvatarURL = &peerAvatar.String
		}
		chats = append(chats, chat)
	}

	c.JSON(http.StatusOK, gin.H{"conversations": chats, "chats": chats})
}

func StartChatConversation(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	friendID, ok := parseUUIDParam(c, "friendId")
	if !ok {
		return
	}
	if userID == friendID {
		jsonError(c, http.StatusBadRequest, "Cannot start chat with yourself")
		return
	}

	exists, err := userExists(friendID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to validate user")
		return
	}
	if !exists {
		jsonError(c, http.StatusNotFound, "User not found")
		return
	}

	chat, err := getDirectConversation(userID, friendID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load conversation")
		return
	}
	if chat == nil {
		chat, err = createDirectConversation(userID, friendID)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to create conversation")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"conversation": chat})
}

func GetChatConversation(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	conversationID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	userID := currentUserID(c)
	if err := requireConversationParticipant(conversationID, userID); err != nil {
		if errors.Is(err, errForbidden) {
			jsonError(c, http.StatusForbidden, "Access denied")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to validate participant")
		return
	}

	var chat models.Chat
	var chatName string
	var lastMessage sql.NullString
	var lastTime sql.NullTime
	var unreadCount int
	err := database.DB.QueryRow(`
		SELECT c.id,
			COALESCE(c.name, u.full_name, 'Диалог') AS chat_name,
			c.type,
			(SELECT content FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message,
			c.last_message_at,
			COALESCE(cp.unread_count, 0) AS unread_count
		FROM chats c
		JOIN chat_participants cp ON cp.chat_id = c.id AND cp.user_id = $2
		LEFT JOIN users u ON c.type = 'direct' AND u.id = CASE
			WHEN c.direct_user_low = $2 THEN c.direct_user_high
			WHEN c.direct_user_high = $2 THEN c.direct_user_low
			ELSE NULL
		END
		WHERE c.id = $1
	`, conversationID, userID).Scan(&chat.ID, &chatName, &chat.Type, &lastMessage, &lastTime, &unreadCount)
	if err != nil {
		if err == sql.ErrNoRows {
			jsonError(c, http.StatusNotFound, "Conversation not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to load conversation")
		return
	}

	chat.Name = &chatName
	chat.UnreadCount = unreadCount
	if lastMessage.Valid {
		chat.LastMessage = &lastMessage.String
	}
	if lastTime.Valid {
		chat.LastTime = &lastTime.Time
	}

	c.JSON(http.StatusOK, gin.H{"conversation": chat})
}

func GetConversationMessages(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	conversationID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	userID := currentUserID(c)
	if err := requireConversationParticipant(conversationID, userID); err != nil {
		if errors.Is(err, errForbidden) {
			jsonError(c, http.StatusForbidden, "Access denied")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to validate participant")
		return
	}

	rows, err := database.DB.Query(`
		SELECT m.id, m.chat_id, m.content, m.sender_id, u.full_name, m.read, m.created_at
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		WHERE m.chat_id = $1
		ORDER BY m.created_at ASC
	`, conversationID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load messages")
		return
	}
	defer rows.Close()

	messages := make([]models.Message, 0)
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.Content, &msg.SenderID, &msg.SenderName, &msg.Read, &msg.CreatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse messages")
			return
		}
		messages = append(messages, msg)
	}

	_, _ = database.DB.Exec(`UPDATE messages SET read = true WHERE chat_id = $1 AND sender_id != $2`, conversationID, userID)
	_, _ = database.DB.Exec(`UPDATE chat_participants SET unread_count = 0, last_read_at = NOW() WHERE chat_id = $1 AND user_id = $2`, conversationID, userID)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

func SendConversationMessage(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	conversationID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	senderID := currentUserID(c)
	if err := requireConversationParticipant(conversationID, senderID); err != nil {
		if errors.Is(err, errForbidden) {
			jsonError(c, http.StatusForbidden, "Access denied")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to validate participant")
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Content is required")
		return
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		jsonError(c, http.StatusBadRequest, "Content is required")
		return
	}

	messageID := uuid.New()
	if _, err := database.DB.Exec(`
		INSERT INTO messages (id, chat_id, sender_id, content)
		VALUES ($1, $2, $3, $4)
	`, messageID, conversationID, senderID, req.Content); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to send message")
		return
	}
	_, _ = database.DB.Exec(`UPDATE chats SET updated_at = NOW(), last_message_at = NOW() WHERE id = $1`, conversationID)
	_, _ = database.DB.Exec(`UPDATE chat_participants SET unread_count = unread_count + 1 WHERE chat_id = $1 AND user_id != $2`, conversationID, senderID)
	_, _ = database.DB.Exec(`UPDATE chat_participants SET last_read_at = NOW() WHERE chat_id = $1 AND user_id = $2`, conversationID, senderID)

	c.JSON(http.StatusCreated, gin.H{"message": "Message sent", "message_id": messageID})
}

// Backward-compatible endpoints.
func GetChats(c *gin.Context)    { GetChatConversations(c) }
func GetMessages(c *gin.Context) { GetConversationMessages(c) }
func SendMessage(c *gin.Context) { SendConversationMessage(c) }
