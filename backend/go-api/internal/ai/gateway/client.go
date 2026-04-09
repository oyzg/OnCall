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

type RAGIndexChunk struct {
	DocumentID    string `json:"document_id"`
	DocumentTitle string `json:"document_title"`
	Category      string `json:"category"`
	Index         int    `json:"index"`
	Content       string `json:"content"`
}

type RAGIndexRequest struct {
	UserID        string          `json:"user_id"`
	DocumentID    string          `json:"document_id"`
	DocumentTitle string          `json:"document_title"`
	Category      string          `json:"category"`
	Chunks        []RAGIndexChunk `json:"chunks"`
}

type RAGIndexResponse struct {
	Status           string `json:"status"`
	IndexedChunks    int    `json:"indexed_chunks"`
	EmbeddingBackend string `json:"embedding_backend"`
	VectorBackend    string `json:"vector_backend"`
	LexicalBackend   string `json:"lexical_backend"`
}

type RAGDeleteRequest struct {
	DocumentID string `json:"document_id"`
}

type RAGDeleteResponse struct {
	Status     string `json:"status"`
	DocumentID string `json:"document_id"`
}

type RAGRetrieveRequest struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	Limit    int    `json:"limit"`
}

type RAGReference struct {
	DocumentID    string   `json:"document_id"`
	DocumentTitle string   `json:"document_title"`
	Category      string   `json:"category"`
	ChunkIndex    int      `json:"chunk_index"`
	Chunk         string   `json:"chunk"`
	Score         float64  `json:"score"`
	LexicalScore  float64  `json:"lexical_score"`
	SemanticScore float64  `json:"semantic_score"`
	BoostScore    float64  `json:"boost_score"`
	MatchReasons  []string `json:"match_reasons"`
}

type RAGRetrieveResponse struct {
	Query              string         `json:"query"`
	RewrittenQuery     string         `json:"rewritten_query"`
	QueryTerms         []string       `json:"query_terms"`
	ExpandedTerms      []string       `json:"expanded_terms"`
	Answer             string         `json:"answer"`
	References         []RAGReference `json:"references"`
	ScannedDocs        int            `json:"scanned_docs"`
	ScannedChunks      int            `json:"scanned_chunks"`
	MatchedChunks      int            `json:"matched_chunks"`
	LexicalCandidates  int            `json:"lexical_candidates"`
	SemanticCandidates int            `json:"semantic_candidates"`
	RerankedChunks     int            `json:"reranked_chunks"`
	Strategy           string         `json:"strategy"`
	RequestedLimit     int            `json:"requested_limit"`
	EmbeddingBackend   string         `json:"embedding_backend"`
	VectorBackend      string         `json:"vector_backend"`
	LexicalBackend     string         `json:"lexical_backend"`
}

type HealthComponent struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type HealthResponse struct {
	Service    string            `json:"service"`
	Env        string            `json:"env"`
	Status     string            `json:"status"`
	Components []HealthComponent `json:"components"`
}

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
	AnalyzeAlert(ctx context.Context, request AlertAnalysisRequest) (AlertAnalysisResponse, error)
	IndexKnowledge(ctx context.Context, request RAGIndexRequest) (RAGIndexResponse, error)
	DeleteKnowledge(ctx context.Context, request RAGDeleteRequest) (RAGDeleteResponse, error)
	RetrieveKnowledge(ctx context.Context, request RAGRetrieveRequest) (RAGRetrieveResponse, error)
	Health(ctx context.Context) (HealthResponse, error)
}

type HTTPClient struct {
	baseURL     string
	httpClient  *http.Client
	indexClient *http.Client
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

	indexTimeout := cfg.IndexTimeout
	if indexTimeout <= 0 {
		indexTimeout = timeout
	}

	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		indexClient: &http.Client{
			Timeout: indexTimeout,
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

func (c *HTTPClient) IndexKnowledge(ctx context.Context, request RAGIndexRequest) (RAGIndexResponse, error) {
	return postJSON[RAGIndexResponse](ctx, c.indexClient, c.baseURL+"/api/v1/rag/index", request)
}

func (c *HTTPClient) RetrieveKnowledge(ctx context.Context, request RAGRetrieveRequest) (RAGRetrieveResponse, error) {
	return postJSON[RAGRetrieveResponse](ctx, c.httpClient, c.baseURL+"/api/v1/rag/retrieve", request)
}

func (c *HTTPClient) DeleteKnowledge(ctx context.Context, request RAGDeleteRequest) (RAGDeleteResponse, error) {
	return postJSON[RAGDeleteResponse](ctx, c.httpClient, c.baseURL+"/api/v1/rag/delete", request)
}

func (c *HTTPClient) Health(ctx context.Context) (HealthResponse, error) {
	var zero HealthResponse

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return zero, err
	}

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return zero, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return zero, fmt.Errorf("%s failed: %s", c.baseURL+"/healthz", strings.TrimSpace(string(body)))
	}

	var parsed envelope[HealthResponse]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return zero, err
	}
	return parsed.Data, nil
}

func postJSON[T any](ctx context.Context, client *http.Client, url string, requestBody any) (T, error) {
	var zero T

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return zero, err
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(payload),
	)
	if err != nil {
		return zero, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := client.Do(httpRequest)
	if err != nil {
		return zero, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return zero, fmt.Errorf("%s failed: %s", url, strings.TrimSpace(string(body)))
	}

	var parsed envelope[T]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return zero, err
	}

	return parsed.Data, nil
}
