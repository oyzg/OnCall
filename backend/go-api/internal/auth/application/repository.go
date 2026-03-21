package application

import (
	"context"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
)

type Repository interface {
	SaveUser(ctx context.Context, record StoredUser) error
	FindByUsername(ctx context.Context, username string) (StoredUser, bool, error)
}

type StoredUser struct {
	User         authDomain.User
	Status       string
	PasswordHash string
}
