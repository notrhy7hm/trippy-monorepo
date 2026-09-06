package users

import (
	"context"
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrInvalidDisplayName = errors.New("display name is too long")
	ErrInvalidBio         = errors.New("bio is too long")
	ErrInvalidAvatarURL   = errors.New("invalid avatar URL")
)

const (
	maxDisplayNameRunes = 80
	maxBioRunes         = 280
	maxAvatarURLBytes   = 512
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,29}$`)

type Service struct{ repo *Repo }

func NewService(r *Repo) *Service { return &Service{repo: r} }

func (s *Service) Create(ctx context.Context, in CreateInput) (User, error) {
	var err error
	in.Email, err = normalizeEmail(in.Email)
	if err != nil {
		return User{}, err
	}
	in.Username, err = normalizeUsername(in.Username)
	if err != nil {
		return User{}, err
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.ByID(ctx, id)
}

func (s *Service) ByUsername(ctx context.Context, username string) (User, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return User{}, ErrNotFound
	}
	return s.repo.ByUsername(ctx, username)
}

func (s *Service) ByEmail(ctx context.Context, email string) (User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, ErrNotFound
	}
	return s.repo.ByEmail(ctx, email)
}

func (s *Service) FindForLogin(ctx context.Context, identifier string) (User, string, error) {
	return s.repo.FindForLogin(ctx, strings.ToLower(strings.TrimSpace(identifier)))
}

func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateInput) (User, error) {
	if in.DisplayName != nil {
		v := strings.TrimSpace(*in.DisplayName)
		if utf8.RuneCountInString(v) > maxDisplayNameRunes {
			return User{}, ErrInvalidDisplayName
		}
		in.DisplayName = &v
	}
	if in.Bio != nil {
		v := strings.TrimSpace(*in.Bio)
		if utf8.RuneCountInString(v) > maxBioRunes {
			return User{}, ErrInvalidBio
		}
		in.Bio = &v
	}
	if in.AvatarURL != nil {
		v := strings.TrimSpace(*in.AvatarURL)
		if len(v) > maxAvatarURLBytes {
			return User{}, ErrInvalidAvatarURL
		}
		if v != "" {
			u, err := url.Parse(v)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return User{}, ErrInvalidAvatarURL
			}
		}
		in.AvatarURL = &v
	}
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

func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidEmail) ||
		errors.Is(err, ErrInvalidUsername) ||
		errors.Is(err, ErrInvalidDisplayName) ||
		errors.Is(err, ErrInvalidBio) ||
		errors.Is(err, ErrInvalidAvatarURL)
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func normalizeUsername(raw string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(raw))
	if !usernamePattern.MatchString(username) {
		return "", ErrInvalidUsername
	}
	return username, nil
}
