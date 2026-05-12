package db

import (
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func Open(dsn string) (*sqlx.DB, error) {
	d, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(25)
	d.SetMaxIdleConns(5)
	d.SetConnMaxLifetime(30 * time.Minute)
	return d, nil
}
