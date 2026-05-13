package trips

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/users"
)

var (
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidRole        = errors.New("invalid role")
	ErrCannotInviteSelf   = errors.New("cannot invite yourself")
	ErrAlreadyMember      = errors.New("user is already a trip member")
	ErrNotFriends         = errors.New("only friends can be invited")
	ErrInviteExpired      = errors.New("trip invite is expired")
	ErrInviteNotYours     = errors.New("invite was not addressed to you")
	ErrInviteBadIdentifier = errors.New("invitee identifier is required")
)

// FriendsChecker is the minimum surface trips needs from the friends module
// to enforce the M1 "only friends can be invited" rule. friends.Service
// satisfies this interface.
type FriendsChecker interface {
	AreFriends(ctx context.Context, a, b uuid.UUID) (bool, error)
}

const defaultInviteTTL = 14 * 24 * time.Hour

type Service struct {
	repo    *Repo
	users   *users.Service
	friends FriendsChecker
}

func NewService(r *Repo, u *users.Service, f FriendsChecker) *Service {
	return &Service{repo: r, users: u, friends: f}
}

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

// ---------------------------------------------------------------------------
// invites
// ---------------------------------------------------------------------------

// CreateInvite mints a pending invite addressed to a username or an email.
// Only the trip's owner or admin may call this. Existing friendship is
// required when the identifier resolves to a registered user.
func (s *Service) CreateInvite(ctx context.Context, callerID uuid.UUID, slug string, in CreateInviteInput) (Invite, error) {
	trip, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Invite{}, err
	}

	callerRole, ok, err := s.repo.IsMember(ctx, trip.ID, callerID)
	if err != nil {
		return Invite{}, err
	}
	if !ok || !canInvite(callerRole) {
		return Invite{}, ErrForbidden
	}

	role := in.Role
	if role == "" {
		role = RoleMember
	}
	if !isAssignableRole(role) {
		return Invite{}, ErrInvalidRole
	}

	ident := strings.ToLower(strings.TrimSpace(in.Identifier))
	if ident == "" {
		return Invite{}, ErrInviteBadIdentifier
	}

	var (
		inviteeUserID *uuid.UUID
		inviteeEmail  *string
	)

	if strings.Contains(ident, "@") {
		// Email path: resolve to user if one exists, else allow email-only.
		u, err := s.users.ByEmail(ctx, ident)
		switch {
		case errors.Is(err, users.ErrNotFound):
			e := ident
			inviteeEmail = &e
		case err != nil:
			return Invite{}, err
		default:
			if err := s.requireFriendship(ctx, callerID, u.ID); err != nil {
				return Invite{}, err
			}
			id := u.ID
			inviteeUserID = &id
		}
	} else {
		u, err := s.users.ByUsername(ctx, ident)
		if err != nil {
			return Invite{}, err
		}
		if u.ID == callerID {
			return Invite{}, ErrCannotInviteSelf
		}
		if err := s.requireFriendship(ctx, callerID, u.ID); err != nil {
			return Invite{}, err
		}
		id := u.ID
		inviteeUserID = &id
	}

	if inviteeUserID != nil {
		_, isMember, err := s.repo.IsMember(ctx, trip.ID, *inviteeUserID)
		if err != nil {
			return Invite{}, err
		}
		if isMember {
			return Invite{}, ErrAlreadyMember
		}
	}

	token, err := newInviteToken()
	if err != nil {
		return Invite{}, err
	}

	row, err := s.repo.CreateInvite(ctx, inviteInsertRow{
		TripID:          trip.ID,
		InviteeUserID:   inviteeUserID,
		InviteeEmail:    inviteeEmail,
		Token:           token,
		ExpiresAt:       time.Now().Add(defaultInviteTTL),
		InvitedByUserID: callerID,
		Role:            role,
	})
	if err != nil {
		return Invite{}, err
	}
	return inviteFromRow(row), nil
}

// ListInvites returns pending, unexpired invites for a trip. Owner/admin only.
func (s *Service) ListInvites(ctx context.Context, callerID uuid.UUID, slug string) ([]Invite, error) {
	trip, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	role, ok, err := s.repo.IsMember(ctx, trip.ID, callerID)
	if err != nil {
		return nil, err
	}
	if !ok || !canInvite(role) {
		return nil, ErrForbidden
	}
	rows, err := s.repo.ListTripInvites(ctx, trip.ID)
	if err != nil {
		return nil, err
	}
	return invitesFromRows(rows), nil
}

// RevokeInvite marks an invite as revoked. Owner/admin only.
func (s *Service) RevokeInvite(ctx context.Context, callerID uuid.UUID, slug, token string) error {
	trip, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return err
	}
	role, ok, err := s.repo.IsMember(ctx, trip.ID, callerID)
	if err != nil {
		return err
	}
	if !ok || !canInvite(role) {
		return ErrForbidden
	}
	return s.repo.RevokeInvite(ctx, trip.ID, token)
}

// PreviewInvite returns the invite addressed to the caller. Anything not
// pending (revoked / declined / accepted / expired) is surfaced as a typed
// error so the handler can produce a clear message.
func (s *Service) PreviewInvite(ctx context.Context, callerID uuid.UUID, token string) (Invite, error) {
	row, err := s.loadActionableInvite(ctx, callerID, token)
	if err != nil {
		return Invite{}, err
	}
	return inviteFromRow(row), nil
}

func (s *Service) AcceptInvite(ctx context.Context, callerID uuid.UUID, token string) (Trip, error) {
	if _, err := s.loadActionableInvite(ctx, callerID, token); err != nil {
		return Trip{}, err
	}
	return s.repo.AcceptInvite(ctx, token, callerID)
}

func (s *Service) DeclineInvite(ctx context.Context, callerID uuid.UUID, token string) error {
	if _, err := s.loadActionableInvite(ctx, callerID, token); err != nil {
		return err
	}
	return s.repo.DeclineInvite(ctx, token)
}

// ListMyInvites returns pending invites addressed to the caller, matched by
// user ID or email.
func (s *Service) ListMyInvites(ctx context.Context, callerID uuid.UUID) ([]Invite, error) {
	caller, err := s.users.ByID(ctx, callerID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListPendingForUser(ctx, callerID, caller.Email)
	if err != nil {
		return nil, err
	}
	return invitesFromRows(rows), nil
}

// loadActionableInvite expires the row lazily, then loads and authorizes it.
func (s *Service) loadActionableInvite(ctx context.Context, callerID uuid.UUID, token string) (inviteRow, error) {
	if err := s.repo.MarkExpiredByToken(ctx, token); err != nil {
		return inviteRow{}, err
	}
	row, err := s.repo.InviteByToken(ctx, token)
	if err != nil {
		return inviteRow{}, err
	}
	switch InviteStatus(row.Status) {
	case InviteStatusPending:
		// continue
	case InviteStatusExpired:
		return inviteRow{}, ErrInviteExpired
	default:
		return inviteRow{}, ErrInviteNotPending
	}
	caller, err := s.users.ByID(ctx, callerID)
	if err != nil {
		return inviteRow{}, err
	}
	switch {
	case row.InviteeUserID != nil:
		if *row.InviteeUserID != callerID {
			return inviteRow{}, ErrInviteNotYours
		}
	case row.InviteeEmail != nil:
		if !strings.EqualFold(*row.InviteeEmail, caller.Email) {
			return inviteRow{}, ErrInviteNotYours
		}
	default:
		// Schema CHECK forbids this; defensive default.
		return inviteRow{}, ErrInviteNotYours
	}
	return row, nil
}

// requireFriendship returns ErrCannotInviteSelf or ErrNotFriends for non-friend pairs.
func (s *Service) requireFriendship(ctx context.Context, caller, target uuid.UUID) error {
	if caller == target {
		return ErrCannotInviteSelf
	}
	ok, err := s.friends.AreFriends(ctx, caller, target)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFriends
	}
	return nil
}

func canInvite(r Role) bool {
	return r == RoleOwner || r == RoleAdmin
}

// isAssignableRole limits invite roles to the set the inviter may grant.
// Owner is intentionally excluded; ownership transfer is out of scope for M1.
func isAssignableRole(r Role) bool {
	switch r {
	case RoleAdmin, RolePlanner, RoleBudgetManager, RoleMember, RoleViewer:
		return true
	}
	return false
}

func newInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func inviteFromRow(row inviteRow) Invite {
	out := Invite{
		Token:     row.Token,
		Status:    InviteStatus(row.Status),
		Role:      Role(row.Role),
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
		Trip:      &InviteTrip{Slug: row.TripSlug, Title: row.TripTitle},
	}
	if row.InviteeUsername.Valid {
		out.Invitee = &Party{
			Username:    row.InviteeUsername.String,
			DisplayName: row.InviteeDisplayName.String,
		}
	}
	if row.InviteeEmail != nil {
		out.InviteeEmail = *row.InviteeEmail
	}
	if row.ByUsername.Valid {
		out.InvitedBy = &Party{
			Username:    row.ByUsername.String,
			DisplayName: row.ByDisplayName.String,
		}
	}
	return out
}

func invitesFromRows(rows []inviteRow) []Invite {
	out := make([]Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, inviteFromRow(r))
	}
	return out
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
