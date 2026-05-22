package service

import (
	"context"

	"github.com/auto-code-os/auto-code-os/server/internal/repository"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

type OrganizationService struct{ repo *repository.OrganizationRepo }

func NewOrganizationService(repo *repository.OrganizationRepo) *OrganizationService {
	return &OrganizationService{repo: repo}
}

func (s *OrganizationService) Create(ctx context.Context, input models.CreateOrganizationInput) (*models.Organization, error) {
	if input.Name == "" {
		return nil, ErrValidation("name is required")
	}
	return s.repo.Create(ctx, input)
}

func (s *OrganizationService) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *OrganizationService) List(ctx context.Context) ([]models.Organization, error) {
	return s.repo.List(ctx)
}

func (s *OrganizationService) Update(ctx context.Context, id string, input models.UpdateOrganizationInput) (*models.Organization, error) {
	return s.repo.Update(ctx, id, input)
}

func (s *OrganizationService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
