package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trippyai/trippy/backend/internal/agents"
	"github.com/trippyai/trippy/backend/internal/auth"
	"github.com/trippyai/trippy/backend/internal/budget"
	"github.com/trippyai/trippy/backend/internal/config"
	"github.com/trippyai/trippy/backend/internal/db"
	"github.com/trippyai/trippy/backend/internal/friends"
	"github.com/trippyai/trippy/backend/internal/httpx"
	"github.com/trippyai/trippy/backend/internal/planning"
	"github.com/trippyai/trippy/backend/internal/trips"
	"github.com/trippyai/trippy/backend/internal/users"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	pool, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Wiring is done here so module dependencies are explicit and one-way.
	// Modules expose Service; handlers depend only on the local Service.
	userSvc := users.NewService(users.NewRepo(pool))
	authSvc := auth.NewService(userSvc, cfg.JWTSecret, cfg.JWTTTL)
	friendsSvc := friends.NewService(friends.NewRepo(pool), userSvc)
	tripSvc := trips.NewService(trips.NewRepo(pool), userSvc, friendsSvc)
	planningSvc := planning.NewService(
		planning.NewRepo(pool),
		planning.NewItineraryRepo(pool),
		userSvc,
		tripSvc,
	)
	budgetSvc := budget.NewService(budget.NewRepo(pool), tripSvc)
	agentSvc := agents.NewService() // M3 — stub, not wired into routes yet

	_ = agentSvc

	r := httpx.NewRouter(httpx.Deps{
		Auth:     authSvc,
		Users:    userSvc,
		Friends:  friendsSvc,
		Trips:    tripSvc,
		Planning: planningSvc,
		Budget:   budgetSvc,

		AllowOrigin: cfg.AllowOrigin,
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}
