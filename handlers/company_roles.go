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

func GetCompanyRoles(c *gin.Context) {
	companyID := c.Param("id")
	rows, err := database.DB.Query(`SELECT id, position_name, responsibilities, permissions FROM company_roles WHERE company_id = $1 ORDER BY position_name`, companyID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	roles := make([]models.CompanyRole, 0)
	for rows.Next() {
		var role models.CompanyRole
		var responsibilities, permissions pq.StringArray
		if err := rows.Scan(&role.ID, &role.PositionName, &responsibilities, &permissions); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		role.Responsibilities = responsibilities
		role.Permissions = permissions
		roles = append(roles, role)
	}
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func CreateCompanyRole(c *gin.Context) {
	companyID := c.Param("id")
	userID := currentUserID(c)
	allowed, err := requireCompanyOwner(companyID, userID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can manage roles")
		return
	}

	var req struct {
		PositionName     string   `json:"position_name" binding:"required"`
		Responsibilities []string `json:"responsibilities"`
		Permissions      []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	roleID := uuid.New()
	_, err = database.DB.Exec(`INSERT INTO company_roles (id, company_id, position_name, responsibilities, permissions) VALUES ($1, $2, $3, $4, $5)`, roleID, companyID, req.PositionName, pq.Array(req.Responsibilities), pq.Array(req.Permissions))
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create role")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Role created", "role_id": roleID})
}

func GetCompanyEmployees(c *gin.Context) {
	companyID := c.Param("id")
	userID := currentUserID(c)
	var isPublic bool
	if err := database.DB.QueryRow(`SELECT is_public FROM companies WHERE id = $1`, companyID).Scan(&isPublic); err == nil && !isPublic && !isCompanyMember(companyID, userID) {
		jsonError(c, http.StatusForbidden, "Employees are hidden for private company")
		return
	}
	rows, err := database.DB.Query(`
        SELECT u.id, u.full_name, u.email, u.avatar_url, ce.position_name, ce.department, ce.hire_date::text, ce.is_active, ce.assigned_at
        FROM company_employees ce
        JOIN users u ON ce.user_id = u.id
        WHERE ce.company_id = $1 AND ce.is_active = true
        ORDER BY ce.assigned_at DESC
    `, companyID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	employees := make([]models.CompanyEmployee, 0)
	for rows.Next() {
		var emp models.CompanyEmployee
		var avatar, department, hireDate sql.NullString
		if err := rows.Scan(&emp.UserID, &emp.UserName, &emp.UserEmail, &avatar, &emp.PositionName, &department, &hireDate, &emp.IsActive, &emp.AssignedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if avatar.Valid {
			emp.UserAvatar = &avatar.String
		}
		if department.Valid {
			emp.Department = &department.String
		}
		if hireDate.Valid {
			emp.HireDate = &hireDate.String
		}
		employees = append(employees, emp)
	}
	c.JSON(http.StatusOK, gin.H{"employees": employees})
}

func InviteToCompany(c *gin.Context) {
	companyID := c.Param("id")
	inviterID := currentUserID(c)
	var req models.InviteToCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	allowed, err := requireCompanyOwner(companyID, inviterID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can invite")
		return
	}
	if !isAcceptedFriend(inviterID, req.UserID) {
		jsonError(c, http.StatusForbidden, "You can invite only users from friends list")
		return
	}

	var isEmployee bool
	_ = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM company_employees WHERE company_id = $1 AND user_id = $2 AND is_active = true)`, companyID, req.UserID).Scan(&isEmployee)
	if isEmployee {
		jsonError(c, http.StatusBadRequest, "User is already an employee")
		return
	}

	var profileID uuid.UUID
	err = database.DB.QueryRow(`
		SELECT id
		FROM corporate_profiles
		WHERE user_id = $1 AND company_id = $2 AND created_by = $3 AND status = 'pending'
	`, req.UserID, companyID, inviterID).Scan(&profileID)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusBadRequest, "Create employee corporate profile before invitation")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to validate employee corporate profile")
		return
	}

	inviteID := uuid.New()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = database.DB.Exec(`
		INSERT INTO company_invites (id, company_id, inviter_id, invitee_id, position_name, department, corporate_profile_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, inviteID, companyID, inviterID, req.UserID, req.PositionName, req.Department, profileID, expiresAt)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to send invite")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Invite sent"})
}

func CreateEmployeeCorporateProfile(c *gin.Context) {
	companyID := c.Param("id")
	creatorID := currentUserID(c)
	var req models.CreateEmployeeCorporateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	allowed, err := requireCompanyOwner(companyID, creatorID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can create employee corporate profiles")
		return
	}
	if !isAcceptedFriend(creatorID, req.UserID) {
		jsonError(c, http.StatusForbidden, "You can create employee profiles only for friends")
		return
	}

	profileID := uuid.New()
	_, err = database.DB.Exec(`
		INSERT INTO corporate_profiles (id, user_id, company_id, created_by, position_name, permissions, status, employment_status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', 'invited')
		ON CONFLICT (user_id) DO UPDATE
		SET company_id = EXCLUDED.company_id,
			created_by = EXCLUDED.created_by,
			position_name = EXCLUDED.position_name,
			permissions = EXCLUDED.permissions,
			status = 'pending',
			employment_status = 'invited',
			updated_at = NOW()
	`, profileID, req.UserID, companyID, creatorID, req.PositionName, pq.Array(req.Permissions))
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create employee corporate profile")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Employee corporate profile created"})
}

func AcceptCompanyInvite(c *gin.Context) {
	inviteID := c.Param("invite_id")
	userID := currentUserID(c)

	var companyID uuid.UUID
	var positionName string
	var department sql.NullString
	var expiresAt time.Time
	var corporateProfileID uuid.NullUUID
	err := database.DB.QueryRow(`
		SELECT company_id, position_name, department, expires_at, corporate_profile_id
		FROM company_invites
		WHERE id = $1 AND invitee_id = $2 AND status = 'pending'
	`, inviteID, userID).Scan(&companyID, &positionName, &department, &expiresAt, &corporateProfileID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Invite not found")
		return
	}
	if expiresAt.Before(time.Now()) {
		_, _ = database.DB.Exec(`UPDATE company_invites SET status = 'expired' WHERE id = $1`, inviteID)
		jsonError(c, http.StatusBadRequest, "Invite expired")
		return
	}

	_, err = database.DB.Exec(`
        INSERT INTO company_employees (company_id, user_id, position_name, department, assigned_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (company_id, user_id) DO UPDATE SET position_name = EXCLUDED.position_name, department = EXCLUDED.department, is_active = true, assigned_at = NOW()
    `, companyID, userID, positionName, department)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to accept invite")
		return
	}
	_, _ = database.DB.Exec(`UPDATE company_invites SET status = 'accepted' WHERE id = $1`, inviteID)
	if corporateProfileID.Valid {
		_, _ = database.DB.Exec(`
			UPDATE corporate_profiles
			SET status = 'active',
				employment_status = 'employed',
				company_id = $1,
				updated_at = NOW()
			WHERE id = $2 AND user_id = $3
		`, companyID, corporateProfileID.UUID, userID)
	}
	recalcCompanyEmployeesCount(companyID.String())
	c.JSON(http.StatusOK, gin.H{"message": "You are now an employee"})
}

func UpdateEmployeeRole(c *gin.Context) {
	companyID := c.Param("id")
	employeeUserID := c.Param("user_id")
	ownerID := currentUserID(c)
	allowed, err := requireCompanyOwner(companyID, ownerID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can update roles")
		return
	}

	var req struct {
		PositionName string  `json:"position_name" binding:"required"`
		Department   *string `json:"department"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := database.DB.Exec(`UPDATE company_employees SET position_name = $1, department = $2 WHERE company_id = $3 AND user_id = $4`, req.PositionName, req.Department, companyID, employeeUserID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update employee")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Employee role updated"})
}

func RemoveEmployee(c *gin.Context) {
	companyID := c.Param("id")
	employeeUserID := c.Param("user_id")
	ownerID := currentUserID(c)
	allowed, err := requireCompanyOwner(companyID, ownerID)
	if err != nil {
		jsonError(c, http.StatusNotFound, "Company not found")
		return
	}
	if !allowed {
		jsonError(c, http.StatusForbidden, "Only company owner can remove employees")
		return
	}
	if ownerID.String() == employeeUserID {
		jsonError(c, http.StatusBadRequest, "Cannot remove company owner")
		return
	}
	if _, err := database.DB.Exec(`UPDATE company_employees SET is_active = false WHERE company_id = $1 AND user_id = $2`, companyID, employeeUserID); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to remove employee")
		return
	}
	recalcCompanyEmployeesCount(companyID)
	c.JSON(http.StatusOK, gin.H{"message": "Employee removed"})
}
