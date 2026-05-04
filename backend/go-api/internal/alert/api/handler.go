package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	agentActionApp "github.com/oyzg/OnCall/backend/go-api/internal/agentaction/application"
	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service      *alertApp.Service
	audit        *auditApp.Service
	agentActions *agentActionApp.Service
}

type ingestRequest struct {
	Title       string            `json:"title"`
	Service     string            `json:"service"`
	Environment string            `json:"environment"`
	Severity    string            `json:"severity"`
	Source      string            `json:"source"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
}

type updateStatusRequest struct {
	Status  string `json:"status"`
	Comment string `json:"comment"`
}

func NewHandler(service *alertApp.Service, auditService *auditApp.Service, agentActionService *agentActionApp.Service) *Handler {
	return &Handler{service: service, audit: auditService, agentActions: agentActionService}
}

func (h *Handler) Ingest(c *gin.Context) {
	var req ingestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	alert := h.service.Ingest(alertApp.IngestInput{
		Title:       req.Title,
		Service:     req.Service,
		Environment: req.Environment,
		Severity:    req.Severity,
		Source:      req.Source,
		Summary:     req.Summary,
		Description: req.Description,
		Labels:      req.Labels,
	})
	h.audit.Record(auditApp.RecordInput{
		Category:   "alert",
		Action:     "ingest",
		Status:     "success",
		TargetType: "alert",
		TargetID:   alert.ID,
		TargetName: alert.Title,
		Detail:     "外部告警已接入平台。",
		Metadata: map[string]any{
			"service":     alert.Service,
			"environment": alert.Environment,
			"severity":    alert.Severity,
		},
	})

	response.Success(c.Writer, http.StatusCreated, requestID(c), gin.H{"alert": alert})
}

func (h *Handler) ListAlerts(c *gin.Context) {
	items := h.service.ListAlerts(
		strings.TrimSpace(c.Query("status")),
		strings.TrimSpace(c.Query("severity")),
		strings.TrimSpace(c.Query("service")),
		strings.TrimSpace(c.Query("query")),
	)
	h.hydrateAlertActions(c, items)
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"alerts": items,
		"stats":  h.service.BuildStats(),
	})
}

func (h *Handler) Stats(c *gin.Context) {
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"stats": h.service.BuildStats(),
	})
}

func (h *Handler) GetDetail(c *gin.Context) {
	detail, ok := h.service.GetDetail(c.Param("alertID"))
	if !ok {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	h.hydrateDetailActions(c, &detail)
	response.Success(c.Writer, http.StatusOK, requestID(c), detail)
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	detail, exists := h.service.UpdateStatus(user, c.Param("alertID"), alertApp.UpdateStatusInput{
		Status:  req.Status,
		Comment: req.Comment,
	})
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}
	h.record(user, "update_status", detail.Alert.ID, detail.Alert.Title, "告警状态已更新。", map[string]any{
		"status": detail.Alert.Status,
	})

	response.Success(c.Writer, http.StatusOK, requestID(c), detail)
}

func (h *Handler) LinkSession(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	detail, exists := h.service.LinkSession(user, c.Param("alertID"))
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}
	h.record(user, "link_session", detail.Alert.ID, detail.Alert.Title, "告警已关联排障会话。", map[string]any{
		"linked_session_id": detail.Alert.LinkedSessionID,
	})

	response.Success(c.Writer, http.StatusOK, requestID(c), detail)
}

func (h *Handler) Analyze(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	detail, exists := h.service.Analyze(user, c.Param("alertID"))
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}
	h.record(user, "analyze", detail.Alert.ID, detail.Alert.Title, "告警 AI 分析已生成。", map[string]any{
		"analysis_status": detail.Alert.Analysis.Status,
		"analysis_source": detail.Alert.Analysis.Source,
	})
	h.persistPendingActions(c, detail.Alert.ID, detail.Alert.Analysis.PendingActions)
	h.hydrateDetailActions(c, &detail)

	response.Success(c.Writer, http.StatusOK, requestID(c), detail)
}

func (h *Handler) persistPendingActions(c *gin.Context, alertID string, actions []alertDomain.PendingAgentAction) {
	if h.agentActions == nil || len(actions) == 0 {
		return
	}
	for _, action := range actions {
		_, _ = h.agentActions.Create(c.Request.Context(), agentActionApp.CreateInput{
			ID:            action.ActionID,
			SourceType:    "alert",
			SourceID:      alertID,
			ActionType:    action.ActionType,
			Title:         action.Title,
			Description:   action.Description,
			ArgumentsJSON: action.ArgumentsJSON,
			RiskLevel:     action.RiskLevel,
		})
	}
}

func (h *Handler) hydrateDetailActions(c *gin.Context, detail *alertApp.Detail) {
	if detail == nil {
		return
	}
	h.hydrateAlertAction(c, &detail.Alert)
}

func (h *Handler) hydrateAlertActions(c *gin.Context, alerts []alertDomain.Alert) {
	for index := range alerts {
		h.hydrateAlertAction(c, &alerts[index])
	}
}

func (h *Handler) hydrateAlertAction(c *gin.Context, alert *alertDomain.Alert) {
	if alert == nil || alert.Analysis == nil {
		return
	}
	alert.Analysis.PendingActions = nil
}

func currentUser(c *gin.Context) (authDomain.User, bool) {
	return authAPI.CurrentUser(c)
}

func requestID(c *gin.Context) string {
	return utils.RequestIDFromContext(c.Request.Context())
}

func writeFailure(c *gin.Context, appErr appErrors.AppError) {
	response.Failure(c.Writer, appErr.HTTPStatus, requestID(c), appErr.Code, appErr.Message)
}

func (h *Handler) record(
	user authDomain.User,
	action string,
	targetID string,
	targetName string,
	detail string,
	metadata map[string]any,
) {
	if h.audit == nil {
		return
	}
	h.audit.Record(auditApp.RecordInput{
		Category:   "alert",
		Action:     action,
		Status:     "success",
		Actor:      &user,
		TargetType: "alert",
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
		Metadata:   metadata,
	})
}
