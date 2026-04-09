package application

import (
	"context"
	"strings"
	"testing"
	"time"

	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
)

func TestDescribeIndexOutcomeFullHybrid(t *testing.T) {
	summary := describeIndexOutcome(IndexResult{
		Status:           "indexed",
		EmbeddingBackend: "openai:text-embedding-3-small",
		VectorBackend:    "milvus",
		LexicalBackend:   "elasticsearch",
	})

	if !strings.Contains(summary, "混合索引") {
		t.Fatalf("expected full hybrid summary, got %q", summary)
	}
	if !strings.Contains(summary, "Milvus") {
		t.Fatalf("expected Milvus mention, got %q", summary)
	}
	if !strings.Contains(summary, "Elasticsearch") {
		t.Fatalf("expected Elasticsearch mention, got %q", summary)
	}
}

func TestDescribeIndexOutcomePartialFallback(t *testing.T) {
	summary := describeIndexOutcome(IndexResult{
		Status:           "indexed",
		EmbeddingBackend: "hash_fallback",
		VectorBackend:    "local_fallback",
		LexicalBackend:   "elasticsearch",
	})

	if !strings.Contains(summary, "部分降级") {
		t.Fatalf("expected degraded summary, got %q", summary)
	}
	if !strings.Contains(summary, "本地兜底") {
		t.Fatalf("expected fallback mention, got %q", summary)
	}
}

func TestIndexReadyDocumentLoadsDocumentFromRepository(t *testing.T) {
	now := time.Now()
	repo := &stubRepository{
		document: knowledgeDomain.Document{
			ID:         "doc_1",
			UserID:     "user_1",
			Title:      "User Service SOP",
			Category:   "runbook",
			Status:     "ready",
			StoragePath: "tmp/doc.txt",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		chunks: []Chunk{
			{
				DocumentID:    "doc_1",
				DocumentTitle: "User Service SOP",
				Category:      "runbook",
				Index:         0,
				Content:       "user-service 5xx runbook",
				Normalized:    "user service 5xx runbook",
				Terms:         []string{"user", "service", "5xx", "runbook"},
				CharTerms:     []string{"us", "se"},
			},
		},
	}
	indexer := &stubIndexer{result: IndexResult{
		Status:           "indexed",
		EmbeddingBackend: "openai:text-embedding-3-small",
		VectorBackend:    "milvus",
		LexicalBackend:   "elasticsearch",
	}}
	service := NewServiceWithRepository(repo)
	service.SetIndexer(indexer)

	result, err := service.indexReadyDocument("user_1", "doc_1")
	if err != nil {
		t.Fatalf("indexReadyDocument returned error: %v", err)
	}
	if result.Status != "indexed" {
		t.Fatalf("expected indexed result, got %+v", result)
	}
	if !indexer.called {
		t.Fatal("expected indexer to be called")
	}
	if indexer.request.DocumentTitle != "User Service SOP" {
		t.Fatalf("expected repository-backed document title, got %s", indexer.request.DocumentTitle)
	}
	if len(indexer.request.Chunks) != 1 {
		t.Fatalf("expected repository-backed chunks, got %d", len(indexer.request.Chunks))
	}
}

type stubRepository struct {
	document knowledgeDomain.Document
	chunks   []Chunk
}

func (s *stubRepository) SaveDocument(context.Context, knowledgeDomain.Document) error { return nil }
func (s *stubRepository) ListDocuments(context.Context, string, string, string, string, int) ([]knowledgeDomain.Document, error) {
	return nil, nil
}
func (s *stubRepository) GetDocument(context.Context, string, string) (knowledgeDomain.Document, bool, error) {
	return knowledgeDomain.Document{}, false, nil
}
func (s *stubRepository) GetDocumentByID(context.Context, string) (knowledgeDomain.Document, bool, error) {
	return s.document, true, nil
}
func (s *stubRepository) DeleteDocument(context.Context, string, string) (knowledgeDomain.Document, bool, error) {
	return knowledgeDomain.Document{}, false, nil
}
func (s *stubRepository) ReplaceChunks(context.Context, string, []Chunk) error { return nil }
func (s *stubRepository) ListChunksByDocument(context.Context, string) ([]Chunk, error) {
	return s.chunks, nil
}
func (s *stubRepository) ListReadyChunks(context.Context, string, string) ([]Chunk, error) {
	return s.chunks, nil
}

type stubIndexer struct {
	called  bool
	request IndexRequest
	result  IndexResult
}

func (s *stubIndexer) IndexDocument(_ context.Context, request IndexRequest) (IndexResult, error) {
	s.called = true
	s.request = request
	return s.result, nil
}

func (s *stubIndexer) DeleteDocument(context.Context, string) error { return nil }
