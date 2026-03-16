package domain

import "time"

type Alert struct {
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Service         string            `json:"service"`
	Environment     string            `json:"environment"`
	Severity        string            `json:"severity"`
	Source          string            `json:"source"`
	Status          string            `json:"status"`
	Summary         string            `json:"summary"`
	Description     string            `json:"description"`
	Labels          map[string]string `json:"labels,omitempty"`
	LinkedSessionID string            `json:"linked_session_id,omitempty"`
	OccurrenceCount int               `json:"occurrence_count"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	TriggeredAt     time.Time         `json:"triggered_at"`
	LastTriggeredAt time.Time         `json:"last_triggered_at"`
}
