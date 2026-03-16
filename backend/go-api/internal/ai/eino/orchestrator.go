package eino

import (
	"context"

	"github.com/oyzg/OnCall/backend/go-api/internal/ai/gateway"
)

type Orchestrator interface {
	HandleChat(ctx context.Context, request gateway.ChatRequest) (gateway.ChatResponse, error)
	HandleAlertAnalysis(ctx context.Context, request gateway.AlertAnalysisRequest) (gateway.AlertAnalysisResponse, error)
}

type StubOrchestrator struct {
	client gateway.Client
}

func NewStubOrchestrator(client gateway.Client) *StubOrchestrator {
	return &StubOrchestrator{client: client}
}

func (o *StubOrchestrator) HandleChat(ctx context.Context, request gateway.ChatRequest) (gateway.ChatResponse, error) {
	if o.client == nil {
		return gateway.ChatResponse{}, nil
	}
	return o.client.Chat(ctx, request)
}

func (o *StubOrchestrator) HandleAlertAnalysis(ctx context.Context, request gateway.AlertAnalysisRequest) (gateway.AlertAnalysisResponse, error) {
	if o.client == nil {
		return gateway.AlertAnalysisResponse{}, nil
	}
	return o.client.AnalyzeAlert(ctx, request)
}
