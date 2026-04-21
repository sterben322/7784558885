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

type createCatalogItemRequest struct {
	Type        string   `json:"type"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Company     string   `json:"company"`
	Price       string   `json:"price"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

func CreateCatalogItem(c *gin.Context) {
	userID := currentUserID(c)
	var req createCatalogItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	req.Type = strings.TrimSpace(strings.ToLower(req.Type))
	if req.Type == "" {
		req.Type = "service"
	}
	if req.Type != "product" && req.Type != "service" {
		jsonError(c, http.StatusBadRequest, "type must be product or service")
		return
	}

	name := strings.TrimSpace(req.Name)
	category := strings.TrimSpace(req.Category)
	company := strings.TrimSpace(req.Company)
	price := strings.TrimSpace(req.Price)
	description := strings.TrimSpace(req.Description)
	if name == "" || category == "" || company == "" || price == "" || description == "" {
		jsonError(c, http.StatusBadRequest, "name, category, company, price and description are required")
		return
	}

	var ownerName string
	if err := database.DB.QueryRow(`SELECT COALESCE(NULLIF(full_name, ''), email) FROM users WHERE id = $1`, userID).Scan(&ownerName); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to resolve owner")
		return
	}

	itemID := uuid.New()
	_, err := database.DB.Exec(`
		INSERT INTO catalog_items (
			id, owner_id, owner_name, type, category, name, company, price, description, tags, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
	`, itemID, userID, ownerName, req.Type, category, name, company, price, description, pq.Array(req.Tags))
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create catalog item")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": itemID})
}

func GetCatalogItems(c *gin.Context) {
	userID := currentUserID(c)
	itemType := strings.TrimSpace(strings.ToLower(c.Query("type")))
	category := strings.TrimSpace(c.Query("category"))
	q := strings.TrimSpace(c.Query("q"))
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}

	rows, err := database.DB.Query(`
		SELECT id, owner_id, owner_name, type, category, name, company, price, description, tags,
		       rating, reviews_count, created_at,
		       owner_id = $1 AS is_owner
		FROM catalog_items
		WHERE ($2 = '' OR type = $2)
		  AND ($3 = '' OR category = $3)
		  AND ($4 = '' OR name ILIKE '%' || $4 || '%' OR company ILIKE '%' || $4 || '%' OR description ILIKE '%' || $4 || '%')
		ORDER BY created_at DESC
		LIMIT $5
	`, userID, itemType, category, q, limit)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load catalog")
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var (
			id, ownerID uuid.UUID
			ownerName   string
			typ         string
			cat         string
			name        string
			company     string
			price       string
			description string
			tags        pq.StringArray
			rating      sql.NullFloat64
			reviews     int
			createdAt   time.Time
			isOwner     bool
		)
		if err := rows.Scan(&id, &ownerID, &ownerName, &typ, &cat, &name, &company, &price, &description, &tags, &rating, &reviews, &createdAt, &isOwner); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse catalog")
			return
		}
		item := gin.H{
			"id":          id,
			"owner_id":    ownerID,
			"owner_name":  ownerName,
			"type":        typ,
			"category":    cat,
			"name":        name,
			"company":     company,
			"price":       price,
			"description": description,
			"tags":        []string(tags),
			"reviews":     reviews,
			"created_at":  createdAt,
			"is_owner":    isOwner,
		}
		if rating.Valid {
			item["rating"] = rating.Float64
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func GetMyCatalogItems(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT id, type, category, name, company, price, description, tags, rating, reviews_count, created_at
		FROM catalog_items
		WHERE owner_id = $1
		ORDER BY created_at DESC
		LIMIT 200
	`, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load my catalog")
		return
	}
	defer rows.Close()
	items := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var typ, category, name, company, price, description string
		var tags pq.StringArray
		var rating sql.NullFloat64
		var reviews int
		var createdAt time.Time
		if err := rows.Scan(&id, &typ, &category, &name, &company, &price, &description, &tags, &rating, &reviews, &createdAt); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse my catalog")
			return
		}
		item := gin.H{
			"id":          id,
			"type":        typ,
			"category":    category,
			"name":        name,
			"company":     company,
			"price":       price,
			"description": description,
			"tags":        []string(tags),
			"reviews":     reviews,
			"created_at":  createdAt,
		}
		if rating.Valid {
			item["rating"] = rating.Float64
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
