package gyazo

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// ImagesService handles image APIs.
type ImagesService service

// MaxPerPage is the maximum number of images per list or search request.
const MaxPerPage = 100

// ListOptions are options for listing images. Zero values use the server defaults (page 1, 20 per page).
type ListOptions struct {
	Page    int
	PerPage int
}

func (o *ListOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	setPositive(q, "page", o.Page)
	setPositive(q, "per_page", o.PerPage)
	return q
}

// List lists the authenticated user's images, newest first.
//
// GET /api/images
func (s *ImagesService) List(ctx context.Context, opts *ListOptions) (*ImageList, error) {
	out := ImageList{Images: []Image{}}
	h, err := s.client.getJSON(ctx, "api/images", opts.values(), &out.Images)
	if err != nil {
		return nil, err
	}
	out.TotalCount = headerInt(h, "X-Total-Count")
	out.Page = headerInt(h, "X-Current-Page")
	out.PerPage = headerInt(h, "X-Per-Page")
	out.UserType = h.Get("X-User-Type")
	return &out, nil
}

// ListAll yields all images from opts.Page to the end, paging automatically.
// opts.PerPage is used as the number of images per request (default MaxPerPage).
func (s *ImagesService) ListAll(ctx context.Context, opts *ListOptions) iter.Seq2[Image, error] {
	return func(yield func(Image, error) bool) {
		o := ListOptions{Page: 1, PerPage: MaxPerPage}
		if opts != nil {
			o.Page, o.PerPage = max(opts.Page, 1), opts.PerPage
			if o.PerPage <= 0 {
				o.PerPage = MaxPerPage
			}
		}
		for {
			list, err := s.List(ctx, &o)
			if err != nil {
				yield(Image{}, err)
				return
			}
			for _, img := range list.Images {
				if !yield(img, nil) {
					return
				}
			}
			if len(list.Images) == 0 || o.Page*o.PerPage >= list.TotalCount {
				return
			}
			o.Page++
		}
	}
}

// Get fetches an image, including its metadata and OCR text.
//
// GET /api/images/:image_id
func (s *ImagesService) Get(ctx context.Context, imageID string) (*Image, error) {
	var out Image
	if _, err := s.client.getJSON(ctx, "api/images/"+url.PathEscape(imageID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete deletes one of the authenticated user's images.
//
// DELETE /api/images/:image_id
func (s *ImagesService) Delete(ctx context.Context, imageID string) (*DeletedImage, error) {
	req, err := s.client.newAPIRequest(ctx, http.MethodDelete, s.client.baseURL, "api/images/"+url.PathEscape(imageID), nil, nil)
	if err != nil {
		return nil, err
	}
	var out DeletedImage
	if _, err := s.client.doJSON(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func setPositive(q url.Values, key string, v int) {
	if v > 0 {
		q.Set(key, strconv.Itoa(v))
	}
}

// headerInt returns an integer header value, or 0 if it is missing or invalid.
func headerInt(h http.Header, key string) int {
	n, _ := strconv.Atoi(h.Get(key))
	return n
}
