package repository

import (
	"context"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type AuthRepo struct{ db *gorm.DB }

func NewAuthRepo(db *gorm.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (r *AuthRepo) CreateUserWithOrganization(ctx context.Context, input models.RegisterInput, passwordHash string) (*models.User, error) {
	var user *models.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		orgName := input.OrgName
		if orgName == "" {
			orgName = input.Email + "'s Organization"
		}
		org := &models.Organization{Name: orgName}
		if err := tx.Create(org).Error; err != nil {
			return fmt.Errorf("create organization: %w", err)
		}

		user = &models.User{
			Email:        input.Email,
			PasswordHash: passwordHash,
			OrgID:        org.ID,
			Role:         models.UserRoleAdmin,
		}
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	if err := r.db.WithContext(ctx).First(user, "email = ?", email).Error; err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *AuthRepo) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	user := &models.User{}
	if err := r.db.WithContext(ctx).First(user, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}
