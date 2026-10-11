package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
)

// uploadFlags are the flags of the upload command.
type uploadFlags struct {
	opts           gyazo.UploadOptions
	metadataPublic bool
	createdAt      string
	stdinName      string
	link           linkFlag
}

func newUploadCmd(app *App) *cobra.Command {
	var uf uploadFlags
	cmd := &cobra.Command{
		Use:   "upload <file|->...",
		Short: "Upload images and print their URLs",
		Long: `Upload image files to Gyazo. Use "-" to read an image from standard input.
Files are uploaded in order; on a failure, the remaining files are not uploaded.
Uploads are never retried, so an image is never uploaded twice.

Defaults of --access-policy and --app can be saved with 'gy config set'.

Text output: one link per uploaded image, in the style of --as (default: permalink).
JSON output: [{image_id, permalink_url, thumb_url, url, type}]`,
		Example: `  gy upload screenshot.png
  gy upload *.png --as scrapbox | pbcopy
  pngpaste - | gy upload - --title "design review" --access-policy only_me
  gy upload chart.png --desc "#weekly report" -o json | jq -r '.[].url'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			link, err := uf.link.formatter()
			if err != nil {
				return err
			}
			opts, err := uf.options(cmd, app)
			if err != nil {
				return err
			}
			if n := countStdin(args); n > 1 {
				return errors.New(`"-" (standard input) can be given only once`)
			}
			client, err := app.Client()
			if err != nil {
				return err
			}

			uploaded := []gyazo.Image{}
			var uploadErr error
			for _, arg := range args {
				img, err := uploadArg(cmd, app, client, arg, uf.stdinName, opts)
				if err != nil {
					uploadErr = fmt.Errorf("upload %s: %w", arg, err)
					break
				}
				uploaded = append(uploaded, *img)
			}
			// Print what was uploaded even on a failure, so the user knows which files are done.
			if err := printImages(app, uploaded, uploaded, link); err != nil {
				return err
			}
			return uploadErr
		},
	}
	f := cmd.Flags()
	f.StringVar(&uf.opts.Title, "title", "", "title of the page the image was captured from")
	f.StringVar(&uf.opts.Desc, "desc", "", "description, searchable in Gyazo (e.g. #tags)")
	f.StringVar(&uf.opts.RefererURL, "referer-url", "", "URL of the page the image was captured from")
	f.StringVar(&uf.opts.App, "app", "", "application name recorded with the image (default: config upload.app)")
	f.StringVar(&uf.opts.AccessPolicy, "access-policy", "", "who can view the image: "+strings.Join(gyazo.AccessPolicies, "|")+" (default: config upload.access-policy, then anyone)")
	f.BoolVar(&uf.metadataPublic, "metadata-public", false, "make the title and referer URL public (default: server setting)")
	f.StringVar(&uf.opts.CollectionID, "collection", "", "collection ID to add the image to")
	f.StringVar(&uf.createdAt, "created-at", "", "capture time, RFC 3339 or Unix seconds (default: upload time)")
	f.StringVar(&uf.stdinName, "stdin-name", "stdin", `file name sent for "-" (the type is detected from the content)`)
	uf.link.add(cmd, "permalink", "link style of the text output")
	return cmd
}

// options resolves the upload options from flags and the config file defaults.
func (uf *uploadFlags) options(cmd *cobra.Command, app *App) (*gyazo.UploadOptions, error) {
	opts := uf.opts
	if opts.App == "" {
		opts.App = app.cfg.Upload.App
	}
	if opts.AccessPolicy == "" {
		opts.AccessPolicy = app.cfg.Upload.AccessPolicy
	}
	if opts.AccessPolicy != "" {
		if err := validateAccessPolicy(opts.AccessPolicy); err != nil {
			return nil, err
		}
	}
	// Send metadata_is_public only when the flag is given, so the server setting applies otherwise.
	if cmd.Flags().Changed("metadata-public") {
		opts.MetadataIsPublic = &uf.metadataPublic
	}
	if uf.createdAt != "" {
		t, err := parseTime(uf.createdAt)
		if err != nil {
			return nil, err
		}
		opts.CreatedAt = t
	}
	return &opts, nil
}

// uploadArg uploads a file path, or standard input for "-".
func uploadArg(cmd *cobra.Command, app *App, client *gyazo.Client, arg, stdinName string, opts *gyazo.UploadOptions) (*gyazo.Image, error) {
	if arg == "-" {
		return client.Images.Upload(cmd.Context(), gyazo.UploadFile{Name: stdinName, Body: app.IO.In, Size: -1}, opts)
	}
	f, err := os.Open(arg)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("is a directory")
	}
	return client.Images.Upload(cmd.Context(), gyazo.UploadFile{Name: filepath.Base(arg), Body: f, Size: info.Size()}, opts)
}

func countStdin(args []string) int {
	n := 0
	for _, a := range args {
		if a == "-" {
			n++
		}
	}
	return n
}

// parseTime parses RFC 3339 or Unix seconds.
func parseTime(s string) (time.Time, error) {
	if sec, err := strconv.ParseFloat(s, 64); err == nil {
		return time.UnixMilli(int64(sec * 1000)), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: use RFC 3339 (2006-01-02T15:04:05+09:00) or Unix seconds", s)
	}
	return t, nil
}
