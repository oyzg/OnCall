package models

import "time"

type User struct {
	ID           string    `gorm:"primaryKey;size:64"`
	Username     string    `gorm:"uniqueIndex;size:64;not null"`
	DisplayName  string    `gorm:"size:128;not null"`
	Status       string    `gorm:"size:16;not null;default:active"`
	PasswordHash string    `gorm:"type:text;not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

type UserRole struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    string    `gorm:"uniqueIndex:idx_user_role,priority:1;index;size:64;not null"`
	Role      string    `gorm:"uniqueIndex:idx_user_role,priority:2;size:64;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (User) TableName() string     { return "users" }
func (UserRole) TableName() string { return "user_roles" }
