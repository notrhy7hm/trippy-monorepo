package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func Err(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, ErrorBody{Error: msg, Code: code})
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.New("invalid json body")
	}
	return nil
}
