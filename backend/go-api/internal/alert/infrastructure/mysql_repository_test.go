package infrastructure

import (
	"path/filepath"
	"testing"
	"time"

	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db"
)

func TestAlertRepositoryStoresAlertsAndRecords(t *testing.T) {
	gdb, err := db.Open(db.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "alerts.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewMySQLRepository(gdb)
	now := time.Now().UTC().Truncate(time.Second)
	alert := alertDomain.Alert{
		ID:              "alert_1",
		Title:           "payment-api p95 latency spike",
		Service:         "payment-api",
		Environment:     "prod",
		Severity:        "P1",
		Source:          "prometheus",
		Status:          "open",
		Summary:         "latency up",
		Description:     "check downstream",
		Labels:          map[string]string{"metric": "p95"},
		LinkedSessionID: "sess_1",
		Analysis: &alertDomain.AlertAnalysis{
			Status:      "completed",
			Summary:     "analysis",
			GeneratedAt: now,
		},
		OccurrenceCount: 2,
		CreatedAt:       now,
		UpdatedAt:       now,
		TriggeredAt:     now,
		LastTriggeredAt: now,
	}
	record := alertDomain.HandlingRecord{
		ID:        "record_1",
		AlertID:   alert.ID,
		Action:    "ingest",
		Operator:  "prometheus",
		Comment:   "created",
		CreatedAt: now,
	}

	if err := repo.SaveAlert(t.Context(), alert); err != nil {
		t.Fatalf("save alert: %v", err)
	}
	if err := repo.AppendRecord(t.Context(), record); err != nil {
		t.Fatalf("append record: %v", err)
	}

	items, err := repo.ListAlerts(t.Context(), "", "", "", "")
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(items) != 1 || items[0].ID != alert.ID {
		t.Fatalf("unexpected alerts: %#v", items)
	}

	records, err := repo.ListRecords(t.Context(), alert.ID)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("unexpected records: %#v", records)
	}
}
