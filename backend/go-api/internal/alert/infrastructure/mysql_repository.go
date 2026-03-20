package infrastructure

import (
	"context"
	"encoding/json"
	"strings"

	alertDomain "github.com/oyzg/OnCall/backend/go-api/internal/alert/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	"gorm.io/gorm"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) SaveAlert(ctx context.Context, alert alertDomain.Alert) error {
	return r.db.WithContext(ctx).Save(toAlertModel(alert)).Error
}

func (r *MySQLRepository) SaveAlertWithRecord(ctx context.Context, alert alertDomain.Alert, record alertDomain.HandlingRecord) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(toAlertModel(alert)).Error; err != nil {
			return err
		}
		return tx.Create(toRecordModel(record)).Error
	})
}

func (r *MySQLRepository) ListAlerts(ctx context.Context, status, severity, service, query string) ([]alertDomain.Alert, error) {
	tx := r.db.WithContext(ctx).Model(&models.Alert{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if severity != "" {
		tx = tx.Where("severity = ?", severity)
	}
	if service != "" {
		tx = tx.Where("LOWER(service) = ?", strings.ToLower(service))
	}
	if normalized := strings.TrimSpace(strings.ToLower(query)); normalized != "" {
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(summary) LIKE ? OR LOWER(description) LIKE ?", "%"+normalized+"%", "%"+normalized+"%", "%"+normalized+"%")
	}
	tx = tx.Order("triggered_at DESC").Order("updated_at DESC")

	var items []models.Alert
	if err := tx.Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]alertDomain.Alert, 0, len(items))
	for _, item := range items {
		result = append(result, toAlertDomain(item))
	}
	return result, nil
}

func (r *MySQLRepository) GetAlert(ctx context.Context, alertID string) (alertDomain.Alert, bool, error) {
	var item models.Alert
	err := r.db.WithContext(ctx).Where("id = ?", alertID).Take(&item).Error
	if err == gorm.ErrRecordNotFound {
		return alertDomain.Alert{}, false, nil
	}
	if err != nil {
		return alertDomain.Alert{}, false, err
	}
	return toAlertDomain(item), true, nil
}

func (r *MySQLRepository) ListRecords(ctx context.Context, alertID string) ([]alertDomain.HandlingRecord, error) {
	var items []models.AlertHandlingRecord
	if err := r.db.WithContext(ctx).Where("alert_id = ?", alertID).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]alertDomain.HandlingRecord, 0, len(items))
	for _, item := range items {
		result = append(result, alertDomain.HandlingRecord{
			ID:        item.ID,
			AlertID:   item.AlertID,
			Action:    item.Action,
			Operator:  item.Operator,
			Comment:   item.Comment,
			CreatedAt: item.CreatedAt,
		})
	}
	return result, nil
}

func (r *MySQLRepository) FindDuplicateOpenAlert(ctx context.Context, candidate alertDomain.Alert) (alertDomain.Alert, bool, error) {
	var item models.Alert
	err := r.db.WithContext(ctx).
		Where("service = ? AND environment = ? AND source = ? AND title = ? AND status <> ?", candidate.Service, candidate.Environment, candidate.Source, candidate.Title, "resolved").
		Order("updated_at DESC").
		Take(&item).Error
	if err == gorm.ErrRecordNotFound {
		return alertDomain.Alert{}, false, nil
	}
	if err != nil {
		return alertDomain.Alert{}, false, err
	}
	return toAlertDomain(item), true, nil
}

func (r *MySQLRepository) AppendRecord(ctx context.Context, record alertDomain.HandlingRecord) error {
	return r.db.WithContext(ctx).Create(toRecordModel(record)).Error
}

func toAlertModel(alert alertDomain.Alert) *models.Alert {
	return &models.Alert{
		ID:              alert.ID,
		Title:           alert.Title,
		Service:         alert.Service,
		Environment:     alert.Environment,
		Severity:        alert.Severity,
		Source:          alert.Source,
		Status:          alert.Status,
		Summary:         alert.Summary,
		Description:     alert.Description,
		LabelsJSON:      mustJSON(alert.Labels),
		LinkedSessionID: alert.LinkedSessionID,
		AnalysisJSON:    mustJSON(alert.Analysis),
		OccurrenceCount: alert.OccurrenceCount,
		TriggeredAt:     alert.TriggeredAt,
		LastTriggeredAt: alert.LastTriggeredAt,
		CreatedAt:       alert.CreatedAt,
		UpdatedAt:       alert.UpdatedAt,
	}
}

func toAlertDomain(model models.Alert) alertDomain.Alert {
	var labels map[string]string
	var analysis *alertDomain.AlertAnalysis
	_ = json.Unmarshal([]byte(model.LabelsJSON), &labels)
	if strings.TrimSpace(model.AnalysisJSON) != "" && strings.TrimSpace(model.AnalysisJSON) != "null" {
		analysis = &alertDomain.AlertAnalysis{}
		_ = json.Unmarshal([]byte(model.AnalysisJSON), analysis)
	}
	return alertDomain.Alert{
		ID:              model.ID,
		Title:           model.Title,
		Service:         model.Service,
		Environment:     model.Environment,
		Severity:        model.Severity,
		Source:          model.Source,
		Status:          model.Status,
		Summary:         model.Summary,
		Description:     model.Description,
		Labels:          labels,
		LinkedSessionID: model.LinkedSessionID,
		Analysis:        analysis,
		OccurrenceCount: model.OccurrenceCount,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
		TriggeredAt:     model.TriggeredAt,
		LastTriggeredAt: model.LastTriggeredAt,
	}
}

func toRecordModel(record alertDomain.HandlingRecord) *models.AlertHandlingRecord {
	return &models.AlertHandlingRecord{
		ID:        record.ID,
		AlertID:   record.AlertID,
		Action:    record.Action,
		Operator:  record.Operator,
		Comment:   record.Comment,
		CreatedAt: record.CreatedAt,
	}
}

func mustJSON(value any) string {
	payload, _ := json.Marshal(value)
	return string(payload)
}
