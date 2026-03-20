package application

import (
	"context"

	auditDomain "github.com/oyzg/OnCall/backend/go-api/internal/audit/domain"
)

type Repository interface {
	AppendLog(ctx context.Context, log auditDomain.Log) error
	ListLogs(ctx context.Context, category, action, status, actor string, limit int) ([]auditDomain.Log, error)
	BuildStats(ctx context.Context) (auditDomain.Stats, error)
	Categories(ctx context.Context) ([]string, error)
}
