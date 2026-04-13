package handlers

import (
	"database/sql"
	"net/http"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func scanCorporateProfile(rowScanner interface{ Scan(dest ...any) error }, profile *models.CorporateProfile) error {
	var companyID, createdBy uuid.NullUUID
	var positionName sql.NullString
	var permissions pq.StringArray
	if err := rowScanner.Scan(
		&profile.ID,
		&profile.UserID,
		&companyID,
		&createdBy,
		&positionName,
		&permissions,
		&profile.Status,
		&profile.EmploymentStatus,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		return err
	}
	profile.Permissions = permissions
	if companyID.Valid {
		profile.CompanyID = &companyID.UUID
	}
	if createdBy.Valid {
		profile.CreatedBy = &createdBy.UUID
	}
	if positionName.Valid {
		profile.PositionName = &positionName.String
	}
	return nil
}

func GetMyCorporateProfile(c *gin.Context) {
	userID := currentUserID(c)
	var profile models.CorporateProfile
	err := scanCorporateProfile(database.DB.QueryRow(`
		SELECT id, user_id, company_id, created_by, position_name, permissions, status, employment_status, created_at, updated_at
		FROM corporate_profiles
		WHERE user_id = $1
	`, userID), &profile)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"corporate_profile": nil})
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"corporate_profile": profile})
}

func CreateCorporateProfile(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateCorporateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	profileID := uuid.New()
	_, err := database.DB.Exec(`
		INSERT INTO corporate_profiles (id, user_id, position_name, permissions, status, employment_status)
		VALUES ($1, $2, $3, ARRAY[]::TEXT[], 'active', 'independent')
		ON CONFLICT (user_id) DO UPDATE
		SET position_name = COALESCE(EXCLUDED.position_name, corporate_profiles.position_name),
			updated_at = NOW()
	`, profileID, userID, req.PositionName)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create corporate profile")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Corporate profile created"})
}
