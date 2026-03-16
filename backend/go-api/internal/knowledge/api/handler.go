package api

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	auditApp "github.com/oyzg/OnCall/backend/go-api/internal/audit/application"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *knowledgeApp.Service
	audit   *auditApp.Service
}

func NewHandler(service *knowledgeApp.Service, auditService *auditApp.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

func (h *Handler) UploadDocument(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	category := strings.TrimSpace(c.PostForm("category"))
	textContent := strings.TrimSpace(c.PostForm("content"))
	fileHeader, err := c.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		writeFailure(c, appErrors.ErrBadRequest)
		return
	}

	if fileHeader == nil && textContent == "" {
		response.Failure(c.Writer, http.StatusBadRequest, requestID(c), "BAD_REQUEST", "file or content is required")
		return
	}

	input := knowledgeApp.UploadInput{
		Title:    title,
		Category: category,
	}

	if fileHeader != nil {
		file, err := fileHeader.Open()
		if err != nil {
			writeFailure(c, appErrors.ErrInternal)
			return
		}
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			writeFailure(c, appErrors.ErrInternal)
			return
		}

		input.SourceType = "file"
		input.FileName = fileHeader.Filename
		input.ContentType = fileHeader.Header.Get("Content-Type")
		input.Content = content
	} else {
		input.SourceType = "text"
		input.FileName = ""
		input.Content = []byte(textContent)
	}

	document, err := h.service.UploadDocument(user, input)
	if err != nil {
		writeFailure(c, appErrors.ErrInternal)
		return
	}

	h.record(user, "upload", "success", document.ID, document.Title, "知识文档已上传。", map[string]any{
		"category":    document.Category,
		"source_type": document.SourceType,
		"status":      document.Status,
	})
	response.Success(c.Writer, http.StatusCreated, requestID(c), gin.H{"document": document})
}

func (h *Handler) ListDocuments(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	items := h.service.ListDocuments(
		user,
		strings.TrimSpace(c.Query("status")),
		strings.TrimSpace(c.Query("category")),
		strings.TrimSpace(c.Query("query")),
		parsePositiveInt(c.Query("limit")),
	)
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"documents": items})
}

func (h *Handler) GetDocument(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	document, exists := h.service.GetDocument(user, c.Param("documentID"))
	if !exists {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"document": document})
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	if deleted := h.service.DeleteDocument(user, c.Param("documentID")); !deleted {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	h.record(user, "delete", "success", c.Param("documentID"), "", "知识文档已删除。", nil)
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"deleted": true})
}

func (h *Handler) RetryDocument(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	document, err := h.service.RetryDocument(user, c.Param("documentID"))
	if err != nil {
		writeFailure(c, appErrors.ErrNotFound)
		return
	}

	h.record(user, "reprocess", "success", document.ID, document.Title, "知识文档已重新处理。", map[string]any{
		"status": document.Status,
	})
	response.Success(c.Writer, http.StatusOK, requestID(c), gin.H{"document": document})
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

func parsePositiveInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func (h *Handler) record(
	user authDomain.User,
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
		Category:   "knowledge",
		Action:     action,
		Status:     status,
		Actor:      &user,
		TargetType: "document",
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
		Metadata:   metadata,
	})
}
