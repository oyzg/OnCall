package infrastructure

import (
	"path/filepath"
	"testing"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
)

func TestAuthRepositoryStoresUserWithRoles(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "auth.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLRepository(gdb)
	record := StoredUser{
		User: authDomain.User{
			ID:          "user-admin",
			Username:    "admin",
			DisplayName: "Platform Admin",
			Roles:       []string{"admin"},
		},
		PasswordHash: "hashed",
	}

	if err := repo.SaveUser(t.Context(), record); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, ok, err := repo.FindByUsername(t.Context(), "admin")
	if err != nil {
		t.Fatalf("find user: %v", err)
	}
	if !ok {
		t.Fatal("expected stored user")
	}
	if stored.User.ID != "user-admin" || len(stored.User.Roles) != 1 || stored.User.Roles[0] != "admin" {
		t.Fatalf("unexpected stored user: %#v", stored)
	}
}
