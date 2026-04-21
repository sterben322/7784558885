package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type createJobRequest struct {
	Title        string   `json:"title"`
	Company      string   `json:"company"`
	Category     string   `json:"category"`
	City         string   `json:"city"`
	WorkFormat   string   `json:"work_format"`
	SalaryFrom   int      `json:"salary_from"`
	SalaryTo     int      `json:"salary_to"`
	Description  string   `json:"description"`
	Requirements string   `json:"requirements"`
	Tags         []string `json:"tags"`
	Status       string   `json:"status"`
}

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

// CreateJob is a frontend-compatible endpoint used by jobs.html.
func CreateJob(c *gin.Context) {
	userID := currentUserID(c)
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		jsonError(c, http.StatusBadRequest, "title is required")
		return
	}

	var publisherName string
	if err := database.DB.QueryRow(`SELECT COALESCE(NULLIF(full_name, ''), email) FROM users WHERE id = $1`, userID).Scan(&publisherName); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to resolve publisher")
		return
	}

	salary := "По договоренности"
	if req.SalaryFrom > 0 || req.SalaryTo > 0 {
		switch {
		case req.SalaryFrom > 0 && req.SalaryTo > 0:
			salary = strconv.Itoa(req.SalaryFrom) + "-" + strconv.Itoa(req.SalaryTo)
		case req.SalaryFrom > 0:
			salary = "от " + strconv.Itoa(req.SalaryFrom)
		case req.SalaryTo > 0:
			salary = "до " + strconv.Itoa(req.SalaryTo)
		}
	}

	vacancyID := uuid.New()
	_, err := database.DB.Exec(`
		INSERT INTO vacancies (
			id, publisher_type, publisher_id, publisher_name, position, salary, expectations, required_skills,
			employment_type, location, creator_id, category, city, work_format, salary_from, salary_to,
			description, requirements, tags, status, views_count, responses_count, created_at, updated_at
		) VALUES (
			$1, 'user', $2, $3, $4, $5, '', $6,
			NULLIF($7, ''), NULLIF($8, ''), $2, NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''),
			$12, $13, NULLIF($14, ''), NULLIF($15, ''), $16, $17, 0, 0, NOW(), NOW()
		)
	`, vacancyID, userID, publisherName, title, salary, pq.Array(req.Tags), strings.TrimSpace(req.WorkFormat), strings.TrimSpace(req.City),
		strings.TrimSpace(req.Category), strings.TrimSpace(req.City), strings.TrimSpace(req.WorkFormat),
		req.SalaryFrom, req.SalaryTo, strings.TrimSpace(req.Description), strings.TrimSpace(req.Requirements),
		pq.Array(req.Tags), strings.TrimSpace(req.Status))
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create job")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"job_id": vacancyID})
}

func GetJobs(c *gin.Context) {
	userID := currentUserID(c)
	limit := 30
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > 100 {
				parsed = 100
			}
			limit = parsed
		}
	}
	q := strings.TrimSpace(c.Query("q"))
	category := strings.TrimSpace(c.Query("category"))

	rows, err := database.DB.Query(`
		SELECT v.id, v.position, v.publisher_name, COALESCE(NULLIF(v.city, ''), NULLIF(v.location, '')) AS city,
			COALESCE(NULLIF(v.work_format, ''), NULLIF(v.employment_type, ''), '') AS work_format,
			COALESCE(v.salary_from, 0), COALESCE(v.salary_to, 0),
			COALESCE(NULLIF(v.description, ''), v.expectations, '') AS description,
			COALESCE(v.requirements, '') AS requirements,
			COALESCE(v.tags, ARRAY[]::TEXT[]), COALESCE(NULLIF(v.status, ''), 'active') AS status,
			COALESCE(v.views_count, 0), COALESCE(v.responses_count, 0),
			v.created_at,
			EXISTS(SELECT 1 FROM job_responses jr WHERE jr.job_id = v.id AND jr.user_id = $1) AS is_responded
		FROM vacancies v
		WHERE
			($2 = '' OR v.position ILIKE '%' || $2 || '%' OR COALESCE(v.description, '') ILIKE '%' || $2 || '%' OR v.publisher_name ILIKE '%' || $2 || '%')
			AND ($3 = '' OR COALESCE(v.category, '') = $3)
		ORDER BY v.created_at DESC
		LIMIT $4
	`, userID, q, category, limit)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load jobs")
		return
	}
	defer rows.Close()

	jobs := make([]gin.H, 0)
	for rows.Next() {
		var (
			id                                     uuid.UUID
			title, company, city, workFormat       string
			description, requirements, status      string
			salaryFrom, salaryTo, views, responses int
			createdAt                              time.Time
			tags                                   pq.StringArray
			isResponded                            bool
		)
		if err := rows.Scan(&id, &title, &company, &city, &workFormat, &salaryFrom, &salaryTo, &description, &requirements, &tags, &status, &views, &responses, &createdAt, &isResponded); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse jobs")
			return
		}
		jobs = append(jobs, gin.H{
			"id":              id,
			"title":           title,
			"company":         company,
			"city":            city,
			"work_format":     workFormat,
			"salary_from":     salaryFrom,
			"salary_to":       salaryTo,
			"description":     description,
			"requirements":    requirements,
			"tags":            []string(tags),
			"status":          status,
			"views_count":     views,
			"responses_count": responses,
			"created_at":      createdAt,
			"is_responded":    isResponded,
		})
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func GetMyJobs(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT id, position, publisher_name, COALESCE(city, location, ''), COALESCE(status, 'active'), COALESCE(responses_count, 0), created_at
		FROM vacancies
		WHERE creator_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load my jobs")
		return
	}
	defer rows.Close()
	jobs := make([]gin.H, 0)
	for rows.Next() {
		var id uuid.UUID
		var title, company, city, status string
		var responses int
		var createdAt time.Time
		if err := rows.Scan(&id, &title, &company, &city, &status, &responses, &createdAt); err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to parse my jobs")
			return
		}
		jobs = append(jobs, gin.H{"id": id, "title": title, "company": company, "city": city, "status": status, "responses_count": responses, "created_at": createdAt})
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func RespondJob(c *gin.Context) {
	userID := currentUserID(c)
	jobID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid job id")
		return
	}
	_, err = database.DB.Exec(`
		INSERT INTO job_responses (job_id, user_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (job_id, user_id) DO NOTHING
	`, jobID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to respond job")
		return
	}
	_, _ = database.DB.Exec(`UPDATE vacancies SET responses_count = (SELECT COUNT(*) FROM job_responses WHERE job_id = $1), updated_at = NOW() WHERE id = $1`, jobID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func GetJobsStats(c *gin.Context) {
	var stats struct {
		Total    int
		Resumes  int
		NewToday int
		Active   int
	}
	if err := database.DB.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM vacancies),
			(SELECT COUNT(*) FROM resumes),
			(SELECT COUNT(*) FROM vacancies WHERE created_at::date = CURRENT_DATE),
			(SELECT COUNT(*) FROM vacancies WHERE COALESCE(status, 'active') <> 'closed')
	`).Scan(&stats.Total, &stats.Resumes, &stats.NewToday, &stats.Active); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load jobs stats")
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": gin.H{"total": stats.Total, "resumes_count": stats.Resumes, "new_today": stats.NewToday, "active": stats.Active}})
}

func GetResumesStats(c *gin.Context) {
	var stats struct {
		Total    int
		Jobs     int
		NewToday int
		Active   int
	}
	if err := database.DB.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM resumes),
			(SELECT COUNT(*) FROM vacancies),
			(SELECT COUNT(*) FROM resumes WHERE updated_at::date = CURRENT_DATE),
			(SELECT COUNT(*) FROM resumes)
	`).Scan(&stats.Total, &stats.Jobs, &stats.NewToday, &stats.Active); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to load resume stats")
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": gin.H{"total": stats.Total, "jobs_count": stats.Jobs, "new_today": stats.NewToday, "active": stats.Active}})
}
