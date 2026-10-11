package gyazo

import "time"

// Image is an image (or video) returned by the image APIs.
// JSON field names follow the API, so -o json keeps the API structure.
type Image struct {
	ImageID      string    `json:"image_id"`
	PermalinkURL string    `json:"permalink_url"`
	ThumbURL     string    `json:"thumb_url"`
	URL          string    `json:"url"`
	Type         string    `json:"type"`
	CreatedAt    Timestamp `json:"created_at,omitempty"`
	Metadata     *Metadata `json:"metadata,omitempty"`
	OCR          *OCR      `json:"ocr,omitempty"`
	// VideoLength (milliseconds) and MP4URL are set for videos.
	VideoLength int    `json:"video_length,omitempty"`
	MP4URL      string `json:"mp4_url,omitempty"`
}

// Title returns the title of the page the image was captured from, falling back to the app name.
func (img *Image) Title() string {
	if img.Metadata == nil {
		return ""
	}
	if img.Metadata.Title != "" {
		return img.Metadata.Title
	}
	return img.Metadata.App
}

// Metadata is the capture context recorded with an image.
type Metadata struct {
	App   string `json:"app"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Desc  string `json:"desc"`
}

// OCR is the text recognized in an image.
type OCR struct {
	Locale      string `json:"locale"`
	Description string `json:"description"`
}

// ImageList is a page of images. The API returns the counts as headers; they are kept here.
type ImageList struct {
	TotalCount int     `json:"total_count"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	UserType   string  `json:"user_type,omitempty"`
	Images     []Image `json:"images"`
}

// DeletedImage is the response of the delete API.
type DeletedImage struct {
	ImageID string `json:"image_id"`
	Type    string `json:"type"`
}

// User is the authenticated user.
type User struct {
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	ProfileImage string `json:"profile_image"`
}

// Timestamp is a time string returned by the API, kept as-is in JSON.
// The API uses several layouts ("2014-05-21 14:23:10+0900", "2018-07-24T07:33:24.771Z").
type Timestamp string

var timestampLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05-0700",
	"2006-01-02T15:04:05-0700",
	"2006-01-02 15:04:05Z07:00",
}

// Time parses t. It returns the zero time if t is empty or in an unknown layout.
func (t Timestamp) Time() time.Time {
	for _, layout := range timestampLayouts {
		if v, err := time.Parse(layout, string(t)); err == nil {
			return v
		}
	}
	return time.Time{}
}

// DefaultPermalinkHost is where image pages are served when the API returns no permalink.
const DefaultPermalinkHost = "https://gyazo.com"

// Permalink returns the URL of the image page: the permalink from the API, or one built from the ID.
func (img *Image) Permalink() string {
	if img.PermalinkURL != "" {
		return img.PermalinkURL
	}
	return DefaultPermalinkHost + "/" + img.ImageID
}
