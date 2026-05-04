package models

import "time"

type Alert struct {
	ID              string    `gorm:"primaryKey;size:64"`
	Title           string    `gorm:"size:255;not null"`
	Service         string    `gorm:"index:idx_alert_duplicate_lookup,priority:1;index;size:128;not null"`
	Environment     string    `gorm:"index:idx_alert_duplicate_lookup,priority:2;size:64;not null"`
	Severity        string    `gorm:"index;size:8;not null"`
	Source          string    `gorm:"index:idx_alert_duplicate_lookup,priority:3;size:64;not null"`
	Status          string    `gorm:"index:idx_alert_duplicate_lookup,priority:5;index;size:32;not null"`
	Summary         string    `gorm:"type:text;not null"`
	Description     string    `gorm:"type:text;not null"`
	LabelsJSON      string    `gorm:"type:longtext;not null"`
	LinkedSessionID string    `gorm:"size:64"`
	AnalysisJSON    string    `gorm:"type:longtext"`
	OccurrenceCount int       `gorm:"not null;default:1"`
	TriggeredAt     time.Time `gorm:"index;not null"`
	LastTriggeredAt time.Time `gorm:"index;not null"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

type AlertHandlingRecord struct {
	ID        string    `gorm:"primaryKey;size:64"`
	AlertID   string    `gorm:"index:idx_alert_record_created,priority:1;size:64;not null"`
	Action    string    `gorm:"size:64;not null"`
	Operator  string    `gorm:"size:128;not null"`
	Comment   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"index:idx_alert_record_created,priority:2;not null"`
}

func (Alert) TableName() string               { return "alerts" }
func (AlertHandlingRecord) TableName() string { return "alert_handling_records" }

type AgentAction struct {
	ID            string     `gorm:"primaryKey;size:128"`
	SourceType    string     `gorm:"index:idx_agent_action_source,priority:1;size:32;not null"`
	SourceID      string     `gorm:"index:idx_agent_action_source,priority:2;size:128;not null"`
	ActionType    string     `gorm:"index;size:64;not null"`
	Status        string     `gorm:"index;size:32;not null"`
	Title         string     `gorm:"size:255;not null"`
	Description   string     `gorm:"type:text"`
	ArgumentsJSON string     `gorm:"type:longtext;not null"`
	RiskLevel     string     `gorm:"size:32;not null"`
	ResultJSON    string     `gorm:"type:longtext"`
	Error         string     `gorm:"type:text"`
	CreatedAt     time.Time  `gorm:"not null"`
	ExecutedAt    *time.Time `gorm:"index"`
}

func (AgentAction) TableName() string { return "agent_actions" }
