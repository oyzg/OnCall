package infrastructure

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	auditDomain "github.com/oyzg/OnCall/backend/go-api/internal/audit/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	"gorm.io/gorm"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) AppendLog(ctx context.Context, log auditDomain.Log) error {
	return r.db.WithContext(ctx).Create(toAuditLogModel(log)).Error
}

func (r *MySQLRepository) ListLogs(ctx context.Context, category, action, status, actor string, limit int) ([]auditDomain.Log, error) {
	tx := r.db.WithContext(ctx).Model(&models.AuditLog{}).Order("created_at DESC")
	if category != "" {
		tx = tx.Where("category = ?", category)
	}
	if action != "" {
		tx = tx.Where("action = ?", action)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if normalized := strings.TrimSpace(strings.ToLower(actor)); normalized != "" {
		tx = tx.Where("LOWER(actor_name) LIKE ? OR LOWER(actor_id) LIKE ?", "%"+normalized+"%", "%"+normalized+"%")
	}
	if limit <= 0 {
		limit = 50
	}
	tx = tx.Limit(limit)

	var items []models.AuditLog
	if err := tx.Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]auditDomain.Log, 0, len(items))
	for _, item := range items {
		result = append(result, toAuditLogDomain(item))
	}
	return result, nil
}

func (r *MySQLRepository) BuildStats(ctx context.Context) (auditDomain.Stats, error) {
	var items []models.AuditLog
	if err := r.db.WithContext(ctx).Find(&items).Error; err != nil {
		return auditDomain.Stats{}, err
	}

	stats := auditDomain.Stats{ByCategory: make(map[string]int)}
	uniqueActors := make(map[string]struct{})
	deadline := time.Now().Add(-24 * time.Hour)
	for _, item := range items {
		stats.Total++
		stats.ByCategory[item.Category]++
		if item.Status == "success" {
			stats.Success++
		} else {
			stats.Failed++
		}
		if !item.CreatedAt.Before(deadline) {
			stats.Last24Hours++
		}
		if item.ActorID != "" {
			uniqueActors[item.ActorID] = struct{}{}
		}
	}
	stats.UniqueActors = len(uniqueActors)
	return stats, nil
}

func (r *MySQLRepository) Categories(ctx context.Context) ([]string, error) {
	var categories []string
	if err := r.db.WithContext(ctx).Model(&models.AuditLog{}).Distinct("category").Pluck("category", &categories).Error; err != nil {
		return nil, err
	}
	sort.Strings(categories)
	return categories, nil
}

func toAuditLogModel(log auditDomain.Log) *models.AuditLog {
	return &models.AuditLog{
		ID:             log.ID,
		Category:       log.Category,
		Action:         log.Action,
		Status:         log.Status,
		ActorID:        log.ActorID,
		ActorName:      log.ActorName,
		ActorRolesJSON: mustJSON(log.ActorRoles),
		TargetType:     log.TargetType,
		TargetID:       log.TargetID,
		TargetName:     log.TargetName,
		Detail:         log.Detail,
		MetadataJSON:   mustJSON(log.Metadata),
		CreatedAt:      log.CreatedAt,
	}
}

func toAuditLogDomain(model models.AuditLog) auditDomain.Log {
	var roles []string
	var metadata map[string]any
	_ = json.Unmarshal([]byte(model.ActorRolesJSON), &roles)
	if model.MetadataJSON != "" && model.MetadataJSON != "null" {
		_ = json.Unmarshal([]byte(model.MetadataJSON), &metadata)
	}
	return auditDomain.Log{
		ID:         model.ID,
		Category:   model.Category,
		Action:     model.Action,
		Status:     model.Status,
		ActorID:    model.ActorID,
		ActorName:  model.ActorName,
		ActorRoles: roles,
		TargetType: model.TargetType,
		TargetID:   model.TargetID,
		TargetName: model.TargetName,
		Detail:     model.Detail,
		Metadata:   metadata,
		CreatedAt:  model.CreatedAt,
	}
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
