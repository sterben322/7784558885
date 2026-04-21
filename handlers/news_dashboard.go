package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastop/database"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type dashboardNewsItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"cat"`
	Ago       string    `json:"ago"`
	Text      string    `json:"text"`
	Views     int       `json:"views"`
	Source    string    `json:"src"`
	CreatedAt time.Time `json:"created_at"`
}

type dashboardTopItem struct {
	Title string `json:"title"`
	Views int    `json:"views"`
}

type dashboardSourceItem struct {
	Source    string `json:"source"`
	Posts     int    `json:"posts"`
	Followers int    `json:"followers"`
}

type newsDashboardResponse struct {
	News    []dashboardNewsItem   `json:"news"`
	TopWeek []dashboardTopItem    `json:"top_week"`
	Sources []dashboardSourceItem `json:"sources"`
}

func GetNewsDashboard(c *gin.Context) {
	userID := currentUserID(c)
	category := strings.TrimSpace(c.Query("category"))
	if strings.EqualFold(category, "все") {
		category = ""
	}

	limit := 30
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	items, err := loadDashboardNews(userID, category, limit)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	topWeek, err := loadDashboardTopWeek()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	sources, err := loadDashboardSources()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, newsDashboardResponse{
		News:    items,
		TopWeek: topWeek,
		Sources: sources,
	})
}

func loadDashboardNews(userID uuid.UUID, category string, limit int) ([]dashboardNewsItem, error) {
	query := `
		SELECT p.id, p.title, p.content, p.short_description, p.tags, p.created_at,
		       p.likes_count, p.comments_count, p.shares_count,
		       COALESCE(NULLIF(p.author_name, ''), 'LASTOP') AS source,
		       EXISTS(SELECT 1 FROM post_likes WHERE post_id = p.id AND user_id = $1) AS _is_liked
		FROM posts p
		WHERE p.author_type IN ('user', 'community', 'company')
		  AND p.privacy_level = 'public'
		  AND p.is_hidden = false
		  AND p.is_unpublished = false
	`
	args := []interface{}{userID}
	argIndex := 2
	if category != "" {
		query += " AND (array_position(p.tags, $" + strconv.Itoa(argIndex) + ") IS NOT NULL OR lower($" + strconv.Itoa(argIndex) + ") = lower('публикации') AND cardinality(p.tags) = 0)"
		args = append(args, category)
		argIndex++
	}
	query += " ORDER BY p.created_at DESC LIMIT $" + strconv.Itoa(argIndex)
	args = append(args, limit)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]dashboardNewsItem, 0)
	for rows.Next() {
		var id uuid.UUID
		var title, content, shortDescription, source sql.NullString
		var tags pq.StringArray
		var createdAt time.Time
		var likes, comments, shares int
		var isLiked bool

		if err := rows.Scan(&id, &title, &content, &shortDescription, &tags, &createdAt, &likes, &comments, &shares, &source, &isLiked); err != nil {
			return nil, err
		}

		text := content.String
		if shortDescription.Valid && strings.TrimSpace(shortDescription.String) != "" {
			text = shortDescription.String
		}

		categoryName := "Публикации"
		if len(tags) > 0 && strings.TrimSpace(tags[0]) != "" {
			categoryName = tags[0]
		}
		itemTitle := "Публикация"
		if title.Valid && strings.TrimSpace(title.String) != "" {
			itemTitle = title.String
		}

		items = append(items, dashboardNewsItem{
			ID:        id,
			Title:     itemTitle,
			Category:  categoryName,
			Ago:       formatAgo(createdAt),
			Text:      text,
			Views:     likes*5 + comments*15 + shares*25,
			Source:    source.String,
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func loadDashboardTopWeek() ([]dashboardTopItem, error) {
	rows, err := database.DB.Query(`
		SELECT COALESCE(NULLIF(p.title, ''), 'Публикация') AS title,
		       (p.likes_count * 5 + p.comments_count * 15 + p.shares_count * 25) AS score
		FROM posts p
		WHERE p.author_type IN ('user', 'community', 'company')
		  AND p.privacy_level = 'public'
		  AND p.is_hidden = false
		  AND p.is_unpublished = false
		  AND p.created_at >= NOW() - INTERVAL '7 days'
		ORDER BY score DESC, p.created_at DESC
		LIMIT 3
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	top := make([]dashboardTopItem, 0)
	for rows.Next() {
		var item dashboardTopItem
		if err := rows.Scan(&item.Title, &item.Views); err != nil {
			return nil, err
		}
		top = append(top, item)
	}
	return top, rows.Err()
}

func loadDashboardSources() ([]dashboardSourceItem, error) {
	rows, err := database.DB.Query(`
		SELECT COALESCE(NULLIF(author_name, ''), 'LASTOP') AS source, COUNT(*) AS posts
		FROM posts
		WHERE author_type IN ('user', 'community', 'company')
		  AND privacy_level = 'public'
		  AND is_hidden = false
		  AND is_unpublished = false
		GROUP BY source
		ORDER BY posts DESC, source ASC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]dashboardSourceItem, 0)
	for rows.Next() {
		var item dashboardSourceItem
		if err := rows.Scan(&item.Source, &item.Posts); err != nil {
			return nil, err
		}
		item.Followers = item.Posts*12 + 100
		sources = append(sources, item)
	}
	return sources, rows.Err()
}

func formatAgo(ts time.Time) string {
	delta := time.Since(ts)
	if delta < time.Hour {
		minutes := int(delta.Minutes())
		if minutes < 1 {
			minutes = 1
		}
		return strconv.Itoa(minutes) + " мин назад"
	}
	if delta < 24*time.Hour {
		hours := int(delta.Hours())
		if hours < 1 {
			hours = 1
		}
		return strconv.Itoa(hours) + " ч назад"
	}
	if delta < 7*24*time.Hour {
		days := int(delta.Hours() / 24)
		if days < 1 {
			days = 1
		}
		return strconv.Itoa(days) + " д назад"
	}
	return ts.Format("02 Jan")
}
