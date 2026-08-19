// Package httpx holds the small conventions every API handler shares:
// JSON encoding, a single error shape, and request parsing helpers.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Error is the single error shape the API returns, so the frontend has exactly
// one thing to parse on a non-2xx response.
type Error struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func BadRequest(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "bad_request", Message: msg}
}

func Invalid(fields map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation_failed",
		Message: "Some fields need attention.", Fields: fields}
}

func Unauthorized(msg string) *Error {
	return &Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: msg}
}

func Forbidden(msg string) *Error {
	return &Error{Status: http.StatusForbidden, Code: "forbidden", Message: msg}
}

func NotFound(what string) *Error {
	return &Error{Status: http.StatusNotFound, Code: "not_found", Message: what + " not found."}
}

func Conflict(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: "conflict", Message: msg}
}

func Internal(err error) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: "internal",
		Message: "Something went wrong on our end."}
}

// Handler is a handler that may fail. Returning an error is the only way a
// handler reports a problem; Wrap turns it into a response.
type Handler func(http.ResponseWriter, *http.Request) error

func Wrap(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			slog.Error("unhandled handler error", "err", err, "path", r.URL.Path)
			apiErr = Internal(err)
		}
		if apiErr.Status >= 500 {
			slog.Error("server error", "err", err, "path", r.URL.Path)
		}
		JSON(w, apiErr.Status, apiErr)
	}
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encoding response failed", "err", err)
	}
}

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Decode reads a JSON body, rejecting unknown fields so a typo in the client
// surfaces as an error instead of being silently dropped.
func Decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return BadRequest("A request body is required.")
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return BadRequest("Could not parse the request body: " + err.Error())
	}
	return nil
}

// PathID reads a {name} path parameter as a positive integer id.
func PathID(r *http.Request, name string) (int64, error) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, BadRequest(fmt.Sprintf("%q is not a valid %s.", raw, name))
	}
	return id, nil
}

func Query(r *http.Request, name, def string) string {
	if v := strings.TrimSpace(r.URL.Query().Get(name)); v != "" {
		return v
	}
	return def
}

func QueryInt(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return v
	}
	return def
}

func QueryBool(r *http.Request, name string) bool {
	v, _ := strconv.ParseBool(r.URL.Query().Get(name))
	return v
}

// QueryDate parses a YYYY-MM-DD query parameter. Dates are the unit the whole
// rental domain works in, so they are always UTC midnight.
func QueryDate(r *http.Request, name string) (time.Time, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false, BadRequest(fmt.Sprintf("%s must be a YYYY-MM-DD date.", name))
	}
	return t, true, nil
}

// Date is a JSON-serialisable calendar date, rendered as YYYY-MM-DD rather than
// a full timestamp so the UI never has to strip a time component.
type Date time.Time

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(d).Format("2006-01-02") + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("expected a YYYY-MM-DD date, got %q", s)
	}
	*d = Date(t)
	return nil
}

func (d Date) Time() time.Time { return time.Time(d) }
func (d Date) IsZero() bool    { return time.Time(d).IsZero() }
