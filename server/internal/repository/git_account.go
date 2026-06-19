package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"gorm.io/gorm"
)

type GitAccountRepo struct {
	db *gorm.DB
}

func NewGitAccountRepo(db *gorm.DB) *GitAccountRepo {
	return &GitAccountRepo{db: db}
}

func (r *GitAccountRepo) Create(ctx context.Context, orgID string, input models.CreateGitAccountInput) (*models.GitAccount, error) {
	acc := &models.GitAccount{
		OrgID:       orgID,
		Provider:    input.Provider,
		DisplayName: input.DisplayName,
		BaseURL:     input.BaseURL,
		Token:       input.Token,
	}
	if err := r.db.WithContext(ctx).Create(acc).Error; err != nil {
		return nil, fmt.Errorf("create git account: %w", err)
	}
	return acc, nil
}

func (r *GitAccountRepo) GetByID(ctx context.Context, id string) (*models.GitAccount, error) {
	acc := &models.GitAccount{}
	if err := r.db.WithContext(ctx).First(acc, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("get git account: %w", mapError(err))
	}
	return acc, nil
}

func (r *GitAccountRepo) ListByOrgID(ctx context.Context, orgID string) ([]models.GitAccount, error) {
	var accounts []models.GitAccount
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("list git accounts: %w", err)
	}
	return accounts, nil
}

func (r *GitAccountRepo) GetDefaultByProjectAndProvider(ctx context.Context, projectID string, provider string) (*models.GitAccount, error) {
	acc := &models.GitAccount{}
	query := `
		SELECT ga.* FROM git_accounts ga
		JOIN projects p ON p.org_id = ga.org_id
		WHERE p.id = ? AND ga.provider = ?
		ORDER BY ga.created_at ASC
		LIMIT 1
	`
	if err := r.db.WithContext(ctx).Raw(query, projectID, provider).Scan(acc).Error; err != nil {
		return nil, fmt.Errorf("get default git account: %w", mapError(err))
	}
	if acc.ID == "" {
		return nil, ErrNotFound
	}
	return acc, nil
}

func (r *GitAccountRepo) Update(ctx context.Context, id string, input models.UpdateGitAccountInput) (*models.GitAccount, error) {
	acc, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if input.DisplayName != nil {
		updates["display_name"] = *input.DisplayName
	}
	if input.BaseURL != nil {
		updates["base_url"] = *input.BaseURL
	}
	if input.Token != nil {
		updates["token"] = *input.Token
	}
	if err := r.db.WithContext(ctx).Model(acc).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update git account: %w", err)
	}
	return acc, nil
}

func (r *GitAccountRepo) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&models.GitAccount{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete git account: %w", mapError(result.Error))
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete git account: %w", ErrNotFound)
	}
	return nil
}
