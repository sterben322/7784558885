package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"lastop/database"
	"lastop/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func CreateProject(c *gin.Context) {
	userID := currentUserID(c)
	var req models.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		jsonError(c, http.StatusBadRequest, "title is required")
		return
	}

	authorType := strings.TrimSpace(strings.ToLower(req.AuthorType))
	if authorType == "" {
		authorType = "user"
	}
	if authorType != "user" && authorType != "company" && authorType != "community" {
		jsonError(c, http.StatusBadRequest, "author_type must be user, company, or community")
		return
	}

	authorID, authorName, err := resolveProjectAuthor(c, userID, authorType, req.AuthorID)
	if err != nil {
		jsonError(c, http.StatusForbidden, err.Error())
		return
	}

	projectID := uuid.New()
	tags := sanitizeStringSlice(req.Tags, 20)
	goals := sanitizeStringSlice(req.Goals, 20)
	images := sanitizeStringSlice(req.Images, 8)
	city := strings.TrimSpace(req.City)
	category := strings.TrimSpace(req.Category)
	description := strings.TrimSpace(req.Description)
	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		goal = "partner"
	}

	tx, err := database.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create project")
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO projects (
			id, creator_id, author_type, author_id, author_name, title, category, goal, city,
			tags, goals, description, images, status, progress, views_count, responses_count, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, NULLIF($9, ''),
			$10, $11, NULLIF($12, ''), $13, 'active', NULL, 0, 0, NOW(), NOW()
		)
	`, projectID, userID, authorType, authorID, authorName, title, category, goal, city, pq.Array(tags), pq.Array(goals), description, pq.Array(images))
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create project")
		return
	}

	for i, need := range req.Needs {
		name := strings.TrimSpace(need.Name)
		if name == "" {
			continue
		}
		amountText := strings.TrimSpace(need.AmountText)
		status := strings.TrimSpace(strings.ToLower(need.Status))
		if status == "" {
			status = "open"
		}
		_, err = tx.Exec(`
			INSERT INTO project_needs (id, project_id, name, amount_text, status, sort_order, created_at)
			VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NOW())
		`, uuid.New(), projectID, name, amountText, status, i)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, "Failed to create project")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to create project")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Project created", "project_id": projectID})
}

func GetProjects(c *gin.Context) {
	userID := currentUserID(c)
	ownerKind := strings.TrimSpace(strings.ToLower(c.DefaultQuery("owner_kind", "company")))
	goal := strings.TrimSpace(strings.ToLower(c.Query("goal")))
	q := strings.TrimSpace(c.Query("q"))
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

	where := []string{"1=1"}
	args := []interface{}{}
	idx := 1

	switch ownerKind {
	case "company":
		where = append(where, "p.author_type = 'company'")
	case "user":
		where = append(where, "p.author_type IN ('user', 'community')")
	default:
		where = append(where, "p.author_type IN ('company', 'user', 'community')")
	}

	if goal != "" {
		where = append(where, "p.goal = $"+strconv.Itoa(idx))
		args = append(args, goal)
		idx++
	}
	if q != "" {
		where = append(where, "(p.title ILIKE $"+strconv.Itoa(idx)+" OR COALESCE(p.description, '') ILIKE $"+strconv.Itoa(idx)+" OR p.author_name ILIKE $"+strconv.Itoa(idx)+")")
		args = append(args, "%"+q+"%")
		idx++
	}

	args = append(args, userID, limit)
	respondedPos := idx
	limitPos := idx + 1

	query := `
		SELECT
			p.id, p.author_type, p.author_id, p.author_name, p.title, p.category, p.goal,
			p.city, p.tags, p.goals, p.description, p.images, p.status, p.progress,
			p.views_count, p.responses_count, p.created_at, p.updated_at,
			EXISTS(
				SELECT 1 FROM project_responses pr
				WHERE pr.project_id = p.id AND pr.user_id = $` + strconv.Itoa(respondedPos) + `
			) AS is_responded
		FROM projects p
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY p.created_at DESC
		LIMIT $` + strconv.Itoa(limitPos)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	projectIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var p models.Project
		var tags, goals, images pq.StringArray
		var authorID, category, city, description, status sql.NullString
		var progress sql.NullInt64
		if err := rows.Scan(
			&p.ID, &p.AuthorType, &authorID, &p.AuthorName, &p.Title, &category, &p.Goal,
			&city, &tags, &goals, &description, &images, &status, &progress,
			&p.ViewsCount, &p.ResponsesCount, &p.CreatedAt, &p.UpdatedAt, &p.IsResponded,
		); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if authorID.Valid {
			parsed, err := uuid.Parse(authorID.String)
			if err == nil {
				p.AuthorID = &parsed
			}
		}
		if category.Valid {
			p.Category = &category.String
		}
		if city.Valid {
			p.City = &city.String
		}
		if description.Valid {
			p.Description = &description.String
		}
		if status.Valid {
			p.Status = &status.String
		}
		if progress.Valid {
			val := int(progress.Int64)
			p.Progress = &val
		}
		p.Tags = tags
		p.Goals = goals
		p.Images = images
		projects = append(projects, p)
		projectIDs = append(projectIDs, p.ID)
	}

	needsMap, err := fetchProjectNeeds(projectIDs)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range projects {
		projects[i].Needs = needsMap[projects[i].ID]
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func GetMyProjects(c *gin.Context) {
	userID := currentUserID(c)
	rows, err := database.DB.Query(`
		SELECT id, author_type, author_id, author_name, title, category, goal, city, tags, goals, description,
			images, status, progress, views_count, responses_count, created_at, updated_at
		FROM projects
		WHERE creator_id = $1
		ORDER BY updated_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	projects := make([]models.Project, 0)
	projectIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var p models.Project
		var tags, goals, images pq.StringArray
		var authorID, category, city, description, status sql.NullString
		var progress sql.NullInt64
		if err := rows.Scan(
			&p.ID, &p.AuthorType, &authorID, &p.AuthorName, &p.Title, &category, &p.Goal, &city,
			&tags, &goals, &description, &images, &status, &progress,
			&p.ViewsCount, &p.ResponsesCount, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		if authorID.Valid {
			if parsed, err := uuid.Parse(authorID.String); err == nil {
				p.AuthorID = &parsed
			}
		}
		if category.Valid {
			p.Category = &category.String
		}
		if city.Valid {
			p.City = &city.String
		}
		if description.Valid {
			p.Description = &description.String
		}
		if status.Valid {
			p.Status = &status.String
		}
		if progress.Valid {
			val := int(progress.Int64)
			p.Progress = &val
		}
		p.Tags = tags
		p.Goals = goals
		p.Images = images
		projects = append(projects, p)
		projectIDs = append(projectIDs, p.ID)
	}

	needsMap, err := fetchProjectNeeds(projectIDs)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range projects {
		projects[i].Needs = needsMap[projects[i].ID]
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func RespondProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Invalid project id")
		return
	}
	userID := currentUserID(c)

	_, err = database.DB.Exec(`
		INSERT INTO project_responses (project_id, user_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (project_id, user_id) DO NOTHING
	`, projectID, userID)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Failed to submit response")
		return
	}

	_, _ = database.DB.Exec(`
		UPDATE projects
		SET responses_count = (SELECT COUNT(*) FROM project_responses WHERE project_id = $1),
			updated_at = NOW()
		WHERE id = $1
	`, projectID)

	c.JSON(http.StatusOK, gin.H{"message": "Response submitted"})
}

func GetProjectStats(c *gin.Context) {
	ownerKind := strings.TrimSpace(strings.ToLower(c.DefaultQuery("owner_kind", "company")))
	where := "1=1"
	switch ownerKind {
	case "company":
		where = "author_type = 'company'"
	case "user":
		where = "author_type IN ('user', 'community')"
	}

	var stats models.ProjectStats
	err := database.DB.QueryRow(`
		SELECT
			COUNT(*)::int AS total,
			COUNT(*) FILTER (WHERE author_type = 'company')::int AS companies,
			COUNT(*) FILTER (WHERE author_type IN ('user', 'community'))::int AS users,
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '7 days')::int AS new_week
		FROM projects
		WHERE `+where).Scan(&stats.Total, &stats.Companies, &stats.Users, &stats.NewWeek)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	rows, err := database.DB.Query(`
		SELECT goal, COUNT(*)::int
		FROM projects
		WHERE ` + where + `
		GROUP BY goal
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		jsonError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	stats.Categories = make([]models.ProjectCategoryStat, 0)
	for rows.Next() {
		var goal sql.NullString
		var count int
		if err := rows.Scan(&goal, &count); err != nil {
			jsonError(c, http.StatusInternalServerError, err.Error())
			return
		}
		goalValue := strings.TrimSpace(strings.ToLower(goal.String))
		if goalValue == "" {
			goalValue = "partner"
		}
		stats.Categories = append(stats.Categories, models.ProjectCategoryStat{
			Goal:  goalValue,
			Count: count,
			Name:  projectCategoryName(goalValue),
		})
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func fetchProjectNeeds(projectIDs []uuid.UUID) (map[uuid.UUID][]models.ProjectNeed, error) {
	out := make(map[uuid.UUID][]models.ProjectNeed)
	if len(projectIDs) == 0 {
		return out, nil
	}

	rows, err := database.DB.Query(`
		SELECT id, project_id, name, amount_text, status, sort_order
		FROM project_needs
		WHERE project_id = ANY($1)
		ORDER BY project_id, sort_order ASC, created_at ASC
	`, pq.Array(projectIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var n models.ProjectNeed
		var amountText, status sql.NullString
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.Name, &amountText, &status, &n.SortOrder); err != nil {
			return nil, err
		}
		if amountText.Valid {
			n.AmountText = &amountText.String
		}
		if status.Valid {
			n.Status = &status.String
		}
		out[n.ProjectID] = append(out[n.ProjectID], n)
	}
	return out, nil
}

func sanitizeStringSlice(input []string, max int) []string {
	result := make([]string, 0, len(input))
	for _, raw := range input {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		result = append(result, v)
		if max > 0 && len(result) >= max {
			break
		}
	}
	return result
}

func resolveProjectAuthor(c *gin.Context, userID uuid.UUID, authorType string, explicitAuthorID *string) (*uuid.UUID, string, error) {
	switch authorType {
	case "user":
		var fullName string
		if err := database.DB.QueryRow(`SELECT full_name FROM users WHERE id = $1`, userID).Scan(&fullName); err != nil {
			return nil, "", err
		}
		return &userID, fullName, nil
	case "company":
		companyID, name, err := resolveCompanyForProject(userID, explicitAuthorID)
		if err != nil {
			return nil, "", err
		}
		return &companyID, name, nil
	case "community":
		communityID, name, err := resolveCommunityForProject(userID, explicitAuthorID)
		if err != nil {
			return nil, "", err
		}
		return &communityID, name, nil
	default:
		return nil, "", errForbidden
	}
}

func resolveCompanyForProject(userID uuid.UUID, explicitAuthorID *string) (uuid.UUID, string, error) {
	if explicitAuthorID != nil && strings.TrimSpace(*explicitAuthorID) != "" {
		companyID, err := uuid.Parse(strings.TrimSpace(*explicitAuthorID))
		if err != nil {
			return uuid.Nil, "", err
		}
		allowed, err := requireCompanyPermission(companyID.String(), userID, "publish_news")
		if err != nil || !allowed {
			return uuid.Nil, "", errForbidden
		}
		var companyName string
		if err := database.DB.QueryRow(`SELECT name FROM companies WHERE id = $1`, companyID).Scan(&companyName); err != nil {
			return uuid.Nil, "", err
		}
		return companyID, companyName, nil
	}

	var companyID uuid.UUID
	var companyName string
	err := database.DB.QueryRow(`SELECT id, name FROM companies WHERE owner_id = $1 ORDER BY created_at ASC LIMIT 1`, userID).Scan(&companyID, &companyName)
	if err != nil {
		return uuid.Nil, "", err
	}
	return companyID, companyName, nil
}

func resolveCommunityForProject(userID uuid.UUID, explicitAuthorID *string) (uuid.UUID, string, error) {
	if explicitAuthorID != nil && strings.TrimSpace(*explicitAuthorID) != "" {
		communityID, err := uuid.Parse(strings.TrimSpace(*explicitAuthorID))
		if err != nil {
			return uuid.Nil, "", err
		}
		owner, err := requireCommunityOwner(communityID.String(), userID)
		if err != nil || !owner {
			return uuid.Nil, "", errForbidden
		}
		var name string
		if err := database.DB.QueryRow(`SELECT name FROM communities WHERE id = $1`, communityID).Scan(&name); err != nil {
			return uuid.Nil, "", err
		}
		return communityID, name, nil
	}

	var communityID uuid.UUID
	var name string
	err := database.DB.QueryRow(`SELECT id, name FROM communities WHERE owner_id = $1 ORDER BY created_at ASC LIMIT 1`, userID).Scan(&communityID, &name)
	if err != nil {
		return uuid.Nil, "", err
	}
	return communityID, name, nil
}

func projectCategoryName(goal string) string {
	switch goal {
	case "invest":
		return "Инвестиции"
	case "contract":
		return "Подрядчики"
	case "attn":
		return "Внимание"
	case "partner":
		return "Партнёры"
	default:
		return "Прочее"
	}
}
