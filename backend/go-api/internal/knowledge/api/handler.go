package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	authAPI "github.com/oyzg/OnCall/backend/go-api/internal/auth/api"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	appErrors "github.com/oyzg/OnCall/backend/go-api/pkg/errors"
	"github.com/oyzg/OnCall/backend/go-api/pkg/response"
	"github.com/oyzg/OnCall/backend/go-api/pkg/utils"
)

type Handler struct {
	service *knowledgeApp.Service
}

func NewHandler(service *knowledgeApp.Service) *Handler {
	return &Handler{service: service}
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

	response.Success(c.Writer, http.StatusCreated, requestID(c), gin.H{"document": document})
}

func (h *Handler) ListDocuments(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		writeFailure(c, appErrors.ErrUnauthorized)
		return
	}

	items := h.service.ListDocuments(user, strings.TrimSpace(c.Query("status")), strings.TrimSpace(c.Query("category")))
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

func currentUser(c *gin.Context) (authDomain.User, bool) {
	return authAPI.CurrentUser(c)
}

func requestID(c *gin.Context) string {
	return utils.RequestIDFromContext(c.Request.Context())
}

func writeFailure(c *gin.Context, appErr appErrors.AppError) {
	response.Failure(c.Writer, appErr.HTTPStatus, requestID(c), appErr.Code, appErr.Message)
}
