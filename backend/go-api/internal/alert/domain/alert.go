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
	Analysis        *AlertAnalysis    `json:"analysis,omitempty"`
	OccurrenceCount int               `json:"occurrence_count"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	TriggeredAt     time.Time         `json:"triggered_at"`
	LastTriggeredAt time.Time         `json:"last_triggered_at"`
}

type AlertAnalysis struct {
	Status             string       `json:"status"`
	Summary            string       `json:"summary"`
	SeverityAssessment string       `json:"severity_assessment"`
	PossibleCauses     []string     `json:"possible_causes,omitempty"`
	SuggestedActions   []string     `json:"suggested_actions,omitempty"`
	RecommendedTools   []string     `json:"recommended_tools,omitempty"`
	KnowledgeQueries   []string     `json:"knowledge_queries,omitempty"`
	ToolCalls          []ToolCall   `json:"tool_calls,omitempty"`
	Workflow           string       `json:"workflow,omitempty"`
	Confidence         string       `json:"confidence,omitempty"`
	Source             string       `json:"source,omitempty"`
	GeneratedAt        time.Time    `json:"generated_at"`
	Error              string       `json:"error,omitempty"`
	Trace              []TraceEvent `json:"trace,omitempty"`
}

type ToolCall struct {
	Name          string `json:"name,omitempty"`
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
