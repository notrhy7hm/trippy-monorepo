package planning

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/api"
)

// ---- list -----------------------------------------------------------------

func (h *Handler) ListItinerary(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	items, err := h.svc.ListItineraryItems(r.Context(), api.UserID(r.Context()), slug)
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []ItineraryItem{}
	}
	api.JSON(w, http.StatusOK, items)
}

// ---- create ---------------------------------------------------------------

type createItineraryReq struct {
	// dayIndex is required, so use a pointer to distinguish "absent"
	// from a legitimate 0.
	DayIndex     *int   `json:"dayIndex"`
	Date         string `json:"date"`
	Title        string `json:"title"`
	Notes        string `json:"notes"`
	LocationName string `json:"locationName"`
	StartsAt     string `json:"startsAt"`
	EndsAt       string `json:"endsAt"`
}

func (h *Handler) CreateItinerary(w http.ResponseWriter, r *http.Request) {
	var in createItineraryReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if in.DayIndex == nil {
		writeError(w, ErrInvalidDayIndex)
		return
	}

	create := ItineraryCreateInput{
		DayIndex: *in.DayIndex,
		Title:    in.Title,
		Notes:    in.Notes,
	}
	if loc := strings.TrimSpace(in.LocationName); loc != "" {
		create.LocationName = &loc
	}
	if in.Date != "" {
		d, err := parseDate(in.Date)
		if err != nil {
			writeError(w, ErrInvalidDate)
			return
		}
		create.Date = &d
	}
	if in.StartsAt != "" {
		t, err := parseTimeOfDay(in.StartsAt)
		if err != nil {
			writeError(w, ErrInvalidTime)
			return
		}
		create.StartsAt = &t
	}
	if in.EndsAt != "" {
		t, err := parseTimeOfDay(in.EndsAt)
		if err != nil {
			writeError(w, ErrInvalidTime)
			return
		}
		create.EndsAt = &t
	}

	slug := chi.URLParam(r, "tripSlug")
	item, err := h.svc.CreateItineraryItem(r.Context(), api.UserID(r.Context()), slug, create)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusCreated, item)
}

// ---- update ---------------------------------------------------------------

// updateItineraryReq uses pointers so the handler can distinguish field-
// omitted from field-present. For Date / LocationName / StartsAt / EndsAt
// the empty-string sentinel clears the column; service translates this
// into the SetX + nil-pointer pair that the repo writes as SQL NULL.
type updateItineraryReq struct {
	DayIndex     *int    `json:"dayIndex,omitempty"`
	Date         *string `json:"date,omitempty"`
	Title        *string `json:"title,omitempty"`
	Notes        *string `json:"notes,omitempty"`
	LocationName *string `json:"locationName,omitempty"`
	StartsAt     *string `json:"startsAt,omitempty"`
	EndsAt       *string `json:"endsAt,omitempty"`
	Position     *int    `json:"position,omitempty"`
}

func (h *Handler) UpdateItinerary(w http.ResponseWriter, r *http.Request) {
	var in updateItineraryReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		api.Err(w, http.StatusNotFound, "itinerary_item_not_found", "itinerary item not found")
		return
	}

	upd := ItineraryUpdateInput{
		DayIndex: in.DayIndex,
		Title:    in.Title,
		Notes:    in.Notes,
		Position: in.Position,
	}

	if in.Date != nil {
		upd.SetDate = true
		if strings.TrimSpace(*in.Date) != "" {
			d, err := parseDate(*in.Date)
			if err != nil {
				writeError(w, ErrInvalidDate)
				return
			}
			upd.Date = &d
		}
	}
	if in.LocationName != nil {
		upd.SetLocationName = true
		if loc := strings.TrimSpace(*in.LocationName); loc != "" {
			upd.LocationName = &loc
		}
	}
	if in.StartsAt != nil {
		upd.SetStartsAt = true
		if strings.TrimSpace(*in.StartsAt) != "" {
			t, err := parseTimeOfDay(*in.StartsAt)
			if err != nil {
				writeError(w, ErrInvalidTime)
				return
			}
			upd.StartsAt = &t
		}
	}
	if in.EndsAt != nil {
		upd.SetEndsAt = true
		if strings.TrimSpace(*in.EndsAt) != "" {
			t, err := parseTimeOfDay(*in.EndsAt)
			if err != nil {
				writeError(w, ErrInvalidTime)
				return
			}
			upd.EndsAt = &t
		}
	}

	item, err := h.svc.UpdateItineraryItem(r.Context(), api.UserID(r.Context()), slug, itemID, upd)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, item)
}

// ---- delete ---------------------------------------------------------------

func (h *Handler) DeleteItinerary(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		api.Err(w, http.StatusNotFound, "itinerary_item_not_found", "itinerary item not found")
		return
	}
	if err := h.svc.DeleteItineraryItem(r.Context(), api.UserID(r.Context()), slug, itemID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers --------------------------------------------------------------

// parseTimeOfDay accepts "HH:MM" or "HH:MM:SS" and returns the value
// normalized to "HH:MM:SS" so storage / comparisons / API output are
// uniform.
func parseTimeOfDay(s string) (string, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("15:04:05", s); err == nil {
		return t.Format("15:04:05"), nil
	}
	if t, err := time.Parse("15:04", s); err == nil {
		return t.Format("15:04:05"), nil
	}
	return "", ErrInvalidTime
}
