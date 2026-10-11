package cli

import (
	"github.com/spf13/cobra"

	"github.com/hiroki1117/gyazo-cli/internal/config"
)

// Command group IDs, used to group commands by purpose in the help.
const (
	groupImage    = "image"
	groupRead     = "read"
	groupWrite    = "write"
	groupSettings = "settings"
)

const rootLong = `gy uploads, lists, searches and downloads Gyazo images from the command line.

Quick start:
  gy auth login                     save your access token
  gy upload screenshot.png          upload an image and print its URL
  gy image list                     list your images
  gy search keyword                 search your images (Gyazo Pro)
  gy image download <id|url>        save an image file

Conventions:
  <id|url>     an image ID, or a Gyazo URL (https://gyazo.com/<id>, https://i.gyazo.com/<id>.png)
  -o json      machine-readable output; recommended for scripts and AI agents
  stdout       results only; counts, hints and errors are written to stderr
  exit code    0 on success, 1 on any error

Run 'gy help environment' for environment variables and credentials.`

// imageArgHelp is appended to the Long help of commands that take image arguments.
const imageArgHelp = `<id|url> is an image ID, or a Gyazo URL like https://gyazo.com/<id> or
https://i.gyazo.com/<id>.png.`

const environmentLong = `Environment variables (flags take precedence over environment variables,
which take precedence over the config file):

  ` + config.EnvAccessToken + `  OAuth access token (create one at https://gyazo.com/oauth/applications)
  ` + config.EnvBaseURL + `       API base URL (default: https://api.gyazo.com)
  ` + config.EnvUploadURL + `    Upload API base URL (default: https://upload.gyazo.com)
  ` + config.EnvTimeout + `       request timeout, e.g. 30s (default: 30s)
  ` + config.EnvMaxRetries + `   max retries on 429/5xx/network errors (default: 3)
  ` + config.EnvConfigFile + `           config file path (default: ~/.config/gy/config.json)

If ` + config.EnvAccessToken + ` is set, the token in the config file is ignored.`

// newEnvironmentTopic is the help topic shown by 'gy help environment'.
// cobra lists a command without Run under "Additional help topics".
func newEnvironmentTopic() *cobra.Command {
	return &cobra.Command{
		Use:   "environment",
		Short: "Environment variables and configuration precedence",
		Long:  environmentLong,
	}
}

// newGroupCmd creates a command that only groups subcommands.
// It prints help without arguments and fails on unknown subcommands (by default cobra prints help and exits 0).
func newGroupCmd(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}
