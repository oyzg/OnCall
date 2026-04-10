package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	toolApp "github.com/oyzg/OnCall/backend/go-api/internal/tool/application"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service             *toolApp.Service
	audit               *auditApp.Service
	runtimeSharedSecret string
}

type callToolRequest struct {
	Parameters map[string]any `json:"parameters"`
}

type internalCallToolRequest struct {
	UserID     string         `json:"user_id"`
	UserRoles  []string       `json:"user_roles"`
	Parameters map[string]any `json:"parameters"`
}

func NewHandler(service *toolApp.Service, auditService *auditApp.Service, runtimeSharedSecret string) *Handler {
	return &Handler{service: service, audit: auditService, runtimeSharedSecret: runtimeSharedSecret}
}

func (h *Handler) ListTools(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"tools": h.service.ListTools(user),
	})
}

func (h *Handler) CallTool(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	var req callToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	result, err := h.service.CallTool(user, c.Param("toolName"), req.Parameters)
	if err != nil {
		h.record(user, c.Param("toolName"), "failed", "工具调用失败。", map[string]any{
			"parameters": req.Parameters,
			"error":      err.Error(),
		})
		if appErr, ok := err.(appErrors.AppError); ok {
			writeFailure(c, appErr)
			return
		}
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "TOOL_EXECUTION_FAILED", err.Error())
		return
	}
	h.record(user, c.Param("toolName"), "success", "工具调用成功。", map[string]any{
		"parameters": req.Parameters,
	})

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"result": result,
	})
}

func (h *Handler) ListLogs(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	_ = user
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"logs": h.service.ListLogs(
			strings.TrimSpace(c.Query("tool_name")),
			strings.TrimSpace(c.Query("status")),
			limit,
		),
	})
}

func (h *Handler) CallToolInternal(c *gin.Context) {
	if strings.TrimSpace(c.GetHeader("X-OnCall-Runtime-Secret")) != h.runtimeSharedSecret {
		response.Failure(c.Writer, http.StatusForbidden, requestID(c), "FORBIDDEN", "invalid runtime secret")
		return
	}

	var req internalCallToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	user := authDomain.User{
		ID:          strings.TrimSpace(req.UserID),
		Username:    strings.TrimSpace(req.UserID),
		DisplayName: strings.TrimSpace(req.UserID),
		Roles:       append([]string(nil), req.UserRoles...),
	}
	result, err := h.service.CallTool(user, c.Param("toolName"), req.Parameters)
	if err != nil {
		if appErr, ok := err.(appErrors.AppError); ok {
			response.Failure(c.Writer, appErr.HTTPStatus, requestID(c), appErr.Code, appErr.Message)
			return
		}
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "TOOL_EXECUTION_FAILED", err.Error())
		return
	}
	h.record(user, c.Param("toolName"), "success", "runtime tool call succeeded.", map[string]any{
		"parameters": req.Parameters,
		"source":     "python-ai-runtime",
	})
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"result": result,
	})
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

func (h *Handler) record(user authDomain.User, toolName, status, detail string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	h.audit.Record(auditApp.RecordInput{
		Category:   "tool",
		Action:     "call",
		Status:     status,
		Actor:      &user,
		TargetType: "tool",
		TargetID:   toolName,
		TargetName: toolName,
		Detail:     detail,
		Metadata:   metadata,
	})
}
