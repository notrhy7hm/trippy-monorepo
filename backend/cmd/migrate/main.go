package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/pressly/goose/v3"

	"github.com/trippyai/trippy/backend/internal/config"
	internaldb "github.com/trippyai/trippy/backend/internal/db"
)

func main() {
	dir := flag.String("dir", "migrations", "directory containing goose migrations")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	pool, err := internaldb.Open(cfg.DatabaseURL)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("set migration dialect", "err", err)
		os.Exit(1)
	}
	if err := goose.Up(pool.DB, *dir); err != nil {
		logger.Error("run migrations", "err", err, "dir", *dir)
		os.Exit(1)
	}
	logger.Info("migrations complete")
}
