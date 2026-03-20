package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
	"github.com/oyzg/OnCall/backend/go-api/pkg/logger"
)

type envelope[T any] struct {
	Code string `json:"code"`
	Data T      `json:"data"`
}

func TestIntegrationCoreWorkflows(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempRoot := t.TempDir()
	if err := os.Chdir(tempRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	gin.SetMode(gin.TestMode)
	router := New(config.Config{
		App: config.AppConfig{
			Name:     "go-api",
			Env:      "test",
			LogLevel: "ERROR",
		},
		MySQL: config.MySQLConfig{
			Enabled:     true,
			Driver:      "sqlite",
			DSN:         filepath.Join(tempRoot, "integration.db"),
			PingTimeout: time.Second,
			AutoMigrate: true,
		},
		AI: config.AIConfig{
			HTTPBaseURL: "http://127.0.0.1:65535",
			HTTPTimeout: 100 * time.Millisecond,
			GRPCTarget:  "127.0.0.1:50051",
			PingTimeout: time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:      "integration-secret",
			TokenExpiresIn: 24 * time.Hour,
		},
	}, logger.New("ERROR"))

	assertStatus(t, router, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, "", "application/json", http.StatusUnauthorized)

	token := loginAndGetToken(t, router)

	sessionID := createSession(t, router, token, "integration session")
	streamBody := request(t, router, http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages/stream", `{"content":"请帮我分析 user-service error ratio increased"}`, token, "application/json", http.StatusOK)
	if !strings.Contains(streamBody, "event: done") {
		t.Fatalf("expected SSE done event, got %s", streamBody)
	}

	uploadKnowledgeDocument(t, router, token, "User Service SOP", "runbook", "user-service error ratio increased handling guide")
	waitForKnowledgeReady(t, router, token)

	retrievalBody := request(t, router, http.MethodPost, "/api/v1/rag/retrieve", `{"query":"user-service error ratio increased","limit":3}`, token, "application/json", http.StatusOK)
	if !strings.Contains(retrievalBody, `"references"`) {
		t.Fatalf("expected retrieval references, got %s", retrievalBody)
	}

	alertID := firstAlertID(t, router, token)
	analyzeBody := request(t, router, http.MethodPost, "/api/v1/alerts/"+alertID+"/analyze", "", token, "application/json", http.StatusOK)
	if !strings.Contains(analyzeBody, `"status":"failed"`) {
		t.Fatalf("expected fallback analysis result, got %s", analyzeBody)
	}

	toolBody := request(t, router, http.MethodPost, "/api/v1/tools/knowledge_search/call", `{"parameters":{"query":"user-service error ratio increased","limit":2}}`, token, "application/json", http.StatusOK)
	if !strings.Contains(toolBody, `"answer"`) {
		t.Fatalf("expected tool result with answer, got %s", toolBody)
	}

	auditStatsBody := request(t, router, http.MethodGet, "/api/v1/audit/stats", "", token, "", http.StatusOK)
	if !strings.Contains(auditStatsBody, `"total"`) {
		t.Fatalf("expected audit stats payload, got %s", auditStatsBody)
	}

	auditLogsBody := request(t, router, http.MethodGet, "/api/v1/audit/logs?limit=20", "", token, "", http.StatusOK)
	if !strings.Contains(auditLogsBody, `"category":"auth"`) || !strings.Contains(auditLogsBody, `"category":"tool"`) {
		t.Fatalf("expected audit logs to include auth and tool records, got %s", auditLogsBody)
	}

	restartedRouter := New(config.Config{
		App: config.AppConfig{
			Name:     "go-api",
			Env:      "test",
			LogLevel: "ERROR",
		},
		MySQL: config.MySQLConfig{
			Enabled:     true,
			Driver:      "sqlite",
			DSN:         filepath.Join(tempRoot, "integration.db"),
			PingTimeout: time.Second,
			AutoMigrate: true,
		},
		AI: config.AIConfig{
			HTTPBaseURL: "http://127.0.0.1:65535",
			HTTPTimeout: 100 * time.Millisecond,
			GRPCTarget:  "127.0.0.1:50051",
			PingTimeout: time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:      "integration-secret",
			TokenExpiresIn: 24 * time.Hour,
		},
	}, logger.New("ERROR"))

	restartedToken := loginAndGetToken(t, restartedRouter)
	sessionsBody := request(t, restartedRouter, http.MethodGet, "/api/v1/sessions", "", restartedToken, "", http.StatusOK)
	if !strings.Contains(sessionsBody, sessionID) {
		t.Fatalf("expected persisted session after restart, got %s", sessionsBody)
	}
	documentsBody := request(t, restartedRouter, http.MethodGet, "/api/v1/knowledge/documents", "", restartedToken, "", http.StatusOK)
	if !strings.Contains(documentsBody, "User Service SOP") {
		t.Fatalf("expected persisted knowledge document after restart, got %s", documentsBody)
	}
	alertsBody := request(t, restartedRouter, http.MethodGet, "/api/v1/alerts", "", restartedToken, "", http.StatusOK)
	if !strings.Contains(alertsBody, alertID) {
		t.Fatalf("expected persisted alert after restart, got %s", alertsBody)
	}
}

func loginAndGetToken(t *testing.T, router *gin.Engine) string {
	t.Helper()

	body := request(t, router, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"OnCallAdmin2026!"}`, "", "application/json", http.StatusOK)

	var payload envelope[struct {
		AccessToken struct {
			Token string `json:"token"`
		} `json:"access_token"`
	}]
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.AccessToken.Token == "" {
		t.Fatalf("expected access token, got %s", body)
	}
	return payload.Data.AccessToken.Token
}

func createSession(t *testing.T, router *gin.Engine, token, title string) string {
	t.Helper()

	body := request(t, router, http.MethodPost, "/api/v1/sessions", `{"title":"`+title+`"}`, token, "application/json", http.StatusCreated)

	var payload envelope[struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}]
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Session.ID == "" {
		t.Fatalf("expected session id, got %s", body)
	}
	return payload.Data.Session.ID
}

func uploadKnowledgeDocument(t *testing.T, router *gin.Engine, token, title, category, content string) {
	t.Helper()

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	_ = writer.WriteField("title", title)
	_ = writer.WriteField("category", category)
	_ = writer.WriteField("content", content)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/documents", &payload)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func waitForKnowledgeReady(t *testing.T, router *gin.Engine, token string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		body := request(t, router, http.MethodGet, "/api/v1/knowledge/documents", "", token, "", http.StatusOK)
		if strings.Contains(body, `"status":"ready"`) {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}

	t.Fatal("knowledge document did not become ready in time")
}

func firstAlertID(t *testing.T, router *gin.Engine, token string) string {
	t.Helper()

	body := request(t, router, http.MethodGet, "/api/v1/alerts", "", token, "", http.StatusOK)
	var payload envelope[struct {
		Alerts []struct {
			ID string `json:"id"`
		} `json:"alerts"`
	}]
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Alerts) == 0 {
		t.Fatalf("expected seeded alerts, got %s", body)
	}
	return payload.Data.Alerts[0].ID
}

func assertStatus(t *testing.T, router *gin.Engine, method, path, body, token, contentType string, expected int) {
	t.Helper()
	request(t, router, method, path, body, token, contentType, expected)
}

func request(t *testing.T, router *gin.Engine, method, path, body, token, contentType string, expectedStatus int) string {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != expectedStatus {
		t.Fatalf("%s %s expected %d got %d: %s", method, path, expectedStatus, recorder.Code, recorder.Body.String())
	}
	return recorder.Body.String()
}
