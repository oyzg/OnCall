package application

import (
	"context"

	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
)

type Repository interface {
	SaveAlert(ctx context.Context, alert alertDomain.Alert) error
	SaveAlertWithRecord(ctx context.Context, alert alertDomain.Alert, record alertDomain.HandlingRecord) error
	ListAlerts(ctx context.Context, status, severity, service, query string) ([]alertDomain.Alert, error)
	GetAlert(ctx context.Context, alertID string) (alertDomain.Alert, bool, error)
	ListRecords(ctx context.Context, alertID string) ([]alertDomain.HandlingRecord, error)
	FindDuplicateOpenAlert(ctx context.Context, candidate alertDomain.Alert) (alertDomain.Alert, bool, error)
}
