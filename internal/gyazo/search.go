package gyazo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
)

// MaxSearchResults is the number of search results the API can return in total (page * per).
const MaxSearchResults = 10000

// MaxQueryLength is the maximum length of a search query.
const MaxQueryLength = 200

// SearchOptions are options for searching images. Zero values use the server defaults (page 1, 10 per page).
type SearchOptions struct {
	Page int
	Per  int
}

// SearchResult is a page of search results.
type SearchResult struct {
	Query  string  `json:"query"`
	Page   int     `json:"page,omitempty"`
	Per    int     `json:"per,omitempty"`
	Images []Image `json:"images"`
}

// UnmarshalJSON accepts both response shapes found in the API docs:
// a plain array of images, and {"captures": [...], "query": "..."}.
func (r *SearchResult) UnmarshalJSON(b []byte) error {
	if b = bytes.TrimSpace(b); len(b) > 0 && b[0] == '[' {
		return json.Unmarshal(b, &r.Images)
	}
	var obj struct {
		Query    string  `json:"query"`
		Captures []Image `json:"captures"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	r.Query, r.Images = obj.Query, obj.Captures
	return nil
}

// Search searches the authenticated user's images by text, including OCR text and metadata.
// Requires a Gyazo Pro plan; otherwise it fails with ErrPaymentRequired.
//
// GET /api/search
func (s *ImagesService) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResult, error) {
	q := url.Values{"query": {query}}
	var out SearchResult
	if opts != nil {
		setPositive(q, "page", opts.Page)
		setPositive(q, "per", opts.Per)
		out.Page, out.Per = opts.Page, opts.Per
	}
	if _, err := s.client.getJSON(ctx, "api/search", q, &out); err != nil {
		return nil, err
	}
	out.Query = query
	if out.Images == nil {
		out.Images = []Image{}
	}
	return &out, nil
}
