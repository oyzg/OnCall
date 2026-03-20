package infrastructure

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
	toolDomain "github.com/oyzg/OnCall/backend/go-api/internal/tool/domain"
)

func TestToolLogRepositoryStoresAndFiltersLogs(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "tool-logs.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLRepository(gdb)
	now := time.Now().UTC().Truncate(time.Second)
	logs := []toolDomain.CallLog{
		{
			ID:         "tool_1",
			ToolName:   "knowledge_search",
			Operator:   "Admin",
			UserID:     "user_1",
			Status:     "success",
			Input:      map[string]any{"query": "user-service"},
			Output:     map[string]any{"answer": "ok"},
			DurationMS: 123,
			CreatedAt:  now,
		},
		{
			ID:         "tool_2",
			ToolName:   "platform_overview",
			Operator:   "Admin",
			UserID:     "user_1",
			Status:     "failed",
			Input:      map[string]any{},
			Error:      "permission denied",
			DurationMS: 3,
			CreatedAt:  now.Add(time.Second),
		},
	}

	for _, log := range logs {
		if err := repo.AppendLog(t.Context(), log); err != nil {
			t.Fatalf("append log: %v", err)
		}
	}

	items, err := repo.ListLogs(t.Context(), "knowledge_search", "", 10)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(items) != 1 || items[0].ID != "tool_1" {
		t.Fatalf("unexpected logs: %#v", items)
	}

	failedItems, err := repo.ListLogs(t.Context(), "", "failed", 10)
	if err != nil {
		t.Fatalf("list failed logs: %v", err)
	}
	if len(failedItems) != 1 || failedItems[0].ID != "tool_2" {
		t.Fatalf("unexpected failed logs: %#v", failedItems)
	}
}
