package planning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var ErrItineraryItemNotFound = errors.New("itinerary item not found")

type ItineraryRepo struct{ db *sqlx.DB }

func NewItineraryRepo(db *sqlx.DB) *ItineraryRepo { return &ItineraryRepo{db: db} }

// itineraryRow is the joined shape every read produces. date / starts_at /
// ends_at are read via ::text so the repo never has to convert through a
// time.Time (which would otherwise carry an unhelpful midnight-UTC date
// for the time-only columns and risk timezone shifts for the date column).
type itineraryRow struct {
	ID                   uuid.UUID      `db:"id"`
	DayIndex             int            `db:"day_index"`
	Date                 sql.NullString `db:"date"`
	Title                string         `db:"title"`
	Notes                string         `db:"notes"`
	LocationName         sql.NullString `db:"location_name"`
	StartsAt             sql.NullString `db:"starts_at"`
	EndsAt               sql.NullString `db:"ends_at"`
	Position             int            `db:"position"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
	CreatedByUsername    string         `db:"created_by_username"`
	CreatedByDisplayName string         `db:"created_by_display_name"`
}

const itinerarySelectColumns = `
	i.id, i.day_index,
	i.date::text     AS date,
	i.title, i.notes, i.location_name,
	i.starts_at::text AS starts_at,
	i.ends_at::text   AS ends_at,
	i.position, i.created_at, i.updated_at,
	cb.username                              AS created_by_username,
	COALESCE(cbp.display_name, cb.username)  AS created_by_display_name
`

const itineraryJoins = `
	FROM trip_itinerary_items i
	JOIN      users cb           ON cb.id = i.created_by_user_id
	LEFT JOIN user_profiles cbp  ON cbp.user_id = cb.id
`

// itineraryListOrder pins the order required by the spec:
//
//	day_index asc -> position asc -> starts_at asc nulls last
//	-> created_at asc -> id asc
const itineraryListOrder = `
	ORDER BY
		i.day_index ASC,
		i.position  ASC,
		i.starts_at ASC NULLS LAST,
		i.created_at ASC,
		i.id ASC
`

func rowToItineraryItem(r itineraryRow) ItineraryItem {
	out := ItineraryItem{
		ID:        r.ID,
		DayIndex:  r.DayIndex,
		Title:     r.Title,
		Notes:     r.Notes,
		Position:  r.Position,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		CreatedBy: Party{
			Username:    r.CreatedByUsername,
			DisplayName: r.CreatedByDisplayName,
		},
	}
	if r.Date.Valid {
		s := r.Date.String
		out.Date = &s
	}
	if r.LocationName.Valid && r.LocationName.String != "" {
		s := r.LocationName.String
		out.LocationName = &s
	}
	if r.StartsAt.Valid {
		s := r.StartsAt.String
		out.StartsAt = &s
	}
	if r.EndsAt.Valid {
		s := r.EndsAt.String
		out.EndsAt = &s
	}
	return out
}

func rowsToItineraryItems(rows []itineraryRow) []ItineraryItem {
	out := make([]ItineraryItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToItineraryItem(r))
	}
	return out
}

// ListItineraryItems returns every item on the trip in spec order.
func (r *ItineraryRepo) ListItineraryItems(ctx context.Context, tripID uuid.UUID) ([]ItineraryItem, error) {
	var rows []itineraryRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT `+itinerarySelectColumns+itineraryJoins+`
		WHERE i.trip_id = $1
	`+itineraryListOrder, tripID)
	if err != nil {
		return nil, err
	}
	return rowsToItineraryItems(rows), nil
}

// ItineraryItemByIDForTrip scopes a lookup to its trip; cross-trip ids
// surface as ErrItineraryItemNotFound rather than the wrong row.
func (r *ItineraryRepo) ItineraryItemByIDForTrip(ctx context.Context, tripID, itemID uuid.UUID) (ItineraryItem, error) {
	var row itineraryRow
	err := r.db.GetContext(ctx, &row, `
		SELECT `+itinerarySelectColumns+itineraryJoins+`
		WHERE i.trip_id = $1 AND i.id = $2
	`, tripID, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return ItineraryItem{}, ErrItineraryItemNotFound
	}
	if err != nil {
		return ItineraryItem{}, err
	}
	return rowToItineraryItem(row), nil
}

// ---------------------------------------------------------------------------
// per-bucket position helpers (advisory lock + MAX)
// ---------------------------------------------------------------------------

// lockItineraryBucketSQL takes a transaction-scoped advisory lock keyed by
// (trip_id, day_index). hashtextextended is stable across connections, so
// the same bucket maps to the same lock key everywhere. Released
// automatically on commit/rollback. Different buckets do not contend.
const lockItineraryBucketSQL = `
	SELECT pg_advisory_xact_lock(
		hashtextextended('trip_itinerary_pos:' || $1::text || ':' || $2::text, 0)
	)
`

func lockItineraryPositionBucket(ctx context.Context, tx *sqlx.Tx, tripID uuid.UUID, dayIndex int) error {
	_, err := tx.ExecContext(ctx, lockItineraryBucketSQL, tripID, dayIndex)
	return err
}

func nextItineraryPosition(ctx context.Context, tx *sqlx.Tx, tripID uuid.UUID, dayIndex int) (int, error) {
	var maxPos sql.NullInt64
	if err := tx.GetContext(ctx, &maxPos, `
		SELECT MAX(position) FROM trip_itinerary_items
		WHERE trip_id = $1 AND day_index = $2
	`, tripID, dayIndex); err != nil {
		return 0, err
	}
	if maxPos.Valid {
		return int(maxPos.Int64) + 1000, nil
	}
	return 1000, nil
}

// ---------------------------------------------------------------------------
// CreateItineraryItem — same race-safety pattern as CreateTask.
// ---------------------------------------------------------------------------

// CreateItineraryItem inserts a new itinerary entry. Position is computed
// inside a transaction-scoped advisory lock on the (trip, day_index)
// bucket so two parallel POSTs in the same bucket cannot tie at the same
// position.
func (r *ItineraryRepo) CreateItineraryItem(ctx context.Context, tripID uuid.UUID, in ItineraryCreateInput) (ItineraryItem, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return ItineraryItem{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockItineraryPositionBucket(ctx, tx, tripID, in.DayIndex); err != nil {
		return ItineraryItem{}, err
	}
	nextPos, err := nextItineraryPosition(ctx, tx, tripID, in.DayIndex)
	if err != nil {
		return ItineraryItem{}, err
	}

	var id uuid.UUID
	if err := tx.QueryRowxContext(ctx, `
		INSERT INTO trip_itinerary_items
			(trip_id, day_index, date, title, notes, location_name,
			 starts_at, ends_at, position, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`,
		tripID, in.DayIndex, in.Date, in.Title, in.Notes,
		in.LocationName, in.StartsAt, in.EndsAt, nextPos, in.CreatedByUserID,
	).Scan(&id); err != nil {
		return ItineraryItem{}, err
	}

	var row itineraryRow
	if err := tx.GetContext(ctx, &row, `
		SELECT `+itinerarySelectColumns+itineraryJoins+`
		WHERE i.trip_id = $1 AND i.id = $2
	`, tripID, id); err != nil {
		return ItineraryItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return ItineraryItem{}, err
	}
	return rowToItineraryItem(row), nil
}

// ---------------------------------------------------------------------------
// UpdateItineraryItem — single transaction with row lock + optional rebucket.
// ---------------------------------------------------------------------------

// UpdateItineraryItem applies a partial update inside a single transaction.
//
//  1. SELECT FOR UPDATE locks the item row, reading its current day_index.
//     Missing row -> ErrItineraryItemNotFound.
//  2. If the request changes day_index AND does not include an explicit
//     position, the item is rebucketed: a per-bucket advisory lock is
//     taken on the new (trip, day_index) bucket and position is set to
//     max(position) + 1000 within that bucket. The same lock that
//     serializes CreateItineraryItem serializes rebucketing.
//  3. Dynamic UPDATE applies all set fields; updated_at = now() is
//     always written.
//  4. Joined item is re-read inside the tx and returned.
func (r *ItineraryRepo) UpdateItineraryItem(ctx context.Context, tripID, itemID uuid.UUID, in ItineraryUpdateInput) (ItineraryItem, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return ItineraryItem{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentDayIndex int
	err = tx.QueryRowxContext(ctx, `
		SELECT day_index FROM trip_itinerary_items
		WHERE trip_id = $1 AND id = $2
		FOR UPDATE
	`, tripID, itemID).Scan(&currentDayIndex)
	if errors.Is(err, sql.ErrNoRows) {
		return ItineraryItem{}, ErrItineraryItemNotFound
	}
	if err != nil {
		return ItineraryItem{}, err
	}

	var rebucketedPos *int
	if in.DayIndex != nil && *in.DayIndex != currentDayIndex && in.Position == nil {
		if err := lockItineraryPositionBucket(ctx, tx, tripID, *in.DayIndex); err != nil {
			return ItineraryItem{}, err
		}
		np, err := nextItineraryPosition(ctx, tx, tripID, *in.DayIndex)
		if err != nil {
			return ItineraryItem{}, err
		}
		rebucketedPos = &np
	}

	parts := []string{"updated_at = now()"}
	args := []any{tripID, itemID}
	next := 3
	add := func(col string, val any) {
		parts = append(parts, fmt.Sprintf("%s = $%d", col, next))
		args = append(args, val)
		next++
	}

	if in.DayIndex != nil {
		add("day_index", *in.DayIndex)
	}
	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.Notes != nil {
		add("notes", *in.Notes)
	}
	switch {
	case in.Position != nil:
		add("position", *in.Position)
	case rebucketedPos != nil:
		add("position", *rebucketedPos)
	}
	if in.SetDate {
		add("date", in.Date) // *time.Time; nil -> NULL
	}
	if in.SetLocationName {
		add("location_name", in.LocationName) // *string; nil -> NULL
	}
	if in.SetStartsAt {
		add("starts_at", in.StartsAt)
	}
	if in.SetEndsAt {
		add("ends_at", in.EndsAt)
	}

	query := fmt.Sprintf(`
		UPDATE trip_itinerary_items SET %s
		WHERE trip_id = $1 AND id = $2
	`, strings.Join(parts, ", "))

	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return ItineraryItem{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ItineraryItem{}, ErrItineraryItemNotFound
	}

	var row itineraryRow
	if err := tx.GetContext(ctx, &row, `
		SELECT `+itinerarySelectColumns+itineraryJoins+`
		WHERE i.trip_id = $1 AND i.id = $2
	`, tripID, itemID); err != nil {
		return ItineraryItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return ItineraryItem{}, err
	}
	return rowToItineraryItem(row), nil
}

// DeleteItineraryItem hard-deletes a single item scoped to its trip.
func (r *ItineraryRepo) DeleteItineraryItem(ctx context.Context, tripID, itemID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM trip_itinerary_items WHERE trip_id = $1 AND id = $2
	`, tripID, itemID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrItineraryItemNotFound
	}
	return nil
}
