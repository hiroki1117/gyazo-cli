package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
)

func newSearchCmd(app *App) *cobra.Command {
	var (
		opts gyazo.SearchOptions
		link linkFlag
	)
	cmd := &cobra.Command{
		Use:   "search <query>...",
		Short: "Search your images (Gyazo Pro)",
		Long: fmt.Sprintf(`Search your images by text, including OCR text, titles, URLs and descriptions.
Multiple arguments are joined with spaces. Requires a Gyazo Pro plan.

The query is up to %d characters. Only the first %d results can be retrieved
(--page x --per must be %d or less).

Text output: CREATED, TYPE, PERMALINK, TITLE (with --as, one link per line).
JSON output: {query, page, per, images: [{image_id, permalink_url, url, type,
             created_at, metadata, ocr, ...}]}`, gyazo.MaxQueryLength, gyazo.MaxSearchResults, gyazo.MaxSearchResults),
		Example: `  gy search invoice
  gy search "error message" --per 50
  gy search whiteboard --as scrapbox
  gy search receipt -o json | jq -r '.images[].url'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := link.formatter()
			if err != nil {
				return err
			}
			query := strings.Join(args, " ")
			if n := utf8.RuneCountInString(query); n > gyazo.MaxQueryLength {
				return fmt.Errorf("query is too long (%d characters, max %d)", n, gyazo.MaxQueryLength)
			}
			if opts.Per > gyazo.MaxPerPage {
				return fmt.Errorf("--per must be %d or less", gyazo.MaxPerPage)
			}
			if max(opts.Page, 1)*max(opts.Per, 1) > gyazo.MaxSearchResults {
				return fmt.Errorf("--page x --per must be %d or less", gyazo.MaxSearchResults)
			}
			client, err := app.Client()
			if err != nil {
				return err
			}
			res, err := client.Images.Search(cmd.Context(), query, &opts)
			if err != nil {
				return err
			}
			return printImages(app, res, res.Images, format)
		},
	}
	f := cmd.Flags()
	f.IntVar(&opts.Page, "page", 1, "page number")
	f.IntVar(&opts.Per, "per", 20, fmt.Sprintf("results per page (max %d)", gyazo.MaxPerPage))
	link.add(cmd, "", "print one link per image instead of a table")
	return cmd
}
