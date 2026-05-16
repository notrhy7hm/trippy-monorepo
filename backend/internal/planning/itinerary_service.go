package planning

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidDayIndex  = errors.New("dayIndex must be a non-negative integer")
	ErrInvalidDate      = errors.New("invalid date")
	ErrInvalidTime      = errors.New("invalid time-of-day")
	ErrInvalidTimeRange = errors.New("startsAt must be earlier than or equal to endsAt")
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

// CreateItineraryItem creates a new itinerary entry. Title is required;
// dayIndex must be >= 0; if both startsAt and endsAt are present, the
// startsAt must not be later than endsAt.
func (s *Service) CreateItineraryItem(ctx context.Context, callerID uuid.UUID, slug string, in ItineraryCreateInput) (ItineraryItem, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return ItineraryItem{}, err
	}

	if in.DayIndex < 0 {
		return ItineraryItem{}, ErrInvalidDayIndex
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		return ItineraryItem{}, ErrInvalidTitle
	}

	if in.StartsAt != nil && in.EndsAt != nil && *in.StartsAt > *in.EndsAt {
		return ItineraryItem{}, ErrInvalidTimeRange
	}

	in.Title = title
	in.CreatedByUserID = callerID
	return s.itinerary.CreateItineraryItem(ctx, tripID, in)
}

// UpdateItineraryItem applies a partial update.
//
// Per-field shape validation (dayIndex >= 0, title trimmed non-empty,
// position >= 0) happens here before any DB work. The cross-field
// startsAt / endsAt range check intentionally lives inside
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

	return s.itinerary.UpdateItineraryItem(ctx, tripID, itemID, in)
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
