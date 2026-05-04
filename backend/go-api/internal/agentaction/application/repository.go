package application

import (
	"context"

	"github.com/oyzg/OnCall/backend/go-api/internal/agentaction/domain"
)

type Repository interface {
	Save(ctx context.Context, action domain.Action) error
	Get(ctx context.Context, id string) (domain.Action, bool, error)
	ListBySource(ctx context.Context, sourceType, sourceID string) ([]domain.Action, error)
}
