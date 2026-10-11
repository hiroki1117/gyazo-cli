package gyazo

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// Access policies of an uploaded image.
const (
	// AccessPolicyAnyone lets anyone with the URL view the image (the server default).
	AccessPolicyAnyone = "anyone"
	// AccessPolicyOnlyMe lets only the uploader view the image.
	AccessPolicyOnlyMe = "only_me"
)

// AccessPolicies are the valid access policies.
var AccessPolicies = []string{AccessPolicyAnyone, AccessPolicyOnlyMe}

// UploadFile is the content to upload.
type UploadFile struct {
	// Name is the file name sent to the server. The API requires one.
	Name string
	Body io.Reader
	// Size is the size of Body in bytes, or -1 if unknown (Body is then read into memory).
	Size int64
}

// UploadOptions are optional metadata of an upload. Zero values are not sent.
type UploadOptions struct {
	// AccessPolicy is AccessPolicyAnyone or AccessPolicyOnlyMe.
	AccessPolicy string
	// MetadataIsPublic controls whether the source URL and title are public. nil leaves it to the server.
	MetadataIsPublic *bool
	// RefererURL is the URL of the page the image was captured from.
	RefererURL string
	// App is the name of the application that captured the image.
	App string
	// Title is the title of the page the image was captured from.
	Title string
	// Desc is a description, searchable in Gyazo.
	Desc string
	// CreatedAt is when the image was captured.
	CreatedAt time.Time
	// CollectionID adds the image to a collection.
	CollectionID string
}

// fields returns the form fields in a stable order.
func (o *UploadOptions) fields() [][2]string {
	if o == nil {
		return nil
	}
	var fs [][2]string
	add := func(k, v string) {
		if v != "" {
			fs = append(fs, [2]string{k, v})
		}
	}
	add("access_policy", o.AccessPolicy)
	if o.MetadataIsPublic != nil {
		add("metadata_is_public", strconv.FormatBool(*o.MetadataIsPublic))
	}
	add("referer_url", o.RefererURL)
	add("app", o.App)
	add("title", o.Title)
	add("desc", o.Desc)
	if !o.CreatedAt.IsZero() {
		add("created_at", strconv.FormatFloat(float64(o.CreatedAt.UnixMilli())/1000, 'f', -1, 64))
	}
	add("collection_id", o.CollectionID)
	return fs
}

// Upload uploads an image and returns it. Uploads are never retried, so an image is never uploaded twice.
//
// POST https://upload.gyazo.com/api/upload
func (s *ImagesService) Upload(ctx context.Context, file UploadFile, opts *UploadOptions) (*Image, error) {
	if file.Name == "" {
		return nil, fmt.Errorf("upload: file name is empty")
	}
	if file.Size < 0 {
		b, err := io.ReadAll(file.Body)
		if err != nil {
			return nil, fmt.Errorf("upload: read %s: %w", file.Name, err)
		}
		file.Body, file.Size = bytes.NewReader(b), int64(len(b))
	}
	body, contentType, size, err := multipartBody(file, opts.fields())
	if err != nil {
		return nil, err
	}
	req, err := s.client.newAPIRequest(ctx, http.MethodPost, s.client.uploadURL, "api/upload", nil, body)
	if err != nil {
		return nil, err
	}
	// Set the length so the body is not sent chunked; the file is streamed, not read into memory.
	req.ContentLength = size
	req.Header.Set("Content-Type", contentType)

	var out Image
	if _, err := s.client.doJSON(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

var quoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// multipartBody builds a multipart/form-data body that streams the file between
// a pre-rendered header and trailer, and returns its exact length.
func multipartBody(file UploadFile, fields [][2]string) (io.Reader, string, int64, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range fields {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			return nil, "", 0, err
		}
	}

	// Detect the type from the content: file extensions are often missing (stdin) or wrong.
	br := bufio.NewReaderSize(file.Body, 512)
	sniff, err := br.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, "", 0, fmt.Errorf("upload: read %s: %w", file.Name, err)
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="imagedata"; filename="%s"`, quoteEscaper.Replace(file.Name)))
	h.Set("Content-Type", http.DetectContentType(sniff))
	if _, err := mw.CreatePart(h); err != nil {
		return nil, "", 0, err
	}
	head := bytes.Clone(buf.Bytes())

	buf.Reset()
	if err := mw.Close(); err != nil {
		return nil, "", 0, err
	}
	tail := buf.Bytes()

	size := int64(len(head)) + file.Size + int64(len(tail))
	return io.MultiReader(bytes.NewReader(head), br, bytes.NewReader(tail)), mw.FormDataContentType(), size, nil
}
