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

	queryArg := any(userID)
	if targetOwnerID != "" {
		queryArg = targetOwnerID
	}

	var company models.Company
	err := scanCompanyBase(database.DB.QueryRow(`
        SELECT c.id, c.owner_id, u.full_name, c.name, c.inn, c.description, c.logo_url,
               c.economic_sector, c.is_public, c.search_tags, c.website, c.phone, c.address,
               c.followers_count, c.employee_count,
               EXISTS(SELECT 1 FROM company_followers WHERE company_id = c.id AND user_id = $1) AS is_following,
               c.created_at
        FROM companies c
        JOIN users u ON c.owner_id = u.id
        WHERE c.owner_id = $2
    `, userID, queryArg), &company)
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
	_, err := database.DB.Exec(`
        INSERT INTO companies (id, owner_id, name, inn, description, logo_url, economic_sector, is_public, search_tags, website, phone, address)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, companyID, userID, req.Name, req.INN, req.Description, req.LogoURL, req.EconomicSector, req.IsPublic, pq.Array(req.SearchTags), req.Website, req.Phone, req.Address)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create company")
		return
	}
	_, _ = database.DB.Exec(`INSERT INTO company_members (company_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, companyID, userID)
	c.JSON(http.StatusCreated, gin.H{"message": "Company created successfully", "company_id": companyID})
}

func UpdateCompany(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err := database.DB.Exec(`
        UPDATE companies
        SET name = $1, inn = $2, description = $3, logo_url = $4, economic_sector = $5,
            is_public = $6, search_tags = $7, website = $8, phone = $9, address = $10, updated_at = NOW()
        WHERE owner_id = $11
    `, req.Name, req.INN, req.Description, req.LogoURL, req.EconomicSector, req.IsPublic, pq.Array(req.SearchTags), req.Website, req.Phone, req.Address, userID)
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
	allowed, err := requireCompanyOwner(companyID, userID)
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can review requests")
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
	allowed, err := requireCompanyOwner(companyID.String(), ownerID)
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can approve requests")
		return
	}
	_, _ = database.DB.Exec(`INSERT INTO company_members (company_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, companyID, requestUserID)
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
	allowed, err := requireCompanyOwner(companyID.String(), ownerID)
	if err != nil || !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can reject requests")
		return
	}
	_, _ = database.DB.Exec(`UPDATE company_join_requests SET status = 'rejected' WHERE id = $1`, requestID)
	c.JSON(http.StatusOK, gin.H{"message": "Request rejected"})
}
