package domain

import "time"

type Log struct {
	ID         string         `json:"id"`
	Category   string         `json:"category"`
	Action     string         `json:"action"`
	Status     string         `json:"status"`
	ActorID    string         `json:"actor_id,omitempty"`
	ActorName  string         `json:"actor_name,omitempty"`
	ActorRoles []string       `json:"actor_roles,omitempty"`
	TargetType string         `json:"target_type,omitempty"`
	TargetID   string         `json:"target_id,omitempty"`
	TargetName string         `json:"target_name,omitempty"`
	Detail     string         `json:"detail,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type Stats struct {
	Total        int            `json:"total"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	Last24Hours  int            `json:"last_24_hours"`
	UniqueActors int            `json:"unique_actors"`
	ByCategory   map[string]int `json:"by_category"`
}
