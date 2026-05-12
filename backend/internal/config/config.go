package config

import (
	"errors"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   []byte
	JWTTTL      time.Duration
	AllowOrigin string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	c := Config{
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   []byte(os.Getenv("JWT_SECRET")),
		AllowOrigin: getenv("ALLOW_ORIGIN", "http://localhost:5173"),
	}

	ttl, err := time.ParseDuration(getenv("JWT_TTL", "168h"))
	if err != nil {
		return c, err
	}
	c.JWTTTL = ttl

	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 16 {
		return c, errors.New("JWT_SECRET must be at least 16 bytes")
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
