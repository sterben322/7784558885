package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// StringSlice — []string, хранится в PG как JSONB
type StringSlice []string

func (s StringSlice) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(s))
}

// Speaker — один спикер мероприятия
type Speaker struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// SpeakerSlice — хранится как JSONB
type SpeakerSlice []Speaker

func (s SpeakerSlice) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]Speaker(s))
}

// Event — мероприятие
type Event struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	Type            string         `json:"type"`             // webinar|workshop|conference|networking|roundtable
	Format          string         `json:"format"`           // online|offline|hybrid
	Category        string         `json:"category"`
	City            sql.NullString `json:"-"`
	CityStr         string         `json:"city,omitempty"`
	Date            string         `json:"date"`             // YYYY-MM-DD
	TimeStart       sql.NullString `json:"-"`
	TimeStartStr    string         `json:"time_start,omitempty"`
	TimeEnd         sql.NullString `json:"-"`
	TimeEndStr      string         `json:"time_end,omitempty"`
	DurationMin     int            `json:"duration_min,omitempty"`
	Fee             int            `json:"fee"`
	SeatsTotal      int            `json:"seats_total"`
	Description     string         `json:"description"`
	Cover           sql.NullString `json:"-"`
	CoverStr        string         `json:"cover,omitempty"`
	OrganizerID     string         `json:"organizer_id"`
	OrganizerName   string         `json:"organizer_name"`
	Tags            StringSlice    `json:"tags"`
	Speakers        SpeakerSlice   `json:"speakers"`
	RegisteredCount int            `json:"registered_count"`
	ViewsCount      int            `json:"views_count"`
	CommentsCount   int            `json:"comments_count"`
	IsRegistered    bool           `json:"is_registered"`
	CreatedAt       time.Time      `json:"created_at"`
}

// EventsStats — статистика для сайдбара
type EventsStats struct {
	Total      int              `json:"total"`
	Today      int              `json:"today"`
	Upcoming   int              `json:"upcoming"`
	Categories []EventCategory  `json:"categories"`
}

type EventCategory struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// CreateEventRequest — тело POST /events
type CreateEventRequest struct {
	Title       string    `json:"title"`
	Type        string    `json:"type"`
	Format      string    `json:"format"`
	Category    string    `json:"category"`
	City        string    `json:"city"`
	Date        string    `json:"date"`
	TimeStart   string    `json:"time_start"`
	TimeEnd     string    `json:"time_end"`
	Fee         int       `json:"fee"`
	SeatsTotal  int       `json:"seats_total"`
	Description string    `json:"description"`
	Cover       string    `json:"cover"`
	Tags        []string  `json:"tags"`
	Speakers    []Speaker `json:"speakers"`
}
