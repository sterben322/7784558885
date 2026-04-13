package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetForumSections(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT s.id, s.title, s.description, s.creator_id, u.full_name,
		       s.topics_count, s.posts_count, s.created_at, s.updated_at
		FROM forum_sections s
		JOIN users u ON u.id = s.creator_id
		ORDER BY s.updated_at DESC, s.created_at DESC
	`)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	sections := make([]models.ForumSection, 0)
	for rows.Next() {
		var section models.ForumSection
		if err := rows.Scan(&section.ID, &section.Title, &section.Description, &section.CreatorID, &section.CreatorName, &section.TopicsCount, &section.PostsCount, &section.CreatedAt, &section.UpdatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		sections = append(sections, section)
	}
	c.JSON(http.StatusOK, gin.H{"sections": sections})
}

func CreateForumSection(c *gin.Context) {
	userID := currentUserID(c)
	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if len(req.Title) < 3 {
		jsonError(c, http.StatusBadRequest, "Название раздела должно быть не короче 3 символов")
		return
	}

	sectionID := uuid.New()
	_, err := database.DB.Exec(`
		INSERT INTO forum_sections (id, title, description, creator_id)
		VALUES ($1, $2, $3, $4)
	`, sectionID, req.Title, strings.TrimSpace(req.Description), userID)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Не удалось создать раздел")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"section_id": sectionID})
}

func GetSectionTopics(c *gin.Context) {
	sectionID := c.Param("id")

	var section models.ForumSection
	err := database.DB.QueryRow(`
		SELECT s.id, s.title, s.description, s.creator_id, u.full_name,
		       s.topics_count, s.posts_count, s.created_at, s.updated_at
		FROM forum_sections s
		JOIN users u ON u.id = s.creator_id
		WHERE s.id = $1
	`, sectionID).Scan(&section.ID, &section.Title, &section.Description, &section.CreatorID, &section.CreatorName, &section.TopicsCount, &section.PostsCount, &section.CreatedAt, &section.UpdatedAt)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Раздел не найден")
		return
	}

	rows, err := database.DB.Query(`
		SELECT t.id, t.section_id, t.title, t.author_id, u.full_name,
		       t.posts_count, t.views_count, t.created_at, t.updated_at,
		       COALESCE(lp.author_name, ''), lp.created_at
		FROM forum_topics t
		JOIN users u ON u.id = t.author_id
		LEFT JOIN LATERAL (
			SELECT u2.full_name AS author_name, p.created_at
			FROM forum_posts p
			JOIN users u2 ON u2.id = p.author_id
			WHERE p.topic_id = t.id
			ORDER BY p.created_at DESC
			LIMIT 1
		) lp ON true
		WHERE t.section_id = $1
		ORDER BY t.updated_at DESC
	`, sectionID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	topics := make([]models.ForumTopicListItem, 0)
	for rows.Next() {
		var topic models.ForumTopicListItem
		var lastPostAt sql.NullTime
		if err := rows.Scan(&topic.ID, &topic.SectionID, &topic.Title, &topic.AuthorID, &topic.AuthorName, &topic.PostsCount, &topic.ViewsCount, &topic.CreatedAt, &topic.UpdatedAt, &topic.LastPostAuthor, &lastPostAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if lastPostAt.Valid {
			topic.LastPostAt = &lastPostAt.Time
		}
		topics = append(topics, topic)
	}

	c.JSON(http.StatusOK, gin.H{"section": section, "topics": topics})
}

func CreateSectionTopic(c *gin.Context) {
	sectionID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if len(req.Title) < 5 || len(req.Content) < 5 {
		jsonError(c, http.StatusBadRequest, "Тема и сообщение должны быть заполнены")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM forum_sections WHERE id = $1)`, sectionID).Scan(&exists); err != nil || !exists {
		jsonError(c, http.StatusNotFound, "Раздел не найден")
		return
	}

	topicID := uuid.New()
	postID := uuid.New()
	now := time.Now().UTC()

	if _, err := tx.Exec(`
		INSERT INTO forum_topics (id, section_id, title, author_id, posts_count, views_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, 0, $5, $5)
	`, topicID, sectionID, req.Title, userID, now); err != nil {
		jsonError(c, http.StatusInternalServerError, "Не удалось создать тему")
		return
	}

	if _, err := tx.Exec(`
		INSERT INTO forum_posts (id, topic_id, author_id, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, postID, topicID, userID, req.Content, now); err != nil {
		jsonError(c, http.StatusInternalServerError, "Не удалось создать первое сообщение")
		return
	}

	if _, err := tx.Exec(`
		UPDATE forum_sections
		SET topics_count = topics_count + 1,
		    posts_count = posts_count + 1,
		    updated_at = $2
		WHERE id = $1
	`, sectionID, now); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"topic_id": topicID})
}

func GetTopicDiscussion(c *gin.Context) {
	topicID := c.Param("id")
	var topic models.ForumTopic
	err := database.DB.QueryRow(`
		SELECT t.id, t.section_id, s.title, t.title, t.author_id, u.full_name,
		       t.posts_count, t.views_count, t.created_at, t.updated_at
		FROM forum_topics t
		JOIN forum_sections s ON s.id = t.section_id
		JOIN users u ON u.id = t.author_id
		WHERE t.id = $1
	`, topicID).Scan(&topic.ID, &topic.SectionID, &topic.SectionTitle, &topic.Title, &topic.AuthorID, &topic.AuthorName, &topic.PostsCount, &topic.ViewsCount, &topic.CreatedAt, &topic.UpdatedAt)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Тема не найдена")
		return
	}

	_, _ = database.DB.Exec(`UPDATE forum_topics SET views_count = views_count + 1 WHERE id = $1`, topicID)

	rows, err := database.DB.Query(`
		SELECT p.id, p.topic_id, p.author_id, u.full_name, p.content,
		       p.created_at, p.updated_at
		FROM forum_posts p
		JOIN users u ON u.id = p.author_id
		WHERE p.topic_id = $1
		ORDER BY p.created_at ASC
	`, topicID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	posts := make([]models.ForumPost, 0)
	userID := currentUserID(c)
	for rows.Next() {
		var post models.ForumPost
		if err := rows.Scan(&post.ID, &post.TopicID, &post.AuthorID, &post.AuthorName, &post.Content, &post.CreatedAt, &post.UpdatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		post.CanEdit = post.AuthorID == userID
		posts = append(posts, post)
	}

	c.JSON(http.StatusOK, gin.H{"topic": topic, "posts": posts})
}

func AddTopicPost(c *gin.Context) {
	topicID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	content := strings.TrimSpace(req.Content)
	if len(content) < 2 {
		jsonError(c, http.StatusBadRequest, "Сообщение слишком короткое")
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var sectionID uuid.UUID
	if err := tx.QueryRow(`SELECT section_id FROM forum_topics WHERE id = $1`, topicID).Scan(&sectionID); err != nil {
		jsonError(c, http.StatusNotFound, "Тема не найдена")
		return
	}

	now := time.Now().UTC()
	postID := uuid.New()
	if _, err := tx.Exec(`
		INSERT INTO forum_posts (id, topic_id, author_id, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, postID, topicID, userID, content, now); err != nil {
		jsonError(c, http.StatusInternalServerError, "Не удалось добавить ответ")
		return
	}

	if _, err := tx.Exec(`UPDATE forum_topics SET posts_count = posts_count + 1, updated_at = $2 WHERE id = $1`, topicID, now); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(`UPDATE forum_sections SET posts_count = posts_count + 1, updated_at = $2 WHERE id = $1`, sectionID, now); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{"post_id": postID})
}

func UpdateForumPost(c *gin.Context) {
	postID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	content := strings.TrimSpace(req.Content)
	if len(content) < 2 {
		jsonError(c, http.StatusBadRequest, "Сообщение слишком короткое")
		return
	}

	res, err := database.DB.Exec(`
		UPDATE forum_posts
		SET content = $1, updated_at = NOW()
		WHERE id = $2 AND author_id = $3
	`, content, postID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		var exists bool
		err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM forum_posts WHERE id = $1)`, postID).Scan(&exists)
		if err != nil || !exists {
			jsonError(c, http.StatusNotFound, "Сообщение не найдено")
			return
		}
		jsonError(c, http.StatusForbidden, "Можно редактировать только свои сообщения")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Сообщение обновлено"})
}

func RecalculateForumStats() error {
	if database.DB == nil {
		return errors.New("db not initialized")
	}
	if _, err := database.DB.Exec(`
		UPDATE forum_topics t
		SET posts_count = sub.count_posts,
		    updated_at = COALESCE(sub.last_post_at, t.updated_at)
		FROM (
			SELECT topic_id, COUNT(*)::int AS count_posts, MAX(created_at) AS last_post_at
			FROM forum_posts
			GROUP BY topic_id
		) sub
		WHERE t.id = sub.topic_id
	`); err != nil {
		return err
	}
	_, err := database.DB.Exec(`
		UPDATE forum_sections s
		SET topics_count = COALESCE(t.cnt, 0),
		    posts_count = COALESCE(p.cnt, 0),
		    updated_at = GREATEST(s.updated_at, COALESCE(p.last_at, s.updated_at))
		FROM (
			SELECT section_id, COUNT(*)::int AS cnt
			FROM forum_topics
			GROUP BY section_id
		) t
		FULL JOIN (
			SELECT t.section_id, COUNT(p.id)::int AS cnt, MAX(p.created_at) AS last_at
			FROM forum_topics t
			LEFT JOIN forum_posts p ON p.topic_id = t.id
			GROUP BY t.section_id
		) p ON p.section_id = t.section_id
		WHERE s.id = COALESCE(t.section_id, p.section_id)
	`)
	return err
}
