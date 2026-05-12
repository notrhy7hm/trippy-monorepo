package api

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// UserID returns the authenticated caller's UUID. Zero UUID if missing.
func UserID(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(userIDKey).(uuid.UUID)
	return v
}
