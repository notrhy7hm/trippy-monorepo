package trips

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var (
	ErrNotFound      = errors.New("trip not found")
	ErrSlugCollision = errors.New("slug already exists")
)

type Repo struct{ db *sqlx.DB }

func NewRepo(db *sqlx.DB) *Repo { return &Repo{db: db} }

const tripColumns = `id, slug, owner_id, title, description, starts_on, ends_on, visibility, created_at, updated_at`

func (r *Repo) Create(ctx context.Context, ownerID uuid.UUID, slug string, in CreateInput) (Trip, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Trip{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var t Trip
	if err := tx.GetContext(ctx, &t, `
		INSERT INTO trips (slug, owner_id, title, description, starts_on, ends_on, visibility)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+tripColumns, slug, ownerID, in.Title, in.Description, in.StartsOn, in.EndsOn, in.Visibility); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Trip{}, ErrSlugCollision
		}
		return Trip{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO trip_members (trip_id, user_id, role, tags)
		VALUES ($1, $2, 'owner', ARRAY[]::text[])
	`, t.ID, ownerID); err != nil {
		return Trip{}, err
	}

	if err := tx.Commit(); err != nil {
		return Trip{}, err
	}
	return t, nil
}

func (r *Repo) BySlug(ctx context.Context, slug string) (Trip, error) {
	var t Trip
	err := r.db.GetContext(ctx, &t, `
		SELECT `+tripColumns+`
		FROM trips
		WHERE slug = $1 AND deleted_at IS NULL
	`, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return Trip{}, ErrNotFound
	}
	return t, err
}

func (r *Repo) ListForUser(ctx context.Context, userID uuid.UUID) ([]Trip, error) {
	var out []Trip
	err := r.db.SelectContext(ctx, &out, `
		SELECT `+tripColumns+`
		FROM trips t
		JOIN trip_members m ON m.trip_id = t.id
		WHERE m.user_id = $1 AND t.deleted_at IS NULL
		ORDER BY COALESCE(t.starts_on, t.created_at) DESC
	`, userID)
	return out, err
}

func (r *Repo) Update(ctx context.Context, slug string, in UpdateInput) (Trip, error) {
	var t Trip
	err := r.db.GetContext(ctx, &t, `
		UPDATE trips SET
			title       = COALESCE($2, title),
			description = COALESCE($3, description),
			starts_on   = COALESCE($4, starts_on),
			ends_on     = COALESCE($5, ends_on),
			visibility  = COALESCE($6, visibility),
			updated_at  = now()
		WHERE slug = $1 AND deleted_at IS NULL
		RETURNING `+tripColumns,
		slug, in.Title, in.Description, in.StartsOn, in.EndsOn, in.Visibility)
	if errors.Is(err, sql.ErrNoRows) {
		return Trip{}, ErrNotFound
	}
	return t, err
}

func (r *Repo) SoftDelete(ctx context.Context, slug string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE trips SET deleted_at = now()
		WHERE slug = $1 AND deleted_at IS NULL
	`, slug)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) IsMember(ctx context.Context, tripID, userID uuid.UUID) (Role, bool, error) {
	var role Role
	err := r.db.GetContext(ctx, &role, `
		SELECT role FROM trip_members WHERE trip_id = $1 AND user_id = $2
	`, tripID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return role, true, nil
}

func (r *Repo) ListMembers(ctx context.Context, tripID uuid.UUID) ([]Member, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT m.user_id, u.username, COALESCE(p.display_name, u.username) AS display_name,
			m.role, m.tags
		FROM trip_members m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN user_profiles p ON p.user_id = m.user_id
		WHERE m.trip_id = $1
		ORDER BY m.joined_at ASC
	`, tripID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		var tags pq.StringArray
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName, &m.Role, &tags); err != nil {
			return nil, err
		}
		m.Tags = []string(tags)
		out = append(out, m)
	}
	return out, rows.Err()
}
