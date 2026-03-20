package application

import (
	"context"

	toolDomain "github.com/oyzg/OnCall/backend/go-api/internal/tool/domain"
)

type Repository interface {
	AppendLog(ctx context.Context, entry toolDomain.CallLog) error
	ListLogs(ctx context.Context, toolName, status string, limit int) ([]toolDomain.CallLog, error)
}
