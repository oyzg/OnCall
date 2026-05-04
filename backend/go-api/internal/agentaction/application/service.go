package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentDomain "github.com/oyzg/OnCall/backend/go-api/internal/agentaction/domain"
	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
)

type CreateInput struct {
	ID            string
	SourceType    string
	SourceID      string
	ActionType    string
	Title         string
	Description   string
	ArgumentsJSON string
	RiskLevel     string
}

type Service struct {
	mu       sync.RWMutex
	items    map[string]agentDomain.Action
	alerts   *alertApp.Service
	sessions *sessionApp.Service
	repo     Repository
}

func NewService(alertService *alertApp.Service, sessionService *sessionApp.Service, repo Repository) *Service {
	return &Service{
		items:    make(map[string]agentDomain.Action),
		alerts:   alertService,
		sessions: sessionService,
		repo:     repo,
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (agentDomain.Action, error) {
	if s == nil {
		return agentDomain.Action{}, fmt.Errorf("agent action service unavailable")
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = nextID("agent_action")
	}
	if existing, ok, err := s.Get(ctx, id); err != nil {
		return agentDomain.Action{}, err
	} else if ok {
		return existing, nil
	}
	action := agentDomain.Action{
		ID:            id,
		SourceType:    strings.TrimSpace(input.SourceType),
		SourceID:      strings.TrimSpace(input.SourceID),
		ActionType:    strings.TrimSpace(input.ActionType),
		Status:        "pending",
		Title:         strings.TrimSpace(input.Title),
		Description:   strings.TrimSpace(input.Description),
		ArgumentsJSON: strings.TrimSpace(input.ArgumentsJSON),
		RiskLevel:     normalizeRisk(input.RiskLevel),
		CreatedAt:     time.Now(),
	}
	if action.ActionType == "" || action.ArgumentsJSON == "" {
		return agentDomain.Action{}, fmt.Errorf("action type and arguments are required")
	}
	if action.Title == "" {
		action.Title = action.ActionType
	}
	return s.save(ctx, action)
}

func (s *Service) Get(ctx context.Context, id string) (agentDomain.Action, bool, error) {
	if s == nil {
		return agentDomain.Action{}, false, fmt.Errorf("agent action service unavailable")
	}
	if s.repo != nil {
		return s.repo.Get(ctx, id)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	action, ok := s.items[id]
	return action, ok, nil
}

func (s *Service) ListBySource(ctx context.Context, sourceType, sourceID string) ([]agentDomain.Action, error) {
	if s == nil {
		return nil, fmt.Errorf("agent action service unavailable")
	}
	normalizedSourceType := strings.TrimSpace(sourceType)
	normalizedSourceID := strings.TrimSpace(sourceID)
	if normalizedSourceType == "" || normalizedSourceID == "" {
		return nil, fmt.Errorf("source type and source id are required")
	}
	if s.repo != nil {
		return s.repo.ListBySource(ctx, normalizedSourceType, normalizedSourceID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]agentDomain.Action, 0)
	for _, action := range s.items {
		if action.SourceType == normalizedSourceType && action.SourceID == normalizedSourceID {
			result = append(result, action)
		}
	}
	return result, nil
}

func (s *Service) Confirm(ctx context.Context, user authDomain.User, id string) (agentDomain.Action, error) {
	action, ok, err := s.Get(ctx, id)
	if err != nil {
		return agentDomain.Action{}, err
	}
	if !ok {
		return agentDomain.Action{}, fmt.Errorf("agent action not found")
	}
	if action.Status == "executed" {
		return action, nil
	}
	if action.Status != "pending" {
		return action, fmt.Errorf("agent action is %s", action.Status)
	}

	result, execErr := s.execute(user, action)
	now := time.Now()
	action.ExecutedAt = &now
	if execErr != nil {
		action.Status = "failed"
		action.Error = execErr.Error()
		_, _ = s.save(ctx, action)
		return action, execErr
	}
	action.Status = "executed"
	action.ResultJSON = mustJSON(result)
	action.Error = ""
	return s.save(ctx, action)
}

func (s *Service) ConfirmWithFollowUp(ctx context.Context, user authDomain.User, id string) (agentDomain.Action, *alertApp.Detail, error) {
	existing, ok, err := s.Get(ctx, id)
	if err != nil {
		return agentDomain.Action{}, nil, err
	}
	if !ok {
		return agentDomain.Action{}, nil, fmt.Errorf("agent action not found")
	}
	wasPending := existing.Status == "pending"

	action, err := s.Confirm(ctx, user, id)
	if err != nil {
		return action, nil, err
	}
	alertID := followUpAlertID(action)
	if !wasPending || action.Status != "executed" || alertID == "" || s.alerts == nil {
		return action, nil, nil
	}
	detail, ok := s.alerts.AnalyzeWithActionObservations(user, alertID, []alertDomain.AgentActionObservation{
		actionObservation(action),
	})
	if !ok {
		return action, nil, nil
	}
	if detail.Alert.Analysis != nil {
		for _, pending := range detail.Alert.Analysis.PendingActions {
			_, _ = s.Create(ctx, CreateInput{
				ID:            pending.ActionID,
				SourceType:    "alert",
				SourceID:      alertID,
				ActionType:    pending.ActionType,
				Title:         pending.Title,
				Description:   pending.Description,
				ArgumentsJSON: pending.ArgumentsJSON,
				RiskLevel:     pending.RiskLevel,
			})
		}
		s.hydratePendingActionStatuses(ctx, &detail)
	}
	return action, &detail, nil
}

func (s *Service) execute(user authDomain.User, action agentDomain.Action) (any, error) {
	args := map[string]string{}
	if err := json.Unmarshal([]byte(action.ArgumentsJSON), &args); err != nil {
		return nil, fmt.Errorf("invalid action arguments: %w", err)
	}
	alertID := strings.TrimSpace(args["alert_id"])
	if alertID == "" {
		return nil, fmt.Errorf("alert_id is required")
	}
	switch action.ActionType {
	case "update_alert_status":
		status := strings.TrimSpace(args["status"])
		if status == "" {
			return nil, fmt.Errorf("status is required")
		}
		detail, ok := s.alerts.UpdateStatus(user, alertID, alertApp.UpdateStatusInput{
			Status:  status,
			Comment: args["comment"],
		})
		if !ok {
			return nil, fmt.Errorf("alert not found")
		}
		return detail.Alert, nil
	case "link_or_create_session":
		detail, ok := s.alerts.LinkSession(user, alertID)
		if !ok {
			return nil, fmt.Errorf("alert not found")
		}
		return detail.Alert, nil
	case "append_alert_record":
		detail, ok := s.alerts.AppendRecord(user, alertID, args["comment"])
		if !ok {
			return nil, fmt.Errorf("alert not found")
		}
		return detail.Records, nil
	default:
		return nil, fmt.Errorf("unsupported action type: %s", action.ActionType)
	}
}

func actionObservation(action agentDomain.Action) alertDomain.AgentActionObservation {
	executedAt := ""
	if action.ExecutedAt != nil {
		executedAt = action.ExecutedAt.Format(time.RFC3339)
	}
	return alertDomain.AgentActionObservation{
		ActionID:   action.ID,
		ActionType: action.ActionType,
		Status:     action.Status,
		Title:      action.Title,
		ResultJSON: action.ResultJSON,
		Error:      action.Error,
		ExecutedAt: executedAt,
	}
}

func followUpAlertID(action agentDomain.Action) string {
	if action.SourceType == "alert" && strings.TrimSpace(action.SourceID) != "" {
		return strings.TrimSpace(action.SourceID)
	}
	args := map[string]string{}
	if err := json.Unmarshal([]byte(action.ArgumentsJSON), &args); err != nil {
		return ""
	}
	return strings.TrimSpace(args["alert_id"])
}

func (s *Service) hydratePendingActionStatuses(ctx context.Context, detail *alertApp.Detail) {
	if detail == nil || detail.Alert.Analysis == nil || len(detail.Alert.Analysis.PendingActions) == 0 {
		return
	}
	actions, err := s.ListBySource(ctx, "alert", detail.Alert.ID)
	if err != nil || len(actions) == 0 {
		return
	}
	statusByID := make(map[string]string, len(actions))
	for _, action := range actions {
		statusByID[action.ID] = action.Status
	}
	for index := range detail.Alert.Analysis.PendingActions {
		if status, ok := statusByID[detail.Alert.Analysis.PendingActions[index].ActionID]; ok {
			detail.Alert.Analysis.PendingActions[index].Status = status
		}
	}
}

func (s *Service) save(ctx context.Context, action agentDomain.Action) (agentDomain.Action, error) {
	if s.repo != nil {
		if err := s.repo.Save(ctx, action); err != nil {
			return agentDomain.Action{}, err
		}
		return action, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[action.ID] = action
	return action, nil
}

func normalizeRisk(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "low"
	}
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}

func nextID(prefix string) string {
	sequence := idSequence.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), sequence)
}

var idSequence atomic.Uint64
