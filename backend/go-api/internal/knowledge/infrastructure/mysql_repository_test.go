package infrastructure

import (
	"path/filepath"
	"testing"
	"time"

	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
)

func TestKnowledgeRepositoryStoresDocumentsAndChunks(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "knowledge.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLRepository(gdb)
	now := time.Now().UTC().Truncate(time.Second)
	document := knowledgeDomain.Document{
		ID:               "doc_1",
		UserID:           "user_1",
		Title:            "runbook",
		Category:         "general",
		SourceType:       "text",
		FileName:         "runbook.txt",
		ContentType:      "text/plain",
		StoragePath:      "/tmp/runbook.txt",
		SizeBytes:        12,
		Status:           "ready",
		Summary:          "summary",
		TextPreview:      "preview",
		ChunkPreviews:    []string{"c1", "c2"},
		ChunkCount:       2,
		IndexStatus:      "indexed",
		EmbeddingBackend: "openai:text-embedding-3-small",
		VectorBackend:    "milvus",
		LexicalBackend:   "elasticsearch",
		CreatedAt:        now,
		UpdatedAt:        now,
		ProcessedAt:      &now,
		IndexedAt:        &now,
	}
	chunks := []knowledgeApp.Chunk{
		{DocumentID: document.ID, DocumentTitle: document.Title, Category: document.Category, Index: 0, Content: "first", Normalized: "first", Terms: []string{"first"}, CharTerms: []string{"f", "i"}},
		{DocumentID: document.ID, DocumentTitle: document.Title, Category: document.Category, Index: 1, Content: "second", Normalized: "second", Terms: []string{"second"}, CharTerms: []string{"s", "e"}},
	}

	if err := repo.SaveDocument(t.Context(), document); err != nil {
		t.Fatalf("save document: %v", err)
	}
	if err := repo.ReplaceChunks(t.Context(), document.ID, chunks); err != nil {
		t.Fatalf("replace chunks: %v", err)
	}

	items, err := repo.ListDocuments(t.Context(), "user_1", "ready", "", "", 10)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(items) != 1 || items[0].Title != "runbook" {
		t.Fatalf("unexpected documents: %#v", items)
	}

	readyChunks, err := repo.ListReadyChunks(t.Context(), "user_1", "")
	if err != nil {
		t.Fatalf("list ready chunks: %v", err)
	}
	if len(readyChunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(readyChunks))
	}
}
