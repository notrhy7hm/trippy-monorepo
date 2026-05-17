package planning

import (
	"time"

	"github.com/google/uuid"
)

// ItineraryItem is the API-facing representation of one day-by-day
// itinerary entry. Date / startsAt / endsAt / locationName are pointers
// so they are simply omitted when null, instead of serialized as null.
//
// Dates and times are emitted as plain strings ("YYYY-MM-DD" and
// "HH:MM:SS") rather than time.Time so the JSON shape never picks up a
// timezone the column doesn't carry. Postgres date/time::text gives us
// those strings directly in the SELECT.
type ItineraryItem struct {
	ID           uuid.UUID `json:"id"`
	DayIndex     int       `json:"dayIndex"`
	Date         *string   `json:"date,omitempty"`
	Title        string    `json:"title"`
	Notes        string    `json:"notes"`
	LocationName *string   `json:"locationName,omitempty"`
	StartsAt     *string   `json:"startsAt,omitempty"`
	EndsAt       *string   `json:"endsAt,omitempty"`
	Position     int       `json:"position"`
	CreatedBy    Party     `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ItineraryCreateInput is the service-facing create payload. The handler
// parses date / time strings and forwards already-validated values.
//
// DayIndex is a *int (not int) so the service can distinguish "omitted"
// from a literal 0. When omitted, the service may derive the dayIndex
// from the trip's startsOn + the provided Date, and falls back to 0 when
// neither is available. The service mutates DayIndex to the resolved
// value before calling the repo, so repo callers can rely on it being
// non-nil.
type ItineraryCreateInput struct {
	DayIndex        *int
	Date            *time.Time
	Title           string
	Notes           string
	LocationName    *string // nil -> no location
	StartsAt        *string // HH:MM:SS or nil
	EndsAt          *string
	CreatedByUserID uuid.UUID
}

// ItineraryUpdateInput is the service-facing partial-update payload.
//
// Tri-state contract:
//
//	scalar pointer (Title/Notes/Position/DayIndex):
//	    nil      -> leave column alone
//	    non-nil  -> set to that value
//
//	nullable column gated by Set* boolean (Date/LocationName/StartsAt/EndsAt):
//	    !SetX                -> leave column alone
//	    SetX && X == nil     -> set to SQL NULL
//	    SetX && X != nil     -> set to *X
//
// The handler is responsible for filling Set* + value from the request
// JSON; the service forwards these unchanged to the repo after validating
// business rules.
type ItineraryUpdateInput struct {
	DayIndex *int
	Title    *string
	Notes    *string
	Position *int

	SetDate bool
	Date    *time.Time

	SetLocationName bool
	LocationName    *string

	SetStartsAt bool
	StartsAt    *string

	SetEndsAt bool
	EndsAt    *string
}
