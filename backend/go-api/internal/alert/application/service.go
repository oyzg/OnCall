package application

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
)

type IngestInput struct {
	Title       string            `json:"title"`
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Severity    string            `json:"severity"`
	Source      string            `json:"source"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	TriggeredAt *time.Time        `json:"triggered_at"`
}

type UpdateStatusInput struct {
	Status  string
	Comment string
}

type Detail struct {
	Alert   alertDomain.Alert            `json:"alert"`
	Records []alertDomain.HandlingRecord `json:"records"`
}

type Service struct {
	mu         sync.RWMutex
	alerts     map[string]alertDomain.Alert
	records    map[string][]alertDomain.HandlingRecord
	sessionSvc *sessionApp.Service
	seedOnce   sync.Once
}

func NewService(sessionSvc *sessionApp.Service) *Service {
	return &Service{
		alerts:     make(map[string]alertDomain.Alert),
		records:    make(map[string][]alertDomain.HandlingRecord),
		sessionSvc: sessionSvc,
	}
}

func (s *Service) EnsureSeeded() {
	s.seedOnce.Do(func() {
		now := time.Now()
		s.mustIngest(IngestInput{
			Title:       "payment-api p95 latency spike",
			Service:     "payment-api",
			Environment: "prod",
			Severity:    "P1",
			Source:      "prometheus",
			Summary:     "5 分钟窗口内 p95 延迟超过 2s",
			Description: "建议先检查支付链路下游依赖和数据库连接池。",
			Labels: map[string]string{
				"metric": "http_server_duration_p95",
				"team":   "payments",
			},
			TriggeredAt: &now,
		})
		s.mustIngest(IngestInput{
			Title:       "user-service error ratio increased",
			Service:     "user-service",
			Environment: "staging",
			Severity:    "P2",
			Source:      "grafana",
			Summary:     "错误率 10 分钟内从 0.4% 升至 6.1%",
			Description: "优先核对最新发布与依赖服务响应情况。",
			Labels: map[string]string{
				"metric": "5xx_ratio",
				"team":   "account",
			},
			TriggeredAt: &now,
		})
	})
}

func (s *Service) Ingest(input IngestInput) alertDomain.Alert {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ingestLocked(input)
}

func (s *Service) ListAlerts(status, severity, service string) []alertDomain.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]alertDomain.Alert, 0)
	for _, alert := range s.alerts {
		if status != "" && alert.Status != status {
			continue
		}
		if severity != "" && alert.Severity != severity {
			continue
		}
		if service != "" && !strings.EqualFold(alert.Service, service) {
			continue
		}
		items = append(items, alert)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].TriggeredAt.After(items[j].TriggeredAt)
	})
	return items
}

func (s *Service) GetDetail(alertID string) (Detail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alert, ok := s.alerts[alertID]
	if !ok {
		return Detail{}, false
	}

	records := append([]alertDomain.HandlingRecord(nil), s.records[alertID]...)
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	return Detail{
		Alert:   alert,
		Records: records,
	}, true
}

func (s *Service) UpdateStatus(user authDomain.User, alertID string, input UpdateStatusInput) (Detail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert, ok := s.alerts[alertID]
	if !ok {
		return Detail{}, false
	}

	now := time.Now()
	alert.Status = normalizeStatus(input.Status)
	alert.UpdatedAt = now
	s.alerts[alertID] = alert
	s.records[alertID] = append(s.records[alertID], alertDomain.HandlingRecord{
		ID:        nextID("record"),
		AlertID:   alertID,
		Action:    "status_change",
		Operator:  displayOperator(user),
		Comment:   buildStatusComment(alert.Status, input.Comment),
		CreatedAt: now,
	})

	return s.detailLocked(alertID), true
}

func (s *Service) LinkSession(user authDomain.User, alertID string) (Detail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert, ok := s.alerts[alertID]
	if !ok {
		return Detail{}, false
	}

	if alert.LinkedSessionID == "" {
		session := s.sessionSvc.CreateSession(user, fmt.Sprintf("告警排障: %s", alert.Title))
		alert.LinkedSessionID = session.ID
		alert.UpdatedAt = time.Now()
		s.alerts[alertID] = alert
		s.records[alertID] = append(s.records[alertID], alertDomain.HandlingRecord{
			ID:        nextID("record"),
			AlertID:   alertID,
			Action:    "link_session",
			Operator:  displayOperator(user),
			Comment:   "已创建排障会话并关联到当前告警。",
			CreatedAt: time.Now(),
		})
	}

	return s.detailLocked(alertID), true
}

func (s *Service) mustIngest(input IngestInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ingestLocked(input)
}

func (s *Service) ingestLocked(input IngestInput) alertDomain.Alert {
	now := time.Now()
	triggeredAt := now
	if input.TriggeredAt != nil {
		triggeredAt = *input.TriggeredAt
	}

	alert := alertDomain.Alert{
		ID:          nextID("alert"),
		Title:       normalizeTitle(input.Title),
		Service:     normalizeFallback(input.Service, "unknown-service"),
		Environment: normalizeFallback(input.Environment, "prod"),
		Severity:    normalizeSeverity(input.Severity),
		Source:      normalizeFallback(input.Source, "external"),
		Status:      "open",
		Summary:     normalizeFallback(input.Summary, "未提供摘要"),
		Description: strings.TrimSpace(input.Description),
		Labels:      input.Labels,
		CreatedAt:   now,
		UpdatedAt:   now,
		TriggeredAt: triggeredAt,
	}

	s.alerts[alert.ID] = alert
	s.records[alert.ID] = append(s.records[alert.ID], alertDomain.HandlingRecord{
		ID:        nextID("record"),
		AlertID:   alert.ID,
		Action:    "ingest",
		Operator:  alert.Source,
		Comment:   "告警已接入平台。",
		CreatedAt: now,
	})

	return alert
}

func (s *Service) detailLocked(alertID string) Detail {
	alert := s.alerts[alertID]
	records := append([]alertDomain.HandlingRecord(nil), s.records[alertID]...)
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return Detail{Alert: alert, Records: records}
}

func normalizeTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "未命名告警"
	}
	return trimmed
}

func normalizeFallback(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func normalizeSeverity(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "P0", "P1", "P2", "P3":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return "P2"
	}
}

func normalizeStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "open", "acknowledged", "investigating", "resolved":
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "open"
	}
}

func buildStatusComment(status, comment string) string {
	if strings.TrimSpace(comment) == "" {
		return fmt.Sprintf("告警状态变更为 %s。", status)
	}
	return fmt.Sprintf("告警状态变更为 %s。%s", status, strings.TrimSpace(comment))
}

func displayOperator(user authDomain.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
