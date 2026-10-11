package gyazo

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy controls retries of transient failures:
// network errors and 429 / 502 / 503 / 504. Only GET requests are retried.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retries after the first attempt. 0 disables retries.
	MaxRetries int
	// BaseDelay is the initial delay of the exponential backoff.
	BaseDelay time.Duration
	// MaxDelay caps each delay, including the Retry-After header value.
	MaxDelay time.Duration
}

// DefaultRetryPolicy is the default retry policy of Client.
var DefaultRetryPolicy = RetryPolicy{
	MaxRetries: 3,
	BaseDelay:  500 * time.Millisecond,
	MaxDelay:   10 * time.Second,
}

// WithRetryPolicy sets the retry policy.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Client) error {
		if p.MaxRetries < 0 {
			return errors.New("retry: MaxRetries must not be negative")
		}
		c.retry = p
		return nil
	}
}

// retryable reports whether a response status should be retried.
func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// delay returns the wait before retry number attempt (0-based).
// It follows Retry-After if present, otherwise uses exponential backoff with full jitter.
func (p RetryPolicy) delay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return min(d, p.MaxDelay)
		}
	}
	backoff := p.BaseDelay << attempt
	if backoff <= 0 || backoff > p.MaxDelay { // also caps shift overflow
		backoff = p.MaxDelay
	}
	if backoff <= 0 {
		return 0
	}
	return rand.N(backoff + 1)
}

// parseRetryAfter parses a Retry-After header (seconds or an HTTP date).
func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if sec, err := strconv.Atoi(v); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0), true
	}
	return 0, false
}

// sleep waits for d, or returns the ctx error as soon as ctx is canceled.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// discard drains and closes the body before a retry so the connection can be reused.
func discard(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBody))
	_ = resp.Body.Close()
}
