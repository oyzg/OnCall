package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	agentApp "github.com/oyzg/OnCall/backend/go-api/internal/agentaction/application"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *agentApp.Service
	audit   *auditApp.Service
}

func NewHandler(service *agentApp.Service, audit *auditApp.Service) *Handler {
	return &Handler{service: service, audit: audit}
}

func (h *Handler) Confirm(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	action, detail, err := h.service.ConfirmWithFollowUp(c.Request.Context(), user, c.Param("actionID"))
	if err != nil {
		h.record(user, c.Param("actionID"), "failed", err.Error())
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "AGENT_ACTION_FAILED", err.Error())
		return
	}
	h.record(user, action.ID, "success", "Agent action confirmed and executed.")
	payload := gin.H{"action": action}
	if detail != nil {
		payload["alert"] = detail.Alert
		payload["records"] = detail.Records
		payload["stats"] = detail.Stats
	}
	response.Success(c.Writer, http.StatusOK, requestID(c), payload)
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

func (h *Handler) record(user authDomain.User, actionID, status, detail string) {
	if h.audit == nil {
		return
	}
	h.audit.Record(auditApp.RecordInput{
		Category:   "agent_action",
		Action:     "confirm",
		Status:     status,
		Actor:      &user,
		TargetType: "agent_action",
		TargetID:   actionID,
		TargetName: actionID,
		Detail:     detail,
	})
}
