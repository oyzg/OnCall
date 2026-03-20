package models

import "time"

type Session struct {
	ID                 string    `gorm:"primaryKey;size:64"`
	UserID             string    `gorm:"index:idx_sessions_user_last_message,priority:1;size:64;not null"`
	Title              string    `gorm:"size:255;not null"`
	LastMessagePreview string    `gorm:"size:512"`
	MessageCount       int       `gorm:"not null;default:0"`
	LastMessageAt      time.Time `gorm:"index:idx_sessions_user_last_message,priority:2"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

type Message struct {
	ID        string    `gorm:"primaryKey;size:64"`
	SessionID string    `gorm:"index:idx_messages_session_created,priority:1;size:64;not null"`
	Role      string    `gorm:"size:16;not null"`
	Content   string    `gorm:"type:longtext;not null"`
	Status    string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"index:idx_messages_session_created,priority:2;not null"`
}

type MessageReference struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	MessageID     string    `gorm:"index;size:64;not null"`
	DocumentID    string    `gorm:"index;size:64;not null"`
	DocumentTitle string    `gorm:"size:255;not null"`
	Category      string    `gorm:"size:64;not null"`
	Excerpt       string    `gorm:"type:text;not null"`
	Score         float64   `gorm:"not null"`
	CreatedAt     time.Time `gorm:"not null"`
}

func (Session) TableName() string          { return "sessions" }
func (Message) TableName() string          { return "messages" }
func (MessageReference) TableName() string { return "message_references" }
