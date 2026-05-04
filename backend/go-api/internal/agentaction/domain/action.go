package domain

import "time"

type Action struct {
	ID            string     `json:"id"`
	SourceType    string     `json:"source_type"`
	SourceID      string     `json:"source_id"`
	ActionType    string     `json:"action_type"`
	Status        string     `json:"status"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	ArgumentsJSON string     `json:"arguments_json"`
	RiskLevel     string     `json:"risk_level"`
	ResultJSON    string     `json:"result_json,omitempty"`
	Error         string     `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ExecutedAt    *time.Time `json:"executed_at,omitempty"`
}
