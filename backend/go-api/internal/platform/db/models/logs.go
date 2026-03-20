package models

import "time"

type ToolCallLog struct {
	ID         string    `gorm:"primaryKey;size:64"`
	ToolName   string    `gorm:"index:idx_tool_logs_tool_created,priority:1;size:64;not null"`
	Operator   string    `gorm:"size:128;not null"`
	UserID     string    `gorm:"size:64;index;not null"`
	Status     string    `gorm:"index:idx_tool_logs_status_created,priority:1;size:32;not null"`
	InputJSON  string    `gorm:"type:longtext;not null"`
	OutputJSON string    `gorm:"type:longtext"`
	Error      string    `gorm:"type:text"`
	DurationMS int64     `gorm:"not null"`
	CreatedAt  time.Time `gorm:"index:idx_tool_logs_tool_created,priority:2;index:idx_tool_logs_status_created,priority:2;not null"`
}

type AuditLog struct {
	ID             string    `gorm:"primaryKey;size:64"`
	Category       string    `gorm:"index:idx_audit_category_created,priority:1;size:64;not null"`
	Action         string    `gorm:"index;size:64;not null"`
	Status         string    `gorm:"index;size:32;not null"`
	ActorID        string    `gorm:"index;size:64"`
	ActorName      string    `gorm:"size:128"`
	ActorRolesJSON string    `gorm:"type:longtext;not null"`
	TargetType     string    `gorm:"size:64"`
	TargetID       string    `gorm:"size:64"`
	TargetName     string    `gorm:"size:255"`
	Detail         string    `gorm:"type:text"`
	MetadataJSON   string    `gorm:"type:longtext"`
	CreatedAt      time.Time `gorm:"index:idx_audit_category_created,priority:2;not null"`
}

func (ToolCallLog) TableName() string { return "tool_call_logs" }
func (AuditLog) TableName() string    { return "audit_logs" }
