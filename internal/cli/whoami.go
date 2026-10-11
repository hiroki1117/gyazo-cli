package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newWhoamiCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print the authenticated user",
		Long: `Print the user of the current access token.

Text output: "<name> (<email>)".
JSON output: {uid, name, email, profile_image}`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := app.Client()
			if err != nil {
				return err
			}
			u, err := client.Users.Me(cmd.Context())
			if err != nil {
				return err
			}
			return app.Printer().Print(u, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s (%s)\n", u.Name, u.Email)
				return err
			})
		},
	}
}

func newVersionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			bi := app.BuildInfo
			return app.Printer().Print(bi, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "gy %s (commit: %s, built: %s)\n", bi.Version, bi.Commit, bi.Date)
				return err
			})
		},
	}
}
