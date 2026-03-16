package domain

import "time"

type HandlingRecord struct {
	ID        string    `json:"id"`
	AlertID   string    `json:"alert_id"`
	Action    string    `json:"action"`
	Operator  string    `json:"operator"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}
