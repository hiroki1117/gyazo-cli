// Package gyazo is a client for the Gyazo API (https://gyazo.com/api/docs).
//
// Endpoints are grouped into services by resource (Images, Users).
// To add an API, add a method to a service, or define a new service and register it in Client.
package gyazo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the base URL of the Gyazo API.
	DefaultBaseURL = "https://api.gyazo.com"
	// DefaultUploadURL is the base URL of the Gyazo Upload API, which uses its own host.
	DefaultUploadURL = "https://upload.gyazo.com"
	// DefaultTimeout is the default HTTP request timeout.
	DefaultTimeout = 30 * time.Second

	defaultUserAgent = "gyazo-cli"
)

// Client is a Gyazo API client.
type Client struct {
	baseURL    string
	uploadURL  string
	httpClient *http.Client
	auth       Authenticator
	userAgent  string
	retry      RetryPolicy

	Images *ImagesService
	Users  *UsersService
}

// service is the Client reference shared by all services.
type service struct {
	client *Client
}

// Option configures a Client.
type Option func(*Client) error

// WithBaseURL sets the API base URL, for tests and proxies.
func WithBaseURL(raw string) Option {
	return func(c *Client) error {
		u, err := parseBaseURL(raw)
		c.baseURL = u
		return err
	}
}

// WithUploadURL sets the Upload API base URL, for tests and proxies.
func WithUploadURL(raw string) Option {
	return func(c *Client) error {
		u, err := parseBaseURL(raw)
		c.uploadURL = u
		return err
	}
}

func parseBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid base URL %q: scheme must be http or https", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// WithHTTPClient sets the *http.Client to use.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		c.httpClient = hc
		return nil
	}
}

// WithAuthenticator sets the auth method.
func WithAuthenticator(a Authenticator) Option {
	return func(c *Client) error {
		c.auth = a
		return nil
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

// NewClient creates a Client.
func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:    DefaultBaseURL,
		uploadURL:  DefaultUploadURL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		auth:       NoAuth{},
		userAgent:  defaultUserAgent,
		retry:      DefaultRetryPolicy,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	common := service{client: c}
	c.Images = (*ImagesService)(&common)
	c.Users = (*UsersService)(&common)
	return c, nil
}

// newRequest builds an unauthenticated request for an absolute URL.
// Use it for requests outside the API, such as image downloads, so the token is never sent there.
func (c *Client) newRequest(ctx context.Context, method, rawURL string, query url.Values, body io.Reader) (*http.Request, error) {
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	return req, nil
}

// newAPIRequest builds an authenticated request for an API path (escaped, without a leading "/") under base.
func (c *Client) newAPIRequest(ctx context.Context, method, base, path string, query url.Values, body io.Reader) (*http.Request, error) {
	req, err := c.newRequest(ctx, method, base+"/"+path, query, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	c.auth.Apply(req)
	return req, nil
}

// do sends req and converts non-2xx responses to *APIError.
// Transient failures of GET requests are retried by the RetryPolicy; other methods are never retried
// because resending an upload or a delete is not safe. On success, the caller must close resp.Body.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	maxRetries := c.retry.MaxRetries
	if req.Method != http.MethodGet {
		maxRetries = 0
	}
	for attempt := 0; ; attempt++ {
		resp, err := c.httpClient.Do(req)
		canRetry := attempt < maxRetries && req.Context().Err() == nil
		if err != nil {
			if !canRetry {
				return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL.Redacted(), err)
			}
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		} else if !canRetry || !retryable(resp.StatusCode) {
			defer resp.Body.Close()
			return nil, newAPIError(req, resp)
		}

		wait := c.retry.delay(attempt, resp)
		if resp != nil {
			discard(resp)
		}
		if err := sleep(req.Context(), wait); err != nil {
			return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL.Redacted(), err)
		}
	}
}

// doJSON sends req and decodes the response body into v.
// It returns the response header for callers that need it, e.g. for paging.
func (c *Client) doJSON(req *http.Request, v any) (http.Header, error) {
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return nil, fmt.Errorf("decode response of %s: %w", req.URL.Redacted(), err)
	}
	return resp.Header, nil
}

// getJSON sends a GET request to the API and decodes the response body into v.
func (c *Client) getJSON(ctx context.Context, path string, query url.Values, v any) (http.Header, error) {
	req, err := c.newAPIRequest(ctx, http.MethodGet, c.baseURL, path, query, nil)
	if err != nil {
		return nil, err
	}
	return c.doJSON(req, v)
}
