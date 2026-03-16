package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
)

type ChatRequest struct {
	Query          string `json:"query"`
	ConversationID string `json:"conversation_id"`
}

type ChatResponse struct {
	Answer    string   `json:"answer"`
	Citations []string `json:"citations"`
}

type AlertAnalysisRequest struct {
	AlertID         string            `json:"alert_id"`
	Title           string            `json:"title"`
	Service         string            `json:"service"`
	Environment     string            `json:"environment"`
	Severity        string            `json:"severity"`
	Source          string            `json:"source"`
	Summary         string            `json:"summary"`
	Description     string            `json:"description"`
	Labels          map[string]string `json:"labels"`
	TriggeredAt     string            `json:"triggered_at"`
	LinkedSessionID string            `json:"linked_session_id"`
}

type AlertAnalysisResponse struct {
	Status             string   `json:"status"`
	Summary            string   `json:"summary"`
	SeverityAssessment string   `json:"severity_assessment"`
	PossibleCauses     []string `json:"possible_causes"`
	SuggestedActions   []string `json:"suggested_actions"`
	RecommendedTools   []string `json:"recommended_tools"`
	KnowledgeQueries   []string `json:"knowledge_queries"`
	Workflow           string   `json:"workflow"`
	Confidence         string   `json:"confidence"`
	Source             string   `json:"source"`
	GeneratedAt        string   `json:"generated_at"`
	Error              string   `json:"error"`
}

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
	AnalyzeAlert(ctx context.Context, request AlertAnalysisRequest) (AlertAnalysisResponse, error)
}

type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

type envelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func NewHTTPClient(cfg config.AIConfig) *HTTPClient {
	baseURL := strings.TrimRight(cfg.HTTPBaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) Chat(_ context.Context, _ ChatRequest) (ChatResponse, error) {
	return ChatResponse{}, nil
}

func (c *HTTPClient) AnalyzeAlert(ctx context.Context, request AlertAnalysisRequest) (AlertAnalysisResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return AlertAnalysisResponse{}, err
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/analysis/alert",
		bytes.NewReader(payload),
	)
	if err != nil {
		return AlertAnalysisResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return AlertAnalysisResponse{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return AlertAnalysisResponse{}, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return AlertAnalysisResponse{}, fmt.Errorf("python ai analyze alert failed: %s", strings.TrimSpace(string(body)))
	}

	var parsed envelope[AlertAnalysisResponse]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return AlertAnalysisResponse{}, err
	}

	return parsed.Data, nil
}
