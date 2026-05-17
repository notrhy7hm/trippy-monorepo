package planning

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidDayIndex  = errors.New("dayIndex must be a non-negative integer")
	ErrInvalidDate      = errors.New("invalid date")
	ErrInvalidTime      = errors.New("invalid time-of-day")
	ErrInvalidTimeRange = errors.New("startsAt must be earlier than or equal to endsAt")
	ErrDateOutOfRange   = errors.New("date must fall within the trip's date range")
	ErrDayOutOfRange    = errors.New("day is outside the trip's day range")
	ErrDateDayMismatch  = errors.New("date and day must refer to the same trip day")
)

// ListItineraryItems returns every itinerary entry on the trip if the
// caller is a trip member.
func (s *Service) ListItineraryItems(ctx context.Context, callerID uuid.UUID, slug string) ([]ItineraryItem, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return nil, err
	}
	return s.itinerary.ListItineraryItems(ctx, tripID)
}

// CreateItineraryItem creates a new itinerary entry.
//
//   - Title is required (trimmed).
//   - DayIndex is optional; nil falls back to 0, or — when the trip has
//     a startsOn and the request includes Date — is derived from
//     date − startsOn so the date the user picked lands in its own day
//     bucket without needing a redundant Day value.
//   - StartsAt/EndsAt must not be inverted when both are present.
//   - Date / final DayIndex are validated against the trip's date range
//     when the trip has startsOn (and, for upper bounds, endsOn).
func (s *Service) CreateItineraryItem(ctx context.Context, callerID uuid.UUID, slug string, in ItineraryCreateInput) (ItineraryItem, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return ItineraryItem{}, err
	}

	startsOn, endsOn, err := s.trips.TripDateRange(ctx, tripID)
	if err != nil {
		return ItineraryItem{}, err
	}

	// Resolve final dayIndex. Explicit value wins; otherwise derive from
	// (Date, startsOn) if both available; otherwise default to 0.
	var finalDayIndex int
	switch {
	case in.DayIndex != nil:
		finalDayIndex = *in.DayIndex
	case in.Date != nil && startsOn != nil:
		finalDayIndex = deriveDayFromDate(*in.Date, *startsOn)
	default:
		finalDayIndex = 0
	}
	if finalDayIndex < 0 {
		return ItineraryItem{}, ErrInvalidDayIndex
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		return ItineraryItem{}, ErrInvalidTitle
	}

	if in.StartsAt != nil && in.EndsAt != nil && *in.StartsAt > *in.EndsAt {
		return ItineraryItem{}, ErrInvalidTimeRange
	}

	if err := validateDateDayAgainstTrip(in.Date, finalDayIndex, startsOn, endsOn); err != nil {
		return ItineraryItem{}, err
	}

	in.Title = title
	in.CreatedByUserID = callerID
	in.DayIndex = &finalDayIndex
	return s.itinerary.CreateItineraryItem(ctx, tripID, in)
}

// UpdateItineraryItem applies a partial update.
//
// Per-field shape validation (dayIndex >= 0, title trimmed non-empty,
// position >= 0) happens here before any DB work. Cross-field checks
// that depend on the current stored row (startsAt vs endsAt; date vs
// dayIndex; date / day vs trip range) intentionally live inside
// repo.UpdateItineraryItem under SELECT ... FOR UPDATE so concurrent
// partial PATCHes cannot both pass a stale-row precheck and then commit
// an invalid combined state. The repo also handles the missing-item
// case (ErrItineraryItemNotFound).
func (s *Service) UpdateItineraryItem(ctx context.Context, callerID uuid.UUID, slug string, itemID uuid.UUID, in ItineraryUpdateInput) (ItineraryItem, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return ItineraryItem{}, err
	}

	if in.DayIndex != nil && *in.DayIndex < 0 {
		return ItineraryItem{}, ErrInvalidDayIndex
	}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return ItineraryItem{}, ErrInvalidTitle
		}
		in.Title = &t
	}

	if in.Position != nil && *in.Position < 0 {
		return ItineraryItem{}, ErrInvalidPosition
	}

	// Trip date bounds threaded into the repo so the post-merge
	// date/day check happens under the same row lock as the rest of
	// the cross-field validation.
	startsOn, endsOn, err := s.trips.TripDateRange(ctx, tripID)
	if err != nil {
		return ItineraryItem{}, err
	}

	return s.itinerary.UpdateItineraryItem(ctx, tripID, itemID, startsOn, endsOn, in)
}

// ---------------------------------------------------------------------------
// shared date/day validation helpers (used by both create and the repo's
// in-tx update path).
// ---------------------------------------------------------------------------

// toUTCDate normalizes a time.Time to midnight UTC for date-only
// comparisons. Postgres date columns already arrive at midnight UTC via
// pgx, but going through this helper keeps comparisons safe even if the
// origin time picks up a timezone.
func toUTCDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// daysBetween returns a − b in whole days, both at midnight UTC.
func daysBetween(a, b time.Time) int {
	return int(toUTCDate(a).Sub(toUTCDate(b)) / (24 * time.Hour))
}

// deriveDayFromDate returns the 0-based day offset between date and
// start. Negative offsets are clamped to 0; out-of-range date is
// surfaced separately by validateDateDayAgainstTrip.
func deriveDayFromDate(date, start time.Time) int {
	d := daysBetween(date, start)
	if d < 0 {
		return 0
	}
	return d
}

// validateDateDayAgainstTrip checks that the (date, dayIndex) pair is
// consistent with the trip's date range. Returns nil when consistent or
// when no startsOn constraint applies. date may be nil. dayIndex is
// always known (final post-merge value for updates, resolved value for
// creates).
func validateDateDayAgainstTrip(
	date *time.Time,
	dayIndex int,
	tripStartsOn, tripEndsOn *time.Time,
) error {
	if tripStartsOn == nil {
		// Trip in relative-day mode; dayIndex is unbounded, date is
		// unconstrained.
		return nil
	}
	start := toUTCDate(*tripStartsOn)

	if date != nil {
		provided := toUTCDate(*date)
		if provided.Before(start) {
			return ErrDateOutOfRange
		}
		if tripEndsOn != nil {
			end := toUTCDate(*tripEndsOn)
			if provided.After(end) {
				return ErrDateOutOfRange
			}
		}
		expected := daysBetween(provided, start)
		if dayIndex != expected {
			return ErrDateDayMismatch
		}
	}

	if tripEndsOn != nil {
		end := toUTCDate(*tripEndsOn)
		tripDays := daysBetween(end, start) + 1
		if dayIndex >= tripDays {
			return ErrDayOutOfRange
		}
	}

	return nil
}

// DeleteItineraryItem hard-deletes an item. Any trip member can call
// this in M2.
func (s *Service) DeleteItineraryItem(ctx context.Context, callerID uuid.UUID, slug string, itemID uuid.UUID) error {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return err
	}
	return s.itinerary.DeleteItineraryItem(ctx, tripID, itemID)
}
