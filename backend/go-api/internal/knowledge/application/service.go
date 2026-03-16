package application

import (
	"encoding/json"
	"errors"
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
	mu           sync.RWMutex
	documents    map[string]knowledgeDomain.Document
	rootDir      string
	metadataPath string
}

func NewService() *Service {
	rootDir := filepath.Join("tmp", "knowledge", "documents")
	metadataPath := filepath.Join("tmp", "knowledge", "metadata.json")
	_ = os.MkdirAll(rootDir, 0o755)

	service := &Service{
		documents:    make(map[string]knowledgeDomain.Document),
		rootDir:      rootDir,
		metadataPath: metadataPath,
	}
	service.load()
	return service
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
		ID:            documentID,
		UserID:        user.ID,
		Title:         title,
		Category:      category,
		SourceType:    sourceType,
		FileName:      fileName,
		ContentType:   normalizeContentType(sourceType, input.ContentType),
		StoragePath:   storagePath,
		SizeBytes:     int64(len(input.Content)),
		Status:        "uploaded",
		Summary:       "文档已上传，等待处理任务开始。",
		TextPreview:   "",
		ChunkPreviews: nil,
		ChunkCount:    0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.mu.Lock()
	s.documents[document.ID] = document
	s.persistLocked()
	s.mu.Unlock()

	go s.processDocument(document.ID)

	return document, nil
}

func (s *Service) ListDocuments(user authDomain.User, status, category, query string, limit int) []knowledgeDomain.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]knowledgeDomain.Document, 0)
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
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
		if normalizedQuery != "" &&
			!strings.Contains(strings.ToLower(document.Title), normalizedQuery) &&
			!strings.Contains(strings.ToLower(document.FileName), normalizedQuery) {
			continue
		}
		items = append(items, document)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

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

func (s *Service) DeleteDocument(user authDomain.User, documentID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	document, ok := s.documents[documentID]
	if !ok || document.UserID != user.ID {
		return false
	}

	_ = os.Remove(document.StoragePath)
	delete(s.documents, documentID)
	s.persistLocked()
	return true
}

func (s *Service) RetryDocument(user authDomain.User, documentID string) (knowledgeDomain.Document, error) {
	s.mu.Lock()
	document, ok := s.documents[documentID]
	if !ok || document.UserID != user.ID {
		s.mu.Unlock()
		return knowledgeDomain.Document{}, errors.New("document not found")
	}

	document.Status = "uploaded"
	document.Summary = "文档已重新入队，等待处理任务开始。"
	document.FailureReason = ""
	document.TextPreview = ""
	document.ChunkPreviews = nil
	document.ChunkCount = 0
	document.UpdatedAt = time.Now()
	document.ProcessedAt = nil
	s.documents[documentID] = document
	s.persistLocked()
	s.mu.Unlock()

	go s.processDocument(documentID)
	return document, nil
}

func (s *Service) processDocument(documentID string) {
	s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
		document.Status = "processing"
		document.Summary = "文档处理中，正在生成切片与预览。"
		document.UpdatedAt = time.Now()
	})

	time.Sleep(180 * time.Millisecond)

	s.mu.RLock()
	document, ok := s.documents[documentID]
	s.mu.RUnlock()
	if !ok {
		return
	}

	content, err := os.ReadFile(document.StoragePath)
	if err != nil {
		s.markFailed(documentID, "无法读取文档文件")
		return
	}

	text := strings.TrimSpace(string(content))
	if !isSupportedForPreview(document.ContentType, text, document.SourceType) {
		s.markFailed(documentID, "当前仅支持文本类文档的基础预处理，请上传 txt、md、log、json 等文本文件")
		return
	}

	chunks := splitTextChunks(text, 320, 60)
	chunkPreviews := make([]string, 0, min(3, len(chunks)))
	for _, chunk := range chunks {
		chunkPreviews = append(chunkPreviews, textPreview(chunk, 120))
		if len(chunkPreviews) >= 3 {
			break
		}
	}

	chunkCount := estimateChunkCount(text, len(content))
	if len(chunks) > 0 {
		chunkCount = len(chunks)
	}
	summary := "文档已完成基础入库，下一阶段将接入解析、切片、Embedding 与索引写入。"
	if text != "" {
		summary = textPreview(text, 96)
	}

	now := time.Now()
	s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
		document.Status = "ready"
		document.Summary = summary
		document.TextPreview = textPreview(text, 480)
		document.ChunkPreviews = chunkPreviews
		document.ChunkCount = chunkCount
		document.FailureReason = ""
		document.UpdatedAt = now
		document.ProcessedAt = &now
	})
}

func (s *Service) markFailed(documentID, reason string) {
	s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
		document.Status = "failed"
		document.Summary = "文档处理失败。"
		document.FailureReason = reason
		document.TextPreview = ""
		document.ChunkPreviews = nil
		document.ChunkCount = 0
		document.UpdatedAt = time.Now()
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
	s.persistLocked()
}

type metadataStore struct {
	Documents map[string]knowledgeDomain.Document `json:"documents"`
}

func (s *Service) load() {
	data, err := os.ReadFile(s.metadataPath)
	if err != nil {
		return
	}

	var stored metadataStore
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	if stored.Documents != nil {
		s.documents = stored.Documents
	}
}

func (s *Service) persistLocked() {
	_ = os.MkdirAll(filepath.Dir(s.metadataPath), 0o755)
	payload, err := json.MarshalIndent(metadataStore{
		Documents: s.documents,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.metadataPath, payload, 0o644)
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

func splitTextChunks(text string, chunkSize, overlap int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 320
	}
	if overlap < 0 {
		overlap = 0
	}

	step := max(1, chunkSize-overlap)
	chunks := make([]string, 0, len(runes)/step+1)
	for start := 0; start < len(runes); start += step {
		end := min(len(runes), start+chunkSize)
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func isSupportedForPreview(contentType, text, sourceType string) bool {
	if sourceType == "text" {
		return strings.TrimSpace(text) != ""
	}

	normalized := strings.ToLower(strings.TrimSpace(contentType))
	if strings.HasPrefix(normalized, "text/") {
		return true
	}
	for _, candidate := range []string{"json", "xml", "yaml", "markdown", "log"} {
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return utf8.ValidString(text)
}

func nextDocumentID() string {
	return fmt.Sprintf("doc_%d", time.Now().UnixNano())
}
