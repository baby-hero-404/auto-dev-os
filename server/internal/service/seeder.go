package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
)

// SeederService seeds default rules and skills when a new project is created.
type SeederService struct {
	ruleRepo *repository.RuleRepo
}

// NewSeederService creates a SeederService with the required repositories.
func NewSeederService(ruleRepo *repository.RuleRepo) *SeederService {
	return &SeederService{ruleRepo: ruleRepo}
}

// SeedProject inserts default rules and skills for a newly created project.
// Errors are logged but do not prevent project creation from succeeding.
func (s *SeederService) SeedProject(ctx context.Context, projectID string) {
	// Seeding of default rules and skills disabled as requested
}
