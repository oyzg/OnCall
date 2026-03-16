package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	toolApp "github.com/oyzg/OnCall/backend/go-api/internal/tool/application"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *toolApp.Service
}

type callToolRequest struct {
	Parameters map[string]any `json:"parameters"`
}

func NewHandler(service *toolApp.Service) *Handler {
	return &Handler{service: service}
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
		if appErr, ok := err.(appErrors.AppError); ok {
			writeFailure(c, appErr)
			return
		}
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "TOOL_EXECUTION_FAILED", err.Error())
		return
	}

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
		"logs": h.service.ListLogs(strings.TrimSpace(c.Query("tool_name")), limit),
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
