package gyazo

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var imageIDPattern = regexp.MustCompile(`^[0-9A-Za-z]+$`)

// ParseImageID extracts an image ID from an image ID or a Gyazo URL:
//
//	8980c52421e452ac3355ca3e5cfe7a0c
//	https://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c
//	https://gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c/raw
//	https://i.gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c.png
//	https://<team>.gyazo.com/8980c52421e452ac3355ca3e5cfe7a0c
func ParseImageID(s string) (string, error) {
	id := s
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("invalid image URL %q: %w", s, err)
		}
		if host := u.Hostname(); host != "gyazo.com" && !strings.HasSuffix(host, ".gyazo.com") {
			return "", fmt.Errorf("invalid image URL %q: not a gyazo.com URL", s)
		}
		first, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
		id = strings.TrimSuffix(first, path.Ext(first))
	}
	if !imageIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid image ID or URL %q", s)
	}
	return id, nil
}
