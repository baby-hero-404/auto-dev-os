package service

import (
	"context"

	"fmt"

	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GitAccountService struct {
	repo        *repository.GitAccountRepo
	gitProvider gitops.GitProvider
	cipher      *SecretCipher
}

func NewGitAccountService(repo *repository.GitAccountRepo, cipher *SecretCipher) *GitAccountService {
	return &GitAccountService{
		repo:        repo,
		gitProvider: gitops.NewGitHubProvider(""), // For now GitHub, can expand later
		cipher:      cipher,
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
	if input.Token != "" && s.cipher != nil {
		enc, err := s.cipher.Encrypt(input.Token)
		if err != nil {
			return nil, err
		}
		input.Token = enc
	}
	acc, err := s.repo.Create(ctx, orgID, input)
	if err == nil && acc.Token != "" && s.cipher != nil {
		if dec, decErr := s.cipher.Decrypt(acc.Token); decErr == nil {
			acc.Token = dec
		} else {
			return nil, fmt.Errorf("decrypt token: %w", decErr)
		}
	}
	return acc, err
}

func (s *GitAccountService) GetByID(ctx context.Context, id string) (*models.GitAccount, error) {
	acc, err := s.repo.GetByID(ctx, id)
	if err == nil && acc.Token != "" && s.cipher != nil {
		if dec, decErr := s.cipher.Decrypt(acc.Token); decErr == nil {
			acc.Token = dec
		} else {
			return nil, fmt.Errorf("decrypt token: %w", decErr)
		}
	}
	return acc, err
}

func (s *GitAccountService) ListByOrgID(ctx context.Context, orgID string) ([]models.GitAccount, error) {
	accounts, err := s.repo.ListByOrgID(ctx, orgID)
	if err == nil && s.cipher != nil {
		for i := range accounts {
			if accounts[i].Token != "" {
				if dec, decErr := s.cipher.Decrypt(accounts[i].Token); decErr == nil {
					accounts[i].Token = dec
				} else {
					return nil, fmt.Errorf("decrypt token: %w", decErr)
				}
			}
		}
	}
	return accounts, err
}

func (s *GitAccountService) Update(ctx context.Context, id string, input models.UpdateGitAccountInput) (*models.GitAccount, error) {
	if input.Token != nil && *input.Token != "" && s.cipher != nil {
		enc, err := s.cipher.Encrypt(*input.Token)
		if err != nil {
			return nil, err
		}
		input.Token = &enc
	}
	acc, err := s.repo.Update(ctx, id, input)
	if err == nil && acc.Token != "" && s.cipher != nil {
		if dec, decErr := s.cipher.Decrypt(acc.Token); decErr == nil {
			acc.Token = dec
		} else {
			return nil, fmt.Errorf("decrypt token: %w", decErr)
		}
	}
	return acc, err
}

func (s *GitAccountService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *GitAccountService) TestConnection(ctx context.Context, id string) error {
	acc, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if acc.Token == "" {
		return ErrValidation("git account token is empty")
	}
	return gitops.NewGitHubProvider(acc.BaseURL).ValidateToken(ctx, acc.Token)
}
