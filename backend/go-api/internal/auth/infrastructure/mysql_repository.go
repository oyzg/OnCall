package infrastructure

import (
	"context"
	"strings"
	"time"

	authApp "github.com/oyzg/OnCall/backend/go-api/internal/auth/application"
	authDomain "github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"github.com/oyzg/OnCall/backend/go-api/internal/platform/db/models"
	"gorm.io/gorm"
)

type StoredUser = authApp.StoredUser

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) SaveUser(ctx context.Context, record StoredUser) error {
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()
	userModel := models.User{
		ID:           record.User.ID,
		Username:     record.User.Username,
		DisplayName:  record.User.DisplayName,
		Status:       defaultStatus(record.Status),
		PasswordHash: record.PasswordHash,
		UpdatedAt:    now,
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.User
		err := tx.Where("id = ? OR username = ?", record.User.ID, record.User.Username).First(&existing).Error
		switch {
		case err == nil:
			userModel.CreatedAt = existing.CreatedAt
			if err := tx.Model(&existing).Updates(userModel).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", existing.ID).Delete(&models.UserRole{}).Error; err != nil {
				return err
			}
			return insertRoles(tx, existing.ID, record.User.Roles, now)
		case err == gorm.ErrRecordNotFound:
			userModel.CreatedAt = now
			if err := tx.Create(&userModel).Error; err != nil {
				return err
			}
			return insertRoles(tx, userModel.ID, record.User.Roles, now)
		default:
			return err
		}
	})
}

func (r *MySQLRepository) FindByUsername(ctx context.Context, username string) (StoredUser, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var userModel models.User
	err := r.db.WithContext(ctx).Where("username = ?", strings.TrimSpace(username)).First(&userModel).Error
	if err == gorm.ErrRecordNotFound {
		return StoredUser{}, false, nil
	}
	if err != nil {
		return StoredUser{}, false, err
	}

	var roleModels []models.UserRole
	if err := r.db.WithContext(ctx).Where("user_id = ?", userModel.ID).Order("role ASC").Find(&roleModels).Error; err != nil {
		return StoredUser{}, false, err
	}

	roles := make([]string, 0, len(roleModels))
	for _, role := range roleModels {
		roles = append(roles, role.Role)
	}

	return StoredUser{
		User: authDomain.User{
			ID:          userModel.ID,
			Username:    userModel.Username,
			DisplayName: userModel.DisplayName,
			Roles:       roles,
		},
		Status:       userModel.Status,
		PasswordHash: userModel.PasswordHash,
	}, true, nil
}

func insertRoles(tx *gorm.DB, userID string, roles []string, now time.Time) error {
	if len(roles) == 0 {
		return nil
	}

	items := make([]models.UserRole, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		items = append(items, models.UserRole{
			UserID:    userID,
			Role:      role,
			CreatedAt: now,
		})
	}
	if len(items) == 0 {
		return nil
	}

	return tx.Create(&items).Error
}

func defaultStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "active"
	}
	return status
}
