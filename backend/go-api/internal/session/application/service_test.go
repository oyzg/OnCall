package application_test

import (
	"path/filepath"
	"testing"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	sessionInfra "github.com/oyzg/OnCall/backend/go-api/internal/session/infrastructure"
)

func TestSessionServiceRepositoryCreateAndReply(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "session-service.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	service := sessionApp.NewServiceWithRepository(sessionInfra.NewMySQLRepository(gdb))
	user := authDomain.User{ID: "u_admin", Username: "admin"}

	session := service.CreateSession(user, "integration session")
	if session.ID == "" {
		t.Fatal("expected session id")
	}

	sessions := service.ListSessions(user, "", 0)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %#v", sessions)
	}

	page, ok := service.ListMessages(user, session.ID, 0, "")
	if !ok {
		t.Fatal("expected ListMessages to find session")
	}
	if len(page.Messages) != 0 {
		t.Fatalf("expected no messages yet, got %#v", page.Messages)
	}

	assistant, ok := service.StartAssistantReply(user, session.ID, "hello")
	if !ok {
		t.Fatal("expected StartAssistantReply to succeed")
	}
	if assistant.ID == "" {
		t.Fatal("expected assistant message id")
	}
}
