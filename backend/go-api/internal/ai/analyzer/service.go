package analyzer

import (
	"context"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/ai/eino"
	"github.com/oyzg/OnCall/backend/go-api/internal/ai/gateway"
	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
)

type Service struct {
	orchestrator eino.Orchestrator
}

func NewService(orchestrator eino.Orchestrator) *Service {
	return &Service{orchestrator: orchestrator}
}

func (s *Service) AnalyzeAlert(ctx context.Context, user authDomain.User, alert alertDomain.Alert) alertDomain.AlertAnalysis {
	if s == nil || s.orchestrator == nil {
		return fallbackAnalysis(alert)
	}

	response, err := s.orchestrator.HandleAlertAnalysis(ctx, gateway.AlertAnalysisRequest{
		AlertID:         alert.ID,
		Title:           alert.Title,
		Service:         alert.Service,
		Environment:     alert.Environment,
		Severity:        alert.Severity,
		Source:          alert.Source,
		Summary:         alert.Summary,
		Description:     alert.Description,
		Labels:          alert.Labels,
		TriggeredAt:     alert.LastTriggeredAt.Format(time.RFC3339),
		LinkedSessionID: alert.LinkedSessionID,
		UserID:          user.ID,
		UserRoles:       append([]string(nil), user.Roles...),
	})
	if err != nil {
		analysis := fallbackAnalysis(alert)
		analysis.Status = "failed"
		analysis.Error = err.Error()
		return analysis
	}

	generatedAt := time.Now()
	if response.GeneratedAt != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, response.GeneratedAt); parseErr == nil {
			generatedAt = parsed
		}
	}

	status := response.Status
	if status == "" {
		status = "ready"
	}

	return alertDomain.AlertAnalysis{
		Status:             status,
		Summary:            response.Summary,
		SeverityAssessment: response.SeverityAssessment,
		PossibleCauses:     append([]string(nil), response.PossibleCauses...),
		SuggestedActions:   append([]string(nil), response.SuggestedActions...),
		RecommendedTools:   append([]string(nil), response.RecommendedTools...),
		KnowledgeQueries:   append([]string(nil), response.KnowledgeQueries...),
		ToolCalls:          toAlertToolCalls(response.ToolCalls),
		Workflow:           response.Workflow,
		Confidence:         response.Confidence,
		Source:             response.Source,
		GeneratedAt:        generatedAt,
		Error:              response.Error,
		Trace:              toAlertTrace(response.Trace),
	}
}

func fallbackAnalysis(alert alertDomain.Alert) alertDomain.AlertAnalysis {
	return alertDomain.AlertAnalysis{
		Status:             "ready",
		Summary:            "当前走本地兜底分析。建议先围绕服务健康、相关告警和知识库 SOP 展开排查。",
		SeverityAssessment: "Python AI 服务不可用时，使用 Go 侧规则分析作为兜底路径。",
		PossibleCauses: []string{
			"最近的发布、配置变更或依赖抖动导致告警触发。",
			"服务自身负载上升，或下游依赖出现超时/错误放大。",
		},
		SuggestedActions: []string{
			"先查看服务状态、实例资源和最近 30 分钟内的相关告警。",
			"结合知识库 SOP 和历史案例继续补充排查上下文。",
		},
		RecommendedTools: []string{"service_status", "recent_alerts", "knowledge_search"},
		KnowledgeQueries: []string{
			alert.Service + " " + alert.Title,
			alert.Service + " " + alert.Summary,
		},
		Workflow:    "go_fallback_rule_analysis",
		Confidence:  "medium",
		Source:      "go-fallback-analyzer",
		GeneratedAt: time.Now(),
		Trace: []alertDomain.TraceEvent{
			{
				Stage:    "alert_analysis",
				Message:  "used go fallback analyzer because python runtime was unavailable",
				Severity: "warning",
			},
		},
	}
}

func toAlertTrace(items []gateway.TraceEvent) []alertDomain.TraceEvent {
	if len(items) == 0 {
		return nil
	}

	result := make([]alertDomain.TraceEvent, 0, len(items))
	for _, item := range items {
		result = append(result, alertDomain.TraceEvent{
			Stage:     item.Stage,
			Message:   item.Message,
			Severity:  item.Severity,
			Timestamp: item.Timestamp,
			Tags:      append([]string(nil), item.Tags...),
		})
	}
	return result
}

func toAlertToolCalls(items []gateway.ToolCall) []alertDomain.ToolCall {
	if len(items) == 0 {
		return nil
	}

	result := make([]alertDomain.ToolCall, 0, len(items))
	for _, item := range items {
		result = append(result, alertDomain.ToolCall{
			Name:          item.Name,
			ArgumentsJSON: item.ArgumentsJSON,
			Outcome:       item.Outcome,
			Summary:       item.Summary,
		})
	}
	return result
}
