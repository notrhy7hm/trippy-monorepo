.PHONY: run tidy migrate-up migrate-down migrate-status

DB_URL ?= postgres://trippy:trippy@localhost:5432/trippy?sslmode=disable

run:
	go run ./cmd/server

tidy:
	go mod tidy

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_URL)" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_URL)" down

migrate-status:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_URL)" status
