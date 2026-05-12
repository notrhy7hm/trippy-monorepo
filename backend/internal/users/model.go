package users

import (
	"time"

	"github.com/google/uuid"
)

// User is the public-safe representation — no password hash, no sensitive fields.
type User struct {
	ID          uuid.UUID `db:"id" json:"-"`
	Username    string    `db:"username" json:"username"`
	Email       string    `db:"email" json:"email"`
	DisplayName string    `db:"display_name" json:"displayName"`
	Bio         string    `db:"bio" json:"bio"`
	AvatarURL   string    `db:"avatar_url" json:"avatarUrl"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
}

type CreateInput struct {
	Email        string
	Username     string
	PasswordHash string
}

type UpdateInput struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
}

// PublicUser is the minimal, privacy-safe shape returned by user-search.
// Email is intentionally omitted to prevent enumeration via search.
type PublicUser struct {
	ID          uuid.UUID `db:"id"           json:"-"`
	Username    string    `db:"username"     json:"username"`
	DisplayName string    `db:"display_name" json:"displayName"`
	AvatarURL   string    `db:"avatar_url"   json:"avatarUrl,omitempty"`
}
