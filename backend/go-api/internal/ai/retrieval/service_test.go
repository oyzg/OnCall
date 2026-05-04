package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/ai/gateway"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
)

func TestBuildQueryPlanExpandsOperationalTerms(t *testing.T) {
	plan := buildQueryPlan("payment-api p95 timeout")

	if plan.Rewritten == "" {
		t.Fatal("expected rewritten query")
	}
	if !contains(plan.Expanded, "latency") {
		t.Fatalf("expected expanded terms to include latency, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "延迟") {
		t.Fatalf("expected expanded terms to include 延迟, got %#v", plan.Expanded)
	}
}

func TestBuildQueryPlanExpandsChineseSlowIncidentPhrases(t *testing.T) {
	plan := buildQueryPlan("支付服务卡了")

	if !contains(plan.Expanded, "延迟") {
		t.Fatalf("expected expanded terms to include 延迟, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "timeout") {
		t.Fatalf("expected expanded terms to include timeout, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "latency") {
		t.Fatalf("expected expanded terms to include latency, got %#v", plan.Expanded)
	}
}

func TestBuildQueryPlanExpandsChineseServiceAliases(t *testing.T) {
	plan := buildQueryPlan("用户服务 SOP")

	if !contains(plan.Expanded, "user-service") {
		t.Fatalf("expected expanded terms to include user-service, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "user service") {
		t.Fatalf("expected expanded terms to include user service, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "runbook") {
		t.Fatalf("expected expanded terms to include runbook, got %#v", plan.Expanded)
	}
}

func TestBuildQueryPlanExpandsLoginErrorPhrases(t *testing.T) {
	plan := buildQueryPlan("登录接口 5xx")

	if !contains(plan.Expanded, "auth") {
		t.Fatalf("expected expanded terms to include auth, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "authentication") {
		t.Fatalf("expected expanded terms to include authentication, got %#v", plan.Expanded)
	}
	if !contains(plan.Expanded, "error") {
		t.Fatalf("expected expanded terms to include error, got %#v", plan.Expanded)
	}
}

func TestRetrieveWithOptionsUsesHybridRanking(t *testing.T) {
	tempDir := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	knowledgeService := knowledgeApp.NewService()
	retrievalService := NewService(knowledgeService)
	user := authDomain.User{ID: "user_test", Username: "tester"}

	_, err = knowledgeService.UploadDocument(user, knowledgeApp.UploadInput{
		Title:      "payment runbook",
		Category:   "runbook",
		SourceType: "text",
		Content: []byte("payment-api p95 latency spike often indicates downstream timeout or database pool saturation. " +
			"Check mysql connections, downstream dependency latency, and recent releases first."),
	})
	if err != nil {
		t.Fatalf("upload payment runbook: %v", err)
	}

	_, err = knowledgeService.UploadDocument(user, knowledgeApp.UploadInput{
		Title:      "user error runbook",
		Category:   "incident",
		SourceType: "text",
		Content:    []byte("user-service 5xx ratio can increase after release or dependency exception. Check grafana, logs, and rollback history."),
	})
	if err != nil {
		t.Fatalf("upload user runbook: %v", err)
	}

	waitUntilReady(t, knowledgeService, user)

	report := retrievalService.RetrieveWithOptions(user, "payment-api p95 timeout", RetrieveOptions{
		Limit:    3,
		Category: "runbook",
	})

	if report.Strategy != "hybrid_lexical_semantic_rerank" {
		t.Fatalf("unexpected strategy: %s", report.Strategy)
	}
	if report.RewrittenQuery == "" {
		t.Fatal("expected rewritten query to be populated")
	}
	if len(report.References) == 0 {
		t.Fatal("expected retrieval references")
	}
	if report.LexicalCandidates == 0 {
		t.Fatal("expected lexical candidates to be counted")
	}
	if report.SemanticCandidates == 0 {
		t.Fatal("expected semantic candidates to be counted")
	}
	if report.References[0].DocumentTitle != "payment runbook" {
		t.Fatalf("expected payment runbook to rank first, got %s", report.References[0].DocumentTitle)
	}
	if report.References[0].LexicalScore <= 0 {
		t.Fatalf("expected lexical score > 0, got %f", report.References[0].LexicalScore)
	}
	if len(report.References[0].MatchReasons) == 0 {
		t.Fatal("expected match reasons")
	}
	if !contains(report.References[0].MatchReasons, "关键词命中") {
		t.Fatalf("expected human readable lexical reason, got %#v", report.References[0].MatchReasons)
	}
	if !contains(report.References[0].MatchReasons, "语义命中") {
		t.Fatalf("expected human readable semantic reason, got %#v", report.References[0].MatchReasons)
	}
}

func TestRetrieveWithOptionsUsesRemoteHybridReportWithoutReferences(t *testing.T) {
	knowledgeService := knowledgeApp.NewService()
	retrievalService := NewService(knowledgeService)
	retrievalService.SetRemoteRetriever(stubRemoteRetriever{
		response: gateway.RAGRetrieveResponse{
			Query:            "payment-api timeout",
			RewrittenQuery:   "payment api timeout latency 延迟",
			Answer:           "当前 Embedding + Milvus + Elasticsearch 混合检索没有命中直接相关的片段。",
			References:       nil,
			Strategy:         "embedding_milvus_es_hybrid",
			RequestedLimit:   4,
			EmbeddingBackend: "openai:text-embedding-3-small",
			VectorBackend:    "milvus",
			LexicalBackend:   "elasticsearch",
		},
	})

	user := authDomain.User{ID: "user_test", Username: "tester"}
	report := retrievalService.RetrieveWithOptions(user, "payment-api timeout", RetrieveOptions{Limit: 4})

	if report.Strategy != "embedding_milvus_es_hybrid" {
		t.Fatalf("expected remote strategy, got %s", report.Strategy)
	}
	if report.EmbeddingBackend != "openai:text-embedding-3-small" {
		t.Fatalf("expected remote embedding backend, got %s", report.EmbeddingBackend)
	}
	if report.VectorBackend != "milvus" {
		t.Fatalf("expected remote vector backend, got %s", report.VectorBackend)
	}
	if report.LexicalBackend != "elasticsearch" {
		t.Fatalf("expected remote lexical backend, got %s", report.LexicalBackend)
	}
	if report.Answer == "" {
		t.Fatal("expected remote answer to be preserved")
	}
}

func TestRetrieveWithOptionsMatchesTitleStyleChineseAliasQuery(t *testing.T) {
	tempDir := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	knowledgeService := knowledgeApp.NewService()
	retrievalService := NewService(knowledgeService)
	user := authDomain.User{ID: "user_test", Username: "tester"}

	_, err = knowledgeService.UploadDocument(user, knowledgeApp.UploadInput{
		Title:      "User Service SOP",
		Category:   "runbook",
		SourceType: "text",
		Content: []byte("Review application logs for 5xx spikes, timeout patterns, and upstream authentication provider health. " +
			"Check recent releases, dependency failures, and rollback history."),
	})
	if err != nil {
		t.Fatalf("upload user service sop: %v", err)
	}

	waitUntilReadySingle(t, knowledgeService, user)

	report := retrievalService.RetrieveWithOptions(user, "用户服务 SOP", RetrieveOptions{Limit: 3})
	if len(report.References) == 0 {
		t.Fatal("expected retrieval references for title-style query")
	}
	if report.References[0].DocumentTitle != "User Service SOP" {
		t.Fatalf("expected User Service SOP to rank first, got %s", report.References[0].DocumentTitle)
	}
}

func waitUntilReady(t *testing.T, knowledgeService *knowledgeApp.Service, user authDomain.User) {
	t.Helper()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		documents := knowledgeService.ListDocuments(user, "", "", "", 0)
		if len(documents) >= 2 {
			readyCount := 0
			for _, document := range documents {
				if document.Status == "ready" {
					readyCount++
				}
			}
			if readyCount == 2 {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	chunksDir := filepath.Join(tempDirFallback(), "tmp", "knowledge", "chunks")
	t.Fatalf("documents were not ready before timeout, chunks dir: %s", chunksDir)
}

func waitUntilReadySingle(t *testing.T, knowledgeService *knowledgeApp.Service, user authDomain.User) {
	t.Helper()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		documents := knowledgeService.ListDocuments(user, "", "", "", 0)
		if len(documents) >= 1 && documents[0].Status == "ready" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	chunksDir := filepath.Join(tempDirFallback(), "tmp", "knowledge", "chunks")
	t.Fatalf("document was not ready before timeout, chunks dir: %s", chunksDir)
}

func tempDirFallback() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

type stubRemoteRetriever struct {
	response gateway.RAGRetrieveResponse
	err      error
}

func (s stubRemoteRetriever) RetrieveKnowledge(_ context.Context, _ gateway.RAGRetrieveRequest) (gateway.RAGRetrieveResponse, error) {
	return s.response, s.err
}
