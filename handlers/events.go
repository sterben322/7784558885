package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lastop/database"
	"lastop/models"
)

// ══════════════════════════════════════════════════════════════════
//  ROUTES — регистрируются в main.go внутри защищённой группы /api
//
//  authGroup.HandleFunc("/events",              EventsList).Methods("GET")
//  authGroup.HandleFunc("/events",              EventCreate).Methods("POST")
//  authGroup.HandleFunc("/events/stats",        EventsStats).Methods("GET")
//  authGroup.HandleFunc("/events/my",           EventsMy).Methods("GET")
//  authGroup.HandleFunc("/events/{id}",         EventGet).Methods("GET")
//  authGroup.HandleFunc("/events/{id}/register", EventRegister).Methods("POST")
//  authGroup.HandleFunc("/events/{id}/register", EventUnregister).Methods("DELETE")
//  authGroup.HandleFunc("/events/{id}/view",    EventView).Methods("POST")
//
// ══════════════════════════════════════════════════════════════════

// ─── helpers ───────────────────────────────────────────────────────

func currentUserIDFromRequest(r *http.Request) string {
	if v := r.Context().Value("userID"); v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func respondJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func pathID(r *http.Request) string {
	parts := strings.Split(r.URL.Path, "/")
	for i, p := range parts {
		if p == "events" && i+1 < len(parts) {
			next := parts[i+1]
			if next != "" && next != "stats" && next != "my" {
				return next
			}
		}
	}
	return ""
}

// scanEvent читает строку из sql.Rows в Event
func scanEvent(rows *sql.Rows, userID string) (models.Event, error) {
	var ev models.Event
	var tagsRaw, speakersRaw []byte
	var timeStart, timeEnd sql.NullString
	var cover sql.NullString
	var city sql.NullString
	var date time.Time

	err := rows.Scan(
		&ev.ID, &ev.Title, &ev.Type, &ev.Format, &ev.Category,
		&city, &date, &timeStart, &timeEnd, &ev.DurationMin,
		&ev.Fee, &ev.SeatsTotal, &ev.Description, &cover,
		&ev.OrganizerID, &ev.OrganizerName,
		&tagsRaw, &speakersRaw,
		&ev.RegisteredCount, &ev.ViewsCount, &ev.CommentsCount,
		&ev.IsRegistered, &ev.CreatedAt,
	)
	if err != nil {
		return ev, err
	}

	ev.Date = date.Format("2006-01-02")
	if timeStart.Valid {
		ev.TimeStartStr = timeStart.String[:5] // HH:MM
	}
	if timeEnd.Valid {
		ev.TimeEndStr = timeEnd.String[:5]
	}
	if cover.Valid {
		ev.CoverStr = cover.String
	}
	if city.Valid {
		ev.CityStr = city.String
	}

	if len(tagsRaw) > 0 {
		json.Unmarshal(tagsRaw, &ev.Tags)
	}
	if ev.Tags == nil {
		ev.Tags = models.StringSlice{}
	}
	if len(speakersRaw) > 0 {
		json.Unmarshal(speakersRaw, &ev.Speakers)
	}
	if ev.Speakers == nil {
		ev.Speakers = models.SpeakerSlice{}
	}

	return ev, nil
}

// eventsBaseQuery — SELECT с подзапросом is_registered
const eventsBaseQuery = `
SELECT
    e.id, e.title, e.type, e.format, e.category,
    e.city, e.date, e.time_start::text, e.time_end::text, e.duration_min,
    e.fee, e.seats_total, e.description, e.cover,
    e.organizer_id,
    COALESCE(u.full_name, u.name, '') AS organizer_name,
    e.tags, e.speakers,
    e.registered_count, e.views_count, e.comments_count,
    EXISTS(
        SELECT 1 FROM event_registrations er
        WHERE er.event_id = e.id AND er.user_id = $1
    ) AS is_registered,
    e.created_at
FROM events e
LEFT JOIN users u ON u.id = e.organizer_id
`

// ══════════════════════════════════════════════════════════════════
//  GET /api/events
//  Параметры: date=YYYY-MM-DD, type=, format=, q=, limit=, offset=
// ══════════════════════════════════════════════════════════════════

func EventsList(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID := currentUserIDFromRequest(r)

	q := r.URL.Query()
	dateStr := q.Get("date")     // конкретная дата
	evType := q.Get("type")      // webinar|workshop|…
	evFormat := q.Get("format")  // online|offline|hybrid
	search := strings.TrimSpace(q.Get("q"))
	limitStr := q.Get("limit")
	offsetStr := q.Get("offset")

	limit := 200
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	// Строим WHERE
	conds := []string{}
	args := []any{userID} // $1 = userID для is_registered
	idx := 2

	if dateStr != "" {
		if _, err := time.Parse("2006-01-02", dateStr); err == nil {
			conds = append(conds, fmt.Sprintf("e.date = $%d", idx))
			args = append(args, dateStr)
			idx++
		}
	}
	if evType != "" {
		conds = append(conds, fmt.Sprintf("e.type = $%d", idx))
		args = append(args, evType)
		idx++
	}
	if evFormat != "" {
		if evFormat == "online" {
			conds = append(conds, fmt.Sprintf("(e.format = $%d OR e.format = 'hybrid')", idx))
		} else {
			conds = append(conds, fmt.Sprintf("e.format = $%d", idx))
		}
		args = append(args, evFormat)
		idx++
	}
	if search != "" {
		conds = append(conds, fmt.Sprintf(
			"(e.title ILIKE $%d OR e.description ILIKE $%d OR e.category ILIKE $%d)",
			idx, idx, idx,
		))
		args = append(args, "%"+search+"%")
		idx++
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	sqlQuery := eventsBaseQuery + where +
		fmt.Sprintf(" ORDER BY e.date ASC, e.time_start ASC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	events := []models.Event{}
	for rows.Next() {
		ev, err := scanEvent(rows, userID)
		if err != nil {
			continue
		}
		events = append(events, ev)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"total":  len(events),
	})
}

// ══════════════════════════════════════════════════════════════════
//  GET /api/events/stats
// ══════════════════════════════════════════════════════════════════

func EventsStatsHandler(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	today := time.Now().Format("2006-01-02")

	var total, todayCount, upcoming int
	db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&total)
	db.QueryRow(`SELECT COUNT(*) FROM events WHERE date = $1`, today).Scan(&todayCount)
	db.QueryRow(`SELECT COUNT(*) FROM events WHERE date > $1`, today).Scan(&upcoming)

	// Категории по типу
	rows, err := db.Query(`
		SELECT type, COUNT(*) FROM events GROUP BY type ORDER BY COUNT(*) DESC
	`)
	typeLabels := map[string]string{
		"webinar":    "Вебинар",
		"workshop":   "Воркшоп",
		"conference": "Конференция",
		"networking": "Нетворкинг",
		"roundtable": "Круглый стол",
	}
	categories := []models.EventCategory{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var cnt int
			if rows.Scan(&t, &cnt) == nil {
				label := typeLabels[t]
				if label == "" {
					label = t
				}
				categories = append(categories, models.EventCategory{
					Type:  t,
					Label: label,
					Count: cnt,
				})
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"stats": models.EventsStats{
			Total:      total,
			Today:      todayCount,
			Upcoming:   upcoming,
			Categories: categories,
		},
	})
}

// ══════════════════════════════════════════════════════════════════
//  GET /api/events/my — мероприятия, на которые пользователь записан
// ══════════════════════════════════════════════════════════════════

func EventsMy(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID := currentUserIDFromRequest(r)

	sqlQuery := eventsBaseQuery + `
    JOIN event_registrations er ON er.event_id = e.id AND er.user_id = $1
    WHERE e.date >= CURRENT_DATE
    ORDER BY e.date ASC, e.time_start ASC
    LIMIT 20`

	rows, err := db.Query(sqlQuery, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	events := []models.Event{}
	for rows.Next() {
		ev, err := scanEvent(rows, userID)
		if err != nil {
			continue
		}
		events = append(events, ev)
	}

	respondJSON(w, http.StatusOK, map[string]any{"events": events})
}

// ══════════════════════════════════════════════════════════════════
//  GET /api/events/{id}
// ══════════════════════════════════════════════════════════════════

func EventGet(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID := currentUserIDFromRequest(r)
	id := pathID(r)
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}

	rows, err := db.Query(eventsBaseQuery+` WHERE e.id = $2`, userID, id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	if !rows.Next() {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	ev, err := scanEvent(rows, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Инкремент просмотров (в фоне, не блокируем ответ)
	go db.Exec(`UPDATE events SET views_count = views_count + 1 WHERE id = $1`, id)

	respondJSON(w, http.StatusOK, map[string]any{"event": ev})
}

// ══════════════════════════════════════════════════════════════════
//  POST /api/events — создать мероприятие
// ══════════════════════════════════════════════════════════════════

func EventCreate(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID := currentUserIDFromRequest(r)

	var req models.CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date"})
		return
	}

	// Дефолты
	if req.Type == "" {
		req.Type = "webinar"
	}
	if req.Format == "" {
		req.Format = "online"
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	if req.Speakers == nil {
		req.Speakers = []models.Speaker{}
	}

	// Длительность (если указаны оба времени)
	durMin := 0
	if req.TimeStart != "" && req.TimeEnd != "" {
		t1, err1 := time.Parse("15:04", req.TimeStart)
		t2, err2 := time.Parse("15:04", req.TimeEnd)
		if err1 == nil && err2 == nil && t2.After(t1) {
			durMin = int(t2.Sub(t1).Minutes())
		}
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	speakersJSON, _ := json.Marshal(req.Speakers)

	var id string
	err := db.QueryRow(`
		INSERT INTO events
			(title, type, format, category, city, date, time_start, time_end,
			 duration_min, fee, seats_total, description, cover,
			 organizer_id, tags, speakers)
		VALUES
			($1,$2,$3,$4,NULLIF($5,''),$6,
			 NULLIF($7,'')::TIME, NULLIF($8,'')::TIME,
			 $9,$10,$11,$12,NULLIF($13,''),
			 $14,$15,$16)
		RETURNING id`,
		req.Title, req.Type, req.Format, req.Category, req.City, req.Date,
		req.TimeStart, req.TimeEnd, durMin, req.Fee, req.SeatsTotal,
		req.Description, req.Cover, userID,
		tagsJSON, speakersJSON,
	).Scan(&id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// ══════════════════════════════════════════════════════════════════
//  POST /api/events/{id}/register — записаться на мероприятие
// ══════════════════════════════════════════════════════════════════

func EventRegister(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID := currentUserIDFromRequest(r)
	id := pathID(r)
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}

	// Проверяем наличие мест (seats_total=0 → без лимита)
	var seatsTotal, registeredCount int
	err := db.QueryRow(
		`SELECT seats_total, registered_count FROM events WHERE id = $1`, id,
	).Scan(&seatsTotal, &registeredCount)
	if err == sql.ErrNoRows {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "event not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if seatsTotal > 0 && registeredCount >= seatsTotal {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "no seats available"})
		return
	}

	// INSERT OR IGNORE (UNIQUE constraint)
	_, err = db.Exec(
		`INSERT INTO event_registrations (event_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		id, userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

// ══════════════════════════════════════════════════════════════════
//  DELETE /api/events/{id}/register — отменить регистрацию
// ══════════════════════════════════════════════════════════════════

func EventUnregister(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	userID := currentUserIDFromRequest(r)
	id := pathID(r)
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}

	_, err := db.Exec(
		`DELETE FROM event_registrations WHERE event_id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unregistered"})
}

// ══════════════════════════════════════════════════════════════════
//  POST /api/events/{id}/view — инкремент просмотров
// ══════════════════════════════════════════════════════════════════

func EventView(w http.ResponseWriter, r *http.Request) {
	db := database.DB
	id := pathID(r)
	if id == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	db.Exec(`UPDATE events SET views_count = views_count + 1 WHERE id = $1`, id)
	w.WriteHeader(http.StatusNoContent)
}
