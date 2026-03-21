package application_test

import (
	"path/filepath"
	"testing"
	"time"

	authApp "github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
	authInfra "github.com/oyzg/OnCall/backend/go-api/internal/auth/infrastructure"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
)

func TestRepositoryBackedLogin(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "auth-service.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := authInfra.NewMySQLRepository(gdb)
	service := authApp.NewServiceWithRepository(config.AuthConfig{
		JWTSecret:      "test-secret",
		TokenExpiresIn: time.Hour,
	}, repo)
	if err := service.EnsureSeeded(); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	result, err := service.Login(authApp.LoginInput{
		Username: "admin",
		Password: "OnCallAdmin2026!",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.User.Username != "admin" || len(result.User.Roles) != 1 || result.User.Roles[0] != "admin" {
		t.Fatalf("unexpected login result: %#v", result)
	}
}
