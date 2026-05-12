package httpx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/trippyai/trippy/backend/internal/api"
	"github.com/trippyai/trippy/backend/internal/auth"
	"github.com/trippyai/trippy/backend/internal/friends"
	"github.com/trippyai/trippy/backend/internal/trips"
	"github.com/trippyai/trippy/backend/internal/users"
)

type Deps struct {
	Auth    *auth.Service
	Users   *users.Service
	Friends *friends.Service
	Trips   *trips.Service
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		api.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// public
		authH := auth.NewHandler(d.Auth)
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		// authenticated
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(d.Auth))

			usersH := users.NewHandler(d.Users)
			r.Get("/auth/me", usersH.Me)
			r.Get("/users/me", usersH.Me)
			r.Patch("/users/me", usersH.UpdateMe)

			// friends module also owns /users/search because the response
			// is enriched with the viewer's relation to each result.
			friendsH := friends.NewHandler(d.Friends)
			r.Get("/users/search", friendsH.Search)
			r.Get("/friends", friendsH.List)
			r.Delete("/friends/{username}", friendsH.Remove)
			r.Get("/friend-requests/incoming", friendsH.ListIncoming)
			r.Get("/friend-requests/outgoing", friendsH.ListOutgoing)
			r.Post("/friend-requests", friendsH.Send)
			r.Post("/friend-requests/from/{username}/accept", friendsH.Accept)
			r.Post("/friend-requests/from/{username}/decline", friendsH.Decline)
			r.Delete("/friend-requests/to/{username}", friendsH.Cancel)

			tripsH := trips.NewHandler(d.Trips)
			r.Post("/trips", tripsH.Create)
			r.Get("/trips", tripsH.ListMine)
			r.Get("/trips/{tripSlug}", tripsH.Get)
			r.Patch("/trips/{tripSlug}", tripsH.Update)
			r.Delete("/trips/{tripSlug}", tripsH.Delete)

			// M0 placeholder — M1 replaces with real invites
			r.Get("/trips/{tripSlug}/members", tripsH.ListMembers)
			r.Post("/trips/{tripSlug}/invites", tripsH.CreateInvitePlaceholder)
		})
	})

	return r
}
