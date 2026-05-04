package analyzer

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/ai/eino"
	"github.com/oyzg/OnCall/backend/go-api/internal/ai/gateway"
	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type failingOrchestrator struct{}

func (failingOrchestrator) HandleChat(context.Context, gateway.ChatRequest) (gateway.ChatResponse, error) {
	return gateway.ChatResponse{}, nil
}

func (failingOrchestrator) HandleAlertAnalysis(context.Context, gateway.AlertAnalysisRequest) (gateway.AlertAnalysisResponse, error) {
	return gateway.AlertAnalysisResponse{}, status.Error(codes.Unavailable, "python ai runtime unavailable")
}

var _ eino.Orchestrator = failingOrchestrator{}

type captureOrchestrator struct {
	request gateway.AlertAnalysisRequest
}

func (c *captureOrchestrator) HandleChat(context.Context, gateway.ChatRequest) (gateway.ChatResponse, error) {
	return gateway.ChatResponse{}, nil
}

func (c *captureOrchestrator) HandleAlertAnalysis(_ context.Context, request gateway.AlertAnalysisRequest) (gateway.AlertAnalysisResponse, error) {
	c.request = request
	return gateway.AlertAnalysisResponse{
		Status:             "ready",
		Summary:            "captured summary",
		SeverityAssessment: "captured severity",
		Confidence:         "high",
		GeneratedAt:        "2026-04-09T10:00:00Z",
		ToolCalls: []gateway.ToolCall{
			{Name: "service_status", ArgumentsJSON: `{"service":"payment-api"}`, Outcome: "success", Summary: "payment-api degraded"},
		},
		Trace: []gateway.TraceEvent{
			{Stage: "router", Message: "alert route selected", Severity: "info"},
		},
	}, nil
}

func TestAnalyzeAlertFallsBackWhenRuntimeUnavailable(t *testing.T) {
	service := NewService(failingOrchestrator{})
	alert := alertDomain.Alert{
		ID:              "alert-1",
		Title:           "Payment API timeout",
		Service:         "payment-api",
		Environment:     "prod",
		Severity:        "P1",
		Source:          "prometheus",
		Summary:         "p95 latency increased",
		Description:     "timeouts are increasing on checkout",
		LinkedSessionID: "session-1",
		LastTriggeredAt: time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC),
	}

	analysis := service.AnalyzeAlert(context.Background(), authDomain.User{}, alert)

	if analysis.Status != "failed" {
		t.Fatalf("expected failed fallback, got %s", analysis.Status)
	}
	if !strings.Contains(analysis.Error, "runtime unavailable") {
		t.Fatalf("expected unavailable error, got %s", analysis.Error)
	}
	if analysis.Workflow != "go_fallback_rule_analysis" {
		t.Fatalf("expected fallback workflow, got %s", analysis.Workflow)
	}
	if len(analysis.Trace) == 0 || analysis.Trace[0].Stage != "alert_analysis" {
		t.Fatalf("expected fallback trace, got %#v", analysis.Trace)
	}
}

func TestAnalyzeAlertPassesUserContextToGateway(t *testing.T) {
	orchestrator := &captureOrchestrator{}
	service := NewService(orchestrator)
	alert := alertDomain.Alert{
		ID:              "alert-1",
		Title:           "Payment API timeout",
		Service:         "payment-api",
		Environment:     "prod",
		Severity:        "P1",
		Source:          "prometheus",
		Summary:         "p95 latency increased",
		Description:     "timeouts are increasing on checkout",
		LinkedSessionID: "session-1",
		LastTriggeredAt: time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC),
	}

	analysis := service.AnalyzeAlert(context.Background(), authDomain.User{
		ID:    "user-123",
		Roles: []string{"oncall", "admin"},
	}, alert)

	if analysis.Status != "ready" {
		t.Fatalf("unexpected analysis status: %s", analysis.Status)
	}
	if orchestrator.request.UserID != "user-123" {
		t.Fatalf("expected user id to be forwarded, got %s", orchestrator.request.UserID)
	}
	if !reflect.DeepEqual(orchestrator.request.UserRoles, []string{"oncall", "admin"}) {
		t.Fatalf("expected roles to be forwarded, got %#v", orchestrator.request.UserRoles)
	}
	if orchestrator.request.AlertID != "alert-1" {
		t.Fatalf("expected alert id to be forwarded, got %s", orchestrator.request.AlertID)
	}
	if len(analysis.ToolCalls) != 1 || analysis.ToolCalls[0].Name != "service_status" {
		t.Fatalf("expected tool calls to be mapped, got %#v", analysis.ToolCalls)
	}
	if len(analysis.Trace) != 1 || analysis.Trace[0].Stage != "router" {
		t.Fatalf("expected runtime trace to be mapped, got %#v", analysis.Trace)
	}
}

func TestAnalyzeAlertWithObservationsPassesConfirmedActionContextToGateway(t *testing.T) {
	orchestrator := &captureOrchestrator{}
	service := NewService(orchestrator)
	alert := alertDomain.Alert{
		ID:              "alert-1",
		Title:           "Payment API timeout",
		Service:         "payment-api",
		Environment:     "prod",
		Severity:        "P1",
		Source:          "prometheus",
		Summary:         "p95 latency increased",
		Description:     "timeouts are increasing on checkout",
		LinkedSessionID: "session-1",
		LastTriggeredAt: time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC),
	}

	service.AnalyzeAlertWithObservations(context.Background(), authDomain.User{ID: "user-123"}, alert, []alertDomain.AgentActionObservation{
		{
			ActionID:   "action-1",
			ActionType: "update_alert_status",
			Status:     "executed",
			ResultJSON: `{"status":"investigating"}`,
		},
	})

	if len(orchestrator.request.ActionObservations) != 1 {
		t.Fatalf("expected one action observation, got %#v", orchestrator.request.ActionObservations)
	}
	if orchestrator.request.ActionObservations[0].ActionType != "update_alert_status" {
		t.Fatalf("unexpected action observation: %#v", orchestrator.request.ActionObservations[0])
	}
}
