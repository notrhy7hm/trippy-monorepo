package trips

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var ErrForbidden = errors.New("forbidden")

type Service struct{ repo *Repo }

func NewService(r *Repo) *Service { return &Service{repo: r} }

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (Trip, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return Trip{}, errors.New("title required")
	}
	if in.Visibility == "" {
		in.Visibility = VisibilityPrivate
	}

	// Retry slug generation a few times to absorb the rare suffix collision.
	for attempt := 0; attempt < 5; attempt++ {
		t, err := s.repo.Create(ctx, ownerID, MakeSlug(in.Title), in)
		if errors.Is(err, ErrSlugCollision) {
			continue
		}
		return t, err
	}
	return Trip{}, errors.New("could not allocate slug")
}

func (s *Service) BySlugForUser(ctx context.Context, slug string, userID uuid.UUID) (Trip, error) {
	t, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Trip{}, err
	}
	if err := s.assertVisible(ctx, t, userID); err != nil {
		return Trip{}, err
	}
	return t, nil
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Trip, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *Service) Update(ctx context.Context, slug string, userID uuid.UUID, in UpdateInput) (Trip, error) {
	t, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Trip{}, err
	}
	role, ok, err := s.repo.IsMember(ctx, t.ID, userID)
	if err != nil {
		return Trip{}, err
	}
	if !ok || (role != RoleOwner && role != RoleAdmin) {
		return Trip{}, ErrForbidden
	}
	return s.repo.Update(ctx, slug, in)
}

func (s *Service) Delete(ctx context.Context, slug string, userID uuid.UUID) error {
	t, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return err
	}
	if t.OwnerID != userID {
		return ErrForbidden
	}
	return s.repo.SoftDelete(ctx, slug)
}

func (s *Service) ListMembers(ctx context.Context, slug string, userID uuid.UUID) ([]Member, error) {
	t, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if err := s.assertVisible(ctx, t, userID); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, t.ID)
}

// assertVisible enforces the trip's visibility rules. Friends-mode falls back to
// private until the friends module lands in M1 — that's deliberate: we'd rather
// be too strict than leak data.
func (s *Service) assertVisible(ctx context.Context, t Trip, userID uuid.UUID) error {
	if t.Visibility == VisibilityPublic {
		return nil
	}
	_, ok, err := s.repo.IsMember(ctx, t.ID, userID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return ErrForbidden
}
