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
		&models.User{},
		&models.UserRole{},
		&models.Session{},
		&models.Message{},
		&models.MessageReference{},
		&models.KnowledgeDocument{},
		&models.KnowledgeChunk{},
		&models.Alert{},
		&models.AlertHandlingRecord{},
		&models.AgentAction{},
		&models.ToolCallLog{},
		&models.AuditLog{},
	)
}
