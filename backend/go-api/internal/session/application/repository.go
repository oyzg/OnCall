package application

import (
	"context"

	"github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
)

type Repository interface {
	CreateSession(ctx context.Context, session domain.Session) error
	ListSessionsByUser(ctx context.Context, userID, query string, limit int) ([]domain.Session, error)
	DeleteSession(ctx context.Context, userID, sessionID string) (bool, error)
	ListMessages(ctx context.Context, userID, sessionID string, limit int, beforeID string) (MessagePage, error)
	UpsertSessionWithMessages(ctx context.Context, session domain.Session, messages []domain.Message) error
}
