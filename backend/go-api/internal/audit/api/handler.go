package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *auditApp.Service
}

func NewHandler(service *auditApp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListLogs(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"logs": h.service.List(auditApp.ListOptions{
			Category: strings.TrimSpace(c.Query("category")),
			Action:   strings.TrimSpace(c.Query("action")),
			Status:   strings.TrimSpace(c.Query("status")),
			Actor:    strings.TrimSpace(c.Query("actor")),
			Limit:    limit,
		}),
		"categories": h.service.Categories(),
	})
}

func (h *Handler) Stats(c *gin.Context) {
	if _, ok := currentUser(c); !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"stats": h.service.BuildStats(),
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
