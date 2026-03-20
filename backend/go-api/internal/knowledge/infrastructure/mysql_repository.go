package infrastructure

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	knowledgeApp "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/application"
	knowledgeDomain "github.com/oyzg/OnCall/backend/go-api/internal/knowledge/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	"gorm.io/gorm"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) SaveDocument(ctx context.Context, document knowledgeDomain.Document) error {
	return r.db.WithContext(ctx).Save(toDocumentModel(document)).Error
}

func (r *MySQLRepository) ListDocuments(ctx context.Context, userID, status, category, query string, limit int) ([]knowledgeDomain.Document, error) {
	tx := r.db.WithContext(ctx).Model(&models.KnowledgeDocument{}).Where("user_id = ?", userID).Order("updated_at DESC")
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if category != "" {
		tx = tx.Where("category = ?", category)
	}
	if normalized := strings.TrimSpace(strings.ToLower(query)); normalized != "" {
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(file_name) LIKE ?", "%"+normalized+"%", "%"+normalized+"%")
	}
	if limit > 0 {
		tx = tx.Limit(limit)
	}

	var items []models.KnowledgeDocument
	if err := tx.Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]knowledgeDomain.Document, 0, len(items))
	for _, item := range items {
		result = append(result, toDocumentDomain(item))
	}
	return result, nil
}

func (r *MySQLRepository) GetDocument(ctx context.Context, userID, documentID string) (knowledgeDomain.Document, bool, error) {
	var item models.KnowledgeDocument
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", documentID, userID).Take(&item).Error
	if err == gorm.ErrRecordNotFound {
		return knowledgeDomain.Document{}, false, nil
	}
	if err != nil {
		return knowledgeDomain.Document{}, false, err
	}
	return toDocumentDomain(item), true, nil
}

func (r *MySQLRepository) GetDocumentByID(ctx context.Context, documentID string) (knowledgeDomain.Document, bool, error) {
	var item models.KnowledgeDocument
	err := r.db.WithContext(ctx).Where("id = ?", documentID).Take(&item).Error
	if err == gorm.ErrRecordNotFound {
		return knowledgeDomain.Document{}, false, nil
	}
	if err != nil {
		return knowledgeDomain.Document{}, false, err
	}
	return toDocumentDomain(item), true, nil
}

func (r *MySQLRepository) DeleteDocument(ctx context.Context, userID, documentID string) (knowledgeDomain.Document, bool, error) {
	var document knowledgeDomain.Document
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.KnowledgeDocument
		if err := tx.Where("id = ? AND user_id = ?", documentID, userID).Take(&item).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}
		document = toDocumentDomain(item)
		found = true
		if err := tx.Where("document_id = ?", documentID).Delete(&models.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		return tx.Delete(&item).Error
	})
	return document, found, err
}

func (r *MySQLRepository) ReplaceChunks(ctx context.Context, documentID string, chunks []knowledgeApp.Chunk) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", documentID).Delete(&models.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		for _, chunk := range chunks {
			if err := tx.Create(toChunkModel(chunk)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *MySQLRepository) ListChunksByDocument(ctx context.Context, documentID string) ([]knowledgeApp.Chunk, error) {
	var items []models.KnowledgeChunk
	if err := r.db.WithContext(ctx).Where("document_id = ?", documentID).Order("chunk_index ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]knowledgeApp.Chunk, 0, len(items))
	for _, item := range items {
		result = append(result, toChunkDomain(item))
	}
	return result, nil
}

func (r *MySQLRepository) ListReadyChunks(ctx context.Context, userID, category string) ([]knowledgeApp.Chunk, error) {
	tx := r.db.WithContext(ctx).
		Table("knowledge_chunks").
		Select("knowledge_chunks.document_id, knowledge_chunks.document_title, knowledge_chunks.category, knowledge_chunks.chunk_index, knowledge_chunks.content, knowledge_chunks.normalized, knowledge_chunks.terms_json, knowledge_chunks.char_terms_json").
		Joins("JOIN knowledge_documents ON knowledge_documents.id = knowledge_chunks.document_id").
		Where("knowledge_documents.user_id = ? AND knowledge_documents.status = ?", userID, "ready").
		Order("knowledge_documents.updated_at DESC").
		Order("knowledge_chunks.chunk_index ASC")
	if category != "" {
		tx = tx.Where("knowledge_documents.category = ?", category)
	}

	var items []models.KnowledgeChunk
	if err := tx.Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]knowledgeApp.Chunk, 0, len(items))
	for _, item := range items {
		result = append(result, toChunkDomain(item))
	}
	return result, nil
}

func toDocumentModel(document knowledgeDomain.Document) *models.KnowledgeDocument {
	return &models.KnowledgeDocument{
		ID:                document.ID,
		UserID:            document.UserID,
		Title:             document.Title,
		Category:          document.Category,
		SourceType:        document.SourceType,
		FileName:          document.FileName,
		ContentType:       document.ContentType,
		StoragePath:       document.StoragePath,
		SizeBytes:         document.SizeBytes,
		Status:            document.Status,
		Summary:           document.Summary,
		TextPreview:       document.TextPreview,
		ChunkPreviewsJSON: mustJSON(document.ChunkPreviews),
		FailureReason:     document.FailureReason,
		ChunkCount:        document.ChunkCount,
		IndexStatus:       document.IndexStatus,
		EmbeddingBackend:  document.EmbeddingBackend,
		VectorBackend:     document.VectorBackend,
		LexicalBackend:    document.LexicalBackend,
		IndexError:        document.IndexError,
		ProcessedAt:       document.ProcessedAt,
		IndexedAt:         document.IndexedAt,
		CreatedAt:         document.CreatedAt,
		UpdatedAt:         document.UpdatedAt,
	}
}

func toDocumentDomain(model models.KnowledgeDocument) knowledgeDomain.Document {
	var previews []string
	_ = json.Unmarshal([]byte(model.ChunkPreviewsJSON), &previews)
	return knowledgeDomain.Document{
		ID:               model.ID,
		UserID:           model.UserID,
		Title:            model.Title,
		Category:         model.Category,
		SourceType:       model.SourceType,
		FileName:         model.FileName,
		ContentType:      model.ContentType,
		StoragePath:      model.StoragePath,
		SizeBytes:        model.SizeBytes,
		Status:           model.Status,
		Summary:          model.Summary,
		TextPreview:      model.TextPreview,
		ChunkPreviews:    previews,
		FailureReason:    model.FailureReason,
		ChunkCount:       model.ChunkCount,
		IndexStatus:      model.IndexStatus,
		EmbeddingBackend: model.EmbeddingBackend,
		VectorBackend:    model.VectorBackend,
		LexicalBackend:   model.LexicalBackend,
		IndexError:       model.IndexError,
		ProcessedAt:      model.ProcessedAt,
		IndexedAt:        model.IndexedAt,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func toChunkModel(chunk knowledgeApp.Chunk) *models.KnowledgeChunk {
	return &models.KnowledgeChunk{
		DocumentID:    chunk.DocumentID,
		DocumentTitle: chunk.DocumentTitle,
		Category:      chunk.Category,
		ChunkIndex:    chunk.Index,
		Content:       chunk.Content,
		Normalized:    chunk.Normalized,
		TermsJSON:     mustJSON(chunk.Terms),
		CharTermsJSON: mustJSON(chunk.CharTerms),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func toChunkDomain(model models.KnowledgeChunk) knowledgeApp.Chunk {
	var terms []string
	var charTerms []string
	_ = json.Unmarshal([]byte(model.TermsJSON), &terms)
	_ = json.Unmarshal([]byte(model.CharTermsJSON), &charTerms)
	return knowledgeApp.Chunk{
		DocumentID:    model.DocumentID,
		DocumentTitle: model.DocumentTitle,
		Category:      model.Category,
		Index:         model.ChunkIndex,
		Content:       model.Content,
		Normalized:    model.Normalized,
		Terms:         terms,
		CharTerms:     charTerms,
	}
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
