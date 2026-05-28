package gitops

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type GitProvider interface {
	CloneRepo(ctx context.Context, repoURL, token, branch, localPath string) (string, error)
	CreateBranch(ctx context.Context, localPath, branchName string) error
	CommitAndPush(ctx context.Context, localPath, message, token string) error
	CreatePR(ctx context.Context, owner, repo, title, head, base, body, token string) (string, error)
	ListRepos(ctx context.Context, token string) ([]models.RemoteRepository, error)
	ValidateToken(ctx context.Context, token string) error
}
