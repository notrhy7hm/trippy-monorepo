package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type claims struct {
	jwt.RegisteredClaims
}

func issueToken(secret []byte, ttl time.Duration, userID uuid.UUID) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret)
}

func parseToken(secret []byte, raw string) (uuid.UUID, error) {
	parsed, err := jwt.ParseWithClaims(raw, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || !parsed.Valid {
		return uuid.Nil, errors.New("invalid token")
	}
	return uuid.Parse(c.Subject)
}
