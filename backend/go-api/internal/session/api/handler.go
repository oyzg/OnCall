package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/internal/ai/retrieval"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	sessionDomain "github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service   *sessionApp.Service
	retrieval *retrieval.Service
	audit     *auditApp.Service
}

type createSessionRequest struct {
	Title string `json:"title"`
}

type streamMessageRequest struct {
	Content string `json:"content"`
}

func NewHandler(service *sessionApp.Service, retrievalService *retrieval.Service, auditService *auditApp.Service) *Handler {
	return &Handler{
		service:   service,
		retrieval: retrievalService,
		audit:     auditService,
	}
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
	h.record(user, "session", "create", "success", session.ID, session.Title, "已创建会话。", map[string]any{
		"title": session.Title,
	})
	response.Success(c.Writer, http.StatusCreated, requestID(c), gin.H{"session": session})
}

func (h *Handler) ListSessions(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"sessions": h.service.ListSessions(
			user,
			strings.TrimSpace(c.Query("query")),
			parsePositiveInt(c.Query("limit")),
		),
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

	h.record(user, "session", "delete", "success", c.Param("sessionID"), "", "已删除会话。", nil)
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"deleted": true})
}

func (h *Handler) ListMessages(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	page, exists := h.service.ListMessages(
		user,
		c.Param("sessionID"),
		parsePositiveInt(c.Query("limit")),
		strings.TrimSpace(c.Query("before_id")),
	)
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{
		"messages":    page.Messages,
		"total":       page.Total,
		"has_more":    page.HasMore,
		"next_cursor": page.NextCursor,
	})
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

	assistantMessage, exists := h.service.StartAssistantReply(user, c.Param("sessionID"), content)
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}
	h.record(user, "session", "send_message", "success", c.Param("sessionID"), "", "用户发送了一条消息。", map[string]any{
		"content_length": len([]rune(content)),
	})

	references := h.retrieval.Retrieve(user, content, 3)
	replyContent := retrieval.BuildAnswer(content, references)
	replyChunks := splitReplyChunks(replyContent, 18)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	fullReply := strings.Builder{}
	for _, chunk := range replyChunks {
		fullReply.WriteString(chunk)
		writeSSE(c, "chunk", gin.H{
			"message_id": assistantMessage.ID,
			"delta":      chunk,
		})
		c.Writer.Flush()
		time.Sleep(60 * time.Millisecond)
	}

	sessionReferences := toSessionReferences(references)
	h.service.CompleteAssistantReply(
		user,
		c.Param("sessionID"),
		assistantMessage.ID,
		fullReply.String(),
		sessionReferences,
	)

	writeSSE(c, "done", gin.H{
		"message_id": assistantMessage.ID,
		"content":    fullReply.String(),
		"references": sessionReferences,
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

func splitReplyChunks(content string, chunkSize int) []string {
	if chunkSize <= 0 || utf8.RuneCountInString(content) <= chunkSize {
		return []string{content}
	}

	runes := []rune(content)
	chunks := make([]string, 0, len(runes)/chunkSize+1)
	for start := 0; start < len(runes); start += chunkSize {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func toSessionReferences(references []retrieval.Reference) []sessionDomain.Reference {
	items := make([]sessionDomain.Reference, 0, len(references))
	for _, reference := range references {
		items = append(items, sessionDomain.Reference{
			DocumentID:    reference.DocumentID,
			DocumentTitle: reference.DocumentTitle,
			Category:      reference.Category,
			Excerpt:       reference.Chunk,
			Score:         reference.Score,
		})
	}
	return items
}

func parsePositiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func (h *Handler) record(
	user authDomain.User,
	category string,
	action string,
	status string,
	targetID string,
	targetName string,
	detail string,
	metadata map[string]any,
) {
	if h.audit == nil {
		return
	}
	h.audit.Record(auditApp.RecordInput{
		Category:   category,
		Action:     action,
		Status:     status,
		Actor:      &user,
		TargetType: "session",
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
		Metadata:   metadata,
	})
}
