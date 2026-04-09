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

func CreateOrUpdateResume(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.PreviousWorkplaces) > 20 {
		jsonError(c, http.StatusBadRequest, "Можно добавить не более 20 прошлых мест работы")
		return
	}
	_, err := database.DB.Exec(`
        INSERT INTO resumes (user_id, title, about, activity_type, skills, education_levels, previous_workplaces, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
        ON CONFLICT (user_id) DO UPDATE
        SET title = EXCLUDED.title,
            about = EXCLUDED.about,
            activity_type = EXCLUDED.activity_type,
            skills = EXCLUDED.skills,
            education_levels = EXCLUDED.education_levels,
            previous_workplaces = EXCLUDED.previous_workplaces,
            updated_at = NOW()
    `, userID, req.Title, req.About, req.ActivityType, pq.Array(req.Skills), pq.Array(req.EducationLevels), pq.Array(req.PreviousWorkplaces))
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to save resume")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Resume saved"})
}

func GetMyResume(c *gin.Context) {
	userID := currentUserID(c)
	var resume models.Resume
	var about, activityType sql.NullString
	var skills, educationLevels, previousWorkplaces pq.StringArray
	err := database.DB.QueryRow(`
        SELECT id, user_id, title, about, activity_type, skills, education_levels, previous_workplaces, created_at, updated_at
        FROM resumes
        WHERE user_id = $1
    `, userID).Scan(&resume.ID, &resume.UserID, &resume.Title, &about, &activityType, &skills, &educationLevels, &previousWorkplaces, &resume.CreatedAt, &resume.UpdatedAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusOK, gin.H{"resume": nil})
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if about.Valid {
		resume.About = &about.String
	}
	if activityType.Valid {
		resume.ActivityType = &activityType.String
	}
	resume.Skills = skills
	resume.EducationLevels = educationLevels
	resume.PreviousWorkplaces = previousWorkplaces
	c.JSON(http.StatusOK, gin.H{"resume": resume})
}

func GetResumes(c *gin.Context) {
	rows, err := database.DB.Query(`
        SELECT id, user_id, title, about, activity_type, skills, education_levels, previous_workplaces, created_at, updated_at
        FROM resumes
        ORDER BY updated_at DESC
        LIMIT 100
    `)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	resumes := make([]models.Resume, 0)
	for rows.Next() {
		var resume models.Resume
		var about, activityType sql.NullString
		var skills, educationLevels, previousWorkplaces pq.StringArray
		if err := rows.Scan(&resume.ID, &resume.UserID, &resume.Title, &about, &activityType, &skills, &educationLevels, &previousWorkplaces, &resume.CreatedAt, &resume.UpdatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if about.Valid {
			resume.About = &about.String
		}
		if activityType.Valid {
			resume.ActivityType = &activityType.String
		}
		resume.Skills = skills
		resume.EducationLevels = educationLevels
		resume.PreviousWorkplaces = previousWorkplaces
		resumes = append(resumes, resume)
	}
	c.JSON(http.StatusOK, gin.H{"resumes": resumes})
}

func CreateVacancy(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateVacancyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	publisherID, err := uuid.Parse(req.PublisherID)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid publisher_id")
		return
	}

	publisherName := ""
	switch req.PublisherType {
	case "community":
		allowed, err := requireCommunityOwner(publisherID.String(), userID)
		if err != nil || !allowed {
			jsonError(c, http.StatusForbidden, "Vacancies can only be created by community owners")
			return
		}
		if err := database.DB.QueryRow(`SELECT name FROM communities WHERE id = $1`, publisherID).Scan(&publisherName); err != nil {
			jsonError(c, http.StatusNotFound, "Community not found")
			return
		}
	case "company":
		allowed, err := requireCompanyOwner(publisherID.String(), userID)
		if err != nil || !allowed {
			jsonError(c, http.StatusForbidden, "Vacancies can only be created by company owners")
			return
		}
		if err := database.DB.QueryRow(`SELECT name FROM companies WHERE id = $1`, publisherID).Scan(&publisherName); err != nil {
			jsonError(c, http.StatusNotFound, "Company not found")
			return
		}
	default:
		jsonError(c, http.StatusBadRequest, "publisher_type must be company or community")
		return
	}

	vacancyID := uuid.New()
	_, err = database.DB.Exec(`
        INSERT INTO vacancies (id, publisher_type, publisher_id, publisher_name, position, salary, expectations, required_skills, required_experience, employment_type, location, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
    `, vacancyID, req.PublisherType, publisherID, publisherName, req.Position, req.Salary, req.Expectations, pq.Array(req.RequiredSkills), req.RequiredExperience, req.EmploymentType, req.Location)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create vacancy")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Vacancy created", "vacancy_id": vacancyID})
}

func GetVacancies(c *gin.Context) {
	rows, err := database.DB.Query(`
        SELECT id, publisher_type, publisher_id, publisher_name, position, salary, expectations, required_skills, required_experience, employment_type, location, created_at, updated_at
        FROM vacancies
        ORDER BY created_at DESC
        LIMIT 100
    `)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	vacancies := make([]models.Vacancy, 0)
	for rows.Next() {
		var vacancy models.Vacancy
		var requiredExperience, employmentType, location sql.NullString
		var requiredSkills pq.StringArray
		if err := rows.Scan(&vacancy.ID, &vacancy.PublisherType, &vacancy.PublisherID, &vacancy.PublisherName, &vacancy.Position, &vacancy.Salary, &vacancy.Expectations, &requiredSkills, &requiredExperience, &employmentType, &location, &vacancy.CreatedAt, &vacancy.UpdatedAt); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		vacancy.RequiredSkills = requiredSkills
		if requiredExperience.Valid {
			vacancy.RequiredExperience = &requiredExperience.String
		}
		if employmentType.Valid {
			vacancy.EmploymentType = &employmentType.String
		}
		if location.Valid {
			vacancy.Location = &location.String
		}
		vacancies = append(vacancies, vacancy)
	}
	c.JSON(http.StatusOK, gin.H{"vacancies": vacancies, "server_time": time.Now().UTC()})
}
