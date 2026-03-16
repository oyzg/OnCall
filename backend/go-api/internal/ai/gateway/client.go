package gateway

import "context"

type ChatRequest struct {
	Query          string
	ConversationID string
}

type ChatResponse struct {
	Answer    string
	Citations []string
}

type AlertAnalysisRequest struct {
	AlertID     string
	Title       string
	PayloadJSON string
}

type AlertAnalysisResponse struct {
	Summary          string
	PossibleCauses   []string
	SuggestedActions []string
}

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
	AnalyzeAlert(ctx context.Context, request AlertAnalysisRequest) (AlertAnalysisResponse, error)
}
