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
	ErrInvalidTitle            = errors.New("title must not be empty")
	ErrInvalidVisibility       = errors.New("invalid visibility")
	ErrInvalidDateRange        = errors.New("starts_on must be before or equal to ends_on")
	ErrForbidden               = errors.New("forbidden")
	ErrInvalidRole             = errors.New("invalid role")
	ErrCannotInviteSelf        = errors.New("cannot invite yourself")
	ErrAlreadyMember           = errors.New("user is already a trip member")
	ErrNotFriends              = errors.New("only friends can be invited")
	ErrInviteExpired           = errors.New("trip invite is expired")
	ErrInviteNotYours          = errors.New("invite was not addressed to you")
	ErrInviteBadIdentifier     = errors.New("invitee identifier is required")
	ErrEmailInvitesUnsupported = errors.New("email invites for unregistered users are not supported in M1")
	ErrCannotChangeOwnerRole   = errors.New("owner role cannot be changed")
	ErrInvalidTag              = errors.New("invalid tag")
	ErrTagTooLong              = errors.New("tag is too long")
	ErrTooManyTags             = errors.New("too many tags")
	ErrTagsRequired            = errors.New("tags field is required")
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
	var err error
	in, err = normalizeCreateInput(in)
	if err != nil {
		return Trip{}, err
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

// AssertMember returns the trip's UUID if userID is a member of the trip
// addressed by slug. ErrNotFound when the trip is gone, ErrForbidden when
// the caller is not a member. Used by sibling packages (e.g. planning) that
// need a one-shot slug -> tripID + membership check without importing the
// repo directly.
func (s *Service) AssertMember(ctx context.Context, slug string, userID uuid.UUID) (uuid.UUID, error) {
	tripID, _, err := s.AssertMemberRole(ctx, slug, userID)
	return tripID, err
}

// AssertMemberRole returns the trip UUID and the caller's role if userID is a
// member of the trip addressed by slug. ErrNotFound means the trip is gone;
// ErrForbidden means the caller is not a member.
func (s *Service) AssertMemberRole(ctx context.Context, slug string, userID uuid.UUID) (uuid.UUID, Role, error) {
	trip, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return uuid.Nil, "", err
	}
	role, ok, err := s.repo.IsMember(ctx, trip.ID, userID)
	if err != nil {
		return uuid.Nil, "", err
	}
	if !ok {
		return uuid.Nil, "", ErrForbidden
	}
	return trip.ID, role, nil
}

// IsTripMember reports whether userID is a member of tripID. Sibling
// packages use this to validate assignees / participants without going
// through the slug lookup again.
func (s *Service) IsTripMember(ctx context.Context, tripID, userID uuid.UUID) (bool, error) {
	_, ok, err := s.repo.IsMember(ctx, tripID, userID)
	return ok, err
}

// TripDateRange returns the trip's optional starts_on / ends_on dates.
// Either may be nil. Used by planning for itinerary date-vs-day
// validation. Returns ErrNotFound for missing trips.
func (s *Service) TripDateRange(ctx context.Context, tripID uuid.UUID) (*time.Time, *time.Time, error) {
	return s.repo.DateRangeByID(ctx, tripID)
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

func (s *Service) PublicBySlug(ctx context.Context, slug string) (Trip, error) {
	t, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Trip{}, err
	}
	if t.Visibility != VisibilityPublic {
		return Trip{}, ErrNotFound
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
	in, err = normalizeUpdateInput(t, in)
	if err != nil {
		return Trip{}, err
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
// member roles
// ---------------------------------------------------------------------------

// UpdateMemberRole changes the role of an existing trip member. Only the
// trip owner may call this. The owner cannot be re-roled and 'owner' may
// not be assigned to anyone — M1 keeps exactly one owner per trip.
func (s *Service) UpdateMemberRole(ctx context.Context, callerID uuid.UUID, slug, targetUsername string, newRole Role) (Member, error) {
	trip, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Member{}, err
	}

	callerRole, ok, err := s.repo.IsMember(ctx, trip.ID, callerID)
	if err != nil {
		return Member{}, err
	}
	if !ok || callerRole != RoleOwner {
		return Member{}, ErrForbidden
	}

	// isAssignableRole excludes 'owner', so requesting owner falls here.
	if !isAssignableRole(newRole) {
		return Member{}, ErrInvalidRole
	}

	target, err := s.users.ByUsername(ctx, strings.ToLower(strings.TrimSpace(targetUsername)))
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return Member{}, ErrMemberNotFound
		}
		return Member{}, err
	}

	targetRole, isMember, err := s.repo.IsMember(ctx, trip.ID, target.ID)
	if err != nil {
		return Member{}, err
	}
	if !isMember {
		return Member{}, ErrMemberNotFound
	}
	if targetRole == RoleOwner {
		// Covers both the "demote self" and "re-role owner" cases.
		return Member{}, ErrCannotChangeOwnerRole
	}

	return s.repo.UpdateMemberRole(ctx, trip.ID, target.ID, newRole)
}

// UpdateMemberTags replaces a trip member's tag list. Only the trip owner
// may call this. Tags are normalized (trim, lowercase, internal whitespace
// to '-', dedup; preserves first-occurrence order) and validated; the
// owner's own tags can be updated because tags are labels, not permissions.
func (s *Service) UpdateMemberTags(ctx context.Context, callerID uuid.UUID, slug, targetUsername string, tags []string) (Member, error) {
	trip, err := s.repo.BySlug(ctx, slug)
	if err != nil {
		return Member{}, err
	}

	callerRole, ok, err := s.repo.IsMember(ctx, trip.ID, callerID)
	if err != nil {
		return Member{}, err
	}
	if !ok || callerRole != RoleOwner {
		return Member{}, ErrForbidden
	}

	normalized, err := normalizeTags(tags)
	if err != nil {
		return Member{}, err
	}

	target, err := s.users.ByUsername(ctx, strings.ToLower(strings.TrimSpace(targetUsername)))
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return Member{}, ErrMemberNotFound
		}
		return Member{}, err
	}

	_, isMember, err := s.repo.IsMember(ctx, trip.ID, target.ID)
	if err != nil {
		return Member{}, err
	}
	if !isMember {
		return Member{}, ErrMemberNotFound
	}

	return s.repo.UpdateMemberTags(ctx, trip.ID, target.ID, normalized)
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

	var inviteeUserID *uuid.UUID

	if strings.Contains(ident, "@") {
		// Email path: must resolve to a registered user. M1 disables
		// actionable email-only invites; the column stays for future use.
		u, err := s.users.ByEmail(ctx, ident)
		if errors.Is(err, users.ErrNotFound) {
			return Invite{}, ErrEmailInvitesUnsupported
		}
		if err != nil {
			return Invite{}, err
		}
		if err := s.requireFriendship(ctx, callerID, u.ID); err != nil {
			return Invite{}, err
		}
		id := u.ID
		inviteeUserID = &id
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

	// inviteeUserID is always non-nil in M1 (email-only path was rejected
	// above). The guard is defensive in case the branches change later.
	if inviteeUserID == nil {
		return Invite{}, ErrEmailInvitesUnsupported
	}

	_, isMember, err := s.repo.IsMember(ctx, trip.ID, *inviteeUserID)
	if err != nil {
		return Invite{}, err
	}
	if isMember {
		return Invite{}, ErrAlreadyMember
	}

	token, err := newInviteToken()
	if err != nil {
		return Invite{}, err
	}

	row, err := s.repo.CreateInvite(ctx, inviteInsertRow{
		TripID:          trip.ID,
		InviteeUserID:   inviteeUserID,
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
	return s.repo.DeclineInvite(ctx, token, callerID)
}

// ListMyInvites returns pending invites addressed to the caller. M1 only
// surfaces user-bound invites (email-only invites are not actionable here).
func (s *Service) ListMyInvites(ctx context.Context, callerID uuid.UUID) ([]Invite, error) {
	rows, err := s.repo.ListPendingForUser(ctx, callerID)
	if err != nil {
		return nil, err
	}
	return invitesFromRows(rows), nil
}

// loadActionableInvite expires the row lazily, then loads and authorizes it.
// M1 only surfaces invites bound to a registered invitee_user_id; any
// email-only row is treated as not-yours.
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
	if row.InviteeUserID == nil || *row.InviteeUserID != callerID {
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

func CanManagePlanning(r Role) bool {
	return r == RoleOwner || r == RoleAdmin || r == RolePlanner
}

func CanManageBudget(r Role) bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleBudgetManager
}

func normalizeCreateInput(in CreateInput) (CreateInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return CreateInput{}, ErrInvalidTitle
	}
	if in.Visibility == "" {
		in.Visibility = VisibilityPrivate
	}
	if !in.Visibility.IsValid() {
		return CreateInput{}, ErrInvalidVisibility
	}
	if invalidDateRange(in.StartsOn, in.EndsOn) {
		return CreateInput{}, ErrInvalidDateRange
	}
	return in, nil
}

func normalizeUpdateInput(current Trip, in UpdateInput) (UpdateInput, error) {
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return UpdateInput{}, ErrInvalidTitle
		}
		in.Title = &title
	}
	if in.Visibility != nil && !in.Visibility.IsValid() {
		return UpdateInput{}, ErrInvalidVisibility
	}

	startsOn := current.StartsOn
	if in.StartsOn != nil {
		startsOn = in.StartsOn
	}
	endsOn := current.EndsOn
	if in.EndsOn != nil {
		endsOn = in.EndsOn
	}
	if invalidDateRange(startsOn, endsOn) {
		return UpdateInput{}, ErrInvalidDateRange
	}
	return in, nil
}

func invalidDateRange(startsOn, endsOn *time.Time) bool {
	if startsOn == nil || endsOn == nil {
		return false
	}
	return dateOnly(*endsOn).Before(dateOnly(*startsOn))
}

func dateOnly(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
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

// assertVisible enforces the trip's visibility rules for authenticated users.
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
	if t.Visibility == VisibilityFriends {
		ok, err := s.friends.AreFriends(ctx, userID, t.OwnerID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return ErrForbidden
}
