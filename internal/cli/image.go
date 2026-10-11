package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hiroki1117/gyazo-cli/internal/gyazo"
	"github.com/hiroki1117/gyazo-cli/internal/output"
)

func newImageCmd(app *App) *cobra.Command {
	cmd := newGroupCmd("image", "List, show, download and delete images", "")
	cmd.AddGroup(
		&cobra.Group{ID: groupRead, Title: "Read commands:"},
		&cobra.Group{ID: groupWrite, Title: "Write commands:"},
	)
	addToGroup(cmd, groupRead,
		newImageListCmd(app),
		newImageGetCmd(app),
		newImageOCRCmd(app),
		newImageDownloadCmd(app),
	)
	addToGroup(cmd, groupWrite, newImageDeleteCmd(app))
	return cmd
}

func newImageListCmd(app *App) *cobra.Command {
	var (
		opts gyazo.ListOptions
		all  bool
		link linkFlag
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List your images, newest first",
		Long: `List your images, newest first. The page position is written to stderr.

Text output: CREATED, TYPE, PERMALINK, TITLE (with --as, one link per line).
JSON output: {total_count, page, per_page, user_type, images: [{image_id,
             permalink_url, thumb_url, url, type, created_at, metadata}]}`,
		Example: `  gy image list
  gy image list --page 2 --per-page 50
  gy image list --per-page 5 --as scrapbox
  gy image list --all -o json | jq -r '.images[].url'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := link.formatter()
			if err != nil {
				return err
			}
			if opts.PerPage < 1 || opts.PerPage > gyazo.MaxPerPage {
				return fmt.Errorf("--per-page must be between 1 and %d", gyazo.MaxPerPage)
			}
			client, err := app.Client()
			if err != nil {
				return err
			}

			var list *gyazo.ImageList
			if all {
				// Wrap in an ImageList so the JSON shape is the same with or without --all.
				list = &gyazo.ImageList{Page: opts.Page, PerPage: opts.PerPage, Images: []gyazo.Image{}}
				for img, err := range client.Images.ListAll(cmd.Context(), &opts) {
					if err != nil {
						return err
					}
					list.Images = append(list.Images, img)
				}
				list.TotalCount = len(list.Images)
			} else {
				list, err = client.Images.List(cmd.Context(), &opts)
				if err != nil {
					return err
				}
				if list.PerPage > 0 {
					pages := (list.TotalCount + list.PerPage - 1) / list.PerPage
					fmt.Fprintf(app.IO.ErrOut, "page %d/%d (%d images)\n", list.Page, pages, list.TotalCount)
				}
			}
			return printImages(app, list, list.Images, format)
		},
	}
	f := cmd.Flags()
	f.IntVar(&opts.Page, "page", 1, "page number")
	f.IntVar(&opts.PerPage, "per-page", 20, fmt.Sprintf("images per page (max %d; with --all, per request)", gyazo.MaxPerPage))
	f.BoolVar(&all, "all", false, "fetch all images from --page, paging automatically")
	link.add(cmd, "", "print one link per image instead of a table")
	return cmd
}

func newImageGetCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id|url>",
		Short: "Show an image's details",
		Long: `Show an image's URLs, capture metadata and OCR language.

` + imageArgHelp + `

Text output: one "Key: value" line per field (empty fields are omitted).
JSON output: {image_id, permalink_url, thumb_url, url, type, created_at,
             metadata: {app, title, url, desc}, ocr: {locale, description}}`,
		Example: `  gy image get 8980c52421e452ac3355ca3e5cfe7a0c
  gy image get https://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c
  gy image get <id> -o json | jq -r .metadata.url`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			img, err := fetchImage(cmd, app, args[0])
			if err != nil {
				return err
			}
			return app.Printer().Print(img, func(w io.Writer) error {
				tw := output.Table(w)
				row := func(k, v string) {
					if v != "" {
						fmt.Fprintf(tw, "%s:\t%s\n", k, v)
					}
				}
				row("ID", img.ImageID)
				row("Permalink", img.Permalink())
				row("URL", img.FileURL())
				row("Type", img.Type)
				row("Created", output.Time(img.CreatedAt.Time()))
				if m := img.Metadata; m != nil {
					row("App", m.App)
					row("Title", m.Title)
					row("Source", m.URL)
					row("Desc", m.Desc)
				}
				if img.OCR != nil {
					row("OCR", img.OCR.Locale)
				}
				return tw.Flush()
			})
		},
	}
}

func newImageOCRCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "ocr <id|url>",
		Short: "Print the text recognized in an image",
		Long: `Print the text recognized in an image by Gyazo's OCR.

` + imageArgHelp + `

Text output: the recognized text as-is (empty if none).
JSON output: {locale, description}`,
		Example: `  gy image ocr https://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c
  gy image ocr <id> | pbcopy`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			img, err := fetchImage(cmd, app, args[0])
			if err != nil {
				return err
			}
			ocr := img.OCR
			if ocr == nil {
				ocr = &gyazo.OCR{}
			}
			return app.Printer().Print(ocr, func(w io.Writer) error {
				if ocr.Description == "" {
					fmt.Fprintln(app.IO.ErrOut, "no text recognized")
					return nil
				}
				_, err := fmt.Fprintln(w, ocr.Description)
				return err
			})
		},
	}
}

// downloaded is the JSON output of image download.
type downloaded struct {
	ImageID string `json:"image_id"`
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
}

func newImageDownloadCmd(app *App) *cobra.Command {
	var (
		dir    string
		stdout bool
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "download <id|url>...",
		Short: "Save image files",
		Long: `Save image files as <dir>/<image_id>.<ext>.
Existing files are not overwritten unless --force is given.

` + imageArgHelp + `

Text output: the saved file paths, one per line.
JSON output: [{image_id, path, bytes}]
With --stdout, the image data is written to stdout as-is (-o is ignored).`,
		Example: `  gy image download https://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c
  gy image download <id1> <id2> --dir ~/Pictures
  gy image list --as permalink | xargs gy image download --dir backup
  gy image download <id> --stdout > image.png`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if stdout && len(args) > 1 {
				return errors.New("--stdout takes exactly one image")
			}
			ids, client, err := app.imageClient(args)
			if err != nil {
				return err
			}
			if stdout {
				img, err := client.Images.Get(cmd.Context(), ids[0])
				if err != nil {
					return err
				}
				_, err = client.Images.Download(cmd.Context(), img, app.IO.Out)
				return err
			}

			saved := []downloaded{}
			var dlErr error
			for _, id := range ids {
				d, err := downloadImage(cmd, client, id, dir, force)
				if err != nil {
					dlErr = fmt.Errorf("download %s: %w", id, err)
					break
				}
				saved = append(saved, d)
			}
			if err := app.Printer().Print(saved, func(w io.Writer) error {
				for _, d := range saved {
					fmt.Fprintln(w, d.Path)
				}
				return nil
			}); err != nil {
				return err
			}
			return dlErr
		},
	}
	f := cmd.Flags()
	f.StringVarP(&dir, "dir", "d", ".", "directory to save files in")
	f.BoolVar(&stdout, "stdout", false, "write the image data to stdout instead of a file")
	f.BoolVarP(&force, "force", "f", false, "overwrite existing files")
	return cmd
}

// downloadImage saves an image into dir. It writes to a temp file and renames it,
// so an interrupted download never leaves a broken file.
func downloadImage(cmd *cobra.Command, client *gyazo.Client, id, dir string, force bool) (downloaded, error) {
	img, err := client.Images.Get(cmd.Context(), id)
	if err != nil {
		return downloaded{}, err
	}
	path := filepath.Join(dir, img.FileName())
	if !force {
		if _, err := os.Stat(path); err == nil {
			return downloaded{}, fmt.Errorf("%s already exists (use --force to overwrite)", path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return downloaded{}, err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return downloaded{}, err
	}
	tmp, err := os.CreateTemp(dir, ".gy-download-*")
	if err != nil {
		return downloaded{}, err
	}
	defer os.Remove(tmp.Name())

	n, err := client.Images.Download(cmd.Context(), img, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return downloaded{}, err
	}
	return downloaded{ImageID: img.ImageID, Path: path, Bytes: n}, nil
}

func newImageDeleteCmd(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id|url>...",
		Short: "Delete your images",
		Long: `Delete your images. Without --yes, only shows the images to be deleted.
All arguments are checked before anything is deleted.

` + imageArgHelp + `

Text output: "Deleted <id>" per image (without --yes, the images to be deleted).
JSON output: [{image_id, type}] (without --yes, the images to be deleted)`,
		Example: `  gy image delete https://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c
  gy image delete <id1> <id2> --yes`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, client, err := app.imageClient(args)
			if err != nil {
				return err
			}
			if !yes {
				images := make([]gyazo.Image, 0, len(ids))
				for _, id := range ids {
					img, err := client.Images.Get(cmd.Context(), id)
					if err != nil {
						return err
					}
					images = append(images, *img)
				}
				if err := printImages(app, images, images, nil); err != nil {
					return err
				}
				fmt.Fprintln(app.IO.ErrOut, "Run again with --yes to delete.")
				return nil
			}

			deleted := []gyazo.DeletedImage{}
			var delErr error
			for _, id := range ids {
				d, err := client.Images.Delete(cmd.Context(), id)
				if err != nil {
					delErr = fmt.Errorf("delete %s: %w", id, err)
					break
				}
				deleted = append(deleted, *d)
			}
			if err := app.Printer().Print(deleted, func(w io.Writer) error {
				for _, d := range deleted {
					fmt.Fprintf(w, "Deleted %s\n", d.ImageID)
				}
				return nil
			}); err != nil {
				return err
			}
			return delErr
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "delete without the preview step")
	return cmd
}

// fetchImage fetches the image given as an argument.
func fetchImage(cmd *cobra.Command, app *App, arg string) (*gyazo.Image, error) {
	ids, client, err := app.imageClient([]string{arg})
	if err != nil {
		return nil, err
	}
	return client.Images.Get(cmd.Context(), ids[0])
}
