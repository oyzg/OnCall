package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *sessionApp.Service
}

type createSessionRequest struct {
	Title string `json:"title"`
}

type streamMessageRequest struct {
	Content string `json:"content"`
}

func NewHandler(service *sessionApp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateSession(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	session := h.service.CreateSession(user, req.Title)
	response.Success(c.Writer, http.StatusCreated, requestID(c), gin.H{"session": session})
}

func (h *Handler) ListSessions(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"sessions": h.service.ListSessions(user),
	})
}

func (h *Handler) DeleteSession(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	if deleted := h.service.DeleteSession(user, c.Param("sessionID")); !deleted {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"deleted": true})
}

func (h *Handler) ListMessages(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	messages, exists := h.service.ListMessages(user, c.Param("sessionID"))
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"messages": messages})
}

func (h *Handler) StreamMessage(c *gin.Context) {
	var req streamMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "BAD_REQUEST", "message content is required")
		return
	}

	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	assistantMessage, chunks, exists := h.service.StartAssistantReply(user, c.Param("sessionID"), content)
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	fullReply := strings.Builder{}
	for _, chunk := range chunks {
		fullReply.WriteString(chunk)
		writeSSE(c, "chunk", gin.H{
			"message_id": assistantMessage.ID,
			"delta":      chunk,
		})
		c.Writer.Flush()
		time.Sleep(60 * time.Millisecond)
	}

	h.service.CompleteAssistantReply(user, c.Param("sessionID"), assistantMessage.ID, fullReply.String())

	writeSSE(c, "done", gin.H{
		"message_id": assistantMessage.ID,
		"content":    fullReply.String(),
	})
	c.Writer.Flush()
}

func writeSSE(c *gin.Context, event string, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(c.Writer, "event: %s\n", event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
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
