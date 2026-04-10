package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	aipb "github.com/oyzg/OnCall/backend/go-api/gen/proto/ai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRuntimeClient struct {
	analyze func(context.Context, *aipb.AnalyzeAlertRequest) (*aipb.AnalyzeAlertResponse, error)
	chat    func(context.Context, *aipb.RunConversationTurnRequest) (*aipb.RunConversationTurnResponse, error)
	health  func(context.Context, *aipb.HealthRequest) (*aipb.HealthResponse, error)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (f *fakeRuntimeClient) AnalyzeAlert(ctx context.Context, request *aipb.AnalyzeAlertRequest, _ ...grpc.CallOption) (*aipb.AnalyzeAlertResponse, error) {
	if f.analyze != nil {
		return f.analyze(ctx, request)
	}
	return &aipb.AnalyzeAlertResponse{}, nil
}

func (f *fakeRuntimeClient) RunConversationTurn(ctx context.Context, request *aipb.RunConversationTurnRequest, _ ...grpc.CallOption) (*aipb.RunConversationTurnResponse, error) {
	if f.chat != nil {
		return f.chat(ctx, request)
	}
	return &aipb.RunConversationTurnResponse{}, nil
}

func (f *fakeRuntimeClient) Health(ctx context.Context, request *aipb.HealthRequest, _ ...grpc.CallOption) (*aipb.HealthResponse, error) {
	if f.health != nil {
		return f.health(ctx, request)
	}
	return &aipb.HealthResponse{}, nil
}

func TestAnalyzeAlertMapsRuntimeRequestAndConfidenceLabel(t *testing.T) {
	var captured *aipb.AnalyzeAlertRequest
	client := &HTTPClient{
		runtimeClient: &fakeRuntimeClient{
			analyze: func(_ context.Context, request *aipb.AnalyzeAlertRequest) (*aipb.AnalyzeAlertResponse, error) {
				captured = request
				return &aipb.AnalyzeAlertResponse{
					Status:             "ready",
					Summary:            "grpc summary",
					SeverityAssessment: "grpc severity assessment",
					PossibleCauses:     []string{"downstream timeout"},
					SuggestedActions:   []string{"check dependency health"},
					RecommendedTools:   []string{"service_status"},
					KnowledgeQueries:   []string{"payment-api timeout"},
					Workflow:           "alert_analysis",
					Confidence:         0.9,
					Source:             "python-ai-runtime",
					GeneratedAt:        "2026-04-09T10:00:00Z",
				}, nil
			},
		},
		runtimeTimeout: 250 * time.Millisecond,
	}

	response, err := client.AnalyzeAlert(context.Background(), AlertAnalysisRequest{
		AlertID:         "alert-1",
		Title:           "Payment API timeout",
		Service:         "payment-api",
		Environment:     "prod",
		Severity:        "P1",
		Source:          "prometheus",
		Summary:         "p95 latency increased",
		Description:     "timeouts are increasing on checkout",
		Labels:          map[string]string{"team": "payments", "tier": "critical"},
		TriggeredAt:     "2026-04-09T10:00:00Z",
		LinkedSessionID: "session-1",
		UserID:          "user-1",
		UserRoles:       []string{"oncall", "admin"},
	})
	if err != nil {
		t.Fatalf("AnalyzeAlert: %v", err)
	}

	if response.Confidence != "high" {
		t.Fatalf("expected high confidence, got %s", response.Confidence)
	}
	if response.Status != "ready" {
		t.Fatalf("unexpected status: %s", response.Status)
	}
	if response.Summary != "grpc summary" {
		t.Fatalf("unexpected summary: %s", response.Summary)
	}

	if captured == nil {
		t.Fatal("expected runtime request to be captured")
	}
	if captured.AlertId != "alert-1" {
		t.Fatalf("unexpected alert id: %s", captured.AlertId)
	}
	if captured.UserId != "user-1" {
		t.Fatalf("unexpected user id: %s", captured.UserId)
	}
	if !reflect.DeepEqual(captured.UserRoles, []string{"oncall", "admin"}) {
		t.Fatalf("unexpected user roles: %#v", captured.UserRoles)
	}
	if captured.Metadata == nil {
		t.Fatal("expected metadata")
	}
	if captured.Metadata.UserId != "user-1" {
		t.Fatalf("unexpected metadata user id: %s", captured.Metadata.UserId)
	}
	if captured.Metadata.SessionId != "session-1" {
		t.Fatalf("unexpected metadata session id: %s", captured.Metadata.SessionId)
	}
	if !reflect.DeepEqual(captured.Labels, []string{"team=payments", "tier=critical"}) {
		t.Fatalf("unexpected labels: %#v", captured.Labels)
	}
}

func TestAnalyzeAlertMapsConfidenceLabels(t *testing.T) {
	cases := []struct {
		name       string
		confidence float32
		expected   string
	}{
		{name: "low", confidence: 0.2, expected: "low"},
		{name: "medium", confidence: 0.6, expected: "medium"},
		{name: "high", confidence: 0.9, expected: "high"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &HTTPClient{
				runtimeClient: &fakeRuntimeClient{
					analyze: func(_ context.Context, _ *aipb.AnalyzeAlertRequest) (*aipb.AnalyzeAlertResponse, error) {
						return &aipb.AnalyzeAlertResponse{Confidence: tc.confidence}, nil
					},
				},
				runtimeTimeout: 250 * time.Millisecond,
			}

			response, err := client.AnalyzeAlert(context.Background(), AlertAnalysisRequest{})
			if err != nil {
				t.Fatalf("AnalyzeAlert: %v", err)
			}
			if response.Confidence != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, response.Confidence)
			}
		})
	}
}

func TestChatMapsRuntimeRequestAndResponseFields(t *testing.T) {
	var captured *aipb.RunConversationTurnRequest
	client := &HTTPClient{
		runtimeClient: &fakeRuntimeClient{
			chat: func(_ context.Context, request *aipb.RunConversationTurnRequest) (*aipb.RunConversationTurnResponse, error) {
				captured = request
				return &aipb.RunConversationTurnResponse{
					Answer: "use the cache invalidation runbook",
					Citations: []*aipb.Citation{
						{Title: "Cache invalidation runbook"},
						{Source: "kb-service"},
						{DocumentId: "doc-3"},
					},
					ToolCalls: []*aipb.ToolCall{
						{Name: "service_status", ArgumentsJson: `{"service":"payment-api"}`, Outcome: "success", Summary: "checked status"},
					},
					Route:  "chat_qa",
					Status: "ready",
					Error:  "",
					Trace: []*aipb.TraceEvent{
						{Stage: "router", Message: "routed to chat", Severity: "info", Timestamp: "2026-04-09T10:00:00Z", Tags: []string{"chat"}},
					},
				}, nil
			},
		},
		runtimeTimeout: 250 * time.Millisecond,
	}

	response, err := client.Chat(context.Background(), ChatRequest{
		Query:          "How do I reset the cache?",
		ConversationID: "conversation-7",
		UserID:         "user-7",
		UserRoles:      []string{"oncall"},
		History: []ChatMessage{
			{
				Role:      "user",
				Content:   "Previous context",
				AuthorID:  "user-7",
				CreatedAt: "2026-04-09T09:00:00Z",
				Citations: []ChatCitation{{Title: "History citation", DocumentID: "doc-1"}},
			},
		},
		LinkedAlert: &LinkedAlert{
			AlertID:         "alert-9",
			Title:           "Payment API timeout",
			Service:         "payment-api",
			Environment:     "prod",
			Severity:        "P1",
			Source:          "prometheus",
			Summary:         "latency spike",
			Description:     "timeouts are increasing on checkout",
			Labels:          map[string]string{"team": "payments"},
			TriggeredAt:     "2026-04-09T10:00:00Z",
			LinkedSessionID: "conversation-7",
		},
		AllowedTools:   []string{"service_status"},
		RetrievalLimit: 3,
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if response.Answer != "use the cache invalidation runbook" {
		t.Fatalf("unexpected answer: %s", response.Answer)
	}
	if response.Route != "chat_qa" {
		t.Fatalf("unexpected route: %s", response.Route)
	}
	if response.Status != "ready" {
		t.Fatalf("unexpected status: %s", response.Status)
	}
	if response.Error != "" {
		t.Fatalf("unexpected error: %s", response.Error)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "service_status" {
		t.Fatalf("unexpected tool calls: %#v", response.ToolCalls)
	}
	if len(response.CitationItems) != 3 || response.CitationItems[0].Title != "Cache invalidation runbook" {
		t.Fatalf("unexpected citation items: %#v", response.CitationItems)
	}
	if len(response.Trace) != 1 || response.Trace[0].Stage != "router" {
		t.Fatalf("unexpected trace: %#v", response.Trace)
	}
	if !reflect.DeepEqual(response.Citations, []string{"Cache invalidation runbook", "kb-service", "doc-3"}) {
		t.Fatalf("unexpected citations: %#v", response.Citations)
	}

	if captured == nil {
		t.Fatal("expected conversation request to be captured")
	}
	if captured.SessionId != "conversation-7" {
		t.Fatalf("unexpected session id: %s", captured.SessionId)
	}
	if captured.UserId != "user-7" {
		t.Fatalf("unexpected user id: %s", captured.UserId)
	}
	if !reflect.DeepEqual(captured.UserRoles, []string{"oncall"}) {
		t.Fatalf("unexpected user roles: %#v", captured.UserRoles)
	}
	if len(captured.History) != 1 || captured.History[0].Role != "user" {
		t.Fatalf("unexpected history: %#v", captured.History)
	}
	if captured.LinkedAlert == nil || captured.LinkedAlert.AlertId != "alert-9" {
		t.Fatalf("unexpected linked alert: %#v", captured.LinkedAlert)
	}
	if !reflect.DeepEqual(captured.AllowedTools, []string{"service_status"}) {
		t.Fatalf("unexpected allowed tools: %#v", captured.AllowedTools)
	}
	if captured.RetrievalLimit != 3 {
		t.Fatalf("unexpected retrieval limit: %d", captured.RetrievalLimit)
	}
	if captured.Metadata == nil {
		t.Fatal("expected metadata")
	}
	if captured.Metadata.UserId != "user-7" {
		t.Fatalf("unexpected metadata user id: %s", captured.Metadata.UserId)
	}
	if captured.Metadata.SessionId != "conversation-7" {
		t.Fatalf("unexpected metadata session id: %s", captured.Metadata.SessionId)
	}
}

func TestRuntimeCallsRespectGatewayTimeouts(t *testing.T) {
	blocking := &fakeRuntimeClient{
		analyze: func(ctx context.Context, _ *aipb.AnalyzeAlertRequest) (*aipb.AnalyzeAlertResponse, error) {
			if _, ok := ctx.Deadline(); !ok {
				return nil, errors.New("missing deadline")
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
		chat: func(ctx context.Context, _ *aipb.RunConversationTurnRequest) (*aipb.RunConversationTurnResponse, error) {
			if _, ok := ctx.Deadline(); !ok {
				return nil, errors.New("missing deadline")
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	client := &HTTPClient{
		runtimeClient:  blocking,
		runtimeTimeout: 30 * time.Millisecond,
	}

	run := func(name string, fn func() error) {
		t.Helper()
		done := make(chan error, 1)
		go func() {
			done <- fn()
		}()

		select {
		case err := <-done:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("%s: expected deadline exceeded, got %v", name, err)
			}
		case <-time.After(250 * time.Millisecond):
			t.Fatalf("%s: call did not return within gateway timeout", name)
		}
	}

	run("AnalyzeAlert", func() error {
		_, err := client.AnalyzeAlert(context.Background(), AlertAnalysisRequest{})
		return err
	})

	run("Chat", func() error {
		_, err := client.Chat(context.Background(), ChatRequest{})
		return err
	})
}

func TestAnalyzeAlertReturnsUnavailableWhenRuntimeMissing(t *testing.T) {
	client := &HTTPClient{
		runtimeErr: status.Error(codes.Unavailable, "python ai runtime unavailable"),
	}

	_, err := client.AnalyzeAlert(context.Background(), AlertAnalysisRequest{
		AlertID: "alert-1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable error, got %v", err)
	}
}

func TestHealthUsesHTTPHealthReport(t *testing.T) {
	client := &HTTPClient{
		baseURL: "http://python-ai.internal",
		healthClient: &http.Client{
			Timeout: 100 * time.Millisecond,
			Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != "http://python-ai.internal/healthz" {
					t.Fatalf("unexpected url: %s", request.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"code":"OK","message":"success","data":{"service":"python-ai","env":"test","status":"degraded","components":[{"name":"llm","status":"up","detail":"reachable"},{"name":"milvus","status":"down","detail":"connection failed"}]}}`,
					)),
				}, nil
			}),
		},
	}

	report, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if report.Service != "python-ai" {
		t.Fatalf("unexpected service: %s", report.Service)
	}
	if report.Env != "test" {
		t.Fatalf("unexpected env: %s", report.Env)
	}
	if report.Status != "degraded" {
		t.Fatalf("unexpected status: %s", report.Status)
	}
	if len(report.Components) != 2 {
		t.Fatalf("unexpected components: %#v", report.Components)
	}
	if report.Components[1].Name != "milvus" || report.Components[1].Status != "down" {
		t.Fatalf("unexpected component detail: %#v", report.Components[1])
	}
}
