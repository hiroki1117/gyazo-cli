package cli

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
	"github.com/hiroki1117/gyazo-cli/internal/output"
)

// linkStyles format an image as a link for the --as flag. Register new styles here.
var linkStyles = map[string]func(img *gyazo.Image) string{
	"permalink": (*gyazo.Image).Permalink,
	"url":       (*gyazo.Image).FileURL,
	"markdown":  func(img *gyazo.Image) string { return fmt.Sprintf("![](%s)", img.FileURL()) },
	// Scrapbox (Cosense) shows a bracketed Gyazo permalink as the image.
	"scrapbox": func(img *gyazo.Image) string { return "[" + img.Permalink() + "]" },
}

func linkStyleNames() string { return strings.Join(slices.Sorted(maps.Keys(linkStyles)), "|") }

// linkFlag is the --as flag. An empty value means the command's default text output.
type linkFlag struct {
	style string
}

func (l *linkFlag) add(cmd *cobra.Command, def, usage string) {
	cmd.Flags().StringVar(&l.style, "as", def, usage+": "+linkStyleNames())
}

// formatter returns the link formatter, or nil if no style is selected.
func (l *linkFlag) formatter() (func(*gyazo.Image) string, error) {
	if l.style == "" {
		return nil, nil
	}
	f, ok := linkStyles[l.style]
	if !ok {
		return nil, fmt.Errorf("unknown link style %q (available: %s)", l.style, linkStyleNames())
	}
	return f, nil
}

// printImages prints images as a table, or one link per line if a link style is selected.
func printImages(app *App, v any, images []gyazo.Image, link func(*gyazo.Image) string) error {
	return app.Printer().Print(v, func(w io.Writer) error {
		if link != nil {
			for i := range images {
				fmt.Fprintln(w, link(&images[i]))
			}
			return nil
		}
		tw := output.Table(w)
		fmt.Fprintln(tw, "CREATED\tTYPE\tPERMALINK\tTITLE")
		for i := range images {
			img := &images[i]
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", output.Time(img.CreatedAt.Time()), img.Type, img.Permalink(), img.Title())
		}
		return tw.Flush()
	})
}
