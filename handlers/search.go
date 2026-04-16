package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"lastop/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GlobalSearchItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Route    string `json:"route"`
}

type GlobalSearchData struct {
	Users     []GlobalSearchItem `json:"users"`
	Companies []GlobalSearchItem `json:"companies"`
	Forums    []GlobalSearchItem `json:"forums"`
	Topics    []GlobalSearchItem `json:"topics"`
	Chats     []GlobalSearchItem `json:"chats"`
	News      []GlobalSearchItem `json:"news"`
}

func GlobalSearch(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": GlobalSearchData{}})
		return
	}

	if len([]rune(query)) < 2 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": GlobalSearchData{}, "message": "Введите минимум 2 символа"})
		return
	}

	limit := 5
	if raw := strings.TrimSpace(c.DefaultQuery("limit", "5")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 20 {
				parsed = 20
			}
			limit = parsed
		}
	}

	userID := currentUserID(c)
	data := GlobalSearchData{
		Users:     make([]GlobalSearchItem, 0),
		Companies: make([]GlobalSearchItem, 0),
		Forums:    make([]GlobalSearchItem, 0),
		Topics:    make([]GlobalSearchItem, 0),
		Chats:     make([]GlobalSearchItem, 0),
		News:      make([]GlobalSearchItem, 0),
	}

	if err := searchUsers(&data, query, limit); err != nil {
		log.Printf("global search users failed: %v", err)
	}
	if err := searchCompanies(&data, query, limit); err != nil {
		log.Printf("global search companies failed: %v", err)
	}
	if err := searchForumSections(&data, query, limit); err != nil {
		log.Printf("global search forums failed: %v", err)
	}
	if err := searchForumTopics(&data, query, limit); err != nil {
		log.Printf("global search topics failed: %v", err)
	}
	if err := searchChatsForStart(&data, query, userID, limit); err != nil {
		log.Printf("global search chats failed: %v", err)
	}
	if err := searchNewsPosts(&data, query, limit); err != nil {
		log.Printf("global search news failed: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func searchUsers(data *GlobalSearchData, query string, limit int) error {
	rows, err := database.DB.Query(`
		SELECT id::text,
		       COALESCE(NULLIF(full_name, ''), NULLIF(name, ''), email) AS title,
		       COALESCE(NULLIF(name, ''), email) AS subtitle
		FROM users
		WHERE COALESCE(full_name, '') ILIKE '%' || $1 || '%'
		   OR COALESCE(name, '') ILIKE '%' || $1 || '%'
		   OR COALESCE(email, '') ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, subtitle string
		if err := rows.Scan(&id, &title, &subtitle); err != nil {
			return err
		}
		data.Users = append(data.Users, GlobalSearchItem{ID: id, Title: title, Subtitle: subtitle, Type: "user", Category: "Пользователи", Route: "/profile.html?userId=" + id})
	}
	return rows.Err()
}

func searchCompanies(data *GlobalSearchData, query string, limit int) error {
	rows, err := database.DB.Query(`
		SELECT c.id::text,
		       c.name,
		       COALESCE(c.economic_sector, 'Без отрасли') AS subtitle,
		       c.owner_id::text
		FROM companies c
		WHERE COALESCE(c.name, '') ILIKE '%' || $1 || '%'
		   OR COALESCE(c.description, '') ILIKE '%' || $1 || '%'
		ORDER BY c.followers_count DESC, c.created_at DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, subtitle, ownerID string
		if err := rows.Scan(&id, &name, &subtitle, &ownerID); err != nil {
			return err
		}
		data.Companies = append(data.Companies, GlobalSearchItem{ID: id, Title: name, Subtitle: subtitle, Type: "company", Category: "Компании", Route: "/company-profile.html?owner_id=" + ownerID})
	}
	return rows.Err()
}

func searchForumSections(data *GlobalSearchData, query string, limit int) error {
	rows, err := database.DB.Query(`
		SELECT id::text,
		       name,
		       COALESCE(NULLIF(description, ''), 'Раздел форума')
		FROM forum_sections
		WHERE COALESCE(name, '') ILIKE '%' || $1 || '%'
		   OR COALESCE(description, '') ILIKE '%' || $1 || '%'
		ORDER BY updated_at DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, subtitle string
		if err := rows.Scan(&id, &title, &subtitle); err != nil {
			return err
		}
		data.Forums = append(data.Forums, GlobalSearchItem{ID: id, Title: title, Subtitle: subtitle, Type: "forum_section", Category: "Форум", Route: "/forum.html?section=" + id})
	}
	return rows.Err()
}

func searchForumTopics(data *GlobalSearchData, query string, limit int) error {
	rows, err := database.DB.Query(`
		SELECT t.id::text,
		       t.title,
		       s.name,
		       t.section_id::text
		FROM forum_topics t
		JOIN forum_sections s ON s.id = t.section_id
		WHERE COALESCE(t.title, '') ILIKE '%' || $1 || '%'
		ORDER BY t.updated_at DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, sectionTitle, sectionID string
		if err := rows.Scan(&id, &title, &sectionTitle, &sectionID); err != nil {
			return err
		}
		data.Topics = append(data.Topics, GlobalSearchItem{ID: id, Title: title, Subtitle: "Раздел: " + sectionTitle, Type: "forum_topic", Category: "Темы", Route: "/forum.html?section=" + sectionID + "&topic=" + id})
	}
	return rows.Err()
}

func searchChatsForStart(data *GlobalSearchData, query string, userID uuid.UUID, limit int) error {
	rows, err := database.DB.Query(`
		SELECT id::text,
		       COALESCE(NULLIF(full_name, ''), email) AS title,
		       COALESCE(NULLIF(email, ''), 'Пользователь') AS subtitle
		FROM users
		WHERE id <> $1
		  AND (
			COALESCE(full_name, '') ILIKE '%' || $2 || '%'
			OR COALESCE(name, '') ILIKE '%' || $2 || '%'
			OR COALESCE(email, '') ILIKE '%' || $2 || '%'
		  )
		ORDER BY full_name ASC
		LIMIT $3
	`, userID, query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, subtitle string
		if err := rows.Scan(&id, &title, &subtitle); err != nil {
			return err
		}
		data.Chats = append(data.Chats, GlobalSearchItem{ID: id, Title: title, Subtitle: subtitle, Type: "chat_user", Category: "Быстрый чат", Route: "/chat.html?userId=" + id})
	}
	return rows.Err()
}

func searchNewsPosts(data *GlobalSearchData, query string, limit int) error {
	rows, err := database.DB.Query(`
		SELECT id::text,
		       COALESCE(NULLIF(title, ''), LEFT(COALESCE(content, ''), 80), 'Публикация') AS title,
		       COALESCE(author_name, 'Автор') AS subtitle
		FROM posts
		WHERE is_hidden = false
		  AND is_unpublished = false
		  AND (
			COALESCE(title, '') ILIKE '%' || $1 || '%'
			OR COALESCE(content, '') ILIKE '%' || $1 || '%'
		  )
		ORDER BY created_at DESC
		LIMIT $2
	`, query, limit)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, subtitle string
		if err := rows.Scan(&id, &title, &subtitle); err != nil {
			return err
		}
		data.News = append(data.News, GlobalSearchItem{ID: id, Title: title, Subtitle: subtitle, Type: "news", Category: "Публикации", Route: "/dashboard.html#post-" + id})
	}
	return rows.Err()
}
