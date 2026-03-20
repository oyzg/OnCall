package infrastructure

import (
	"context"
	"encoding/json"

	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	toolDomain "github.com/oyzg/OnCall/backend/go-api/internal/tool/domain"
	"gorm.io/gorm"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) AppendLog(ctx context.Context, entry toolDomain.CallLog) error {
	return r.db.WithContext(ctx).Create(toToolLogModel(entry)).Error
}

func (r *MySQLRepository) ListLogs(ctx context.Context, toolName, status string, limit int) ([]toolDomain.CallLog, error) {
	tx := r.db.WithContext(ctx).Model(&models.ToolCallLog{}).Order("created_at DESC")
	if toolName != "" {
		tx = tx.Where("tool_name = ?", toolName)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if limit <= 0 {
		limit = 20
	}
	tx = tx.Limit(limit)

	var items []models.ToolCallLog
	if err := tx.Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]toolDomain.CallLog, 0, len(items))
	for _, item := range items {
		result = append(result, toToolLogDomain(item))
	}
	return result, nil
}

func toToolLogModel(entry toolDomain.CallLog) *models.ToolCallLog {
	return &models.ToolCallLog{
		ID:         entry.ID,
		ToolName:   entry.ToolName,
		Operator:   entry.Operator,
		UserID:     entry.UserID,
		Status:     entry.Status,
		InputJSON:  mustJSON(entry.Input),
		OutputJSON: mustJSON(entry.Output),
		Error:      entry.Error,
		DurationMS: entry.DurationMS,
		CreatedAt:  entry.CreatedAt,
	}
}

func toToolLogDomain(model models.ToolCallLog) toolDomain.CallLog {
	var input map[string]any
	var output any
	_ = json.Unmarshal([]byte(model.InputJSON), &input)
	if model.OutputJSON != "" && model.OutputJSON != "null" {
		_ = json.Unmarshal([]byte(model.OutputJSON), &output)
	}
	return toolDomain.CallLog{
		ID:         model.ID,
		ToolName:   model.ToolName,
		Operator:   model.Operator,
		UserID:     model.UserID,
		Status:     model.Status,
		Input:      input,
		Output:     output,
		Error:      model.Error,
		DurationMS: model.DurationMS,
		CreatedAt:  model.CreatedAt,
	}
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
