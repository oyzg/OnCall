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
	Status             string               `json:"status"`
	Summary            string               `json:"summary"`
	SeverityAssessment string               `json:"severity_assessment"`
	PossibleCauses     []string             `json:"possible_causes,omitempty"`
	SuggestedActions   []string             `json:"suggested_actions,omitempty"`
	RecommendedTools   []string             `json:"recommended_tools,omitempty"`
	KnowledgeQueries   []string             `json:"knowledge_queries,omitempty"`
	ToolCalls          []ToolCall           `json:"tool_calls,omitempty"`
	AgentPlan          []AgentPlanStep      `json:"agent_plan,omitempty"`
	PendingActions     []PendingAgentAction `json:"pending_actions,omitempty"`
	Workflow           string               `json:"workflow,omitempty"`
	Confidence         string               `json:"confidence,omitempty"`
	Source             string               `json:"source,omitempty"`
	GeneratedAt        time.Time            `json:"generated_at"`
	Error              string               `json:"error,omitempty"`
	Trace              []TraceEvent         `json:"trace,omitempty"`
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

type AgentActionObservation struct {
	ActionID   string `json:"action_id,omitempty"`
	ActionType string `json:"action_type,omitempty"`
	Status     string `json:"status,omitempty"`
	Title      string `json:"title,omitempty"`
	ResultJSON string `json:"result_json,omitempty"`
	Error      string `json:"error,omitempty"`
	ExecutedAt string `json:"executed_at,omitempty"`
}
