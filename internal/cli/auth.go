package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/hiroki1117/gyazo-cli/internal/config"
	"github.com/hiroki1117/gyazo-cli/internal/output"
)

func newAuthCmd(app *App) *cobra.Command {
	cmd := newGroupCmd("auth", "Manage the access token", `Manage the OAuth access token used by all API requests.

Get a token:
  1. Open https://gyazo.com/oauth/applications and register an application
     (any name; the callback URL is not used by gy)
  2. Open the application and click "Your Access Token"
  3. Run 'gy auth login' and paste it

The token is saved in the config file (permission 0600).
`+config.EnvAccessToken+` takes precedence over it (see 'gy help environment').`)
	cmd.AddCommand(newAuthLoginCmd(app), newAuthStatusCmd(app), newAuthLogoutCmd(app))
	return cmd
}

func newAuthLoginCmd(app *App) *cobra.Command {
	var (
		withToken bool
		noVerify  bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save the access token to the config file",
		Long: `Save the access token to the config file.

The token is read from a hidden prompt, or from standard input with --with-token.
It is never accepted as a command line argument to keep it out of shell history.`,
		Example: `  gy auth login
  pbpaste | gy auth login --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			secret, err := readSecret(app, withToken, "Paste your access token: ")
			if err != nil {
				return err
			}
			creds := config.Credentials{AccessToken: secret}

			if !noVerify {
				if err := verifyCredentials(cmd, app, creds); err != nil {
					return err
				}
			}
			if err := config.UpdateFile(app.cfg.FilePath, func(f *config.File) { f.Credentials = creds }); err != nil {
				return err
			}
			fmt.Fprintf(app.IO.ErrOut, "Saved access token to %s\n", app.cfg.FilePath)
			if app.cfg.CredentialSource == config.SourceEnv {
				fmt.Fprintf(app.IO.ErrOut, "warning: %s takes precedence over the saved token.\n", config.EnvAccessToken)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.BoolVar(&withToken, "with-token", false, "read the token from standard input")
	f.BoolVar(&noVerify, "no-verify", false, "save without verifying the token")
	return cmd
}

// readSecret reads a credential: from a hidden prompt on a terminal, otherwise from the first line of stdin.
func readSecret(app *App, fromStdin bool, prompt string) (string, error) {
	var secret string
	if f, ok := app.IO.In.(*os.File); ok && !fromStdin && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(app.IO.ErrOut, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(app.IO.ErrOut)
		if err != nil {
			return "", fmt.Errorf("read token: %w", err)
		}
		secret = string(b)
	} else {
		line, err := bufio.NewReader(app.IO.In).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read token: %w", err)
		}
		secret = line
	}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", errors.New("token is empty")
	}
	return secret, nil
}

// verifyCredentials checks that the credential can access the API.
func verifyCredentials(cmd *cobra.Command, app *App, creds config.Credentials) error {
	client, err := app.NewClient(creds.Authenticator())
	if err != nil {
		return err
	}
	u, err := client.Users.Me(cmd.Context())
	if err != nil {
		return fmt.Errorf("verify token: %w", err)
	}
	fmt.Fprintf(app.IO.ErrOut, "Logged in as %s\n", u.Name)
	return nil
}

// authStatus is the output of auth status. It never contains the token itself.
type authStatus struct {
	ConfigFile string        `json:"configFile"`
	Source     config.Source `json:"source"`
	Credential string        `json:"credential"`
	User       string        `json:"user,omitempty"`
}

func newAuthStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current authentication state",
		Long: `Show which token is used, where it comes from (env or file), and the logged-in user.
The token itself is masked. Fails if the token is rejected by the API.

JSON output: {configFile, source, credential, user}`,
		Example: `  gy auth status
  gy auth status -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token := app.cfg.Credentials.AccessToken
			st := authStatus{
				ConfigFile: app.cfg.FilePath,
				Source:     app.cfg.CredentialSource,
				Credential: mask(token),
			}
			if token != "" {
				client, err := app.Client()
				if err != nil {
					return err
				}
				u, err := client.Users.Me(cmd.Context())
				if err != nil {
					return err
				}
				st.User = u.Name
			}
			return app.Printer().Print(st, func(w io.Writer) error {
				tw := output.Table(w)
				fmt.Fprintf(tw, "Config file:\t%s\n", st.ConfigFile)
				if token == "" {
					fmt.Fprintln(tw, "Auth:\tnot configured (run 'gy auth login')")
					return tw.Flush()
				}
				fmt.Fprintf(tw, "Auth:\taccess token (from %s)\n", st.Source)
				fmt.Fprintf(tw, "Credential:\t%s\n", st.Credential)
				fmt.Fprintf(tw, "User:\t%s\n", st.User)
				return tw.Flush()
			})
		},
	}
}

func newAuthLogoutCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the access token from the config file",
		Long:  "Remove the access token from the config file. " + config.EnvAccessToken + " is not affected.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			f, err := config.ReadFile(app.cfg.FilePath)
			if err != nil {
				return err
			}
			if f.Credentials.IsZero() {
				fmt.Fprintln(app.IO.ErrOut, "No access token saved in the config file")
			} else {
				f.Credentials = config.Credentials{}
				if err := config.WriteFile(app.cfg.FilePath, f); err != nil {
					return err
				}
				fmt.Fprintf(app.IO.ErrOut, "Removed access token from %s\n", app.cfg.FilePath)
			}
			if app.cfg.CredentialSource == config.SourceEnv {
				fmt.Fprintf(app.IO.ErrOut, "note: %s is still used. unset it to log out completely.\n", config.EnvAccessToken)
			}
			return nil
		},
	}
}

// mask hides a credential except its first 4 characters.
func mask(s string) string {
	if s == "" {
		return ""
	}
	const visible = 4
	if len(s) <= visible*2 {
		return strings.Repeat("*", len(s))
	}
	return s[:visible] + strings.Repeat("*", 8)
}
