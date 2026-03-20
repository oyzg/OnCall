package db

import (
	"path/filepath"
	"testing"
	"time"

	sessionDomain "github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
	sessionInfra "github.com/oyzg/OnCall/backend/go-api/internal/session/infrastructure"
)

func TestOpenAndMigrateSQLite(t *testing.T) {
	db, err := Open(Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "oncall.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
}

func TestSessionRepositoryPersistsMessages(t *testing.T) {
	db, err := Open(Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "session.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := sessionInfra.NewMySQLRepository(db)
	now := time.Now().UTC().Truncate(time.Second)
	session := sessionDomain.Session{
		ID:                 "sess_1",
		UserID:             "user_1",
		Title:              "db session",
		LastMessagePreview: "hello",
		MessageCount:       2,
		CreatedAt:          now,
		UpdatedAt:          now,
		LastMessageAt:      now,
	}
	messages := []sessionDomain.Message{
		{
			ID:        "msg_1",
			SessionID: session.ID,
			Role:      "user",
			Content:   "hello",
			Status:    "completed",
			CreatedAt: now,
		},
		{
			ID:        "msg_2",
			SessionID: session.ID,
			Role:      "assistant",
			Content:   "world",
			Status:    "completed",
			CreatedAt: now.Add(time.Second),
			References: []sessionDomain.Reference{
				{
					DocumentID:    "doc_1",
					DocumentTitle: "runbook",
					Category:      "runbook",
					Excerpt:       "world",
					Score:         0.9,
				},
			},
		},
	}

	if err := repo.UpsertSessionWithMessages(t.Context(), session, messages); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	storedSessions, err := repo.ListSessionsByUser(t.Context(), "user_1", "", 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(storedSessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(storedSessions))
	}

	page, err := repo.ListMessages(t.Context(), "user_1", session.ID, 10, "")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(page.Messages))
	}
	if len(page.Messages[1].References) != 1 {
		t.Fatalf("expected assistant references, got %#v", page.Messages[1].References)
	}
}
