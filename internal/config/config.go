// Package config loads the CLI config from the config file and environment variables.
// Precedence: flags > env > config file > defaults. The cli package applies flags.
package config

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
)

// Environment variable names.
const (
	EnvAccessToken = "GYAZO_ACCESS_TOKEN"
	EnvBaseURL     = "GYAZO_API_URL"
	EnvUploadURL   = "GYAZO_UPLOAD_URL"
	EnvTimeout     = "GYAZO_TIMEOUT"
	EnvMaxRetries  = "GYAZO_MAX_RETRIES"
	// EnvConfigFile overrides the config file path.
	EnvConfigFile = "GY_CONFIG"
)

// Source is where a setting came from.
type Source string

const (
	SourceNone Source = ""
	SourceEnv  Source = "env"
	SourceFile Source = "file"
)

// Config is the CLI config.
type Config struct {
	BaseURL   string
	UploadURL string
	Timeout   time.Duration
	// MaxRetries is the maximum number of retries on transient failures (429 / 5xx / network errors).
	MaxRetries int
	// Upload holds the defaults of 'gy upload' flags.
	Upload UploadDefaults
	// Credentials are never accepted as flags, to keep them out of shell history.
	Credentials Credentials
	// CredentialSource is where Credentials came from.
	CredentialSource Source
	// FilePath is the config file path to read from and write to.
	FilePath string
}

// Credentials are API credentials.
type Credentials struct {
	AccessToken string `json:"accessToken,omitempty"`
}

// IsZero reports whether no credential is set.
func (c Credentials) IsZero() bool { return c == Credentials{} }

// Authenticator returns the Authenticator for the credentials.
// To add an auth method, add a field and return its gyazo.Authenticator here.
func (c Credentials) Authenticator() gyazo.Authenticator {
	if c.AccessToken != "" {
		return gyazo.AccessToken(c.AccessToken)
	}
	return gyazo.NoAuth{}
}

// Load returns defaults overlaid by the config file, then env. getenv is usually os.Getenv.
func Load(getenv func(string) string) (Config, error) {
	path, err := FilePath(getenv)
	if err != nil {
		return Config{}, err
	}
	file, err := ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		BaseURL:    gyazo.DefaultBaseURL,
		UploadURL:  gyazo.DefaultUploadURL,
		Timeout:    gyazo.DefaultTimeout,
		MaxRetries: gyazo.DefaultRetryPolicy.MaxRetries,
		Upload:     file.Upload,
		FilePath:   path,
	}
	if !file.Credentials.IsZero() {
		cfg.Credentials, cfg.CredentialSource = file.Credentials, SourceFile
	}

	if v := getenv(EnvAccessToken); v != "" {
		cfg.Credentials, cfg.CredentialSource = Credentials{AccessToken: v}, SourceEnv
	}
	if v := getenv(EnvBaseURL); v != "" {
		cfg.BaseURL = v
	}
	if v := getenv(EnvUploadURL); v != "" {
		cfg.UploadURL = v
	}
	if v := getenv(EnvTimeout); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid %s %q: %w", EnvTimeout, v, err)
		}
		cfg.Timeout = d
	}
	if v := getenv(EnvMaxRetries); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("invalid %s %q: must be a non-negative integer", EnvMaxRetries, v)
		}
		cfg.MaxRetries = n
	}
	return cfg, nil
}
