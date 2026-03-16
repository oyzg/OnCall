package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *retrieval.Service
}

type retrieveRequest struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	Limit    int    `json:"limit"`
}

func NewHandler(service *retrieval.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Retrieve(c *gin.Context) {
	var req retrieveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "BAD_REQUEST", "query is required")
		return
	}

	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	limit := req.Limit
	if limit <= 0 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(c.Query("limit"))); err == nil {
			limit = parsed
		}
	}
	report := h.service.RetrieveWithOptions(user, query, retrieval.RetrieveOptions{
		Limit:    limit,
		Category: strings.TrimSpace(req.Category),
	})
	response.Success(c.Writer, http.StatusOK, requestID(c), report)
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
