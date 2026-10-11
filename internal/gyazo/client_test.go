package gyazo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient returns a Client that uses handler as both the API and the Upload API server.
func newTestClient(t *testing.T, handler http.Handler, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	base := []Option{
		WithBaseURL(srv.URL),
		WithUploadURL(srv.URL),
		WithRetryPolicy(RetryPolicy{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}),
	}
	c, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestImagesList_Headers(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/images" || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("per_page") != "5" {
			t.Errorf("request = %s", r.URL)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("X-Total-Count", "12")
		w.Header().Set("X-Current-Page", "2")
		w.Header().Set("X-Per-Page", "5")
		w.Header().Set("X-User-Type", "pro")
		fmt.Fprint(w, `[{"image_id":"abc","type":"png","created_at":"2014-05-21 14:23:10+0900","metadata":{"app":null,"title":"T","url":null,"desc":""}}]`)
	}), WithAuthenticator(AccessToken("tok")))

	list, err := c.Images.List(context.Background(), &ListOptions{Page: 2, PerPage: 5})
	if err != nil {
		t.Fatal(err)
	}
	if list.TotalCount != 12 || list.Page != 2 || list.PerPage != 5 || list.UserType != "pro" {
		t.Errorf("list = %+v", list)
	}
	img := list.Images[0]
	if img.Title() != "T" || img.CreatedAt.Time().IsZero() {
		t.Errorf("image = %+v", img)
	}
}

func TestImagesListAll_Pages(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Total-Count", "3")
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, `[{"image_id":"a"},{"image_id":"b"}]`)
		case "2":
			fmt.Fprint(w, `[{"image_id":"c"}]`)
		default:
			t.Errorf("unexpected page %s", r.URL.RawQuery)
			fmt.Fprint(w, `[]`)
		}
	}))
	var ids []string
	for img, err := range c.Images.ListAll(context.Background(), &ListOptions{PerPage: 2}) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, img.ImageID)
	}
	if strings.Join(ids, ",") != "a,b,c" || calls.Load() != 2 {
		t.Errorf("ids = %v, calls = %d", ids, calls.Load())
	}
}

func TestUpload_Multipart(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 1000)...)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/upload" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		if r.ContentLength <= 0 || len(r.TransferEncoding) > 0 {
			t.Errorf("body must have a length, got %d %v", r.ContentLength, r.TransferEncoding)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		for k, want := range map[string]string{"title": "My title", "access_policy": "only_me", "metadata_is_public": "false", "created_at": "1700000000.5"} {
			if got := r.FormValue(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		if _, ok := r.MultipartForm.Value["desc"]; ok {
			t.Error("empty fields must not be sent")
		}
		f, h, err := r.FormFile("imagedata")
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(f)
		if h.Filename != `a"b.png` || h.Header.Get("Content-Type") != "image/png" || !bytes.Equal(got, png) {
			t.Errorf("file = %q %q %d bytes", h.Filename, h.Header.Get("Content-Type"), len(got))
		}
		fmt.Fprint(w, `{"image_id":"id1","permalink_url":"https://gyazo.com/id1","url":"https://i.gyazo.com/id1.png","type":"png"}`)
	}))

	public := false
	opts := &UploadOptions{Title: "My title", AccessPolicy: AccessPolicyOnlyMe, MetadataIsPublic: &public, CreatedAt: time.UnixMilli(1700000000500)}
	for _, size := range []int64{int64(len(png)), -1} {
		img, err := c.Images.Upload(context.Background(), UploadFile{Name: `a"b.png`, Body: bytes.NewReader(png), Size: size}, opts)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if img.ImageID != "id1" {
			t.Errorf("image = %+v", img)
		}
	}
}

func TestUpload_NotRetried(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	_, err := c.Images.Upload(context.Background(), UploadFile{Name: "a.png", Body: strings.NewReader("x"), Size: 1}, nil)
	if err == nil || calls.Load() != 1 {
		t.Errorf("err = %v, calls = %d; uploads must not be retried", err, calls.Load())
	}
}

func TestGet_Retried(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"image_id":"abc"}`)
	}))
	img, err := c.Images.Get(context.Background(), "abc")
	if err != nil || img.ImageID != "abc" || calls.Load() != 3 {
		t.Errorf("img = %+v, err = %v, calls = %d", img, err, calls.Load())
	}
}

func TestAPIError(t *testing.T) {
	tests := []struct {
		status int
		target error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusPaymentRequired, ErrPaymentRequired},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusNotFound, ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, `{"message":"boom","request":"/api/images","method":"GET"}`)
			}))
			_, err := c.Images.Get(context.Background(), "abc")
			if !errors.Is(err, tt.target) {
				t.Errorf("err = %v, want %v", err, tt.target)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Message != "boom" {
				t.Errorf("err = %#v", err)
			}
		})
	}
}

func TestSearch_ResponseShapes(t *testing.T) {
	for name, body := range map[string]string{
		"array":  `[{"image_id":"a"}]`,
		"object": `{"captures":[{"image_id":"a"}],"number_of_captures":1,"query":"q"}`,
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/search" || r.URL.Query().Get("query") != "cat dog" || r.URL.Query().Get("per") != "5" {
					t.Errorf("request = %s", r.URL)
				}
				fmt.Fprint(w, body)
			}))
			res, err := c.Images.Search(context.Background(), "cat dog", &SearchOptions{Per: 5})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Images) != 1 || res.Images[0].ImageID != "a" || res.Query != "cat dog" {
				t.Errorf("result = %+v", res)
			}
		})
	}
}

func TestDownload_NoToken(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("the token must not be sent to the image host")
		}
		fmt.Fprint(w, "PNGDATA")
	}), WithAuthenticator(AccessToken("tok")))
	srvURL := c.baseURL

	var buf bytes.Buffer
	n, err := c.Images.Download(context.Background(), &Image{ImageID: "a", URL: srvURL + "/a.png"}, &buf)
	if err != nil || n != 7 || buf.String() != "PNGDATA" {
		t.Errorf("n = %d, err = %v, body = %q", n, err, buf.String())
	}
}

func TestImageFileHelpers(t *testing.T) {
	img := Image{ImageID: "abc", Type: "png"}
	if got := img.FileURL(); got != "https://i.gyazo.com/abc.png" {
		t.Errorf("FileURL = %q", got)
	}
	if got := img.Permalink(); got != "https://gyazo.com/abc" {
		t.Errorf("Permalink = %q", got)
	}
	img.URL = "https://i.gyazo.com/abc.jpg"
	if got := img.FileName(); got != "abc.jpg" {
		t.Errorf("FileName = %q", got)
	}
}

func TestParseImageID(t *testing.T) {
	const id = "8980c52421e452ac3355ca3e5cfe7a0c"
	for _, in := range []string{
		id,
		"https://gyazo.com/" + id,
		"https://gyazo.com/" + id + "/raw",
		"https://i.gyazo.com/" + id + ".png",
		"https://myteam.gyazo.com/" + id,
	} {
		got, err := ParseImageID(in)
		if err != nil || got != id {
			t.Errorf("ParseImageID(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "https://example.com/" + id, "a/b", "https://gyazo.com/"} {
		if _, err := ParseImageID(in); err == nil {
			t.Errorf("ParseImageID(%q): expected error", in)
		}
	}
}

func TestTimestamp(t *testing.T) {
	for _, s := range []string{"2014-05-21 14:23:10+0900", "2018-07-24T07:33:24.771Z"} {
		if Timestamp(s).Time().IsZero() {
			t.Errorf("Timestamp(%q) did not parse", s)
		}
	}
	if !Timestamp("").Time().IsZero() {
		t.Error("empty timestamp must be zero")
	}
}
