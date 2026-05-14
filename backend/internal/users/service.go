package users

import (
	"context"
	"strings"

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

func (s *Service) ByEmail(ctx context.Context, email string) (User, error) {
	return s.repo.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
}

func (s *Service) FindForLogin(ctx context.Context, identifier string) (User, string, error) {
	return s.repo.FindForLogin(ctx, identifier)
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateInput) (User, error) {
	return s.repo.UpdateProfile(ctx, id, in)
}

// Search normalizes the query, clamps the limit, and returns matching users.
// An empty query yields an empty slice without touching the DB.
func (s *Service) Search(ctx context.Context, q string, limit int) ([]PublicUser, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []PublicUser{}, nil
	}
	if len(q) > 64 {
		q = q[:64]
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.Search(ctx, q, limit)
}
