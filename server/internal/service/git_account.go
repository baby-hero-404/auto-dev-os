package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GitAccountService struct {
	repo        *repository.GitAccountRepo
	gitProvider gitops.GitProvider
}

func NewGitAccountService(repo *repository.GitAccountRepo) *GitAccountService {
	return &GitAccountService{
		repo:        repo,
		gitProvider: gitops.NewGitHubProvider(""), // For now GitHub, can expand later
	}
}

func (s *GitAccountService) Create(ctx context.Context, orgID string, input models.CreateGitAccountInput) (*models.GitAccount, error) {
	if input.Provider == "" {
		return nil, ErrValidation("provider is required")
	}
	if input.DisplayName == "" {
		return nil, ErrValidation("display_name is required")
	}
	if input.Token == "" {
		return nil, ErrValidation("token is required")
	}
	return s.repo.Create(ctx, orgID, input)
}

func (s *GitAccountService) GetByID(ctx context.Context, id string) (*models.GitAccount, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *GitAccountService) ListByOrgID(ctx context.Context, orgID string) ([]models.GitAccount, error) {
	return s.repo.ListByOrgID(ctx, orgID)
}

func (s *GitAccountService) Update(ctx context.Context, id string, input models.UpdateGitAccountInput) (*models.GitAccount, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *GitAccountService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *GitAccountService) TestConnection(ctx context.Context, id string) error {
	acc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if acc.Token == "" {
		return ErrValidation("git account token is empty")
	}
	return gitops.NewGitHubProvider(acc.BaseURL).ValidateToken(ctx, acc.Token)
}
