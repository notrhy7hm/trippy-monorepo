package trips

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var (
	ErrNotFound        = errors.New("trip not found")
	ErrSlugCollision   = errors.New("slug already exists")
	ErrInviteExists    = errors.New("a pending invite already exists")
	ErrInviteNotFound  = errors.New("trip invite not found")
	ErrInviteNotPending = errors.New("trip invite is no longer pending")
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

// ---------------------------------------------------------------------------
// trip_invites
// ---------------------------------------------------------------------------

// inviteRow is the joined shape used by all invite read queries: enough
// context to build an Invite response without further round-trips.
type inviteRow struct {
	Token              string         `db:"token"`
	Status             string         `db:"status"`
	Role               string         `db:"role"`
	InviteeUserID      *uuid.UUID     `db:"invitee_user_id"`
	InviteeEmail       *string        `db:"invitee_email"`
	InviteeUsername    sql.NullString `db:"invitee_username"`
	InviteeDisplayName sql.NullString `db:"invitee_display_name"`
	ByUsername         sql.NullString `db:"by_username"`
	ByDisplayName      sql.NullString `db:"by_display_name"`
	TripSlug           string         `db:"trip_slug"`
	TripTitle          string         `db:"trip_title"`
	ExpiresAt          time.Time      `db:"expires_at"`
	CreatedAt          time.Time      `db:"created_at"`
}

const inviteSelectColumns = `
	i.token, i.status, i.role,
	i.invitee_user_id, i.invitee_email,
	iu.username                                       AS invitee_username,
	COALESCE(ip.display_name, iu.username)            AS invitee_display_name,
	bu.username                                       AS by_username,
	COALESCE(bp.display_name, bu.username)            AS by_display_name,
	t.slug                                            AS trip_slug,
	t.title                                           AS trip_title,
	i.expires_at, i.created_at
`

const inviteJoins = `
	FROM trip_invites i
	JOIN trips t            ON t.id = i.trip_id AND t.deleted_at IS NULL
	LEFT JOIN users iu      ON iu.id = i.invitee_user_id
	LEFT JOIN user_profiles ip ON ip.user_id = iu.id
	LEFT JOIN users bu      ON bu.id = i.invited_by_user_id
	LEFT JOIN user_profiles bp ON bp.user_id = bu.id
`

// CreateInvite inserts a pending invite. Pg 23505 (partial unique idx on
// pending (trip, user) / (trip, email)) is translated to ErrInviteExists.
func (r *Repo) CreateInvite(ctx context.Context, in inviteInsertRow) (inviteRow, error) {
	// Insert, then read back the joined row so the caller has the full
	// shape ready for serialization.
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO trip_invites
			(trip_id, invitee_user_id, invitee_email, token, status,
			 expires_at, invited_by_user_id, role)
		VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7)
	`, in.TripID, in.InviteeUserID, in.InviteeEmail, in.Token,
		in.ExpiresAt, in.InvitedByUserID, in.Role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return inviteRow{}, ErrInviteExists
		}
		return inviteRow{}, err
	}
	return r.InviteByToken(ctx, in.Token)
}

// InviteByToken returns a single invite by token, in the joined row shape.
func (r *Repo) InviteByToken(ctx context.Context, token string) (inviteRow, error) {
	var row inviteRow
	err := r.db.GetContext(ctx, &row, `
		SELECT `+inviteSelectColumns+inviteJoins+`
		WHERE i.token = $1
	`, token)
	if errors.Is(err, sql.ErrNoRows) {
		return inviteRow{}, ErrInviteNotFound
	}
	return row, err
}

// ListTripInvites returns pending, unexpired invites for a trip.
func (r *Repo) ListTripInvites(ctx context.Context, tripID uuid.UUID) ([]inviteRow, error) {
	var rows []inviteRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT `+inviteSelectColumns+inviteJoins+`
		WHERE i.trip_id = $1
		  AND i.status = 'pending'
		  AND i.expires_at > now()
		ORDER BY i.created_at DESC
	`, tripID)
	return rows, err
}

// ListPendingForUser returns the viewer's pending invites — matched either
// by invitee_user_id or by lowercase email.
func (r *Repo) ListPendingForUser(ctx context.Context, userID uuid.UUID, email string) ([]inviteRow, error) {
	var rows []inviteRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT `+inviteSelectColumns+inviteJoins+`
		WHERE i.status = 'pending'
		  AND i.expires_at > now()
		  AND (i.invitee_user_id = $1
		       OR lower(i.invitee_email) = lower($2))
		ORDER BY i.created_at DESC
	`, userID, email)
	return rows, err
}

// RevokeInvite flips a pending invite to revoked.
func (r *Repo) RevokeInvite(ctx context.Context, tripID uuid.UUID, token string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE trip_invites
		   SET status='revoked', responded_at=now()
		 WHERE trip_id = $1 AND token = $2 AND status='pending'
	`, tripID, token)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInviteNotFound
	}
	return nil
}

// MarkExpiredByToken lazily expires a single pending row past its TTL.
// It's safe to call before any read/accept/decline; rows in other states
// are left alone.
func (r *Repo) MarkExpiredByToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE trip_invites
		   SET status='expired', responded_at=now()
		 WHERE token = $1 AND status='pending' AND expires_at < now()
	`, token)
	return err
}

// DeclineInvite flips a pending invite to declined.
func (r *Repo) DeclineInvite(ctx context.Context, token string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE trip_invites
		   SET status='declined', responded_at=now()
		 WHERE token = $1 AND status='pending'
	`, token)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInviteNotPending
	}
	return nil
}

// AcceptInvite atomically flips a pending invite to accepted and inserts
// the membership row using the invite's stored role. Returns the joined
// trip so the caller can redirect / show confirmation.
func (r *Repo) AcceptInvite(ctx context.Context, token string, userID uuid.UUID) (Trip, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Trip{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var tripID uuid.UUID
	var role Role
	err = tx.QueryRowxContext(ctx, `
		UPDATE trip_invites
		   SET status='accepted', responded_at=now()
		 WHERE token = $1 AND status='pending'
		 RETURNING trip_id, role
	`, token).Scan(&tripID, &role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Trip{}, ErrInviteNotPending
		}
		return Trip{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO trip_members (trip_id, user_id, role, tags)
		VALUES ($1, $2, $3, ARRAY[]::text[])
		ON CONFLICT (trip_id, user_id) DO NOTHING
	`, tripID, userID, role); err != nil {
		return Trip{}, err
	}

	var trip Trip
	err = tx.GetContext(ctx, &trip, `
		SELECT `+tripColumns+`
		FROM trips
		WHERE id = $1 AND deleted_at IS NULL
	`, tripID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Trip{}, ErrNotFound
		}
		return Trip{}, err
	}

	if err := tx.Commit(); err != nil {
		return Trip{}, err
	}
	return trip, nil
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
