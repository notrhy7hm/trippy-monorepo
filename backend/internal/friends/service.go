package friends

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/users"
)

// ErrMessageTooLong is surfaced by SendRequest when the optional message
// exceeds maxMessageLen runes.
var ErrMessageTooLong = errors.New("message too long")

const maxMessageLen = 280

// Service orchestrates friend requests and friendships. It depends on
// users.Service for username -> UUID lookups.
type Service struct {
	repo  *Repo
	users *users.Service
}

func NewService(r *Repo, u *users.Service) *Service {
	return &Service{repo: r, users: u}
}

// AreFriends is exposed for cross-module checks (e.g. trip visibility=friends
// in a later M1 commit).
func (s *Service) AreFriends(ctx context.Context, a, b uuid.UUID) (bool, error) {
	return s.repo.AreFriends(ctx, a, b)
}

func (s *Service) ListFriends(ctx context.Context, userID uuid.UUID) ([]Friend, error) {
	return s.repo.ListFriends(ctx, userID)
}

func (s *Service) ListIncomingRequests(ctx context.Context, userID uuid.UUID) ([]Request, error) {
	return s.repo.ListIncomingRequests(ctx, userID)
}

func (s *Service) ListOutgoingRequests(ctx context.Context, userID uuid.UUID) ([]Request, error) {
	return s.repo.ListOutgoingRequests(ctx, userID)
}

// SendRequest creates a pending friend request from fromID to the user whose
// username is toUsername.
func (s *Service) SendRequest(ctx context.Context, fromID uuid.UUID, toUsername, message string) (Request, error) {
	target, err := s.users.ByUsername(ctx, normalizeUsername(toUsername))
	if err != nil {
		return Request{}, err // users.ErrNotFound bubbles
	}
	if target.ID == fromID {
		return Request{}, ErrSelf
	}

	friends, err := s.repo.AreFriends(ctx, fromID, target.ID)
	if err != nil {
		return Request{}, err
	}
	if friends {
		return Request{}, ErrAlreadyFriends
	}

	pending, err := s.repo.PendingExists(ctx, fromID, target.ID)
	if err != nil {
		return Request{}, err
	}
	if pending {
		return Request{}, ErrAlreadyPending
	}

	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > maxMessageLen {
		return Request{}, ErrMessageTooLong
	}

	created, err := s.repo.CreateRequest(ctx, fromID, target.ID, message)
	if err != nil {
		return Request{}, err
	}
	return Request{
		Username:    target.Username,
		DisplayName: orUsername(target.DisplayName, target.Username),
		Message:     message,
		CreatedAt:   created,
	}, nil
}

// AcceptRequest accepts a pending request from `fromUsername` addressed to
// the current user. Returns the new friendship from the current user's
// perspective.
func (s *Service) AcceptRequest(ctx context.Context, currentID uuid.UUID, fromUsername string) (Friend, error) {
	from, err := s.users.ByUsername(ctx, normalizeUsername(fromUsername))
	if err != nil {
		return Friend{}, err
	}
	since, err := s.repo.AcceptRequest(ctx, from.ID, currentID)
	if err != nil {
		return Friend{}, err
	}
	return Friend{
		Username:    from.Username,
		DisplayName: orUsername(from.DisplayName, from.Username),
		Since:       since,
	}, nil
}

func (s *Service) DeclineRequest(ctx context.Context, currentID uuid.UUID, fromUsername string) error {
	from, err := s.users.ByUsername(ctx, normalizeUsername(fromUsername))
	if err != nil {
		return err
	}
	return s.repo.DeclineRequest(ctx, from.ID, currentID)
}

func (s *Service) CancelRequest(ctx context.Context, currentID uuid.UUID, toUsername string) error {
	to, err := s.users.ByUsername(ctx, normalizeUsername(toUsername))
	if err != nil {
		return err
	}
	return s.repo.CancelRequest(ctx, currentID, to.ID)
}

func (s *Service) RemoveFriend(ctx context.Context, currentID uuid.UUID, otherUsername string) error {
	other, err := s.users.ByUsername(ctx, normalizeUsername(otherUsername))
	if err != nil {
		return err
	}
	return s.repo.DeleteFriendship(ctx, currentID, other.ID)
}

// SearchPeople runs a user search and enriches the results with the viewer's
// relation to each candidate.
func (s *Service) SearchPeople(ctx context.Context, viewerID uuid.UUID, query string) ([]SearchResult, error) {
	results, err := s.users.Search(ctx, query, 20)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SearchResult{}, nil
	}

	ids := make([]uuid.UUID, 0, len(results))
	for _, u := range results {
		ids = append(ids, u.ID)
	}
	relations, err := s.repo.RelationsFor(ctx, viewerID, ids)
	if err != nil {
		return nil, err
	}

	out := make([]SearchResult, 0, len(results))
	for _, u := range results {
		rel, ok := relations[u.ID]
		if !ok {
			rel = RelationNone
		}
		out = append(out, SearchResult{
			Username:    u.Username,
			DisplayName: orUsername(u.DisplayName, u.Username),
			AvatarURL:   u.AvatarURL,
			Relation:    rel,
		})
	}
	return out, nil
}

func normalizeUsername(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func orUsername(displayName, username string) string {
	if strings.TrimSpace(displayName) == "" {
		return username
	}
	return displayName
}
