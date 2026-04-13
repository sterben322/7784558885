package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func scanCompanyBase(rowScanner interface{ Scan(dest ...any) error }, company *models.Company) error {
	var inn, description, logoURL, sector, website, phone, address sql.NullString
	var tags pq.StringArray
	if err := rowScanner.Scan(&company.ID, &company.OwnerID, &company.OwnerName, &company.Name, &inn, &description, &logoURL, &sector, &company.IsPublic, &tags, &website, &phone, &address, &company.FollowersCount, &company.EmployeeCount, &company.IsFollowing, &company.CreatedAt); err != nil {
		return err
	}
	company.SearchTags = tags
	if inn.Valid {
		company.INN = &inn.String
	}
	if description.Valid {
		company.Description = &description.String
	}
	if logoURL.Valid {
		company.LogoURL = &logoURL.String
	}
	if sector.Valid {
		company.EconomicSector = &sector.String
	}
	if website.Valid {
		company.Website = &website.String
	}
	if phone.Valid {
		company.Phone = &phone.String
	}
	if address.Valid {
		company.Address = &address.String
	}
	return nil
}

func GetCompany(c *gin.Context) {
	userID := currentUserID(c)
	targetOwnerID := c.Query("owner_id")
	targetCompanyID := c.Query("id")

	query := `
        SELECT c.id, c.owner_id, u.full_name, c.name, c.inn, c.description, c.logo_url,
               c.economic_sector, c.is_public, c.search_tags, c.website, c.phone, c.address,
               c.followers_count, c.employee_count,
               EXISTS(SELECT 1 FROM company_followers WHERE company_id = c.id AND user_id = $1) AS is_following,
               c.created_at
        FROM companies c
        JOIN users u ON c.owner_id = u.id
    `
	args := []any{userID}
	if targetCompanyID != "" {
		query += " WHERE c.id = $2"
		args = append(args, targetCompanyID)
	} else {
		query += " WHERE c.owner_id = $2"
		queryArg := any(userID)
		if targetOwnerID != "" {
			queryArg = targetOwnerID
		}
		args = append(args, queryArg)
	}
	var company models.Company
	err := scanCompanyBase(database.DB.QueryRow(query, args...), &company)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"company": nil})
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !company.IsPublic && !isCompanyMember(company.ID.String(), userID) && company.OwnerID != userID {
		company.Description = nil
		company.Website = nil
		company.Phone = nil
		company.Address = nil
	}
	c.JSON(http.StatusOK, gin.H{"company": company})
}

func CreateCompany(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	var exists bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM companies WHERE owner_id = $1)`, userID).Scan(&exists)
	if exists {
		jsonError(c, http.StatusConflict, "You already have a registered company")
		return
	}
	companyID := uuid.New()
	tx, err := database.DB.Begin()
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create company")
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
        INSERT INTO companies (id, owner_id, name, inn, description, logo_url, economic_sector, is_public, search_tags, website, phone, address)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, companyID, userID, req.Name, req.INN, req.Description, req.LogoURL, req.EconomicSector, req.IsPublic, pq.Array(req.SearchTags), req.Website, req.Phone, req.Address)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create company")
		return
	}
	defaultRoleStatements := []string{
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'owner', 'Owner', ARRAY['Полный доступ'], ARRAY['*'])`,
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'admin', 'Admin', ARRAY['Операционное управление'], ARRAY['invite_employees','manage_roles','edit_company_profile','publish_news','manage_employees'])`,
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'editor', 'Editor', ARRAY['Редактирование профиля и новостей'], ARRAY['edit_company_profile','publish_news'])`,
		`INSERT INTO company_roles (company_id, role_code, position_name, responsibilities, permissions)
		 VALUES ($1, 'member', 'Member', ARRAY['Базовый доступ'], ARRAY[]::TEXT[])`,
	}
	for _, stmt := range defaultRoleStatements {
		if _, err := tx.Exec(stmt, companyID); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to create default roles")
			return
		}
	}
	_, _ = tx.Exec(`
		UPDATE corporate_profiles
		SET company_id = $1,
			created_by = $2,
			status = 'active',
			employment_status = 'owner',
			updated_at = NOW()
		WHERE user_id = $2
	`, companyID, userID)
	_, _ = tx.Exec(`
		INSERT INTO corporate_profiles (id, user_id, company_id, created_by, position_name, permissions, status, employment_status)
		VALUES ($1, $2, $3, $2, 'Owner', ARRAY['*']::TEXT[], 'active', 'owner')
		ON CONFLICT (user_id) DO UPDATE
		SET company_id = EXCLUDED.company_id,
			created_by = EXCLUDED.created_by,
			position_name = EXCLUDED.position_name,
			permissions = EXCLUDED.permissions,
			status = 'active',
			employment_status = 'owner',
			updated_at = NOW()
	`, uuid.New(), userID, companyID)
	_, _ = tx.Exec(`INSERT INTO company_members (company_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, companyID, userID)
	_, _ = tx.Exec(`
		INSERT INTO company_user_roles (company_id, user_id, role_id, assigned_by)
		SELECT $1, $2, id, $2 FROM company_roles WHERE company_id = $1 AND role_code = 'owner'
		ON CONFLICT (company_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id, assigned_by = EXCLUDED.assigned_by, assigned_at = NOW()
	`, companyID, userID)
	_, _ = tx.Exec(`
		INSERT INTO company_employees (company_id, user_id, position_name, role_id, assigned_by)
		SELECT $1, $2, 'Owner', id, $2 FROM company_roles WHERE company_id = $1 AND role_code = 'owner'
		ON CONFLICT (company_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id, is_active = true, position_name = EXCLUDED.position_name
	`, companyID, userID)
	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to save company")
		return
	}
	recalcCompanyEmployeesCount(companyID.String())
	c.JSON(http.StatusCreated, gin.H{"message": "Company created successfully", "company_id": companyID})
}

func UpdateCompany(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	var companyID string
	if err := database.DB.QueryRow(`SELECT id::text FROM companies WHERE owner_id = $1`, userID).Scan(&companyID); err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	hasPermission, err := requireCompanyPermission(companyID, userID, "edit_company_profile")
	if err != nil || !hasPermission {
		jsonError(c, http.StatusForbidden, "Insufficient permissions to edit company")
		return
	}

	_, err = database.DB.Exec(`
        UPDATE companies
        SET name = $1, inn = $2, description = $3, logo_url = $4, economic_sector = $5,
            is_public = $6, search_tags = $7, website = $8, phone = $9, address = $10, updated_at = NOW()
        WHERE id = $11
    `, req.Name, req.INN, req.Description, req.LogoURL, req.EconomicSector, req.IsPublic, pq.Array(req.SearchTags), req.Website, req.Phone, req.Address, companyID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update company")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Company updated successfully"})
}

func FollowCompany(c *gin.Context) {
	companyID := c.Param("id")
	userID := currentUserID(c)
	_, err := database.DB.Exec(`INSERT INTO company_followers (company_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, companyID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to follow company")
		return
	}
	recalcCompanyFollowersCount(companyID)
	c.JSON(http.StatusOK, gin.H{"message": "Company followed"})
}

func UnfollowCompany(c *gin.Context) {
	companyID := c.Param("id")
	userID := currentUserID(c)
	_, err := database.DB.Exec(`DELETE FROM company_followers WHERE company_id = $1 AND user_id = $2`, companyID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to unfollow company")
		return
	}
	recalcCompanyFollowersCount(companyID)
	c.JSON(http.StatusOK, gin.H{"message": "Company unfollowed"})
}

func SearchCompanies(c *gin.Context) {
	query := c.Query("q")
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
        SELECT c.id, c.owner_id, u.full_name, c.name, c.inn, c.description, c.logo_url, c.economic_sector, c.is_public,
               c.search_tags, c.website, c.phone, c.address, c.followers_count, c.employee_count,
               EXISTS(SELECT 1 FROM company_followers WHERE company_id = c.id AND user_id = $1) AS is_following,
               c.created_at
        FROM companies c
        JOIN users u ON c.owner_id = u.id
        WHERE c.name ILIKE '%' || $2 || '%' OR EXISTS (SELECT 1 FROM unnest(c.search_tags) tag WHERE tag ILIKE '%' || $2 || '%')
        ORDER BY c.followers_count DESC, c.created_at DESC
        LIMIT 50
    `, userID, query)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	companies := make([]models.Company, 0)
	for rows.Next() {
		var company models.Company
		if err := scanCompanyBase(rows, &company); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if !company.IsPublic && !isCompanyMember(company.ID.String(), userID) && company.OwnerID != userID {
			company.Description = nil
			company.Website = nil
			company.Phone = nil
			company.Address = nil
		}
		companies = append(companies, company)
	}
	c.JSON(http.StatusOK, gin.H{"companies": companies})
}

func RequestJoinCompany(c *gin.Context) {
	companyID := c.Param("id")
	userID := currentUserID(c)
	var req struct {
		Message *string `json:"message"`
	}
	_ = c.ShouldBindJSON(&req)

	var isPublic bool
	if err := database.DB.QueryRow(`SELECT is_public FROM companies WHERE id = $1`, companyID).Scan(&isPublic); err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if isPublic {
		_, _ = database.DB.Exec(`INSERT INTO company_members (company_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, companyID, userID)
		c.JSON(http.StatusOK, gin.H{"message": "Company is public, access granted"})
		return
	}
	if isCompanyMember(companyID, userID) {
		jsonError(c, http.StatusBadRequest, "Already approved")
		return
	}
	requestID := uuid.New()
	if _, err := database.DB.Exec(`INSERT INTO company_join_requests (id, company_id, user_id, message) VALUES ($1, $2, $3, $4) ON CONFLICT (company_id, user_id) DO UPDATE SET status = 'pending', message = EXCLUDED.message, created_at = NOW()`, requestID, companyID, userID, req.Message); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create join request")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Join request sent"})
}

func GetCompanyJoinRequests(c *gin.Context) {
	companyID := c.Param("id")
	userID := currentUserID(c)
	allowed, err := requireCompanyPermission(companyID, userID, "manage_employees")
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Insufficient permissions to review requests")
		return
	}
	rows, err := database.DB.Query(`
        SELECT cjr.id, cjr.user_id, u.full_name, u.email, cjr.message, cjr.status, cjr.created_at
        FROM company_join_requests cjr
        JOIN users u ON u.id = cjr.user_id
        WHERE cjr.company_id = $1 AND cjr.status = 'pending'
        ORDER BY cjr.created_at ASC
    `, companyID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	requests := make([]map[string]interface{}, 0)
	for rows.Next() {
		var requestID, requestUserID uuid.UUID
		var fullName, email, status string
		var message sql.NullString
		var createdAt time.Time
		if err := rows.Scan(&requestID, &requestUserID, &fullName, &email, &message, &status, &createdAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		item := map[string]interface{}{"id": requestID, "user_id": requestUserID, "full_name": fullName, "email": email, "status": status, "created_at": createdAt}
		if message.Valid {
			item["message"] = message.String
		}
		requests = append(requests, item)
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func ApproveCompanyJoinRequest(c *gin.Context) {
	requestID := c.Param("request_id")
	ownerID := currentUserID(c)
	var companyID, requestUserID uuid.UUID
	err := database.DB.QueryRow(`SELECT company_id, user_id FROM company_join_requests WHERE id = $1 AND status = 'pending'`, requestID).Scan(&companyID, &requestUserID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Request not found")
		return
	}
	allowed, err := requireCompanyPermission(companyID.String(), ownerID, "manage_employees")
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Insufficient permissions to approve requests")
		return
	}
	_, _ = database.DB.Exec(`INSERT INTO company_members (company_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, companyID, requestUserID)
	_, _ = database.DB.Exec(`
		INSERT INTO company_user_roles (company_id, user_id, role_id, assigned_by)
		SELECT $1, $2, id, $3 FROM company_roles WHERE company_id = $1 AND role_code = 'member'
		ON CONFLICT (company_id, user_id) DO NOTHING
	`, companyID, requestUserID, ownerID)
	_, _ = database.DB.Exec(`UPDATE company_join_requests SET status = 'approved' WHERE id = $1`, requestID)
	c.JSON(http.StatusOK, gin.H{"message": "Request approved"})
}

func RejectCompanyJoinRequest(c *gin.Context) {
	requestID := c.Param("request_id")
	ownerID := currentUserID(c)
	var companyID uuid.UUID
	if err := database.DB.QueryRow(`SELECT company_id FROM company_join_requests WHERE id = $1 AND status = 'pending'`, requestID).Scan(&companyID); err != nil {
		jsonError(c, http.StatusNotFound, "Request not found")
		return
	}
	allowed, err := requireCompanyPermission(companyID.String(), ownerID, "manage_employees")
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Insufficient permissions to reject requests")
		return
	}
	_, _ = database.DB.Exec(`UPDATE company_join_requests SET status = 'rejected' WHERE id = $1`, requestID)
	c.JSON(http.StatusOK, gin.H{"message": "Request rejected"})
}

func GetPublicCompany(c *gin.Context) {
	companyID := c.Param("id")
	var company models.Company
	err := scanCompanyBase(database.DB.QueryRow(`
		SELECT c.id, c.owner_id, u.full_name, c.name, c.inn, c.description, c.logo_url,
		       c.economic_sector, c.is_public, c.search_tags, c.website, c.phone, c.address,
		       c.followers_count, c.employee_count, false AS is_following, c.created_at
		FROM companies c
		JOIN users u ON u.id = c.owner_id
		WHERE c.id = $1
	`, companyID), &company)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !company.IsPublic {
		jsonError(c, http.StatusForbidden, "Company profile is private")
		return
	}
	c.JSON(http.StatusOK, gin.H{"company": company})
}

func GetPublicCompanyNews(c *gin.Context) {
	companyID := c.Param("id")
	rows, err := database.DB.Query(`
		SELECT p.id, p.author_id, p.author_type, p.author_name, p.author_avatar, p.title, p.content,
		       p.short_description, p.image_url, p.tags, p.privacy_level, p.target_id,
		       p.is_hidden, p.is_unpublished, p.likes_count, p.comments_count, p.shares_count, p.created_at, p.updated_at, false AS is_liked
		FROM posts p
		WHERE p.author_type = 'company'
		  AND p.target_id = $1
		  AND p.privacy_level = 'public'
		  AND p.is_hidden = false
		  AND p.is_unpublished = false
		ORDER BY p.created_at DESC
		LIMIT 30
	`, companyID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var post models.Post
		var title, shortDesc, imageURL, targetID sql.NullString
		var tags pq.StringArray
		if err := rows.Scan(&post.ID, &post.AuthorID, &post.AuthorType, &post.AuthorName, &post.AuthorAvatar, &title, &post.Content, &shortDesc, &imageURL, &tags, &post.PrivacyLevel, &targetID, &post.IsHidden, &post.IsUnpublished, &post.LikesCount, &post.CommentsCount, &post.SharesCount, &post.CreatedAt, &post.UpdatedAt, &post.IsLiked); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if title.Valid {
			post.Title = &title.String
		}
		if shortDesc.Valid {
			post.ShortDescription = &shortDesc.String
		}
		if imageURL.Valid {
			post.ImageURL = &imageURL.String
		}
		if targetID.Valid {
			if parsed, parseErr := uuid.Parse(targetID.String); parseErr == nil {
				post.TargetID = &parsed
			}
		}
		post.Tags = tags
		posts = append(posts, post)
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}
