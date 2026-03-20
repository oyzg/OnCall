package application

import (
	"context"
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

type Chunk struct {
	DocumentID    string   `json:"document_id"`
	DocumentTitle string   `json:"document_title"`
	Category      string   `json:"category"`
	Index         int      `json:"index"`
	Content       string   `json:"content"`
	Normalized    string   `json:"normalized"`
	Terms         []string `json:"terms"`
	CharTerms     []string `json:"char_terms"`
}

type IndexRequest struct {
	UserID        string
	DocumentID    string
	DocumentTitle string
	Category      string
	Chunks        []Chunk
}

type IndexResult struct {
	Status           string
	EmbeddingBackend string
	VectorBackend    string
	LexicalBackend   string
}

type Indexer interface {
	IndexDocument(ctx context.Context, request IndexRequest) (IndexResult, error)
	DeleteDocument(ctx context.Context, documentID string) error
}

type Service struct {
	mu           sync.RWMutex
	documents    map[string]knowledgeDomain.Document
	rootDir      string
	chunksDir    string
	metadataPath string
	indexer      Indexer
	repo         Repository
}

func NewService() *Service {
	rootDir := filepath.Join("tmp", "knowledge", "documents")
	chunksDir := filepath.Join("tmp", "knowledge", "chunks")
	metadataPath := filepath.Join("tmp", "knowledge", "metadata.json")
	_ = os.MkdirAll(rootDir, 0o755)
	_ = os.MkdirAll(chunksDir, 0o755)

	service := &Service{
		documents:    make(map[string]knowledgeDomain.Document),
		rootDir:      rootDir,
		chunksDir:    chunksDir,
		metadataPath: metadataPath,
	}
	service.load()
	return service
}

func NewServiceWithRepository(repo Repository) *Service {
	rootDir := filepath.Join("tmp", "knowledge", "documents")
	_ = os.MkdirAll(rootDir, 0o755)
	return &Service{
		documents: make(map[string]knowledgeDomain.Document),
		rootDir:   rootDir,
		repo:      repo,
	}
}

func (s *Service) SetIndexer(indexer Indexer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.indexer = indexer
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
		IndexStatus:   "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if s.repo != nil {
		if err := s.repo.SaveDocument(context.Background(), document); err != nil {
			return knowledgeDomain.Document{}, err
		}
	} else {
		s.mu.Lock()
		s.documents[document.ID] = document
		s.persistLocked()
		s.mu.Unlock()
	}

	go s.processDocument(document.ID)

	return document, nil
}

func (s *Service) ListDocuments(user authDomain.User, status, category, query string, limit int) []knowledgeDomain.Document {
	if s.repo != nil {
		items, err := s.repo.ListDocuments(context.Background(), user.ID, status, category, query, limit)
		if err == nil {
			return items
		}
	}

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
	if s.repo != nil {
		document, ok, err := s.repo.GetDocument(context.Background(), user.ID, documentID)
		if err == nil {
			return document, ok
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	document, ok := s.documents[documentID]
	if !ok || document.UserID != user.ID {
		return knowledgeDomain.Document{}, false
	}

	return document, true
}

func (s *Service) DeleteDocument(user authDomain.User, documentID string) bool {
	if s.repo != nil {
		document, ok, err := s.repo.DeleteDocument(context.Background(), user.ID, documentID)
		if err != nil || !ok {
			return false
		}
		_ = os.Remove(document.StoragePath)
		indexer := s.indexer
		go deleteIndexedDocument(indexer, documentID)
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	document, ok := s.documents[documentID]
	if !ok || document.UserID != user.ID {
		return false
	}

	_ = os.Remove(document.StoragePath)
	_ = os.Remove(s.chunkPath(documentID))
	delete(s.documents, documentID)
	s.persistLocked()
	indexer := s.indexer
	go deleteIndexedDocument(indexer, documentID)
	return true
}

func (s *Service) RetryDocument(user authDomain.User, documentID string) (knowledgeDomain.Document, error) {
	if s.repo != nil {
		document, ok, err := s.repo.GetDocument(context.Background(), user.ID, documentID)
		if err != nil || !ok {
			return knowledgeDomain.Document{}, errors.New("document not found")
		}

		document.Status = "uploaded"
		document.Summary = "文档已重新入队，等待处理任务开始。"
		document.FailureReason = ""
		document.TextPreview = ""
		document.ChunkPreviews = nil
		document.ChunkCount = 0
		document.IndexStatus = ""
		document.EmbeddingBackend = ""
		document.VectorBackend = ""
		document.LexicalBackend = ""
		document.IndexError = ""
		document.UpdatedAt = time.Now()
		document.ProcessedAt = nil
		document.IndexedAt = nil
		if err := s.repo.SaveDocument(context.Background(), document); err != nil {
			return knowledgeDomain.Document{}, err
		}
		if err := s.repo.ReplaceChunks(context.Background(), documentID, nil); err != nil {
			return knowledgeDomain.Document{}, err
		}

		indexer := s.indexer
		go deleteIndexedDocument(indexer, documentID)
		go s.processDocument(documentID)
		return document, nil
	}

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
	document.IndexStatus = ""
	document.EmbeddingBackend = ""
	document.VectorBackend = ""
	document.LexicalBackend = ""
	document.IndexError = ""
	document.UpdatedAt = time.Now()
	document.ProcessedAt = nil
	document.IndexedAt = nil
	_ = os.Remove(s.chunkPath(documentID))
	s.documents[documentID] = document
	s.persistLocked()
	indexer := s.indexer
	s.mu.Unlock()

	go deleteIndexedDocument(indexer, documentID)
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
	if s.repo != nil {
		var err error
		document, ok, err = s.repo.GetDocumentByID(context.Background(), documentID)
		if err != nil || !ok {
			return
		}
	}
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
	if err := s.writeChunks(document, chunks); err != nil {
		s.markFailed(documentID, "无法写入文档切片结果")
		return
	}
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

	if result, err := s.indexReadyDocument(document.UserID, documentID); err != nil {
		s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
			document.IndexStatus = "failed"
			document.EmbeddingBackend = ""
			document.VectorBackend = ""
			document.LexicalBackend = ""
			document.IndexError = err.Error()
			document.IndexedAt = nil
			document.Summary = textPreview(summary+" 外部索引同步失败，当前仍可使用本地检索兜底。", 96)
			document.UpdatedAt = time.Now()
		})
	} else if result.Status != "" {
		s.updateDocument(documentID, func(document *knowledgeDomain.Document) {
			now := time.Now()
			document.IndexStatus = result.Status
			document.EmbeddingBackend = result.EmbeddingBackend
			document.VectorBackend = result.VectorBackend
			document.LexicalBackend = result.LexicalBackend
			document.IndexError = ""
			document.IndexedAt = &now
			document.UpdatedAt = now
		})
	}
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
	if s.repo != nil {
		_ = s.repo.ReplaceChunks(context.Background(), documentID, nil)
		return
	}
	_ = os.Remove(s.chunkPath(documentID))
}

func (s *Service) ListReadyChunks(user authDomain.User, category string) []Chunk {
	if s.repo != nil {
		items, err := s.repo.ListReadyChunks(context.Background(), user.ID, category)
		if err == nil {
			return items
		}
	}

	documents := s.ListDocuments(user, "ready", category, "", 0)
	items := make([]Chunk, 0)
	for _, document := range documents {
		chunks, err := s.loadDocumentChunks(document)
		if err != nil {
			continue
		}
		items = append(items, chunks...)
	}
	return items
}

func (s *Service) updateDocument(documentID string, updater func(document *knowledgeDomain.Document)) {
	if s.repo != nil {
		document, ok, err := s.repo.GetDocumentByID(context.Background(), documentID)
		if err != nil || !ok {
			return
		}
		updater(&document)
		_ = s.repo.SaveDocument(context.Background(), document)
		return
	}

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

func (s *Service) writeChunks(document knowledgeDomain.Document, chunks []string) error {
	items := make([]Chunk, 0, len(chunks))
	for index, chunk := range chunks {
		normalized := normalizeChunkContent(chunk)
		items = append(items, Chunk{
			DocumentID:    document.ID,
			DocumentTitle: document.Title,
			Category:      document.Category,
			Index:         index,
			Content:       chunk,
			Normalized:    normalized,
			Terms:         extractChunkTerms(normalized),
			CharTerms:     extractChunkCharTerms(normalized),
		})
	}

	if s.repo != nil {
		return s.repo.ReplaceChunks(context.Background(), document.ID, items)
	}
	payload, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.chunkPath(document.ID), payload, 0o644)
}

func (s *Service) loadDocumentChunks(document knowledgeDomain.Document) ([]Chunk, error) {
	if s.repo != nil {
		return s.repo.ListChunksByDocument(context.Background(), document.ID)
	}
	data, err := os.ReadFile(s.chunkPath(document.ID))
	if err != nil {
		return nil, err
	}

	var chunks []Chunk
	if err := json.Unmarshal(data, &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

func (s *Service) chunkPath(documentID string) string {
	return filepath.Join(s.chunksDir, documentID+".json")
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

func (s *Service) indexReadyDocument(userID, documentID string) (IndexResult, error) {
	s.mu.RLock()
	document, ok := s.documents[documentID]
	indexer := s.indexer
	s.mu.RUnlock()
	if !ok || indexer == nil {
		return IndexResult{}, nil
	}

	chunks, err := s.loadDocumentChunks(document)
	if err != nil {
		return IndexResult{}, err
	}

	return indexer.IndexDocument(context.Background(), IndexRequest{
		UserID:        userID,
		DocumentID:    document.ID,
		DocumentTitle: document.Title,
		Category:      document.Category,
		Chunks:        chunks,
	})
}

func deleteIndexedDocument(indexer Indexer, documentID string) {
	if indexer == nil {
		return
	}
	_ = indexer.DeleteDocument(context.Background(), documentID)
}

func normalizeChunkContent(content string) string {
	var builder strings.Builder
	for _, r := range []rune(strings.ToLower(strings.TrimSpace(content))) {
		switch {
		case r == '\n', r == '\r', r == '\t':
			builder.WriteRune(' ')
		case strings.ContainsRune("[](){}<>:;,./!?@#$%^&*-_=+|\"'`~", r):
			builder.WriteRune(' ')
		default:
			builder.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func extractChunkTerms(normalized string) []string {
	if normalized == "" {
		return nil
	}

	parts := strings.Fields(normalized)
	seen := make(map[string]struct{}, len(parts))
	items := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, exists := seen[part]; !exists {
			seen[part] = struct{}{}
			items = append(items, part)
		}
		for _, expanded := range expandChunkToken(part) {
			if _, exists := seen[expanded]; exists {
				continue
			}
			seen[expanded] = struct{}{}
			items = append(items, expanded)
		}
	}
	return items
}

func extractChunkCharTerms(normalized string) []string {
	runes := []rune(strings.ReplaceAll(normalized, " ", ""))
	if len(runes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(runes)*2)
	items := make([]string, 0, len(runes)*2)
	for size := 2; size <= min(4, len(runes)); size++ {
		for start := 0; start+size <= len(runes); start++ {
			token := string(runes[start : start+size])
			if _, exists := seen[token]; exists {
				continue
			}
			seen[token] = struct{}{}
			items = append(items, token)
		}
	}
	return items
}

func expandChunkToken(token string) []string {
	runes := []rune(strings.TrimSpace(token))
	if len(runes) == 0 {
		return nil
	}

	items := make([]string, 0, len(runes)*2)
	for _, r := range runes {
		if r > 127 || (r >= '0' && r <= '9') {
			items = append(items, string(r))
		}
	}
	for size := 2; size <= min(4, len(runes)); size++ {
		for start := 0; start+size <= len(runes); start++ {
			items = append(items, string(runes[start:start+size]))
		}
	}
	return items
}
