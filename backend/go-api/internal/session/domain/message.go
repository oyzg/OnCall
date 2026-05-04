package domain

import "time"

type Message struct {
	ID             string               `json:"id"`
	SessionID      string               `json:"session_id"`
	Role           string               `json:"role"`
	Content        string               `json:"content"`
	Status         string               `json:"status"`
	Route          string               `json:"route,omitempty"`
	ToolCalls      []ToolCall           `json:"tool_calls,omitempty"`
	Trace          []TraceEvent         `json:"trace,omitempty"`
	AgentPlan      []AgentPlanStep      `json:"agent_plan,omitempty"`
	PendingActions []PendingAgentAction `json:"pending_actions,omitempty"`
	References     []Reference          `json:"references,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
}

type ToolCall struct {
	Name          string `json:"name"`
	ArgumentsJSON string `json:"arguments_json,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
	Summary       string `json:"summary,omitempty"`
}

type TraceEvent struct {
	Stage     string   `json:"stage,omitempty"`
	Message   string   `json:"message,omitempty"`
	Severity  string   `json:"severity,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type AgentPlanStep struct {
	StepID      string `json:"step_id,omitempty"`
	Phase       string `json:"phase,omitempty"`
	Description string `json:"description,omitempty"`
	ToolName    string `json:"tool_name,omitempty"`
	Observation string `json:"observation,omitempty"`
	Status      string `json:"status,omitempty"`
}

type PendingAgentAction struct {
	ActionID      string `json:"action_id,omitempty"`
	ActionType    string `json:"action_type,omitempty"`
	Status        string `json:"status,omitempty"`
	Title         string `json:"title,omitempty"`
	Description   string `json:"description,omitempty"`
	ArgumentsJSON string `json:"arguments_json,omitempty"`
	RiskLevel     string `json:"risk_level,omitempty"`
}

type Reference struct {
	DocumentID    string  `json:"document_id"`
	DocumentTitle string  `json:"document_title"`
	Category      string  `json:"category"`
	Excerpt       string  `json:"excerpt"`
	Score         float64 `json:"score"`
}
