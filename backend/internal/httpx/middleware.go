package httpx

import (
	"net/http"
	"strings"

	"github.com/trippyai/trippy/backend/internal/api"
	"github.com/trippyai/trippy/backend/internal/auth"
)

// RequireAuth verifies the JWT and injects the caller's user UUID into context.
func RequireAuth(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				api.Err(w, http.StatusUnauthorized, "unauthenticated", "missing bearer token")
				return
			}
			uid, err := authSvc.VerifyToken(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				api.Err(w, http.StatusUnauthorized, "unauthenticated", "invalid token")
				return
			}
			next.ServeHTTP(w, r.WithContext(api.WithUserID(r.Context(), uid)))
		})
	}
}
