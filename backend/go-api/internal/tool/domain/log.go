package domain

import "time"

type CallLog struct {
	ID         string         `json:"id"`
	ToolName   string         `json:"tool_name"`
	Operator   string         `json:"operator"`
	UserID     string         `json:"user_id"`
	Status     string         `json:"status"`
	Input      map[string]any `json:"input"`
	Output     any            `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms"`
	CreatedAt  time.Time      `json:"created_at"`
}
