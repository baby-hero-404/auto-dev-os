package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type RepositoryService struct {
	repo         *repository.RepositoryRepo
	gitProvider  gitops.GitProvider
	workspaceDir string
}

func NewRepositoryService(repo *repository.RepositoryRepo) *RepositoryService {
	return &RepositoryService{
		repo:         repo,
		gitProvider:  gitops.NewGitHubProvider(),
		workspaceDir: filepath.Join(os.TempDir(), "autocodeos", "repositories"),
	}
}

func (s *RepositoryService) Create(ctx context.Context, projectID string, input models.CreateRepositoryInput) (*models.Repository, error) {
	if input.URL == "" {
		return nil, ErrValidation("url is required")
	}
	return s.repo.Create(ctx, projectID, input)
}

func (s *RepositoryService) GetByID(ctx context.Context, id string) (*models.Repository, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RepositoryService) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *RepositoryService) Update(ctx context.Context, id string, input models.UpdateRepositoryInput) (*models.Repository, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *RepositoryService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *RepositoryService) ValidateToken(ctx context.Context, id string) error {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if repo.Token == "" {
		return ErrValidation("repository token is required")
	}
	if err := s.gitProvider.ValidateToken(ctx, repo.Token); err != nil {
		return err
	}
	return s.repo.MarkValidated(ctx, id)
}

func (s *RepositoryService) ListRemoteRepos(ctx context.Context, token string) ([]models.RemoteRepository, error) {
	if token == "" {
		return nil, ErrValidation("token is required")
	}
	return s.gitProvider.ListRepos(ctx, token)
}

func (s *RepositoryService) Clone(ctx context.Context, id string) (*models.Repository, error) {
	repo, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	clonePath := filepath.Join(s.workspaceDir, repo.ID)
	if err := os.RemoveAll(clonePath); err != nil {
		return nil, fmt.Errorf("prepare clone path: %w", err)
	}
	if err := os.MkdirAll(s.workspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	cloning := "cloning"
	if _, err := s.repo.Update(ctx, id, models.UpdateRepositoryInput{CloneStatus: &cloning}); err != nil {
		return nil, err
	}
	actualBranch, err := s.gitProvider.CloneRepo(ctx, repo.URL, repo.Token, repo.Branch, clonePath)
	if err != nil {
		failed := "failed"
		_, _ = s.repo.Update(context.Background(), id, models.UpdateRepositoryInput{CloneStatus: &failed})
		return nil, err
	}
	cloned := "cloned"
	return s.repo.Update(ctx, id, models.UpdateRepositoryInput{
		ClonePath:   &clonePath,
		CloneStatus: &cloned,
		Branch:      &actualBranch,
	})
}
