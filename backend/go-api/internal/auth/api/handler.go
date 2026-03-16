package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	"github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *application.Service
	audit   *auditApp.Service
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewHandler(service *application.Service, auditService *auditApp.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		h.recordLoginAttempt(req.Username, "failed", "username and password are required")
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "BAD_REQUEST", "username and password are required")
		return
	}

	result, err := h.service.Login(application.LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		if application.IsInvalidCredentials(err) {
			h.recordLoginAttempt(req.Username, "failed", "invalid username or password")
			response.Failure(c.Writer, http.StatusUnauthorized, requestID(c), "INVALID_CREDENTIALS", "invalid username or password")
			return
		}

		h.recordLoginAttempt(req.Username, "failed", "unexpected login error")
		writeFailure(c, appErrors.ErrInternal)
		return
	}

	h.recordLoginSuccess(result.User)
	response.Success(c.Writer, http.StatusOK, requestID(c), result)
}

func (h *Handler) Me(c *gin.Context) {
	user, ok := CurrentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"user": user,
	})
}

func requestID(c *gin.Context) string {
	return utils.RequestIDFromContext(c.Request.Context())
}

func writeFailure(c *gin.Context, appErr appErrors.AppError) {
	response.Failure(c.Writer, appErr.HTTPStatus, requestID(c), appErr.Code, appErr.Message)
}

func (h *Handler) recordLoginAttempt(username, status, detail string) {
	if h.audit == nil {
		return
	}

	h.audit.Record(auditApp.RecordInput{
		Category:   "auth",
		Action:     "login",
		Status:     status,
		TargetType: "user",
		TargetName: username,
		Detail:     detail,
		Metadata: map[string]any{
			"username": username,
		},
	})
}

func (h *Handler) recordLoginSuccess(user authDomain.User) {
	if h.audit == nil {
		return
	}

	h.audit.Record(auditApp.RecordInput{
		Category:   "auth",
		Action:     "login",
		Status:     "success",
		Actor:      &user,
		TargetType: "user",
		TargetID:   user.ID,
		TargetName: user.Username,
		Detail:     "用户登录成功。",
	})
}
