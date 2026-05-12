package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound   = errors.New("user not found")
	ErrUserExists = errors.New("user already exists")
)

type Repo struct{ db *sqlx.DB }

func NewRepo(db *sqlx.DB) *Repo { return &Repo{db: db} }

const userColumns = `u.id, u.username, u.email,
	COALESCE(p.display_name, '') AS display_name,
	COALESCE(p.bio, '') AS bio,
	COALESCE(p.avatar_url, '') AS avatar_url,
	u.created_at`

func (r *Repo) Create(ctx context.Context, in CreateInput) (User, error) {
	var u User
	err := r.db.GetContext(ctx, &u, `
		WITH new_user AS (
			INSERT INTO users (email, username, password_hash)
			VALUES ($1, $2, $3)
			RETURNING id, email, username, created_at
		), new_profile AS (
			INSERT INTO user_profiles (user_id, display_name)
			SELECT id, username FROM new_user
			RETURNING user_id, display_name, bio, avatar_url
		)
		SELECT u.id, u.username, u.email,
			p.display_name, COALESCE(p.bio, '') AS bio, COALESCE(p.avatar_url, '') AS avatar_url,
			u.created_at
		FROM new_user u
		JOIN new_profile p ON p.user_id = u.id
	`, in.Email, in.Username, in.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrUserExists
		}
		return User{}, err
	}
	return u, nil
}

func (r *Repo) ByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := r.db.GetContext(ctx, &u, `
		SELECT `+userColumns+`
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *Repo) ByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := r.db.GetContext(ctx, &u, `
		SELECT `+userColumns+`
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.username = $1 AND u.deleted_at IS NULL
	`, username)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// FindForLogin accepts either email or username and returns user + password hash.
func (r *Repo) FindForLogin(ctx context.Context, identifier string) (User, string, error) {
	row := r.db.QueryRowxContext(ctx, `
		SELECT `+userColumns+`, u.password_hash
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE (u.email = $1 OR u.username = $1) AND u.deleted_at IS NULL
	`, identifier)
	var u User
	var hash string
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.Bio, &u.AvatarURL, &u.CreatedAt, &hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, "", ErrNotFound
		}
		return User{}, "", err
	}
	return u, hash, nil
}

func (r *Repo) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateInput) (User, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_profiles SET
			display_name = COALESCE($2, display_name),
			bio          = COALESCE($3, bio),
			avatar_url   = COALESCE($4, avatar_url),
			updated_at   = now()
		WHERE user_id = $1
	`, id, in.DisplayName, in.Bio, in.AvatarURL)
	if err != nil {
		return User{}, err
	}
	return r.ByID(ctx, id)
}
