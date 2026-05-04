package infrastructure

import (
	"context"

	agentDomain "github.com/oyzg/OnCall/backend/go-api/internal/agentaction/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	"gorm.io/gorm"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Save(ctx context.Context, action agentDomain.Action) error {
	return r.db.WithContext(ctx).Save(toModel(action)).Error
}

func (r *MySQLRepository) Get(ctx context.Context, id string) (agentDomain.Action, bool, error) {
	var model models.AgentAction
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
	if err == nil {
		return toDomain(model), true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return agentDomain.Action{}, false, nil
	}
	return agentDomain.Action{}, false, err
}

func (r *MySQLRepository) ListBySource(ctx context.Context, sourceType, sourceID string) ([]agentDomain.Action, error) {
	var items []models.AgentAction
	if err := r.db.WithContext(ctx).
		Where("source_type = ? AND source_id = ?", sourceType, sourceID).
		Order("created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]agentDomain.Action, 0, len(items))
	for _, item := range items {
		result = append(result, toDomain(item))
	}
	return result, nil
}

func toModel(action agentDomain.Action) *models.AgentAction {
	return &models.AgentAction{
		ID:            action.ID,
		SourceType:    action.SourceType,
		SourceID:      action.SourceID,
		ActionType:    action.ActionType,
		Status:        action.Status,
		Title:         action.Title,
		Description:   action.Description,
		ArgumentsJSON: action.ArgumentsJSON,
		RiskLevel:     action.RiskLevel,
		ResultJSON:    action.ResultJSON,
		Error:         action.Error,
		CreatedAt:     action.CreatedAt,
		ExecutedAt:    action.ExecutedAt,
	}
}

func toDomain(model models.AgentAction) agentDomain.Action {
	return agentDomain.Action{
		ID:            model.ID,
		SourceType:    model.SourceType,
		SourceID:      model.SourceID,
		ActionType:    model.ActionType,
		Status:        model.Status,
		Title:         model.Title,
		Description:   model.Description,
		ArgumentsJSON: model.ArgumentsJSON,
		RiskLevel:     model.RiskLevel,
		ResultJSON:    model.ResultJSON,
		Error:         model.Error,
		CreatedAt:     model.CreatedAt,
		ExecutedAt:    model.ExecutedAt,
	}
}
