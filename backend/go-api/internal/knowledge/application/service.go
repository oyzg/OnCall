package application

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
)

type UploadInput struct {
	Title       string
	Category    string
	SourceType  string
	FileName    string
	ContentType string
	Content     []byte
}

type Service struct {
	mu        sync.RWMutex
	documents map[string]knowledgeDomain.Document
	rootDir   string
}

func NewService() *Service {
	rootDir := filepath.Join("tmp", "knowledge", "documents")
	_ = os.MkdirAll(rootDir, 0o755)

	return &Service{
		documents: make(map[string]knowledgeDomain.Document),
		rootDir:   rootDir,
	}
}

func (s *Service) UploadDocument(user authDomain.User, input UploadInput) (knowledgeDomain.Document, error) {
	now := time.Now()
	documentID := nextDocumentID()
	sourceType := normalizeSourceType(input.SourceType, input.FileName)
	fileName := normalizeFileName(sourceType, input.FileName, documentID)
	title := normalizeTitle(input.Title, fileName)
	category := normalizeCategory(input.Category)
	storagePath := filepath.Join(s.rootDir, fmt.Sprintf("%s-%s", documentID, sanitizeFileName(fileName)))

	if err := os.WriteFile(storagePath, input.Content, 0o644); err != nil {
		return knowledgeDomain.Document{}, err
	}

	document := knowledgeDomain.Document{
		ID:          documentID,
		UserID:      user.ID,
		Title:       title,
		Category:    category,
		SourceType:  sourceType,
		FileName:    fileName,
		ContentType: normalizeContentType(sourceType, input.ContentType),
		StoragePath: storagePath,
		SizeBytes:   int64(len(input.Content)),
		Status:      "uploaded",
		Summary:     "文档已上传，等待处理任务开始。",
		ChunkCount:  0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	s.documents[document.ID] = document
	s.mu.Unlock()

	go s.processDocument(document.ID, input.Content)

	return document, nil
}

func (s *Service) ListDocuments(user authDomain.User, status, category string) []knowledgeDomain.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]knowledgeDomain.Document, 0)
	for _, document := range s.documents {
		if document.UserID != user.ID {
			continue
		}
		if status != "" && document.Status != status {
			continue
		}
		if category != "" && document.Category != category {
			continue
		}
		items = append(items, document)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	return items
}

func (s *Service) GetDocument(user authDomain.User, documentID string) (knowledgeDomain.Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	document, ok := s.documents[documentID]
	if !ok || document.UserID != user.ID {
		return knowledgeDomain.Document{}, false
	}

	return document, true
}

func (s *Service) processDocument(documentID string, content []byte) {
	s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
		document.Status = "processing"
		document.Summary = "文档处理中，正在生成切片任务骨架。"
		document.UpdatedAt = time.Now()
	})

	time.Sleep(180 * time.Millisecond)

	text := strings.TrimSpace(string(content))
	chunkCount := estimateChunkCount(text, len(content))
	summary := "文档已完成基础入库，下一阶段将接入解析、切片、Embedding 与索引写入。"
	if text != "" {
		summary = textPreview(text, 96)
	}

	now := time.Now()
	s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
		document.Status = "ready"
		document.Summary = summary
		document.ChunkCount = chunkCount
		document.UpdatedAt = now
		document.ProcessedAt = &now
	})
}

func (s *Service) updateDocument(documentID string, updater func(document *knowledgeDomain.Document)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	document, ok := s.documents[documentID]
	if !ok {
		return
	}

	updater(&document)
	s.documents[documentID] = document
}

func normalizeSourceType(sourceType, fileName string) string {
	trimmed := strings.TrimSpace(sourceType)
	if trimmed != "" {
		return trimmed
	}
	if strings.TrimSpace(fileName) != "" {
		return "file"
	}
	return "text"
}

func normalizeFileName(sourceType, fileName, documentID string) string {
	trimmed := strings.TrimSpace(fileName)
	if trimmed != "" {
		return trimmed
	}
	if sourceType == "text" {
		return documentID + ".txt"
	}
	return documentID + ".dat"
}

func normalizeTitle(title, fileName string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed != "" {
		return trimmed
	}
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if base == "" {
		return "未命名文档"
	}
	return base
}

func normalizeCategory(category string) string {
	trimmed := strings.TrimSpace(category)
	if trimmed == "" {
		return "general"
	}
	return trimmed
}

func normalizeContentType(sourceType, contentType string) string {
	if strings.TrimSpace(contentType) != "" {
		return contentType
	}
	if sourceType == "text" {
		return "text/plain"
	}
	return "application/octet-stream"
}

func sanitizeFileName(fileName string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "_")
	return replacer.Replace(fileName)
}

func textPreview(content string, maxRunes int) string {
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	return string(runes[:maxRunes]) + "..."
}

func estimateChunkCount(text string, byteSize int) int {
	if utf8.ValidString(text) && strings.TrimSpace(text) != "" {
		return max(1, int(math.Ceil(float64(utf8.RuneCountInString(text))/300.0)))
	}
	return max(1, int(math.Ceil(float64(byteSize)/1024.0)))
}

func nextDocumentID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}
