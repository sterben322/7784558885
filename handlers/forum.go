package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"lastop/database"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

// ForumSection represents a top-level category on the forum.
type ForumSection struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	ColorIdx      int        `json:"color_idx"`
	SortOrder     int        `json:"sort_order"`
	TopicsCount   int        `json:"topics_count"`
	MessagesCount int        `json:"messages_count"`
	LastAuthor    string     `json:"last_author,omitempty"`
	LastAt        *time.Time `json:"last_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ForumTopic represents a thread inside a section.
type ForumTopic struct {
	ID           string         `json:"id"`
	SectionID    string         `json:"section_id"`
	AuthorID     string         `json:"author_id"`
	AuthorName   string         `json:"author_name"`
	Title        string         `json:"title"`
	Tags         pq.StringArray `json:"tags"`
	RepliesCount int            `json:"replies_count"`
	ViewsCount   int            `json:"views_count"`
	IsHot        bool           `json:"is_hot"`
	IsPinned     bool           `json:"is_pinned"`
	IsNew        bool           `json:"is_new"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ForumMessage represents a single post inside a topic.
type ForumMessage struct {
	ID           string    `json:"id"`
	TopicID      string    `json:"topic_id"`
	ParentID     *string   `json:"parent_id"`
	ParentAuthor string    `json:"parent_author,omitempty"`
	ParentText   string    `json:"parent_text,omitempty"`
	AuthorID     string    `json:"author_id"`
	Author       string    `json:"author"`
	IsModerator  bool      `json:"is_moderator"`
	Text         string    `json:"text"`
	LikesCount   int       `json:"likes_count"`
	IsLiked      bool      `json:"is_liked"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListSections GET /api/forum/sections
func GetForumSections(c *gin.Context) {
	rows, err := database.DB.QueryContext(c, `
		SELECT
			s.id::text, s.name, s.description, s.color_idx, s.sort_order,
			s.topics_count, s.messages_count,
			s.last_author, s.last_at, s.created_at
		FROM forum_sections s
		WHERE s.deleted_at IS NULL
		ORDER BY s.sort_order ASC, s.id ASC`)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	sections := make([]ForumSection, 0)
	for rows.Next() {
		var s ForumSection
		var lastAuthor sql.NullString
		var lastAt sql.NullTime
		if err := rows.Scan(
			&s.ID, &s.Name, &s.Description, &s.ColorIdx, &s.SortOrder,
			&s.TopicsCount, &s.MessagesCount,
			&lastAuthor, &lastAt, &s.CreatedAt,
		); err != nil {
			continue
		}
		if lastAuthor.Valid {
			s.LastAuthor = lastAuthor.String
		}
		if lastAt.Valid {
			s.LastAt = &lastAt.Time
		}
		sections = append(sections, s)
	}
	c.JSON(http.StatusOK, gin.H{"sections": sections})
}

// CreateForumSection POST /api/forum/sections
func CreateForumSection(c *gin.Context) {
	userID := currentUserID(c)
	if !isForumAdmin(c, userID.String()) {
		jsonError(c, http.StatusForbidden, "only admins can create sections")
		return
	}

	var body struct {
		Name        string `json:"name" binding:"required,min=2,max=120"`
		Description string `json:"description"`
		ColorIdx    int    `json:"color_idx"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if body.ColorIdx < 0 || body.ColorIdx > 5 {
		body.ColorIdx = 0
	}

	var id string
	err := database.DB.QueryRowContext(c, `
		INSERT INTO forum_sections (name, description, color_idx)
		VALUES ($1, $2, $3)
		RETURNING id::text`,
		body.Name, body.Description, body.ColorIdx,
	).Scan(&id)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

// GetSectionTopics GET /api/forum/sections/:id/topics
func GetSectionTopics(c *gin.Context) {
	sectionID := c.Param("id")
	limit := queryIntForum(c, "limit", 50)
	offset := queryIntForum(c, "offset", 0)

	rows, err := database.DB.QueryContext(c, `
		SELECT
			t.id::text, t.section_id::text, t.author_id::text,
			COALESCE(u.full_name, u.name, split_part(u.email, '@', 1), 'Участник') AS author_name,
			t.title, t.tags,
			t.replies_count, t.views_count,
			t.is_hot, t.is_pinned,
			(NOW() - t.created_at < INTERVAL '24 hours') AS is_new,
			t.created_at
		FROM forum_topics t
		INNER JOIN users u ON u.id = t.author_id
		WHERE t.section_id = $1::uuid AND t.deleted_at IS NULL
		ORDER BY t.is_pinned DESC, t.created_at DESC
		LIMIT $2 OFFSET $3`,
		sectionID, limit, offset)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	topics := make([]ForumTopic, 0)
	for rows.Next() {
		var t ForumTopic
		if err := rows.Scan(
			&t.ID, &t.SectionID, &t.AuthorID, &t.AuthorName,
			&t.Title, &t.Tags,
			&t.RepliesCount, &t.ViewsCount,
			&t.IsHot, &t.IsPinned, &t.IsNew, &t.CreatedAt,
		); err != nil {
			continue
		}
		topics = append(topics, t)
	}
	c.JSON(http.StatusOK, gin.H{"topics": topics})
}

// CreateSectionTopic POST /api/forum/sections/:id/topics
func CreateSectionTopic(c *gin.Context) {
	userID := currentUserID(c)
	sectionID := c.Param("id")

	var body struct {
		Title string   `json:"title" binding:"required,min=3,max=300"`
		Text  string   `json:"text"`
		Tags  []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	tags := pq.StringArray(body.Tags)

	tx, err := database.DB.BeginTx(c, nil)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var topicID string
	err = tx.QueryRowContext(c, `
		INSERT INTO forum_topics (section_id, author_id, title, tags)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING id::text`,
		sectionID, userID.String(), body.Title, tags,
	).Scan(&topicID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	_, _ = tx.ExecContext(c, `
		UPDATE forum_sections
		SET topics_count = topics_count + 1, updated_at = NOW()
		WHERE id = $1::uuid`, sectionID)

	if body.Text != "" {
		_, _ = tx.ExecContext(c, `
			INSERT INTO forum_messages (topic_id, author_id, text)
			VALUES ($1::uuid, $2::uuid, $3)`,
			topicID, userID.String(), body.Text)
		_, _ = tx.ExecContext(c, `
			UPDATE forum_topics SET replies_count = replies_count + 1 WHERE id = $1::uuid`, topicID)
		_, _ = tx.ExecContext(c, `
			UPDATE forum_sections SET messages_count = messages_count + 1 WHERE id = $1::uuid`, sectionID)
	}

	_, _ = tx.ExecContext(c, `
		UPDATE forum_sections s
		SET last_author = (SELECT COALESCE(full_name, name, split_part(email, '@', 1)) FROM users WHERE id = $2::uuid),
		    last_at = NOW()
		WHERE s.id = $1::uuid`, sectionID, userID.String())

	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": topicID})
}

// GetTopicMessages GET /api/forum/topics/:id/messages
func GetTopicMessages(c *gin.Context) {
	userID := currentUserID(c)
	topicID := c.Param("id")
	limit := queryIntForum(c, "limit", 50)
	offset := queryIntForum(c, "offset", 0)

	_, _ = database.DB.ExecContext(c, `UPDATE forum_topics SET views_count = views_count + 1 WHERE id = $1::uuid`, topicID)

	rows, err := database.DB.QueryContext(c, `
		SELECT
			m.id::text, m.topic_id::text, m.parent_id::text,
			COALESCE(p.full_name, p.name, split_part(p.email, '@', 1), '') AS parent_author,
			COALESCE(pm.text, '') AS parent_text,
			m.author_id::text,
			COALESCE(u.full_name, u.name, split_part(u.email, '@', 1), 'Участник') AS author_name,
			COALESCE(u.is_moderator, false) AS is_moderator,
			m.text, m.likes_count,
			EXISTS(
				SELECT 1 FROM forum_message_likes l
				WHERE l.message_id = m.id AND l.user_id = $2::uuid
			) AS is_liked,
			m.created_at
		FROM forum_messages m
		INNER JOIN users u ON u.id = m.author_id
		LEFT JOIN forum_messages pm ON pm.id = m.parent_id
		LEFT JOIN users p ON p.id = pm.author_id
		WHERE m.topic_id = $1::uuid AND m.deleted_at IS NULL
		ORDER BY m.created_at ASC
		LIMIT $3 OFFSET $4`,
		topicID, userID.String(), limit, offset)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	messages := make([]ForumMessage, 0)
	for rows.Next() {
		var m ForumMessage
		var parentID sql.NullString
		if err := rows.Scan(
			&m.ID, &m.TopicID, &parentID,
			&m.ParentAuthor, &m.ParentText,
			&m.AuthorID, &m.Author,
			&m.IsModerator, &m.Text, &m.LikesCount, &m.IsLiked, &m.CreatedAt,
		); err != nil {
			continue
		}
		if parentID.Valid {
			m.ParentID = &parentID.String
			if len(m.ParentText) > 80 {
				m.ParentText = m.ParentText[:80] + "…"
			}
		}
		messages = append(messages, m)
	}
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// AddTopicPost POST /api/forum/topics/:id/messages
func AddTopicPost(c *gin.Context) {
	userID := currentUserID(c)
	topicID := c.Param("id")

	var body struct {
		Text     string  `json:"text" binding:"required,min=1"`
		ParentID *string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := database.DB.BeginTx(c, nil)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var sectionID string
	err = tx.QueryRowContext(c,
		`SELECT section_id::text FROM forum_topics WHERE id = $1::uuid AND deleted_at IS NULL`, topicID,
	).Scan(&sectionID)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "topic not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	var msgID string
	err = tx.QueryRowContext(c, `
		INSERT INTO forum_messages (topic_id, author_id, parent_id, text)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4)
		RETURNING id::text`,
		topicID, userID.String(), body.ParentID, body.Text,
	).Scan(&msgID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	_, _ = tx.ExecContext(c, `UPDATE forum_topics SET replies_count = replies_count + 1 WHERE id = $1::uuid`, topicID)
	_, _ = tx.ExecContext(c, `UPDATE forum_sections SET messages_count = messages_count + 1, last_at = NOW() WHERE id = $1::uuid`, sectionID)
	_, _ = tx.ExecContext(c, `
		UPDATE forum_sections
		SET last_author = (SELECT COALESCE(full_name, name, split_part(email, '@', 1)) FROM users WHERE id = $2::uuid),
		    last_at = NOW()
		WHERE id = $1::uuid`, sectionID, userID.String())

	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": msgID})
}

// LikeForumMessage POST /api/forum/messages/:id/like
func LikeForumMessage(c *gin.Context) {
	userID := currentUserID(c)
	msgID := c.Param("id")

	tx, err := database.DB.BeginTx(c, nil)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(c, `
		INSERT INTO forum_message_likes (message_id, user_id)
		VALUES ($1::uuid, $2::uuid)
		ON CONFLICT DO NOTHING`, msgID, userID.String())
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	_, _ = tx.ExecContext(c, `
		UPDATE forum_messages
		SET likes_count = (SELECT COUNT(*) FROM forum_message_likes WHERE message_id = $1::uuid)
		WHERE id = $1::uuid`, msgID)
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// UnlikeForumMessage DELETE /api/forum/messages/:id/like
func UnlikeForumMessage(c *gin.Context) {
	userID := currentUserID(c)
	msgID := c.Param("id")

	tx, err := database.DB.BeginTx(c, nil)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	_, _ = tx.ExecContext(c, `
		DELETE FROM forum_message_likes WHERE message_id = $1::uuid AND user_id = $2::uuid`,
		msgID, userID.String())
	_, _ = tx.ExecContext(c, `
		UPDATE forum_messages
		SET likes_count = (SELECT COUNT(*) FROM forum_message_likes WHERE message_id = $1::uuid)
		WHERE id = $1::uuid`, msgID)
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func isForumAdmin(c *gin.Context, userID string) bool {
	var isAdmin bool
	_ = database.DB.QueryRowContext(c, `SELECT COALESCE(is_admin, false) FROM users WHERE id = $1::uuid`, userID).Scan(&isAdmin)
	return isAdmin
}

func queryIntForum(c *gin.Context, key string, def int) int {
	if v := c.Query(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
