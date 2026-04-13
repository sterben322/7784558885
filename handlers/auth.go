package handlers

import (
	"database/sql"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"lastop/database"
	"lastop/models"
	"lastop/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)
	displayName := strings.TrimSpace(req.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(req.FullName)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))
	}

	if req.Email == "" {
		jsonError(c, http.StatusBadRequest, "email is required")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		jsonError(c, http.StatusBadRequest, "email format is invalid")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		jsonError(c, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		jsonError(c, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if firstName == "" && displayName == "" {
		jsonError(c, http.StatusBadRequest, "name is required")
		return
	}

	var exists bool
	if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, req.Email).Scan(&exists); err != nil {
		jsonError(c, http.StatusInternalServerError, "Database error")
		return
	}
	if exists {
		jsonError(c, http.StatusConflict, "User already exists")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	userID := uuid.New()
	if displayName == "" {
		displayName = req.Email
	}

	createdAt := time.Now().UTC()
	_, err = database.DB.Exec(`
		INSERT INTO users (id, first_name, last_name, full_name, name, email, password_hash, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
	`, userID, firstName, lastName, displayName, nullIfEmpty(displayName), req.Email, string(hashedPassword), nullIfEmpty(req.AvatarURL), createdAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			jsonError(c, http.StatusConflict, "User with this email already exists")
			return
		}
		jsonError(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	user := models.User{
		ID:        userID,
		FirstName: firstName,
		LastName:  lastName,
		FullName:  displayName,
		Email:     req.Email,
		AvatarURL: nullIfEmpty(req.AvatarURL),
		CreatedAt: createdAt,
	}
	token, err := createSessionToken(user.ID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create session")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"token":   token,
		"user":    user,
	})
}

func Login(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)
	if req.Email == "" || req.Password == "" {
		jsonError(c, http.StatusBadRequest, "email and password are required")
		return
	}

	var user models.User
	var passwordHash string
	err := database.DB.QueryRow(`
        SELECT id, first_name, last_name, full_name, email, company_name, phone, position, avatar_url, created_at, password_hash
        FROM users WHERE email = $1
    `, req.Email).Scan(&user.ID, &user.FirstName, &user.LastName, &user.FullName, &user.Email, &user.CompanyName, &user.Phone, &user.Position, &user.AvatarURL, &user.CreatedAt, &passwordHash)
	if err == sql.ErrNoRows {
		jsonError(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Database error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		jsonError(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	token, err := createSessionToken(user.ID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create session")
		return
	}

	c.JSON(http.StatusOK, models.LoginResponse{Token: token, User: user})
}

func createSessionToken(userID uuid.UUID) (string, error) {
	token, err := utils.GenerateJWT(userID)
	if err != nil {
		return "", err
	}

	sessionID := uuid.New()
	expiresAt := time.Now().Add(24 * time.Hour)
	if _, err := database.DB.Exec(`INSERT INTO sessions (id, user_id, token, expires_at) VALUES ($1, $2, $3, $4)`, sessionID, userID, token, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func nullIfEmpty(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	value := strings.TrimSpace(v)
	return &value
}

func Logout(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	token := currentToken(c)
	if token == "" {
		jsonError(c, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if _, err := database.DB.Exec(`DELETE FROM sessions WHERE token = $1`, token); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to logout")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func GetMe(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	var user models.User
	err := database.DB.QueryRow(`
        SELECT id, first_name, last_name, full_name, email, company_name, phone, position, avatar_url, created_at
        FROM users WHERE id = $1
    `, userID).Scan(&user.ID, &user.FirstName, &user.LastName, &user.FullName, &user.Email, &user.CompanyName, &user.Phone, &user.Position, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		jsonError(c, http.StatusNotFound, "User not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func UpdateProfile(c *gin.Context) {
	if !ensureDatabase(c) {
		return
	}

	userID := currentUserID(c)
	var req struct {
		FirstName   string  `json:"first_name"`
		LastName    string  `json:"last_name"`
		FullName    string  `json:"full_name"`
		CompanyName *string `json:"company_name"`
		Phone       *string `json:"phone"`
		Position    *string `json:"position"`
		AvatarURL   *string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.FullName = strings.TrimSpace(req.FullName)
	if req.FirstName == "" || req.LastName == "" {
		jsonError(c, http.StatusBadRequest, "first_name and last_name are required")
		return
	}
	if req.FullName == "" {
		req.FullName = strings.TrimSpace(strings.Join([]string{req.FirstName, req.LastName}, " "))
	}
	if req.FullName == "" {
		jsonError(c, http.StatusBadRequest, "full_name is required")
		return
	}
	_, err := database.DB.Exec(`
        UPDATE users
        SET first_name = $1, last_name = $2, full_name = $3, company_name = $4, phone = $5, position = $6, avatar_url = $7, updated_at = NOW()
        WHERE id = $8
    `, req.FirstName, req.LastName, req.FullName, req.CompanyName, req.Phone, req.Position, req.AvatarURL, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}
