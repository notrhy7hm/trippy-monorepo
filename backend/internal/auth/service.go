package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/users"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidInput       = errors.New("invalid input")
)

// Service is intentionally a thin orchestrator over users.Service.
// We keep auth-specific concerns (hashing, tokens) here and let
// users own everything about user data.
type Service struct {
	users  *users.Service
	secret []byte
	ttl    time.Duration
}

func NewService(u *users.Service, secret []byte, ttl time.Duration) *Service {
	return &Service{users: u, secret: secret, ttl: ttl}
}

type RegisterInput struct {
	Email    string
	Username string
	Password string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (users.User, string, error) {
	if len(in.Password) < 8 {
		return users.User{}, "", ErrInvalidInput
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return users.User{}, "", err
	}
	u, err := s.users.Create(ctx, users.CreateInput{
		Email:        in.Email,
		Username:     in.Username,
		PasswordHash: hash,
	})
	if err != nil {
		if errors.Is(err, users.ErrUserExists) {
			return users.User{}, "", ErrUserExists
		}
		if users.IsValidationError(err) {
			return users.User{}, "", ErrInvalidInput
		}
		return users.User{}, "", err
	}
	tok, err := issueToken(s.secret, s.ttl, u.ID)
	if err != nil {
		return users.User{}, "", err
	}
	return u, tok, nil
}

func (s *Service) Login(ctx context.Context, emailOrUsername, password string) (users.User, string, error) {
	id := strings.ToLower(strings.TrimSpace(emailOrUsername))
	u, hash, err := s.users.FindForLogin(ctx, id)
	if err != nil {
		return users.User{}, "", ErrInvalidCredentials
	}
	if !verifyPassword(hash, password) {
		return users.User{}, "", ErrInvalidCredentials
	}
	tok, err := issueToken(s.secret, s.ttl, u.ID)
	if err != nil {
		return users.User{}, "", err
	}
	return u, tok, nil
}

func (s *Service) VerifyToken(raw string) (uuid.UUID, error) {
	return parseToken(s.secret, raw)
}
