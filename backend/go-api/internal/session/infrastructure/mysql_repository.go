package infrastructure

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	sessionApp "github.com/oyzg/OnCall/backend/go-api/internal/session/application"
	sessionDomain "github.com/oyzg/OnCall/backend/go-api/internal/session/domain"
	"gorm.io/gorm"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) CreateSession(ctx context.Context, session sessionDomain.Session) error {
	return r.db.WithContext(ctx).Create(toSessionModel(session)).Error
}

func (r *MySQLRepository) ListSessionsByUser(ctx context.Context, userID, query string, limit int) ([]sessionDomain.Session, error) {
	tx := r.db.WithContext(ctx).
		Model(&models.Session{}).
		Where("user_id = ?", userID).
		Order("updated_at DESC")
	if normalized := strings.TrimSpace(strings.ToLower(query)); normalized != "" {
		tx = tx.Where("LOWER(title) LIKE ?", "%"+normalized+"%")
	}
	if limit > 0 {
		tx = tx.Limit(limit)
	}

	var items []models.Session
	if err := tx.Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]sessionDomain.Session, 0, len(items))
	for _, item := range items {
		result = append(result, toSessionDomain(item))
	}
	return result, nil
}

func (r *MySQLRepository) DeleteSession(ctx context.Context, userID, sessionID string) (bool, error) {
	return deleteSession(ctx, r.db, userID, sessionID)
}

func deleteSession(ctx context.Context, db *gorm.DB, userID, sessionID string) (bool, error) {
	returnValue := false
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session models.Session
		if err := tx.Where("id = ? AND user_id = ?", sessionID, userID).Take(&session).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil
			}
			return err
		}

		if err := tx.Where("message_id IN (?)",
			tx.Model(&models.Message{}).Select("id").Where("session_id = ?", sessionID),
		).Delete(&models.MessageReference{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ?", sessionID).Delete(&models.Message{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&session).Error; err != nil {
			return err
		}
		returnValue = true
		return nil
	})
	return returnValue, err
}

func (r *MySQLRepository) ListMessages(ctx context.Context, userID, sessionID string, limit int, beforeID string) (sessionApp.MessagePage, error) {
	var page sessionApp.MessagePage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session models.Session
		if err := tx.Where("id = ? AND user_id = ?", sessionID, userID).Take(&session).Error; err != nil {
			return err
		}

		messageBase := tx.Model(&models.Message{}).Where("session_id = ?", sessionID)
		if beforeID != "" {
			var before models.Message
			if err := tx.Where("id = ? AND session_id = ?", beforeID, sessionID).Take(&before).Error; err == nil {
				messageBase = messageBase.Where("created_at < ? OR (created_at = ? AND id < ?)", before.CreatedAt, before.CreatedAt, before.ID)
			}
		}

		var total int64
		if err := tx.Model(&models.Message{}).Where("session_id = ?", sessionID).Count(&total).Error; err != nil {
			return err
		}

		var items []models.Message
		query := messageBase.Order("created_at DESC").Order("id DESC")
		if limit > 0 {
			query = query.Limit(limit + 1)
		}
		if err := query.Find(&items).Error; err != nil {
			return err
		}

		hasMore := false
		if limit > 0 && len(items) > limit {
			hasMore = true
			items = items[:limit]
		}

		for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
			items[left], items[right] = items[right], items[left]
		}

		messageIDs := make([]string, 0, len(items))
		for _, item := range items {
			messageIDs = append(messageIDs, item.ID)
		}
		referenceMap, err := loadReferences(tx, messageIDs)
		if err != nil {
			return err
		}

		page.Total = int(total)
		page.HasMore = hasMore
		page.Messages = make([]sessionDomain.Message, 0, len(items))
		for _, item := range items {
			msg := toMessageDomain(item)
			msg.References = referenceMap[item.ID]
			page.Messages = append(page.Messages, msg)
		}
		if hasMore && len(page.Messages) > 0 {
			page.NextCursor = page.Messages[0].ID
		}
		return nil
	})
	return page, err
}

func (r *MySQLRepository) UpsertSessionWithMessages(ctx context.Context, session sessionDomain.Session, messages []sessionDomain.Message) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(toSessionModel(session)).Error; err != nil {
			return err
		}

		if err := tx.Where("message_id IN (?)",
			tx.Model(&models.Message{}).Select("id").Where("session_id = ?", session.ID),
		).Delete(&models.MessageReference{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id = ?", session.ID).Delete(&models.Message{}).Error; err != nil {
			return err
		}

		for _, message := range messages {
			if err := tx.Create(toMessageModel(message)).Error; err != nil {
				return err
			}
			for _, reference := range message.References {
				if err := tx.Create(toReferenceModel(message.ID, reference)).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func loadReferences(tx *gorm.DB, messageIDs []string) (map[string][]sessionDomain.Reference, error) {
	result := make(map[string][]sessionDomain.Reference, len(messageIDs))
	if len(messageIDs) == 0 {
		return result, nil
	}

	var refs []models.MessageReference
	if err := tx.Where("message_id IN ?", messageIDs).Order("id ASC").Find(&refs).Error; err != nil {
		return nil, err
	}
	for _, ref := range refs {
		result[ref.MessageID] = append(result[ref.MessageID], sessionDomain.Reference{
			DocumentID:    ref.DocumentID,
			DocumentTitle: ref.DocumentTitle,
			Category:      ref.Category,
			Excerpt:       ref.Excerpt,
			Score:         ref.Score,
		})
	}
	return result, nil
}

func toSessionModel(session sessionDomain.Session) *models.Session {
	return &models.Session{
		ID:                 session.ID,
		UserID:             session.UserID,
		Title:              session.Title,
		LastMessagePreview: session.LastMessagePreview,
		MessageCount:       session.MessageCount,
		LastMessageAt:      session.LastMessageAt,
		CreatedAt:          session.CreatedAt,
		UpdatedAt:          session.UpdatedAt,
	}
}

func toSessionDomain(model models.Session) sessionDomain.Session {
	return sessionDomain.Session{
		ID:                 model.ID,
		UserID:             model.UserID,
		Title:              model.Title,
		LastMessagePreview: model.LastMessagePreview,
		MessageCount:       model.MessageCount,
		LastMessageAt:      model.LastMessageAt,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func toMessageModel(message sessionDomain.Message) *models.Message {
	return &models.Message{
		ID:        message.ID,
		SessionID: message.SessionID,
		Role:      message.Role,
		Content:   message.Content,
		Status:    message.Status,
		CreatedAt: message.CreatedAt,
	}
}

func toMessageDomain(model models.Message) sessionDomain.Message {
	return sessionDomain.Message{
		ID:        model.ID,
		SessionID: model.SessionID,
		Role:      model.Role,
		Content:   model.Content,
		Status:    model.Status,
		CreatedAt: model.CreatedAt,
	}
}

func toReferenceModel(messageID string, reference sessionDomain.Reference) *models.MessageReference {
	return &models.MessageReference{
		MessageID:     messageID,
		DocumentID:    reference.DocumentID,
		DocumentTitle: reference.DocumentTitle,
		Category:      reference.Category,
		Excerpt:       reference.Excerpt,
		Score:         reference.Score,
		CreatedAt:     time.Now(),
	}
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
