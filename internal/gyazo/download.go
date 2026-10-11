package gyazo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

// DefaultImageHost is where image files are served when the API returns no URL.
const DefaultImageHost = "https://i.gyazo.com"

// FileURL returns the URL of the image file: the URL from the API, or one built from the ID and type.
func (img *Image) FileURL() string {
	if img.URL != "" {
		return img.URL
	}
	return fmt.Sprintf("%s/%s.%s", DefaultImageHost, url.PathEscape(img.ImageID), img.Type)
}

// FileName returns a file name for the image, "<image_id>.<ext>".
// The extension comes from the file URL, falling back to the type.
func (img *Image) FileName() string {
	ext := img.Type
	if u, err := url.Parse(img.FileURL()); err == nil {
		if e := path.Ext(u.Path); len(e) > 1 {
			ext = e[1:]
		}
	}
	if ext == "" {
		return img.ImageID
	}
	return img.ImageID + "." + ext
}

// Download writes the image file to w and returns the number of bytes written.
// The access token is not sent, because the file is served outside the API.
func (s *ImagesService) Download(ctx context.Context, img *Image, w io.Writer) (int64, error) {
	req, err := s.client.newRequest(ctx, http.MethodGet, img.FileURL(), nil, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.client.do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("download %s: %w", req.URL.Redacted(), err)
	}
	return n, nil
}
