package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	aipb "github.com/oyzg/OnCall/backend/go-api/gen/proto/ai"
	commonpb "github.com/oyzg/OnCall/backend/go-api/gen/proto/common"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/grpcclient"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
)

type ChatRequest struct {
	Query          string        `json:"query"`
	ConversationID string        `json:"conversation_id"`
	UserID         string        `json:"user_id,omitempty"`
	UserRoles      []string      `json:"user_roles,omitempty"`
	History        []ChatMessage `json:"history,omitempty"`
	LinkedAlert    *LinkedAlert  `json:"linked_alert,omitempty"`
	AllowedTools   []string      `json:"allowed_tools,omitempty"`
	RetrievalLimit int           `json:"retrieval_limit,omitempty"`
}

type ChatResponse struct {
	Answer        string         `json:"answer"`
	Citations     []string       `json:"citations"`
	CitationItems []ChatCitation `json:"citation_items,omitempty"`
	Route         string         `json:"route,omitempty"`
	Status        string         `json:"status,omitempty"`
	Error         string         `json:"error,omitempty"`
	ToolCalls     []ToolCall     `json:"tool_calls,omitempty"`
	Trace         []TraceEvent   `json:"trace,omitempty"`
}

type ChatMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	AuthorID  string         `json:"author_id,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	Citations []ChatCitation `json:"citations,omitempty"`
}

type ChatCitation struct {
	Source     string  `json:"source,omitempty"`
	Title      string  `json:"title,omitempty"`
	URL        string  `json:"url,omitempty"`
	Snippet    string  `json:"snippet,omitempty"`
	DocumentID string  `json:"document_id,omitempty"`
	Score      float64 `json:"score,omitempty"`
}

type LinkedAlert struct {
	AlertID         string            `json:"alert_id,omitempty"`
	Title           string            `json:"title,omitempty"`
	Service         string            `json:"service,omitempty"`
	Environment     string            `json:"environment,omitempty"`
	Severity        string            `json:"severity,omitempty"`
	Source          string            `json:"source,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Description     string            `json:"description,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	TriggeredAt     string            `json:"triggered_at,omitempty"`
	LinkedSessionID string            `json:"linked_session_id,omitempty"`
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
	UserID          string            `json:"user_id,omitempty"`
	UserRoles       []string          `json:"user_roles,omitempty"`
}

type AlertAnalysisResponse struct {
	Status             string       `json:"status"`
	Summary            string       `json:"summary"`
	SeverityAssessment string       `json:"severity_assessment"`
	PossibleCauses     []string     `json:"possible_causes"`
	SuggestedActions   []string     `json:"suggested_actions"`
	RecommendedTools   []string     `json:"recommended_tools"`
	KnowledgeQueries   []string     `json:"knowledge_queries"`
	Workflow           string       `json:"workflow"`
	Confidence         string       `json:"confidence"`
	Source             string       `json:"source"`
	GeneratedAt        string       `json:"generated_at"`
	Error              string       `json:"error"`
	Trace              []TraceEvent `json:"trace,omitempty"`
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
	baseURL        string
	httpClient     *http.Client
	indexClient    *http.Client
	healthClient   *http.Client
	runtimeClient  aipb.RuntimeServiceClient
	runtimeErr     error
	runtimeTimeout time.Duration
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

	grpcClient := grpcclient.NewClient(cfg.GRPCTarget)
	runtimeClient, runtimeErr := grpcClient.RuntimeClient()

	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		indexClient: &http.Client{
			Timeout: indexTimeout,
		},
		healthClient: &http.Client{
			Timeout: pingTimeout(cfg.PingTimeout, timeout),
		},
		runtimeClient:  runtimeClient,
		runtimeErr:     runtimeErr,
		runtimeTimeout: timeout,
	}
}

func (c *HTTPClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	ctx, cancel := c.runtimeCallContext(ctx, c.runtimeTimeout)
	defer cancel()

	client, err := c.grpcRuntimeClient()
	if err != nil {
		return ChatResponse{}, err
	}

	response, err := client.RunConversationTurn(ctx, &aipb.RunConversationTurnRequest{
		Metadata: &commonpb.RequestMetadata{
			SessionId: request.ConversationID,
			UserId:    request.UserID,
		},
		SessionId:      request.ConversationID,
		UserId:         request.UserID,
		UserRoles:      append([]string(nil), request.UserRoles...),
		Message:        request.Query,
		History:        chatHistoryToProto(request.History),
		LinkedAlert:    linkedAlertToProto(request.LinkedAlert),
		AllowedTools:   append([]string(nil), request.AllowedTools...),
		RetrievalLimit: int32(request.RetrievalLimit),
	})
	if err != nil {
		return ChatResponse{}, err
	}

	citations := make([]string, 0, len(response.GetCitations()))
	for _, citation := range response.GetCitations() {
		if label := citationLabel(citation); label != "" {
			citations = append(citations, label)
		}
	}

	return ChatResponse{
		Answer:        response.GetAnswer(),
		Citations:     citations,
		CitationItems: chatCitationsFromProto(response.GetCitations()),
		Route:         response.GetRoute(),
		Status:        response.GetStatus(),
		Error:         response.GetError(),
		ToolCalls:     chatToolCallsFromProto(response.GetToolCalls()),
		Trace:         traceEventsFromProto(response.GetTrace()),
	}, nil
}

func (c *HTTPClient) AnalyzeAlert(ctx context.Context, request AlertAnalysisRequest) (AlertAnalysisResponse, error) {
	ctx, cancel := c.runtimeCallContext(ctx, c.runtimeTimeout)
	defer cancel()

	client, err := c.grpcRuntimeClient()
	if err != nil {
		return AlertAnalysisResponse{}, err
	}

	response, err := client.AnalyzeAlert(ctx, &aipb.AnalyzeAlertRequest{
		Metadata: &commonpb.RequestMetadata{
			SessionId: request.LinkedSessionID,
			UserId:    request.UserID,
		},
		AlertId:         request.AlertID,
		Title:           request.Title,
		Service:         request.Service,
		Environment:     request.Environment,
		Severity:        request.Severity,
		Source:          request.Source,
		Summary:         request.Summary,
		Description:     request.Description,
		Labels:          mapLabels(request.Labels),
		TriggeredAt:     request.TriggeredAt,
		LinkedSessionId: request.LinkedSessionID,
		UserId:          request.UserID,
		UserRoles:       append([]string(nil), request.UserRoles...),
	})
	if err != nil {
		return AlertAnalysisResponse{}, err
	}

	return AlertAnalysisResponse{
		Status:             response.GetStatus(),
		Summary:            response.GetSummary(),
		SeverityAssessment: response.GetSeverityAssessment(),
		PossibleCauses:     append([]string(nil), response.GetPossibleCauses()...),
		SuggestedActions:   append([]string(nil), response.GetSuggestedActions()...),
		RecommendedTools:   append([]string(nil), response.GetRecommendedTools()...),
		KnowledgeQueries:   append([]string(nil), response.GetKnowledgeQueries()...),
		Workflow:           response.GetWorkflow(),
		Confidence:         confidenceLabel(response.GetConfidence()),
		Source:             response.GetSource(),
		GeneratedAt:        response.GetGeneratedAt(),
		Error:              response.GetError(),
		Trace:              traceEventsFromProto(response.GetTrace()),
	}, nil
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
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return HealthResponse{}, err
	}

	response, err := c.healthClient.Do(httpRequest)
	if err != nil {
		return HealthResponse{}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return HealthResponse{}, err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return HealthResponse{}, fmt.Errorf("%s failed: %s", c.baseURL+"/healthz", strings.TrimSpace(string(body)))
	}

	var parsed envelope[HealthResponse]
	if err := json.Unmarshal(body, &parsed); err != nil {
		return HealthResponse{}, err
	}

	return parsed.Data, nil
}

func (c *HTTPClient) runtimeCallContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (c *HTTPClient) grpcRuntimeClient() (aipb.RuntimeServiceClient, error) {
	if c == nil {
		return nil, fmt.Errorf("grpc runtime client unavailable")
	}
	if c.runtimeErr != nil {
		return nil, c.runtimeErr
	}
	if c.runtimeClient == nil {
		return nil, fmt.Errorf("grpc runtime client unavailable")
	}
	return c.runtimeClient, nil
}

func pingTimeout(fallback, runtimeTimeout time.Duration) time.Duration {
	if fallback > 0 {
		return fallback
	}
	if runtimeTimeout > 0 {
		return runtimeTimeout
	}
	return 2 * time.Second
}

func mapLabels(labels map[string]string) []string {
	if len(labels) == 0 {
		return nil
	}

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+labels[key])
	}
	return result
}

func citationLabel(citation *aipb.Citation) string {
	if citation == nil {
		return ""
	}
	if title := strings.TrimSpace(citation.GetTitle()); title != "" {
		return title
	}
	if source := strings.TrimSpace(citation.GetSource()); source != "" {
		return source
	}
	if documentID := strings.TrimSpace(citation.GetDocumentId()); documentID != "" {
		return documentID
	}
	if url := strings.TrimSpace(citation.GetUrl()); url != "" {
		return url
	}
	return strings.TrimSpace(citation.GetSnippet())
}

func confidenceLabel(confidence float32) string {
	switch {
	case confidence >= 0.8:
		return "high"
	case confidence >= 0.5:
		return "medium"
	default:
		return "low"
	}
}

func chatHistoryToProto(history []ChatMessage) []*aipb.ConversationMessage {
	if len(history) == 0 {
		return nil
	}

	result := make([]*aipb.ConversationMessage, 0, len(history))
	for _, item := range history {
		result = append(result, &aipb.ConversationMessage{
			Role:      item.Role,
			Content:   item.Content,
			AuthorId:  item.AuthorID,
			CreatedAt: item.CreatedAt,
			Citations: chatCitationsToProto(item.Citations),
		})
	}
	return result
}

func chatCitationsToProto(citations []ChatCitation) []*aipb.Citation {
	if len(citations) == 0 {
		return nil
	}

	result := make([]*aipb.Citation, 0, len(citations))
	for _, item := range citations {
		result = append(result, &aipb.Citation{
			Source:     item.Source,
			Title:      item.Title,
			Url:        item.URL,
			Snippet:    item.Snippet,
			DocumentId: item.DocumentID,
			Score:      float32(item.Score),
		})
	}
	return result
}

func linkedAlertToProto(linkedAlert *LinkedAlert) *aipb.LinkedAlert {
	if linkedAlert == nil {
		return nil
	}
	return &aipb.LinkedAlert{
		AlertId:         linkedAlert.AlertID,
		Title:           linkedAlert.Title,
		Service:         linkedAlert.Service,
		Environment:     linkedAlert.Environment,
		Severity:        linkedAlert.Severity,
		Source:          linkedAlert.Source,
		Summary:         linkedAlert.Summary,
		Description:     linkedAlert.Description,
		Labels:          mapLabels(linkedAlert.Labels),
		TriggeredAt:     linkedAlert.TriggeredAt,
		LinkedSessionId: linkedAlert.LinkedSessionID,
	}
}

func chatToolCallsFromProto(toolCalls []*aipb.ToolCall) []ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]ToolCall, 0, len(toolCalls))
	for _, item := range toolCalls {
		if item == nil {
			continue
		}
		result = append(result, ToolCall{
			Name:          item.GetName(),
			ArgumentsJSON: item.GetArgumentsJson(),
			Outcome:       item.GetOutcome(),
			Summary:       item.GetSummary(),
		})
	}
	return result
}

func chatCitationsFromProto(citations []*aipb.Citation) []ChatCitation {
	if len(citations) == 0 {
		return nil
	}

	result := make([]ChatCitation, 0, len(citations))
	for _, item := range citations {
		if item == nil {
			continue
		}
		result = append(result, ChatCitation{
			Source:     item.GetSource(),
			Title:      item.GetTitle(),
			URL:        item.GetUrl(),
			Snippet:    item.GetSnippet(),
			DocumentID: item.GetDocumentId(),
			Score:      float64(item.GetScore()),
		})
	}
	return result
}

func traceEventsFromProto(trace []*aipb.TraceEvent) []TraceEvent {
	if len(trace) == 0 {
		return nil
	}

	result := make([]TraceEvent, 0, len(trace))
	for _, item := range trace {
		if item == nil {
			continue
		}
		result = append(result, TraceEvent{
			Stage:     item.GetStage(),
			Message:   item.GetMessage(),
			Severity:  item.GetSeverity(),
			Timestamp: item.GetTimestamp(),
			Tags:      append([]string(nil), item.GetTags()...),
		})
	}
	return result
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
