package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	alertApp "github.com/oyzg/OnCall/backend/go-api/internal/alert/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *alertApp.Service
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

func NewHandler(service *alertApp.Service) *Handler {
	return &Handler{service: service}
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

	response.Success(c.Writer, http.StatusCreated, requestID(c), gin.H{"alert": alert})
}

func (h *Handler) ListAlerts(c *gin.Context) {
	items := h.service.ListAlerts(
		strings.TrimSpace(c.Query("status")),
		strings.TrimSpace(c.Query("severity")),
		strings.TrimSpace(c.Query("service")),
		strings.TrimSpace(c.Query("query")),
	)
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

	response.Success(c.Writer, http.StatusOK, requestID(c), detail)
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
