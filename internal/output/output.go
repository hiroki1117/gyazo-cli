// Package output prints command results as text or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// Format is an output format.
type Format string

const (
	// Text is formatted text for humans.
	Text Format = "text"
	// JSON keeps the API response structure. Use it with scripts and jq.
	JSON Format = "json"
)

var formats = []Format{Text, JSON}

// ParseFormat parses a Format.
func ParseFormat(s string) (Format, error) {
	for _, f := range formats {
		if Format(s) == f {
			return f, nil
		}
	}
	return "", fmt.Errorf("unknown output format %q (available: %v)", s, formats)
}

// Printer prints values in its Format.
type Printer struct {
	w      io.Writer
	format Format
}

// NewPrinter creates a Printer.
func NewPrinter(w io.Writer, format Format) *Printer {
	return &Printer{w: w, format: format}
}

// Print prints v as JSON in JSON format, or calls text in Text format.
func (p *Printer) Print(v any, text func(w io.Writer) error) error {
	if p.format == JSON {
		enc := json.NewEncoder(p.w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(v)
	}
	return text(p.w)
}

// Table returns a tabwriter that aligns tab-separated columns. Callers must Flush it.
func Table(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

// Time formats t in local time for lists. The zero time is "-".
func Time(t time.Time) string {
	if t.IsZero() || t.Unix() == 0 {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}
