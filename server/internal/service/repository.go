package service

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/gitops"
	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type RepositoryService struct {
	repo           *repository.RepositoryRepo
	projRepo       *repository.ProjectRepo
	gitAccountRepo *repository.GitAccountRepo
	gitProvider    gitops.GitProvider
	workspaceDir   string
}

type gitCredentials struct {
	token   string
	baseURL string
}

func NewRepositoryService(repo *repository.RepositoryRepo) *RepositoryService {
	return &RepositoryService{
		repo:         repo,
		gitProvider:  gitops.NewGitHubProvider(""),
		workspaceDir: filepath.Join(os.TempDir(), "autocodeos", "repositories"),
	}
}

func (s *RepositoryService) SetProjectRepo(r *repository.ProjectRepo) {
	s.projRepo = r
}

func (s *RepositoryService) SetGitAccountRepo(r *repository.GitAccountRepo) {
	s.gitAccountRepo = r
}

func (s *RepositoryService) resolveCredentials(ctx context.Context, repo *models.Repository) (gitCredentials, error) {
	// 1. Project/Repo-level override (manual token)
	if repo.Token != "" {
		return gitCredentials{token: repo.Token}, nil
	}

	// 2. Specific Git Account link
	if repo.GitAccountID != nil && *repo.GitAccountID != "" && s.gitAccountRepo != nil {
		acc, err := s.gitAccountRepo.GetByID(ctx, *repo.GitAccountID)
		if err == nil && acc.Token != "" {
			return gitCredentials{token: acc.Token, baseURL: acc.BaseURL}, nil
		}
	}

	// 3. Fallback: Org-level git account matching the repo provider
	if s.projRepo != nil && s.gitAccountRepo != nil {
		project, err := s.projRepo.GetByID(ctx, repo.ProjectID)
		if err == nil && project != nil {
			accounts, err := s.gitAccountRepo.ListByOrgID(ctx, project.OrgID)
			if err == nil {
				for _, acc := range accounts {
					if acc.Provider == repo.Provider && acc.Token != "" {
						return gitCredentials{token: acc.Token, baseURL: acc.BaseURL}, nil
					}
				}
			}
		}
	}

	return gitCredentials{}, nil
}

func (s *RepositoryService) Create(ctx context.Context, projectID string, input models.CreateRepositoryInput) (*models.Repository, error) {
	if input.URL == "" {
		return nil, ErrValidation("url is required")
	}
	normalized, err := gitops.NormalizeGitURLToHTTPS(input.URL)
	if err != nil {
		return nil, ErrValidation(fmt.Sprintf("invalid repository URL: %v", err))
	}
	input.URL = normalized
	return s.repo.Create(ctx, projectID, input)
}

func (s *RepositoryService) GetByID(ctx context.Context, id string) (*models.Repository, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RepositoryService) ListByProjectID(ctx context.Context, projectID string) ([]models.Repository, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *RepositoryService) Update(ctx context.Context, id string, input models.UpdateRepositoryInput) (*models.Repository, error) {
	if input.URL != nil && *input.URL != "" {
		normalized, err := gitops.NormalizeGitURLToHTTPS(*input.URL)
		if err != nil {
			return nil, ErrValidation(fmt.Sprintf("invalid repository URL: %v", err))
		}
		input.URL = &normalized
	}
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
	creds, err := s.resolveCredentials(ctx, repo)
	if err != nil {
		return err
	}
	if creds.token == "" {
		return ErrValidation("repository token or linked git account token is required")
	}
	provider := s.gitProvider
	if creds.baseURL != "" {
		provider = gitops.NewGitHubProvider(creds.baseURL)
	}
	if err := provider.ValidateToken(ctx, creds.token); err != nil {
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
	creds, err := s.resolveCredentials(ctx, repo)
	if err != nil {
		failed := "failed"
		_, _ = s.repo.Update(context.Background(), id, models.UpdateRepositoryInput{CloneStatus: &failed})
		return nil, err
	}
	actualBranch, err := s.gitProvider.CloneRepo(ctx, repo.URL, creds.token, repo.Branch, clonePath)
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

func (s *RepositoryService) GetRemoteBranches(ctx context.Context, repoURL string, token string, gitAccountID *string) ([]string, error) {
	if repoURL == "" {
		return nil, ErrValidation("repository url is required")
	}

	actualToken := token
	if actualToken == "" && gitAccountID != nil && *gitAccountID != "" && s.gitAccountRepo != nil {
		acc, err := s.gitAccountRepo.GetByID(ctx, *gitAccountID)
		if err == nil && acc.Token != "" {
			actualToken = acc.Token
		}
	}

	normalized, err := gitops.NormalizeGitURLToHTTPS(repoURL)
	if err != nil {
		return nil, ErrValidation(fmt.Sprintf("invalid repository URL: %v", err))
	}

	cloneURL := normalized
	if actualToken != "" {
		parsed, err := url.Parse(normalized)
		if err == nil && parsed.Scheme == "https" {
			parsed.User = url.UserPassword("x-access-token", actualToken)
			cloneURL = parsed.String()
		}
	}

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", cloneURL)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=false",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if actualToken != "" {
			errMsg = strings.ReplaceAll(errMsg, actualToken, "[redacted]")
		}
		return nil, fmt.Errorf("git ls-remote failed: %w: %s", err, errMsg)
	}

	var branches []string
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			ref := parts[1]
			if strings.HasPrefix(ref, "refs/heads/") {
				branch := strings.TrimPrefix(ref, "refs/heads/")
				branches = append(branches, branch)
			}
		}
	}
	return branches, nil
}
