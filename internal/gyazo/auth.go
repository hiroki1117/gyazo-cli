package gyazo

import "net/http"

// Authenticator adds credentials to requests.
// Implement it to add a new auth method.
type Authenticator interface {
	Apply(req *http.Request)
}

// NoAuth adds no credentials. Only oEmbed-like public endpoints work without a token.
type NoAuth struct{}

// Apply does nothing.
func (NoAuth) Apply(*http.Request) {}

// AccessToken authenticates with an OAuth access token,
// issued for your application at https://gyazo.com/oauth/applications.
type AccessToken string

// Apply sets the Authorization: Bearer header.
func (t AccessToken) Apply(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+string(t))
}
