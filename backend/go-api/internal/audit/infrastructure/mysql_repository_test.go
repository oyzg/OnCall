package infrastructure

import (
	"path/filepath"
	"testing"
	"time"

	auditDomain "github.com/oyzg/OnCall/backend/go-api/internal/audit/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
)

func TestAuditRepositoryStoresLogsAndBuildsStats(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "audit-logs.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLRepository(gdb)
	now := time.Now().UTC().Truncate(time.Second)
	logs := []auditDomain.Log{
		{
			ID:         "audit_1",
			Category:   "auth",
			Action:     "login",
			Status:     "success",
			ActorID:    "user_1",
			ActorName:  "Admin",
			ActorRoles: []string{"admin"},
			CreatedAt:  now,
		},
		{
			ID:         "audit_2",
			Category:   "tool",
			Action:     "call",
			Status:     "failed",
			ActorID:    "user_1",
			ActorName:  "Admin",
			ActorRoles: []string{"admin"},
			CreatedAt:  now.Add(time.Second),
		},
	}

	for _, log := range logs {
		if err := repo.AppendLog(t.Context(), log); err != nil {
			t.Fatalf("append audit log: %v", err)
		}
	}

	items, err := repo.ListLogs(t.Context(), "tool", "", "", "", 10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(items) != 1 || items[0].ID != "audit_2" {
		t.Fatalf("unexpected audit logs: %#v", items)
	}

	stats, err := repo.BuildStats(t.Context())
	if err != nil {
		t.Fatalf("build stats: %v", err)
	}
	if stats.Total != 2 || stats.Failed != 1 || stats.Success != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}

	categories, err := repo.Categories(t.Context())
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("unexpected categories: %#v", categories)
	}
}
