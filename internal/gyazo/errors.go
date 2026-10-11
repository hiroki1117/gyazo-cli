package gyazo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Sentinel errors for errors.Is.
var (
	ErrUnauthorized = errors.New("unauthorized")
	// ErrPaymentRequired means the API needs a Gyazo Pro (or Teams) plan, e.g. search.
	ErrPaymentRequired = errors.New("payment required")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("not found")
	// ErrRateLimited means the daily request limit was exceeded.
	ErrRateLimited = errors.New("rate limited")
)

// APIError is returned when the API responds with a non-2xx status.
type APIError struct {
	StatusCode int
	Method     string
	URL        string
	// Message comes from the Gyazo error response {"message": "...", "request": "...", "method": "..."}.
	Message string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.URL, e.StatusCode, msg)
}

// Is maps the status code to the sentinel errors.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrPaymentRequired:
		return e.StatusCode == http.StatusPaymentRequired
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrRateLimited:
		return e.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// maxErrorBody is the maximum size of an error response body to read.
const maxErrorBody = 64 << 10

func newAPIError(req *http.Request, resp *http.Response) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		Method:     req.Method,
		URL:        req.URL.Redacted(),
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		return e
	}
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) == nil {
		e.Message = payload.Message
	}
	return e
}
