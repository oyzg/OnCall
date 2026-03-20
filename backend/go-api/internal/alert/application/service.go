package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

type Stats struct {
	Total           int            `json:"total"`
	Open            int            `json:"open"`
	Investigating   int            `json:"investigating"`
	Resolved        int            `json:"resolved"`
	BySeverity      map[string]int `json:"by_severity"`
	LinkedSessions  int            `json:"linked_sessions"`
	DeduplicatedHit int            `json:"deduplicated_hit"`
}

type Detail struct {
	Alert   alertDomain.Alert            `json:"alert"`
	Records []alertDomain.HandlingRecord `json:"records"`
	Stats   Stats                        `json:"stats,omitempty"`
}

type Service struct {
	mu          sync.RWMutex
	alerts      map[string]alertDomain.Alert
	records     map[string][]alertDomain.HandlingRecord
	sessionSvc  *sessionApp.Service
	analyzer    Analyzer
	seedOnce    sync.Once
	storagePath string
	repo        Repository
}

type Analyzer interface {
	AnalyzeAlert(ctx context.Context, user authDomain.User, alert alertDomain.Alert) alertDomain.AlertAnalysis
}

type store struct {
	Alerts  map[string]alertDomain.Alert            `json:"alerts"`
	Records map[string][]alertDomain.HandlingRecord `json:"records"`
}

func NewService(sessionSvc *sessionApp.Service, analyzer Analyzer) *Service {
	service := &Service{
		alerts:      make(map[string]alertDomain.Alert),
		records:     make(map[string][]alertDomain.HandlingRecord),
		sessionSvc:  sessionSvc,
		analyzer:    analyzer,
		storagePath: filepath.Join("tmp", "alerts", "alerts.json"),
	}
	service.load()
	return service
}

func NewServiceWithRepository(sessionSvc *sessionApp.Service, analyzer Analyzer, repo Repository) *Service {
	return &Service{
		alerts:     make(map[string]alertDomain.Alert),
		records:    make(map[string][]alertDomain.HandlingRecord),
		sessionSvc: sessionSvc,
		analyzer:   analyzer,
		repo:       repo,
	}
}

func (s *Service) EnsureSeeded() {
	s.seedOnce.Do(func() {
		s.mu.RLock()
		alreadySeeded := len(s.alerts) > 0
		s.mu.RUnlock()
		if alreadySeeded {
			return
		}

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
	if s.repo != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		alert := s.ingestRepositoryLocked(input)
		return alert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	alert := s.ingestLocked(input)
	s.persistLocked()
	return alert
}

func (s *Service) ListAlerts(status, severity, service, query string) []alertDomain.Alert {
	if s.repo != nil {
		items, err := s.repo.ListAlerts(context.Background(), status, severity, service, query)
		if err == nil {
			return items
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]alertDomain.Alert, 0)
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
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
		if normalizedQuery != "" &&
			!strings.Contains(strings.ToLower(alert.Title), normalizedQuery) &&
			!strings.Contains(strings.ToLower(alert.Summary), normalizedQuery) &&
			!strings.Contains(strings.ToLower(alert.Description), normalizedQuery) {
			continue
		}
		items = append(items, alert)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].TriggeredAt.Equal(items[j].TriggeredAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		return items[i].TriggeredAt.After(items[j].TriggeredAt)
	})
	return items
}

func (s *Service) BuildStats() Stats {
	if s.repo != nil {
		items, err := s.repo.ListAlerts(context.Background(), "", "", "", "")
		if err == nil {
			return statsFromAlerts(items)
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statsLocked()
}

func (s *Service) GetDetail(alertID string) (Detail, bool) {
	if s.repo != nil {
		alert, ok, err := s.repo.GetAlert(context.Background(), alertID)
		if err != nil || !ok {
			return Detail{}, false
		}
		records, err := s.repo.ListRecords(context.Background(), alertID)
		if err != nil {
			return Detail{}, false
		}
		return Detail{
			Alert:   alert,
			Records: records,
			Stats:   s.BuildStats(),
		}, true
	}

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
		Stats:   s.statsLocked(),
	}, true
}

func (s *Service) UpdateStatus(user authDomain.User, alertID string, input UpdateStatusInput) (Detail, bool) {
	if s.repo != nil {
		alert, ok, err := s.repo.GetAlert(context.Background(), alertID)
		if err != nil || !ok {
			return Detail{}, false
		}

		now := time.Now()
		alert.Status = normalizeStatus(input.Status)
		alert.UpdatedAt = now
		record := alertDomain.HandlingRecord{
			ID:        nextID("record"),
			AlertID:   alertID,
			Action:    "status_change",
			Operator:  displayOperator(user),
			Comment:   buildStatusComment(alert.Status, input.Comment),
			CreatedAt: now,
		}
		if err := s.repo.SaveAlertWithRecord(context.Background(), alert, record); err != nil {
			return Detail{}, false
		}
		return s.GetDetail(alertID)
	}

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
	s.persistLocked()

	return s.detailLocked(alertID), true
}

func (s *Service) LinkSession(user authDomain.User, alertID string) (Detail, bool) {
	if s.repo != nil {
		alert, ok, err := s.repo.GetAlert(context.Background(), alertID)
		if err != nil || !ok {
			return Detail{}, false
		}
		if alert.LinkedSessionID == "" {
			session := s.sessionSvc.CreateSession(user, fmt.Sprintf("告警排障: %s", alert.Title))
			alert.LinkedSessionID = session.ID
			alert.UpdatedAt = time.Now()
			record := alertDomain.HandlingRecord{
				ID:        nextID("record"),
				AlertID:   alertID,
				Action:    "link_session",
				Operator:  displayOperator(user),
				Comment:   "已创建排障会话并关联到当前告警。",
				CreatedAt: time.Now(),
			}
			if err := s.repo.SaveAlertWithRecord(context.Background(), alert, record); err != nil {
				return Detail{}, false
			}
		}
		return s.GetDetail(alertID)
	}

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
		s.persistLocked()
	}

	return s.detailLocked(alertID), true
}

func (s *Service) Analyze(user authDomain.User, alertID string) (Detail, bool) {
	if s.repo != nil {
		alert, ok, err := s.repo.GetAlert(context.Background(), alertID)
		if err != nil || !ok {
			return Detail{}, false
		}
		analysis := alertDomain.AlertAnalysis{
			Status:      "failed",
			Summary:     "当前分析器未初始化，无法生成告警分析结果。",
			Source:      "go-alert-service",
			GeneratedAt: time.Now(),
			Error:       "analyzer unavailable",
		}
		if s.analyzer != nil {
			analysis = s.analyzer.AnalyzeAlert(context.Background(), user, alert)
		}
		alert.Analysis = &analysis
		alert.UpdatedAt = time.Now()
		record := alertDomain.HandlingRecord{
			ID:        nextID("record"),
			AlertID:   alertID,
			Action:    "ai_analysis",
			Operator:  displayOperator(user),
			Comment:   buildAnalysisComment(analysis),
			CreatedAt: time.Now(),
		}
		if err := s.repo.SaveAlertWithRecord(context.Background(), alert, record); err != nil {
			return Detail{}, false
		}
		return s.GetDetail(alertID)
	}

	s.mu.Lock()
	alert, ok := s.alerts[alertID]
	if !ok {
		s.mu.Unlock()
		return Detail{}, false
	}
	s.mu.Unlock()

	analysis := alertDomain.AlertAnalysis{
		Status:      "failed",
		Summary:     "当前分析器未初始化，无法生成告警分析结果。",
		Source:      "go-alert-service",
		GeneratedAt: time.Now(),
		Error:       "analyzer unavailable",
	}
	if s.analyzer != nil {
		analysis = s.analyzer.AnalyzeAlert(context.Background(), user, alert)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	alert, ok = s.alerts[alertID]
	if !ok {
		return Detail{}, false
	}

	alert.Analysis = &analysis
	alert.UpdatedAt = time.Now()
	s.alerts[alertID] = alert
	s.records[alertID] = append(s.records[alertID], alertDomain.HandlingRecord{
		ID:        nextID("record"),
		AlertID:   alertID,
		Action:    "ai_analysis",
		Operator:  displayOperator(user),
		Comment:   buildAnalysisComment(analysis),
		CreatedAt: time.Now(),
	})
	s.persistLocked()

	return s.detailLocked(alertID), true
}

func (s *Service) mustIngest(input IngestInput) {
	if s.repo != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.ingestRepositoryLocked(input)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ingestLocked(input)
	s.persistLocked()
}

func (s *Service) ingestLocked(input IngestInput) alertDomain.Alert {
	now := time.Now()
	triggeredAt := now
	if input.TriggeredAt != nil {
		triggeredAt = *input.TriggeredAt
	}

	normalized := alertDomain.Alert{
		ID:              nextID("alert"),
		Title:           normalizeTitle(input.Title),
		Service:         normalizeFallback(input.Service, "unknown-service"),
		Environment:     normalizeFallback(input.Environment, "prod"),
		Severity:        normalizeSeverity(input.Severity),
		Source:          normalizeFallback(input.Source, "external"),
		Status:          "open",
		Summary:         normalizeFallback(input.Summary, "未提供摘要"),
		Description:     strings.TrimSpace(input.Description),
		Labels:          cloneLabels(input.Labels),
		OccurrenceCount: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
		TriggeredAt:     triggeredAt,
		LastTriggeredAt: triggeredAt,
	}

	if existingID, ok := s.findDuplicateAlertID(normalized); ok {
		existing := s.alerts[existingID]
		existing.Severity = maxSeverity(existing.Severity, normalized.Severity)
		existing.Summary = normalized.Summary
		existing.Description = normalized.Description
		existing.Labels = mergeLabels(existing.Labels, normalized.Labels)
		existing.Status = "open"
		existing.UpdatedAt = now
		existing.LastTriggeredAt = triggeredAt
		existing.OccurrenceCount++
		if existing.Analysis != nil {
			existing.Analysis.Status = "stale"
			existing.Analysis.Error = ""
		}
		s.alerts[existingID] = existing
		s.records[existingID] = append(s.records[existingID], alertDomain.HandlingRecord{
			ID:        nextID("record"),
			AlertID:   existingID,
			Action:    "deduplicate_ingest",
			Operator:  existing.Source,
			Comment:   "重复告警已合并到现有事件。",
			CreatedAt: now,
		})
		return existing
	}

	s.alerts[normalized.ID] = normalized
	s.records[normalized.ID] = append(s.records[normalized.ID], alertDomain.HandlingRecord{
		ID:        nextID("record"),
		AlertID:   normalized.ID,
		Action:    "ingest",
		Operator:  normalized.Source,
		Comment:   "告警已接入平台。",
		CreatedAt: now,
	})

	return normalized
}

func (s *Service) ingestRepositoryLocked(input IngestInput) alertDomain.Alert {
	now := time.Now()
	triggeredAt := now
	if input.TriggeredAt != nil {
		triggeredAt = *input.TriggeredAt
	}

	normalized := alertDomain.Alert{
		ID:              nextID("alert"),
		Title:           normalizeTitle(input.Title),
		Service:         normalizeFallback(input.Service, "unknown-service"),
		Environment:     normalizeFallback(input.Environment, "prod"),
		Severity:        normalizeSeverity(input.Severity),
		Source:          normalizeFallback(input.Source, "external"),
		Status:          "open",
		Summary:         normalizeFallback(input.Summary, "未提供摘要"),
		Description:     strings.TrimSpace(input.Description),
		Labels:          cloneLabels(input.Labels),
		OccurrenceCount: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
		TriggeredAt:     triggeredAt,
		LastTriggeredAt: triggeredAt,
	}

	existing, ok, err := s.repo.FindDuplicateOpenAlert(context.Background(), normalized)
	if err == nil && ok {
		existing.Severity = maxSeverity(existing.Severity, normalized.Severity)
		existing.Summary = normalized.Summary
		existing.Description = normalized.Description
		existing.Labels = mergeLabels(existing.Labels, normalized.Labels)
		existing.Status = "open"
		existing.UpdatedAt = now
		existing.LastTriggeredAt = triggeredAt
		existing.OccurrenceCount++
		if existing.Analysis != nil {
			existing.Analysis.Status = "stale"
			existing.Analysis.Error = ""
		}
		record := alertDomain.HandlingRecord{
			ID:        nextID("record"),
			AlertID:   existing.ID,
			Action:    "deduplicate_ingest",
			Operator:  existing.Source,
			Comment:   "重复告警已合并到现有事件。",
			CreatedAt: now,
		}
		_ = s.repo.SaveAlertWithRecord(context.Background(), existing, record)
		return existing
	}

	record := alertDomain.HandlingRecord{
		ID:        nextID("record"),
		AlertID:   normalized.ID,
		Action:    "ingest",
		Operator:  normalized.Source,
		Comment:   "告警已接入平台。",
		CreatedAt: now,
	}
	_ = s.repo.SaveAlertWithRecord(context.Background(), normalized, record)
	return normalized
}

func (s *Service) detailLocked(alertID string) Detail {
	alert := s.alerts[alertID]
	records := append([]alertDomain.HandlingRecord(nil), s.records[alertID]...)
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return Detail{
		Alert:   alert,
		Records: records,
		Stats:   s.statsLocked(),
	}
}

func (s *Service) statsLocked() Stats {
	stats := Stats{
		BySeverity: map[string]int{
			"P0": 0,
			"P1": 0,
			"P2": 0,
			"P3": 0,
		},
	}

	for _, alert := range s.alerts {
		stats.Total++
		stats.BySeverity[alert.Severity]++
		if alert.LinkedSessionID != "" {
			stats.LinkedSessions++
		}
		if alert.OccurrenceCount > 1 {
			stats.DeduplicatedHit += alert.OccurrenceCount - 1
		}
		switch alert.Status {
		case "resolved":
			stats.Resolved++
		case "investigating":
			stats.Investigating++
		default:
			stats.Open++
		}
	}
	return stats
}

func statsFromAlerts(items []alertDomain.Alert) Stats {
	stats := Stats{
		BySeverity: map[string]int{
			"P0": 0,
			"P1": 0,
			"P2": 0,
			"P3": 0,
		},
	}
	for _, alert := range items {
		stats.Total++
		stats.BySeverity[alert.Severity]++
		if alert.LinkedSessionID != "" {
			stats.LinkedSessions++
		}
		if alert.OccurrenceCount > 1 {
			stats.DeduplicatedHit += alert.OccurrenceCount - 1
		}
		switch alert.Status {
		case "resolved":
			stats.Resolved++
		case "investigating":
			stats.Investigating++
		default:
			stats.Open++
		}
	}
	return stats
}

func (s *Service) findDuplicateAlertID(candidate alertDomain.Alert) (string, bool) {
	for id, alert := range s.alerts {
		if alert.Service != candidate.Service {
			continue
		}
		if alert.Environment != candidate.Environment {
			continue
		}
		if alert.Source != candidate.Source {
			continue
		}
		if alert.Title != candidate.Title {
			continue
		}
		if alert.Status == "resolved" {
			continue
		}
		return id, true
	}
	return "", false
}

func (s *Service) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return
	}

	var stored store
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}
	if stored.Alerts != nil {
		s.alerts = stored.Alerts
	}
	if stored.Records != nil {
		s.records = stored.Records
	}
}

func (s *Service) persistLocked() {
	_ = os.MkdirAll(filepath.Dir(s.storagePath), 0o755)
	payload, err := json.MarshalIndent(store{
		Alerts:  s.alerts,
		Records: s.records,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.storagePath, payload, 0o644)
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

func buildAnalysisComment(analysis alertDomain.AlertAnalysis) string {
	if strings.TrimSpace(analysis.Summary) == "" {
		return "AI 分析已执行。"
	}
	return fmt.Sprintf("AI 分析已更新：%s", analysis.Summary)
}

func displayOperator(user authDomain.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(labels))
	for key, value := range labels {
		cloned[key] = value
	}
	return cloned
}

func mergeLabels(origin, incoming map[string]string) map[string]string {
	if len(origin) == 0 && len(incoming) == 0 {
		return nil
	}
	merged := cloneLabels(origin)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range incoming {
		merged[key] = value
	}
	return merged
}

func maxSeverity(left, right string) string {
	if severityRank(left) <= severityRank(right) {
		return left
	}
	return right
}

func severityRank(severity string) int {
	switch severity {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 99
	}
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
