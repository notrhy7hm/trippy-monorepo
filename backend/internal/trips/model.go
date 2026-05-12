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

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

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
