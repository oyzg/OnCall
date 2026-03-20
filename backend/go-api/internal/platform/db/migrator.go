package db

import (
	"gorm.io/gorm"

	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
)

type Migrator struct {
	Path string
}

func NewMigrator(path string) Migrator {
	return Migrator{Path: path}
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Session{},
		&models.Message{},
		&models.MessageReference{},
		&models.KnowledgeDocument{},
		&models.KnowledgeChunk{},
		&models.Alert{},
		&models.AlertHandlingRecord{},
		&models.ToolCallLog{},
		&models.AuditLog{},
	)
}
