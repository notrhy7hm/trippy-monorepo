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

// UpdateItineraryItem applies a partial update. The current item is
// loaded first so the time-range check sees the *final* (post-merge)
// startsAt / endsAt — otherwise a request that only changes one of the
// two could end up storing an invalid range.
func (s *Service) UpdateItineraryItem(ctx context.Context, callerID uuid.UUID, slug string, itemID uuid.UUID, in ItineraryUpdateInput) (ItineraryItem, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return ItineraryItem{}, err
	}

	current, err := s.itinerary.ItineraryItemByIDForTrip(ctx, tripID, itemID)
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

	// Compute the final startsAt / endsAt after this update would apply
	// and reject impossible ranges. SetX gates take precedence over the
	// item's current value; nil after a Set means clear.
	finalStartsAt := current.StartsAt
	if in.SetStartsAt {
		finalStartsAt = in.StartsAt
	}
	finalEndsAt := current.EndsAt
	if in.SetEndsAt {
		finalEndsAt = in.EndsAt
	}
	if finalStartsAt != nil && finalEndsAt != nil && *finalStartsAt > *finalEndsAt {
		return ItineraryItem{}, ErrInvalidTimeRange
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
