package friends

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var (
	ErrAlreadyFriends  = errors.New("already friends")
	ErrAlreadyPending  = errors.New("request already pending")
	ErrRequestNotFound = errors.New("friend request not found")
	ErrNotFriends      = errors.New("not friends")
	ErrSelf            = errors.New("cannot friend yourself")
)

type Repo struct{ db *sqlx.DB }

func NewRepo(db *sqlx.DB) *Repo { return &Repo{db: db} }

// ---------------------------------------------------------------------------
// friendships
// ---------------------------------------------------------------------------

func (r *Repo) AreFriends(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1 FROM friendships
			WHERE user_a = LEAST($1::uuid, $2::uuid)
			  AND user_b = GREATEST($1::uuid, $2::uuid)
		)
	`, a, b)
	return exists, err
}

func (r *Repo) DeleteFriendship(ctx context.Context, a, b uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM friendships
		WHERE user_a = LEAST($1::uuid, $2::uuid)
		  AND user_b = GREATEST($1::uuid, $2::uuid)
	`, a, b)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFriends
	}
	return nil
}

func (r *Repo) ListFriends(ctx context.Context, userID uuid.UUID) ([]Friend, error) {
	var rows []Friend
	err := r.db.SelectContext(ctx, &rows, `
		SELECT u.username,
		       COALESCE(p.display_name, u.username) AS display_name,
		       f.created_at AS since
		FROM friendships f
		JOIN users u ON u.id = (CASE WHEN f.user_a = $1 THEN f.user_b ELSE f.user_a END)
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE (f.user_a = $1 OR f.user_b = $1)
		  AND u.deleted_at IS NULL
		ORDER BY u.username ASC
	`, userID)
	return rows, err
}

// ---------------------------------------------------------------------------
// friend_requests
// ---------------------------------------------------------------------------

func (r *Repo) PendingExists(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1 FROM friend_requests
			WHERE status = 'pending'
			  AND ((from_user_id = $1 AND to_user_id = $2)
			    OR (from_user_id = $2 AND to_user_id = $1))
		)
	`, a, b)
	return exists, err
}

// CreateRequest inserts a pending request and returns its created_at.
// Translates pg unique/check violations into typed errors so the service
// does not have to inspect pg codes.
func (r *Repo) CreateRequest(ctx context.Context, fromID, toID uuid.UUID, message string) (time.Time, error) {
	var created time.Time
	err := r.db.GetContext(ctx, &created, `
		INSERT INTO friend_requests (from_user_id, to_user_id, message)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`, fromID, toID, message)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return time.Time{}, ErrAlreadyPending
			case "23514":
				return time.Time{}, ErrSelf
			}
		}
		return time.Time{}, err
	}
	return created, nil
}

// AcceptRequest flips the pending request to accepted and inserts the
// canonical friendship row in a single transaction. Returns the friendship's
// created_at as `since`. Returns ErrRequestNotFound if no pending request
// from fromID to toID exists.
func (r *Repo) AcceptRequest(ctx context.Context, fromID, toID uuid.UUID) (time.Time, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE friend_requests
		   SET status='accepted', responded_at=now()
		 WHERE from_user_id = $1 AND to_user_id = $2 AND status='pending'
	`, fromID, toID)
	if err != nil {
		return time.Time{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return time.Time{}, ErrRequestNotFound
	}

	var since time.Time
	err = tx.GetContext(ctx, &since, `
		INSERT INTO friendships (user_a, user_b)
		VALUES (LEAST($1::uuid, $2::uuid), GREATEST($1::uuid, $2::uuid))
		ON CONFLICT (user_a, user_b) DO UPDATE SET created_at = friendships.created_at
		RETURNING created_at
	`, fromID, toID)
	if err != nil {
		return time.Time{}, err
	}

	if err := tx.Commit(); err != nil {
		return time.Time{}, err
	}
	return since, nil
}

func (r *Repo) DeclineRequest(ctx context.Context, fromID, toID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE friend_requests
		   SET status='declined', responded_at=now()
		 WHERE from_user_id = $1 AND to_user_id = $2 AND status='pending'
	`, fromID, toID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrRequestNotFound
	}
	return nil
}

func (r *Repo) CancelRequest(ctx context.Context, fromID, toID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE friend_requests
		   SET status='canceled', responded_at=now()
		 WHERE from_user_id = $1 AND to_user_id = $2 AND status='pending'
	`, fromID, toID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrRequestNotFound
	}
	return nil
}

func (r *Repo) ListIncomingRequests(ctx context.Context, userID uuid.UUID) ([]Request, error) {
	var rows []Request
	err := r.db.SelectContext(ctx, &rows, `
		SELECT u.username,
		       COALESCE(p.display_name, u.username) AS display_name,
		       r.message,
		       r.created_at
		FROM friend_requests r
		JOIN users u ON u.id = r.from_user_id
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE r.to_user_id = $1 AND r.status = 'pending'
		  AND u.deleted_at IS NULL
		ORDER BY r.created_at DESC
	`, userID)
	return rows, err
}

func (r *Repo) ListOutgoingRequests(ctx context.Context, userID uuid.UUID) ([]Request, error) {
	var rows []Request
	err := r.db.SelectContext(ctx, &rows, `
		SELECT u.username,
		       COALESCE(p.display_name, u.username) AS display_name,
		       r.message,
		       r.created_at
		FROM friend_requests r
		JOIN users u ON u.id = r.to_user_id
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE r.from_user_id = $1 AND r.status = 'pending'
		  AND u.deleted_at IS NULL
		ORDER BY r.created_at DESC
	`, userID)
	return rows, err
}

// ---------------------------------------------------------------------------
// relations (used to enrich /users/search results)
// ---------------------------------------------------------------------------

// RelationsFor returns the viewer's relation to each target user. Returns a
// map keyed by target UUID. Targets the viewer has no link to map to "none".
func (r *Repo) RelationsFor(ctx context.Context, viewerID uuid.UUID, targetIDs []uuid.UUID) (map[uuid.UUID]Relation, error) {
	if len(targetIDs) == 0 {
		return map[uuid.UUID]Relation{}, nil
	}
	// lib/pq cannot encode []uuid.UUID directly; pass strings and cast.
	strs := make([]string, len(targetIDs))
	for i, id := range targetIDs {
		strs[i] = id.String()
	}

	rows, err := r.db.QueryxContext(ctx, `
		WITH t AS (
			SELECT u::uuid AS id FROM unnest($2::text[]) AS u
		)
		SELECT t.id,
		       CASE
		           WHEN t.id = $1::uuid THEN 'self'
		           WHEN EXISTS (
		               SELECT 1 FROM friendships f
		                WHERE f.user_a = LEAST($1::uuid, t.id)
		                  AND f.user_b = GREATEST($1::uuid, t.id)
		           ) THEN 'friend'
		           WHEN EXISTS (
		               SELECT 1 FROM friend_requests r
		                WHERE r.status = 'pending'
		                  AND r.from_user_id = t.id
		                  AND r.to_user_id   = $1::uuid
		           ) THEN 'incoming_request'
		           WHEN EXISTS (
		               SELECT 1 FROM friend_requests r
		                WHERE r.status = 'pending'
		                  AND r.from_user_id = $1::uuid
		                  AND r.to_user_id   = t.id
		           ) THEN 'outgoing_request'
		           ELSE 'none'
		       END AS relation
		FROM t
	`, viewerID, pq.Array(strs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]Relation, len(targetIDs))
	for rows.Next() {
		var id uuid.UUID
		var rel string
		if err := rows.Scan(&id, &rel); err != nil {
			return nil, err
		}
		out[id] = Relation(rel)
	}
	return out, rows.Err()
}

