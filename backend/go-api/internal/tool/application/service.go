package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval"
	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	toolDomain "github.com/oyzg/OnCall/backend/go-api/internal/tool/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
)

type executor func(user authDomain.User, params map[string]any) (any, error)

type registration struct {
	definition toolDomain.Tool
	exec       executor
}

type Service struct {
	mu          sync.RWMutex
	tools       map[string]registration
	logs        []toolDomain.CallLog
	alerts      *alertApp.Service
	retrieval   *retrieval.Service
	sessions    *sessionApp.Service
	knowledge   *knowledgeApp.Service
	storagePath string
	repo        Repository
}

func NewService(
	alertService *alertApp.Service,
	retrievalService *retrieval.Service,
	sessionService *sessionApp.Service,
	knowledgeService *knowledgeApp.Service,
) *Service {
	service := &Service{
		tools:       make(map[string]registration),
		logs:        make([]toolDomain.CallLog, 0, 32),
		alerts:      alertService,
		retrieval:   retrievalService,
		sessions:    sessionService,
		knowledge:   knowledgeService,
		storagePath: filepath.Join("tmp", "tools", "call-logs.json"),
	}
	service.load()

	service.register(toolDomain.Tool{
		Name:         "service_status",
		DisplayName:  "服务状态查询",
		Description:  "根据服务名聚合相关告警与风险概览，快速判断当前服务状态。",
		Category:     "observability",
		AllowedRoles: []string{"admin", "ops"},
		Parameters: []toolDomain.Parameter{
			{Name: "service", Type: "string", Description: "服务名，如 payment-api", Required: true},
			{Name: "environment", Type: "string", Description: "环境，可选，如 prod/staging", Required: false},
		},
	}, service.executeServiceStatus)

	service.register(toolDomain.Tool{
		Name:         "recent_alerts",
		DisplayName:  "近期告警查询",
		Description:  "按服务与状态查询近期告警列表，用于排障时快速回看上下文。",
		Category:     "alerts",
		AllowedRoles: []string{"admin", "ops"},
		Parameters: []toolDomain.Parameter{
			{Name: "service", Type: "string", Description: "服务名，可选", Required: false},
			{Name: "status", Type: "string", Description: "告警状态，可选", Required: false},
			{Name: "limit", Type: "number", Description: "返回条数，默认 5", Required: false, Default: 5},
		},
	}, service.executeRecentAlerts)

	service.register(toolDomain.Tool{
		Name:         "knowledge_search",
		DisplayName:  "知识检索测试",
		Description:  "直接调用当前文本检索 RAG，对问题返回命中片段与简要总结。",
		Category:     "knowledge",
		AllowedRoles: []string{"admin", "ops"},
		Parameters: []toolDomain.Parameter{
			{Name: "query", Type: "string", Description: "检索问题或故障描述", Required: true},
			{Name: "limit", Type: "number", Description: "返回片段条数，默认 3", Required: false, Default: 3},
		},
	}, service.executeKnowledgeSearch)

	service.register(toolDomain.Tool{
		Name:         "platform_overview",
		DisplayName:  "平台概览",
		Description:  "查看当前账号可见的会话、知识文档与告警概览，仅管理员可用。",
		Category:     "admin",
		AllowedRoles: []string{"admin"},
		Parameters:   []toolDomain.Parameter{},
	}, service.executePlatformOverview)

	return service
}

func NewServiceWithRepository(
	alertService *alertApp.Service,
	retrievalService *retrieval.Service,
	sessionService *sessionApp.Service,
	knowledgeService *knowledgeApp.Service,
	repo Repository,
) *Service {
	service := NewService(alertService, retrievalService, sessionService, knowledgeService)
	service.repo = repo
	return service
}

func (s *Service) register(definition toolDomain.Tool, exec executor) {
	s.tools[definition.Name] = registration{
		definition: definition,
		exec:       exec,
	}
}

func (s *Service) ListTools(user authDomain.User) []toolDomain.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]toolDomain.Tool, 0, len(s.tools))
	for _, item := range s.tools {
		definition := item.definition
		definition.Available = hasAnyRole(user.Roles, definition.AllowedRoles)
		if !definition.Available {
			definition.UnavailableReason = "当前账号无权调用该工具"
		}
		items = append(items, definition)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			return items[i].DisplayName < items[j].DisplayName
		}
		return items[i].Category < items[j].Category
	})
	return items
}

func (s *Service) CallTool(user authDomain.User, toolName string, params map[string]any) (any, error) {
	s.mu.RLock()
	registration, ok := s.tools[toolName]
	s.mu.RUnlock()
	if !ok {
		return nil, appErrors.ErrNotFound
	}

	if !hasAnyRole(user.Roles, registration.definition.AllowedRoles) {
		s.appendLog(toolDomain.CallLog{
			ID:         nextID("toolcall"),
			ToolName:   toolName,
			Operator:   displayOperator(user),
			UserID:     user.ID,
			Status:     "forbidden",
			Input:      cloneMap(params),
			Error:      "permission denied",
			CreatedAt:  time.Now(),
			DurationMS: 0,
		})
		return nil, appErrors.ErrForbidden
	}

	validatedParams, err := validateParams(registration.definition.Parameters, params)
	if err != nil {
		s.appendLog(toolDomain.CallLog{
			ID:         nextID("toolcall"),
			ToolName:   toolName,
			Operator:   displayOperator(user),
			UserID:     user.ID,
			Status:     "failed",
			Input:      cloneMap(params),
			Error:      err.Error(),
			CreatedAt:  time.Now(),
			DurationMS: 0,
		})
		return nil, appErrors.New("BAD_REQUEST", err.Error(), appErrors.ErrBadRequest.HTTPStatus)
	}

	startedAt := time.Now()
	output, execErr := registration.exec(user, validatedParams)
	duration := time.Since(startedAt).Milliseconds()

	logEntry := toolDomain.CallLog{
		ID:         nextID("toolcall"),
		ToolName:   toolName,
		Operator:   displayOperator(user),
		UserID:     user.ID,
		Status:     "success",
		Input:      cloneMap(validatedParams),
		Output:     output,
		CreatedAt:  startedAt,
		DurationMS: duration,
	}

	if execErr != nil {
		logEntry.Status = "failed"
		logEntry.Error = execErr.Error()
		s.appendLog(logEntry)
		return nil, appErrors.New("TOOL_EXECUTION_FAILED", execErr.Error(), appErrors.ErrBadRequest.HTTPStatus)
	}

	s.appendLog(logEntry)
	return output, nil
}

func (s *Service) ListLogs(toolName, status string, limit int) []toolDomain.CallLog {
	if s.repo != nil {
		items, err := s.repo.ListLogs(context.Background(), toolName, status, limit)
		if err == nil {
			return items
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	items := make([]toolDomain.CallLog, 0, len(s.logs))
	for _, log := range s.logs {
		if toolName != "" && log.ToolName != toolName {
			continue
		}
		if status != "" && log.Status != status {
			continue
		}
		items = append(items, log)
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (s *Service) appendLog(entry toolDomain.CallLog) {
	if s.repo != nil {
		_ = s.repo.AppendLog(context.Background(), entry)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.logs = append([]toolDomain.CallLog{entry}, s.logs...)
	if len(s.logs) > 200 {
		s.logs = s.logs[:200]
	}
	s.persistLocked()
}

func (s *Service) executeServiceStatus(_ authDomain.User, params map[string]any) (any, error) {
	serviceName := strings.TrimSpace(asString(params["service"]))
	environment := strings.TrimSpace(asString(params["environment"]))
	alerts := s.alerts.ListAlerts("", "", serviceName, "")

	totalAlerts := 0
	activeAlerts := 0
	highestSeverity := ""
	latestSummary := ""
	for _, alert := range alerts {
		if environment != "" && alert.Environment != environment {
			continue
		}
		totalAlerts++
		if alert.Status != "resolved" {
			activeAlerts++
		}
		if highestSeverity == "" || severityRank(alert.Severity) < severityRank(highestSeverity) {
			highestSeverity = alert.Severity
		}
		if latestSummary == "" {
			latestSummary = alert.Summary
		}
	}

	risk := "healthy"
	if activeAlerts > 0 {
		risk = "degraded"
	}
	if highestSeverity == "P0" || highestSeverity == "P1" {
		risk = "critical"
	}

	return map[string]any{
		"service":          serviceName,
		"environment":      fallback(environment, "all"),
		"total_alerts":     totalAlerts,
		"active_alerts":    activeAlerts,
		"highest_severity": fallback(highestSeverity, "none"),
		"latest_summary":   fallback(latestSummary, "暂无相关告警"),
		"risk_level":       risk,
	}, nil
}

func (s *Service) executeRecentAlerts(_ authDomain.User, params map[string]any) (any, error) {
	serviceName := strings.TrimSpace(asString(params["service"]))
	status := strings.TrimSpace(asString(params["status"]))
	limit := asInt(params["limit"], 5)
	if limit <= 0 {
		limit = 5
	}

	items := s.alerts.ListAlerts(status, "", serviceName, "")
	if len(items) > limit {
		items = items[:limit]
	}

	return map[string]any{
		"count":  len(items),
		"alerts": items,
	}, nil
}

func (s *Service) executeKnowledgeSearch(user authDomain.User, params map[string]any) (any, error) {
	query := strings.TrimSpace(asString(params["query"]))
	limit := asInt(params["limit"], 3)
	if limit <= 0 {
		limit = 3
	}

	report := s.retrieval.RetrieveWithOptions(user, query, retrieval.RetrieveOptions{
		Limit: limit,
	})
	return report, nil
}

func (s *Service) executePlatformOverview(user authDomain.User, _ map[string]any) (any, error) {
	alerts := s.alerts.ListAlerts("", "", "", "")
	openAlerts := 0
	for _, alert := range alerts {
		if alert.Status != "resolved" {
			openAlerts++
		}
	}

	return map[string]any{
		"user": map[string]any{
			"id":           user.ID,
			"display_name": displayOperator(user),
			"roles":        user.Roles,
		},
		"sessions": map[string]any{
			"count": len(s.sessions.ListSessions(user, "", 0)),
		},
		"knowledge_documents": map[string]any{
			"count": len(s.knowledge.ListDocuments(user, "", "", "", 0)),
			"ready": len(s.knowledge.ListDocuments(user, "ready", "", "", 0)),
		},
		"alerts": map[string]any{
			"count": len(alerts),
			"open":  openAlerts,
		},
	}, nil
}

func validateParams(definitions []toolDomain.Parameter, params map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(definitions))

	for _, parameter := range definitions {
		value, exists := params[parameter.Name]
		if !exists || value == nil || strings.TrimSpace(asString(value)) == "" {
			if parameter.Required {
				return nil, fmt.Errorf("%s is required", parameter.Name)
			}
			if parameter.Default != nil {
				normalized[parameter.Name] = parameter.Default
			}
			continue
		}

		switch parameter.Type {
		case "number":
			number, err := toInt(value)
			if err != nil {
				return nil, fmt.Errorf("%s must be a number", parameter.Name)
			}
			normalized[parameter.Name] = number
		default:
			normalized[parameter.Name] = strings.TrimSpace(asString(value))
		}
	}

	return normalized, nil
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	items := make(map[string]any, len(input))
	for key, value := range input {
		items[key] = value
	}
	return items
}

func toInt(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(typed))
	default:
		return 0, fmt.Errorf("unsupported type")
	}
}

func asInt(value any, fallbackValue int) int {
	number, err := toInt(value)
	if err != nil {
		return fallbackValue
	}
	return number
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.Itoa(int(typed))
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func hasAnyRole(userRoles, allowedRoles []string) bool {
	for _, userRole := range userRoles {
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				return true
			}
		}
	}
	return false
}

func displayOperator(user authDomain.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
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

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func nextID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func (s *Service) load() {
	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		return
	}
	var logs []toolDomain.CallLog
	if err := json.Unmarshal(data, &logs); err != nil {
		return
	}
	s.logs = logs
}

func (s *Service) persistLocked() {
	_ = os.MkdirAll(filepath.Dir(s.storagePath), 0o755)
	payload, err := json.MarshalIndent(s.logs, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.storagePath, payload, 0o644)
}
