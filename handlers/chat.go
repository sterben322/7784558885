package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errForbidden = errors.New("forbidden")

const maxChatAttachmentSize int64 = 50 << 20

var typingState = struct {
	mu    sync.Mutex
	chats map[uuid.UUID]map[uuid.UUID]time.Time
}{
	chats: make(map[uuid.UUID]map[uuid.UUID]time.Time),
}

func parseUUIDParam(c *gin.Context, key string) (uuid.UUID, bool) {
	value := strings.TrimSpace(c.Param(key))
	id, err := uuid.Parse(value)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func orderedUUIDPair(first, second uuid.UUID) (uuid.UUID, uuid.UUID) {
	if strings.Compare(first.String(), second.String()) <= 0 {
		return first, second
	}
	return second, first
}

func getDirectConversation(userID, friendID uuid.UUID) (*models.Chat, error) {
	lowUserID, highUserID := orderedUUIDPair(userID, friendID)
	row := database.DB.QueryRow(`
		SELECT c.id,
			c.name,
			COALESCE(c.type, 'direct') AS type,
			c.last_message_at,
			(SELECT content FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC LIMIT 1) AS last_message
		FROM chats c
		WHERE c.type = 'direct'
		  AND c.direct_user_low = $1
		  AND c.direct_user_high = $2
	`, lowUserID, highUserID)

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
	lowUserID, highUserID := orderedUUIDPair(userID, friendID)
	_, err := database.DB.Exec(`
		INSERT INTO chats (id, type, direct_user_low, direct_user_high)
		VALUES ($1, 'direct', $2, $3)
		ON CONFLICT (direct_user_low, direct_user_high) WHERE type = 'direct' DO NOTHING
	`, chatID, lowUserID, highUserID)
	if err != nil {
		return nil, err
	}

	if err := ensureDirectConversationParticipants(userID, friendID); err != nil {
		return nil, err
	}

	return getDirectConversation(userID, friendID)
}

func ensureDirectConversationParticipants(userID, friendID uuid.UUID) error {
	lowUserID, highUserID := orderedUUIDPair(userID, friendID)
	_, err := database.DB.Exec(`
		INSERT INTO chat_participants (chat_id, user_id)
		SELECT c.id, p.user_id
		FROM chats c
		CROSS JOIN (VALUES ($1::uuid), ($2::uuid)) AS p(user_id)
		WHERE c.type = 'direct'
		  AND c.direct_user_low = $3
		  AND c.direct_user_high = $4
		ON CONFLICT (chat_id, user_id) DO NOTHING
	`, userID, friendID, lowUserID, highUserID)
	return err
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
	} else {
		if err := ensureDirectConversationParticipants(userID, friendID); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to restore conversation participants")
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
		SELECT m.id, m.chat_id, m.content, m.sender_id, u.full_name, m.read, m.created_at, m.updated_at,
			m.attachment_url, m.attachment_name, m.attachment_size, m.attachment_type, m.image_url,
			m.reply_to_id, rm.sender_id, COALESCE(ru.full_name, ''), COALESCE(NULLIF(rm.content, ''), rm.attachment_name, 'Вложение')
		FROM messages m
		JOIN users u ON m.sender_id = u.id
		LEFT JOIN messages rm ON rm.id = m.reply_to_id
		LEFT JOIN users ru ON ru.id = rm.sender_id
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
		var replyToID sql.NullString
		var replySenderID sql.NullString
		var replySenderName sql.NullString
		var replyText sql.NullString
		if err := rows.Scan(
			&msg.ID,
			&msg.ChatID,
			&msg.Content,
			&msg.SenderID,
			&msg.SenderName,
			&msg.Read,
			&msg.CreatedAt,
			&msg.UpdatedAt,
			&msg.AttachmentURL,
			&msg.AttachmentName,
			&msg.AttachmentSize,
			&msg.AttachmentType,
			&msg.ImageURL,
			&replyToID,
			&replySenderID,
			&replySenderName,
			&replyText,
		); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse messages")
			return
		}
		if replyToID.Valid {
			replyID, parseErr := uuid.Parse(replyToID.String)
			if parseErr == nil {
				msg.ReplyToID = &replyID
			}
		}
		if msg.ReplyToID != nil && replySenderID.Valid {
			replySenderUUID, parseErr := uuid.Parse(replySenderID.String)
			if parseErr == nil {
				msg.ReplyTo = &models.MessageReplyPreview{
					ID:         *msg.ReplyToID,
					SenderID:   replySenderUUID,
					SenderName: replySenderName.String,
					Text:       replyText.String,
				}
			}
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

	content := ""
	var attachmentURL *string
	var attachmentName *string
	var attachmentSize *int64
	var attachmentType *string
	var imageURL *string
	var replyToID *uuid.UUID
	hasMultipart := strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data")

	if hasMultipart {
		if err := c.Request.ParseMultipartForm(maxChatAttachmentSize + (1 << 20)); err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid form data")
			return
		}
		content = strings.TrimSpace(c.PostForm("content"))
		if content == "" {
			content = strings.TrimSpace(c.PostForm("text"))
		}
		replyToRaw := strings.TrimSpace(c.PostForm("reply_to_id"))
		if replyToRaw != "" {
			parsedReplyID, parseErr := uuid.Parse(replyToRaw)
			if parseErr != nil {
				jsonError(c, http.StatusBadRequest, "Invalid reply_to_id")
				return
			}
			replyToID = &parsedReplyID
		}
		fileHeader, err := c.FormFile("attachment")
		if err != nil && !errors.Is(err, http.ErrMissingFile) {
			jsonError(c, http.StatusBadRequest, "Invalid attachment")
			return
		}
		if fileHeader != nil {
			url, name, size, fileType, isImage, saveErr := saveChatAttachment(fileHeader)
			if saveErr != nil {
				switch {
				case errors.Is(saveErr, errAttachmentTooLarge):
					jsonError(c, http.StatusBadRequest, "File size exceeds 50MB limit")
				default:
					jsonError(c, http.StatusInternalServerError, "Failed to save attachment")
				}
				return
			}
			attachmentURL = &url
			attachmentName = &name
			attachmentSize = &size
			attachmentType = &fileType
			if isImage {
				imageURL = &url
			}
		}
	} else {
		var req struct {
			Content   string  `json:"content"`
			Text      string  `json:"text"`
			ReplyToID *string `json:"reply_to_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonError(c, http.StatusBadRequest, "Content is required")
			return
		}
		content = strings.TrimSpace(req.Content)
		if content == "" {
			content = strings.TrimSpace(req.Text)
		}
		if req.ReplyToID != nil {
			parsedReplyID, parseErr := uuid.Parse(strings.TrimSpace(*req.ReplyToID))
			if parseErr != nil {
				jsonError(c, http.StatusBadRequest, "Invalid reply_to_id")
				return
			}
			replyToID = &parsedReplyID
		}
	}

	if content == "" && attachmentURL == nil {
		jsonError(c, http.StatusBadRequest, "Content or attachment is required")
		return
	}
	if replyToID != nil {
		var exists bool
		if err := database.DB.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM messages
				WHERE id = $1 AND chat_id = $2
			)
		`, *replyToID, conversationID).Scan(&exists); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to validate reply message")
			return
		}
		if !exists {
			jsonError(c, http.StatusBadRequest, "Reply message not found")
			return
		}
	}

	messageID := uuid.New()
	if _, err := database.DB.Exec(`
		INSERT INTO messages (
			id, chat_id, sender_id, content, attachment_url, attachment_name, attachment_size, attachment_type, image_url, reply_to_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, messageID, conversationID, senderID, content, attachmentURL, attachmentName, attachmentSize, attachmentType, imageURL, replyToID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to send message")
		return
	}
	_, _ = database.DB.Exec(`UPDATE chats SET updated_at = NOW(), last_message_at = NOW() WHERE id = $1`, conversationID)
	_, _ = database.DB.Exec(`UPDATE chat_participants SET unread_count = unread_count + 1 WHERE chat_id = $1 AND user_id != $2`, conversationID, senderID)
	_, _ = database.DB.Exec(`UPDATE chat_participants SET last_read_at = NOW() WHERE chat_id = $1 AND user_id = $2`, conversationID, senderID)

	c.JSON(http.StatusCreated, gin.H{
		"message":         "Message sent",
		"message_id":      messageID,
		"attachment_url":  attachmentURL,
		"attachment_name": attachmentName,
		"attachment_size": attachmentSize,
		"attachment_type": attachmentType,
		"image_url":       imageURL,
		"reply_to_id":     replyToID,
	})
}

func UpdateConversationMessage(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	conversationID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	messageID, ok := parseUUIDParam(c, "messageId")
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

	var senderID uuid.UUID
	var existingAttachmentURL sql.NullString
	err := database.DB.QueryRow(`
		SELECT sender_id, attachment_url
		FROM messages
		WHERE id = $1 AND chat_id = $2
	`, messageID, conversationID).Scan(&senderID, &existingAttachmentURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(c, http.StatusNotFound, "Message not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to load message")
		return
	}
	if senderID != userID {
		jsonError(c, http.StatusForbidden, "Only sender can edit message")
		return
	}

	var (
		content          *string
		attachmentURL    *string
		attachmentName   *string
		attachmentSize   *int64
		attachmentType   *string
		imageURL         *string
		removeAttachment bool
	)

	hasMultipart := strings.HasPrefix(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data")
	if hasMultipart {
		if err := c.Request.ParseMultipartForm(maxChatAttachmentSize + (1 << 20)); err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid form data")
			return
		}
		trimmed := strings.TrimSpace(c.PostForm("content"))
		content = &trimmed
		removeAttachment = strings.EqualFold(strings.TrimSpace(c.PostForm("remove_attachment")), "true")

		fileHeader, err := c.FormFile("attachment")
		if err != nil && !errors.Is(err, http.ErrMissingFile) {
			jsonError(c, http.StatusBadRequest, "Invalid attachment")
			return
		}
		if fileHeader != nil {
			url, name, size, fileType, isImage, saveErr := saveChatAttachment(fileHeader)
			if saveErr != nil {
				switch {
				case errors.Is(saveErr, errAttachmentTooLarge):
					jsonError(c, http.StatusBadRequest, "File size exceeds 50MB limit")
				default:
					jsonError(c, http.StatusInternalServerError, "Failed to save attachment")
				}
				return
			}
			attachmentURL = &url
			attachmentName = &name
			attachmentSize = &size
			attachmentType = &fileType
			if isImage {
				imageURL = &url
			}
			removeAttachment = false
		}
	} else {
		var req struct {
			Content          *string `json:"content"`
			Text             *string `json:"text"`
			RemoveAttachment bool    `json:"remove_attachment"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			jsonError(c, http.StatusBadRequest, "Invalid payload")
			return
		}
		if req.Content != nil {
			trimmed := strings.TrimSpace(*req.Content)
			content = &trimmed
		} else if req.Text != nil {
			trimmed := strings.TrimSpace(*req.Text)
			content = &trimmed
		}
		removeAttachment = req.RemoveAttachment
	}

	if content == nil && attachmentURL == nil && !removeAttachment {
		jsonError(c, http.StatusBadRequest, "No changes provided")
		return
	}

	query := `
		UPDATE messages
		SET content = COALESCE($3, content),
			attachment_url = CASE WHEN $4 THEN NULL ELSE COALESCE($5, attachment_url) END,
			attachment_name = CASE WHEN $4 THEN NULL ELSE COALESCE($6, attachment_name) END,
			attachment_size = CASE WHEN $4 THEN NULL ELSE COALESCE($7, attachment_size) END,
			attachment_type = CASE WHEN $4 THEN NULL ELSE COALESCE($8, attachment_type) END,
			image_url = CASE
				WHEN $4 THEN NULL
				WHEN $9 IS NOT NULL THEN $9
				WHEN $5 IS NOT NULL THEN NULL
				ELSE image_url
			END,
			updated_at = NOW()
		WHERE id = $1 AND chat_id = $2
		RETURNING content, attachment_url, attachment_name, attachment_size, attachment_type, image_url, updated_at
	`

	var (
		resultContent        string
		resultAttachmentURL  sql.NullString
		resultAttachmentName sql.NullString
		resultAttachmentSize sql.NullInt64
		resultAttachmentType sql.NullString
		resultImageURL       sql.NullString
		resultUpdatedAt      time.Time
	)

	if err := database.DB.QueryRow(
		query,
		messageID,
		conversationID,
		content,
		removeAttachment,
		attachmentURL,
		attachmentName,
		attachmentSize,
		attachmentType,
		imageURL,
	).Scan(
		&resultContent,
		&resultAttachmentURL,
		&resultAttachmentName,
		&resultAttachmentSize,
		&resultAttachmentType,
		&resultImageURL,
		&resultUpdatedAt,
	); err != nil {
		if attachmentURL != nil {
			deleteChatAttachmentByURL(*attachmentURL)
		}
		jsonError(c, http.StatusInternalServerError, "Failed to update message")
		return
	}

	if strings.TrimSpace(resultContent) == "" && !resultAttachmentURL.Valid {
		if attachmentURL != nil {
			deleteChatAttachmentByURL(*attachmentURL)
		}
		jsonError(c, http.StatusBadRequest, "Content or attachment is required")
		return
	}

	if existingAttachmentURL.Valid && (removeAttachment || attachmentURL != nil) {
		deleteChatAttachmentByURL(existingAttachmentURL.String)
	}

	_, _ = database.DB.Exec(`UPDATE chats SET updated_at = NOW() WHERE id = $1`, conversationID)

	response := gin.H{
		"message":    "Message updated",
		"id":         messageID,
		"content":    resultContent,
		"updated_at": resultUpdatedAt,
	}
	if resultAttachmentURL.Valid {
		response["attachment_url"] = resultAttachmentURL.String
	}
	if resultAttachmentName.Valid {
		response["attachment_name"] = resultAttachmentName.String
	}
	if resultAttachmentSize.Valid {
		response["attachment_size"] = resultAttachmentSize.Int64
	}
	if resultAttachmentType.Valid {
		response["attachment_type"] = resultAttachmentType.String
	}
	if resultImageURL.Valid {
		response["image_url"] = resultImageURL.String
	}

	c.JSON(http.StatusOK, response)
}

func DeleteConversationMessage(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	conversationID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	messageID, ok := parseUUIDParam(c, "messageId")
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

	var (
		senderID      uuid.UUID
		attachmentURL sql.NullString
	)
	err := database.DB.QueryRow(`
		SELECT sender_id, attachment_url
		FROM messages
		WHERE id = $1 AND chat_id = $2
	`, messageID, conversationID).Scan(&senderID, &attachmentURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(c, http.StatusNotFound, "Message not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to load message")
		return
	}
	if senderID != userID {
		jsonError(c, http.StatusForbidden, "Only sender can delete message")
		return
	}

	if _, err := database.DB.Exec(`DELETE FROM messages WHERE id = $1 AND chat_id = $2`, messageID, conversationID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to delete message")
		return
	}

	if attachmentURL.Valid {
		deleteChatAttachmentByURL(attachmentURL.String)
	}

	_, _ = database.DB.Exec(`
		UPDATE chats
		SET updated_at = NOW(),
			last_message_at = (SELECT MAX(created_at) FROM messages WHERE chat_id = $1)
		WHERE id = $1
	`, conversationID)

	c.JSON(http.StatusOK, gin.H{"message": "Message deleted", "id": messageID})
}

var errAttachmentTooLarge = errors.New("attachment too large")

func saveChatAttachment(fileHeader *multipart.FileHeader) (url, fileName string, fileSize int64, contentType string, isImage bool, err error) {
	if fileHeader == nil {
		return "", "", 0, "", false, errors.New("missing file")
	}
	if fileHeader.Size > maxChatAttachmentSize {
		return "", "", 0, "", false, errAttachmentTooLarge
	}
	src, err := fileHeader.Open()
	if err != nil {
		return "", "", 0, "", false, err
	}
	defer src.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(src, head)
	contentType = http.DetectContentType(head[:n])
	isImage = strings.HasPrefix(contentType, "image/")
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", "", 0, "", false, err
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	storedName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	relPath := filepath.Join("uploads", "chat", storedName)
	fullPath := filepath.Join("web", relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", "", 0, "", false, err
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", "", 0, "", false, err
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		_ = os.Remove(fullPath)
		return "", "", 0, "", false, err
	}
	if written > maxChatAttachmentSize {
		_ = os.Remove(fullPath)
		return "", "", 0, "", false, errAttachmentTooLarge
	}
	fileSize = written
	fileName = strings.TrimSpace(fileHeader.Filename)
	if fileName == "" {
		fileName = "attachment"
	}

	url = "/" + filepath.ToSlash(relPath)
	return url, fileName, fileSize, contentType, isImage, nil
}

func deleteChatAttachmentByURL(url string) {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return
	}
	rel := strings.TrimPrefix(trimmed, "/")
	cleanRel := filepath.Clean(rel)
	baseDir := filepath.Clean(filepath.Join("web", "uploads", "chat"))
	fullPath := filepath.Clean(filepath.Join("web", cleanRel))
	if !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) && fullPath != baseDir {
		return
	}
	_ = os.Remove(fullPath)
}

func SetConversationTyping(c *gin.Context) {
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

	var req struct {
		Typing bool `json:"typing"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid payload")
		return
	}

	typingState.mu.Lock()
	defer typingState.mu.Unlock()
	chatState, ok := typingState.chats[conversationID]
	if !ok {
		chatState = make(map[uuid.UUID]time.Time)
		typingState.chats[conversationID] = chatState
	}
	if req.Typing {
		chatState[userID] = time.Now().UTC().Add(6 * time.Second)
	} else {
		delete(chatState, userID)
	}
	if len(chatState) == 0 {
		delete(typingState.chats, conversationID)
	}

	c.JSON(http.StatusOK, gin.H{"typing": req.Typing})
}

func GetConversationTyping(c *gin.Context) {
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

	now := time.Now().UTC()
	isTyping := false

	typingState.mu.Lock()
	defer typingState.mu.Unlock()
	chatState, ok := typingState.chats[conversationID]
	if ok {
		for participantID, expiresAt := range chatState {
			if expiresAt.Before(now) {
				delete(chatState, participantID)
				continue
			}
			if participantID != userID {
				isTyping = true
			}
		}
		if len(chatState) == 0 {
			delete(typingState.chats, conversationID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"typing": isTyping})
}

// Backward-compatible endpoints.
func GetChats(c *gin.Context)    { GetChatConversations(c) }
func GetMessages(c *gin.Context) { GetConversationMessages(c) }
func SendMessage(c *gin.Context) { SendConversationMessage(c) }
func UpdateMessage(c *gin.Context) {
	UpdateConversationMessage(c)
}
func DeleteMessage(c *gin.Context) {
	DeleteConversationMessage(c)
}
