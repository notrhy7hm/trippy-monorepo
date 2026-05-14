package trips

import (
	"time"

	"github.com/google/uuid"
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityFriends Visibility = "friends"
	VisibilityPublic  Visibility = "public"
)

type Role string

// Role values mirror the trip_role enum (migrations 0001 + 0003).
const (
	RoleOwner         Role = "owner"
	RoleAdmin         Role = "admin"
	RolePlanner       Role = "planner"
	RoleBudgetManager Role = "budget_manager"
	RoleMember        Role = "member"
	RoleViewer        Role = "viewer"
)

type InviteStatus string

const (
	InviteStatusPending  InviteStatus = "pending"
	InviteStatusAccepted InviteStatus = "accepted"
	InviteStatusDeclined InviteStatus = "declined"
	InviteStatusRevoked  InviteStatus = "revoked"
	InviteStatusExpired  InviteStatus = "expired"
)

// Party identifies a person in an invite payload by their public username
// and displayName. UUIDs are intentionally omitted from API responses.
type Party struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// InviteTrip is the trip snapshot embedded in invite responses so the
// invitee can recognize the trip without a second fetch.
type InviteTrip struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Invite is the API-facing representation of a trip invite. Different
// endpoints populate different subsets — empty pointers/strings are
// omitted by the json tags.
type Invite struct {
	Token        string       `json:"token"`
	Status       InviteStatus `json:"status"`
	Role         Role         `json:"role"`
	Trip         *InviteTrip  `json:"trip,omitempty"`
	Invitee      *Party       `json:"invitee,omitempty"`
	InviteeEmail string       `json:"inviteeEmail,omitempty"`
	InvitedBy    *Party       `json:"invitedBy,omitempty"`
	ExpiresAt    time.Time    `json:"expiresAt"`
	CreatedAt    time.Time    `json:"createdAt"`
}

type CreateInviteInput struct {
	Identifier string
	Role       Role
}

// inviteInsertRow carries the values written to trip_invites by Repo.
type inviteInsertRow struct {
	TripID          uuid.UUID
	InviteeUserID   *uuid.UUID
	InviteeEmail    *string
	Token           string
	ExpiresAt       time.Time
	InvitedByUserID uuid.UUID
	Role            Role
}

type Trip struct {
	ID          uuid.UUID  `db:"id" json:"-"`
	Slug        string     `db:"slug" json:"slug"`
	OwnerID     uuid.UUID  `db:"owner_id" json:"-"`
	Title       string     `db:"title" json:"title"`
	Description string     `db:"description" json:"description"`
	StartsOn    *time.Time `db:"starts_on" json:"startsOn,omitempty"`
	EndsOn      *time.Time `db:"ends_on" json:"endsOn,omitempty"`
	Visibility  Visibility `db:"visibility" json:"visibility"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updatedAt"`
}

type Member struct {
	UserID      uuid.UUID `db:"user_id" json:"-"`
	Username    string    `db:"username" json:"username"`
	DisplayName string    `db:"display_name" json:"displayName"`
	Role        Role      `db:"role" json:"role"`
	Tags        []string  `db:"tags" json:"tags"`
}

type CreateInput struct {
	Title       string
	Description string
	StartsOn    *time.Time
	EndsOn      *time.Time
	Visibility  Visibility
}

type UpdateInput struct {
	Title       *string
	Description *string
	StartsOn    *time.Time
	EndsOn      *time.Time
	Visibility  *Visibility
}
