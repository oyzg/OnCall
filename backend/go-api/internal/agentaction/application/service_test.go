package application

import (
	"testing"

	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
)

func TestServicePersistsAndConfirmsUpdateAlertStatusAction(t *testing.T) {
	sessionService := sessionApp.NewService()
	alertService := alertApp.NewServiceWithRepository(sessionService, nil, nil)
	alert := alertService.Ingest(alertApp.IngestInput{
		Title:       "payment-api latency",
		Service:     "payment-api",
		Environment: "prod",
		Severity:    "P1",
		Source:      "prometheus",
		Summary:     "latency is high",
		Description: "checkout is degraded",
	})
	user := authDomain.User{ID: "user-1", Username: "ops", DisplayName: "Ops", Roles: []string{"ops"}}
	service := NewService(alertService, sessionService, nil)

	action, err := service.Create(t.Context(), CreateInput{
		SourceType:    "alert",
		SourceID:      alert.ID,
		ActionType:    "update_alert_status",
		Title:         "Move alert to investigating",
		Description:   "The autonomous agent found degraded service health.",
		ArgumentsJSON: `{"alert_id":"` + alert.ID + `","status":"investigating","comment":"Agent confirmed degraded health."}`,
		RiskLevel:     "medium",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if action.ID == "" || action.Status != "pending" {
		t.Fatalf("unexpected created action: %#v", action)
	}

	confirmed, err := service.Confirm(t.Context(), user, action.ID)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if confirmed.Status != "executed" {
		t.Fatalf("expected executed action, got %#v", confirmed)
	}

	detail, ok := alertService.GetDetail(alert.ID)
	if !ok {
		t.Fatal("expected alert detail")
	}
	if detail.Alert.Status != "investigating" {
		t.Fatalf("expected alert to be investigating, got %s", detail.Alert.Status)
	}

	confirmedAgain, err := service.Confirm(t.Context(), user, action.ID)
	if err != nil {
		t.Fatalf("Confirm idempotent: %v", err)
	}
	if confirmedAgain.Status != "executed" {
		t.Fatalf("expected executed action on repeat confirm, got %#v", confirmedAgain)
	}
}

func TestServiceListsActionsBySource(t *testing.T) {
	sessionService := sessionApp.NewService()
	alertService := alertApp.NewServiceWithRepository(sessionService, nil, nil)
	service := NewService(alertService, sessionService, nil)

	first, err := service.Create(t.Context(), CreateInput{
		ID:            "action-alert-1",
		SourceType:    "alert",
		SourceID:      "alert-1",
		ActionType:    "append_alert_record",
		Title:         "Record note",
		ArgumentsJSON: `{"alert_id":"alert-1","comment":"note"}`,
		RiskLevel:     "low",
	})
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	_, err = service.Create(t.Context(), CreateInput{
		ID:            "action-alert-2",
		SourceType:    "alert",
		SourceID:      "alert-2",
		ActionType:    "append_alert_record",
		Title:         "Record other note",
		ArgumentsJSON: `{"alert_id":"alert-2","comment":"note"}`,
		RiskLevel:     "low",
	})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	actions, err := service.ListBySource(t.Context(), "alert", "alert-1")
	if err != nil {
		t.Fatalf("ListBySource: %v", err)
	}
	if len(actions) != 1 || actions[0].ID != first.ID {
		t.Fatalf("expected only %s, got %#v", first.ID, actions)
	}
}
