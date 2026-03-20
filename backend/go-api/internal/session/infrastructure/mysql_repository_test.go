package infrastructure

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
	sessionDomain "github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
)

func TestListMessagesBeforeCursor(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "messages.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLRepository(gdb)
	now := time.Now().UTC().Truncate(time.Second)
	session := sessionDomain.Session{
		ID:                 "sess_cursor",
		UserID:             "user_cursor",
		Title:              "cursor",
		MessageCount:       3,
		LastMessagePreview: "m3",
		CreatedAt:          now,
		UpdatedAt:          now,
		LastMessageAt:      now.Add(2 * time.Second),
	}
	messages := []sessionDomain.Message{
		{ID: "m1", SessionID: session.ID, Role: "user", Content: "1", Status: "completed", CreatedAt: now},
		{ID: "m2", SessionID: session.ID, Role: "assistant", Content: "2", Status: "completed", CreatedAt: now.Add(time.Second)},
		{ID: "m3", SessionID: session.ID, Role: "user", Content: "3", Status: "completed", CreatedAt: now.Add(2 * time.Second)},
	}

	if err := repo.UpsertSessionWithMessages(t.Context(), session, messages); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	page, err := repo.ListMessages(t.Context(), "user_cursor", session.ID, 1, "m3")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(page.Messages) != 1 || page.Messages[0].ID != "m2" {
		t.Fatalf("expected cursor page to return m2, got %#v", page.Messages)
	}
	if !page.HasMore {
		t.Fatalf("expected has_more for cursor page")
	}
}
