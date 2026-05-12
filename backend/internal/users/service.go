package users

import (
	"context"

	"github.com/google/uuid"
)

type Service struct{ repo *Repo }

func NewService(r *Repo) *Service { return &Service{repo: r} }

func (s *Service) Create(ctx context.Context, in CreateInput) (User, error) {
	return s.repo.Create(ctx, in)
}

func (s *Service) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.ByID(ctx, id)
}

func (s *Service) ByUsername(ctx context.Context, username string) (User, error) {
	return s.repo.ByUsername(ctx, username)
}

func (s *Service) FindForLogin(ctx context.Context, identifier string) (User, string, error) {
	return s.repo.FindForLogin(ctx, identifier)
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateInput) (User, error) {
	return s.repo.UpdateProfile(ctx, id, in)
}
