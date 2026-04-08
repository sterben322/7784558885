package handlers

import (
	"net/http"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetTopics(c *gin.Context) {
	rows, err := database.DB.Query(`
        SELECT t.id, t.title, t.content, t.category, t.author_id, u.full_name,
               t.replies_count, t.views_count, t.created_at, t.updated_at
        FROM forum_topics t
        JOIN users u ON t.author_id = u.id
        ORDER BY t.created_at DESC
    `)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	topics := make([]models.ForumTopic, 0)
	for rows.Next() {
		var topic models.ForumTopic
		if err := rows.Scan(&topic.ID, &topic.Title, &topic.Content, &topic.Category, &topic.AuthorID, &topic.AuthorName, &topic.RepliesCount, &topic.ViewsCount, &topic.CreatedAt, &topic.LastActivity); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		topics = append(topics, topic)
	}
	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

func GetTopic(c *gin.Context) {
	id := c.Param("id")
	var topic models.ForumTopic
	err := database.DB.QueryRow(`
        SELECT t.id, t.title, t.content, t.category, t.author_id, u.full_name,
               t.replies_count, t.views_count, t.created_at, t.updated_at
        FROM forum_topics t
        JOIN users u ON t.author_id = u.id
        WHERE t.id = $1
    `, id).Scan(&topic.ID, &topic.Title, &topic.Content, &topic.Category, &topic.AuthorID, &topic.AuthorName, &topic.RepliesCount, &topic.ViewsCount, &topic.CreatedAt, &topic.LastActivity)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Topic not found")
		return
	}

	_, _ = database.DB.Exec(`UPDATE forum_topics SET views_count = views_count + 1 WHERE id = $1`, id)
	rows, err := database.DB.Query(`
        SELECT r.id, r.topic_id, r.author_id, u.full_name, r.content, r.created_at
        FROM forum_replies r
        JOIN users u ON r.author_id = u.id
        WHERE r.topic_id = $1
        ORDER BY r.created_at ASC
    `, id)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	replies := make([]models.ForumReply, 0)
	for rows.Next() {
		var reply models.ForumReply
		if err := rows.Scan(&reply.ID, &reply.TopicID, &reply.AuthorID, &reply.AuthorName, &reply.Content, &reply.CreatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		replies = append(replies, reply)
	}
	c.JSON(http.StatusOK, gin.H{"topic": topic, "replies": replies})
}

func CreateTopic(c *gin.Context) {
	userID := currentUserID(c)
	var req struct {
		Title    string `json:"title" binding:"required"`
		Content  string `json:"content" binding:"required"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	topicID := uuid.New()
	if _, err := database.DB.Exec(`INSERT INTO forum_topics (id, title, content, category, author_id) VALUES ($1, $2, $3, $4, $5)`, topicID, req.Title, req.Content, req.Category, userID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create topic")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Topic created", "topic_id": topicID})
}

func AddReply(c *gin.Context) {
	topicID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	replyID := uuid.New()
	if _, err := database.DB.Exec(`INSERT INTO forum_replies (id, topic_id, author_id, content) VALUES ($1, $2, $3, $4)`, replyID, topicID, userID, req.Content); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to add reply")
		return
	}
	_, _ = database.DB.Exec(`UPDATE forum_topics SET replies_count = replies_count + 1, updated_at = NOW() WHERE id = $1`, topicID)
	c.JSON(http.StatusCreated, gin.H{"message": "Reply added"})
}
