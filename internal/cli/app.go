// Package cli defines the subcommands of gy.
//
// Each subcommand gets the Client, Printer and config from App.
// To add a command, define newXxxCmd(app *App) *cobra.Command and AddCommand it to its parent.
package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hiroki1117/gyazo-cli/internal/config"
	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
	"github.com/hiroki1117/gyazo-cli/internal/output"
)

// IOStreams are the command's input and outputs. Tests replace them with buffers.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// BuildInfo is the version information embedded at build time.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// App holds the dependencies shared by commands.
// The config and output format are resolved in the root PersistentPreRunE; Printer and Client are created on first use.
type App struct {
	IO        IOStreams
	Getenv    func(string) string
	BuildInfo BuildInfo
	// If HTTPClient is nil, the gyazo package default is used.
	HTTPClient *http.Client

	cfg     config.Config
	format  output.Format
	printer *output.Printer
	client  *gyazo.Client
}

// Printer returns the Printer for the output format.
func (a *App) Printer() *output.Printer {
	if a.printer == nil {
		a.printer = output.NewPrinter(a.IO.Out, a.format)
	}
	return a.printer
}

// Client returns the API client using the configured credentials, created on first call.
func (a *App) Client() (*gyazo.Client, error) {
	if a.client != nil {
		return a.client, nil
	}
	c, err := a.NewClient(a.cfg.Credentials.Authenticator())
	if err != nil {
		return nil, err
	}
	a.client = c
	return c, nil
}

// NewClient creates an API client with the given authenticator.
// auth login uses it to verify a credential before saving it.
func (a *App) NewClient(auth gyazo.Authenticator) (*gyazo.Client, error) {
	// Copy the injected *http.Client before setting the timeout so it is not modified.
	var hc http.Client
	if a.HTTPClient != nil {
		hc = *a.HTTPClient
	}
	hc.Timeout = a.cfg.Timeout

	ua := "gyazo-cli"
	if a.BuildInfo.Version != "" {
		ua += "/" + a.BuildInfo.Version
	}
	retry := gyazo.DefaultRetryPolicy
	retry.MaxRetries = a.cfg.MaxRetries
	return gyazo.NewClient(
		gyazo.WithBaseURL(a.cfg.BaseURL),
		gyazo.WithUploadURL(a.cfg.UploadURL),
		gyazo.WithRetryPolicy(retry),
		gyazo.WithHTTPClient(&hc),
		gyazo.WithAuthenticator(auth),
		gyazo.WithUserAgent(ua),
	)
}

// imageClient parses image arguments and returns their IDs with the API client.
// All arguments are parsed before any request so a typo fails before anything is changed.
func (a *App) imageClient(args []string) ([]string, *gyazo.Client, error) {
	ids := make([]string, len(args))
	for i, arg := range args {
		id, err := gyazo.ParseImageID(arg)
		if err != nil {
			return nil, nil, err
		}
		ids[i] = id
	}
	client, err := a.Client()
	return ids, client, err
}

// ErrorHint returns a hint for fixing err, or an empty string if there is none.
func ErrorHint(err error) string {
	switch {
	case errors.Is(err, gyazo.ErrUnauthorized):
		return fmt.Sprintf("hint: this request requires an access token. run 'gy auth login', or set %s.", config.EnvAccessToken)
	case errors.Is(err, gyazo.ErrPaymentRequired):
		return "hint: this API requires a Gyazo Pro plan."
	case errors.Is(err, gyazo.ErrForbidden):
		return "hint: you can only access (and delete) your own images."
	case errors.Is(err, gyazo.ErrNotFound):
		return "hint: check the image ID or URL."
	case errors.Is(err, gyazo.ErrRateLimited):
		return "hint: the API allows 12,500 requests per day. try again later."
	}
	return ""
}
