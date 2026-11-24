// Package provider talks to the three model hubs ggufget supports. Each hub is
// behind the same small interface so the rest of the program never branches on
// which mirror it is talking to.
package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// File is one downloadable object inside a repository.
type File struct {
	// Path is slash separated and relative to the repository root.
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	LFS    bool   `json:"lfs,omitempty"`
}

// Base returns the last path component.
func (f File) Base() string {
	if i := strings.LastIndexByte(f.Path, '/'); i >= 0 {
		return f.Path[i+1:]
	}
	return f.Path
}

// IsGGUF reports whether the file looks like a quantized model.
func (f File) IsGGUF() bool { return strings.HasSuffix(strings.ToLower(f.Path), ".gguf") }

// Meta describes a repository at one revision.
type Meta struct {
	Repo    string
	Commit  string
	Branch  string
	Gated   bool
	Private bool
	License string
}

// Provider is one hub.
type Provider interface {
	// Name is the identifier used by --provider and in messages.
	Name() string
	// Host is the base URL, for display and logging.
	Host() string
	// Probe reports whether the repo exists and at which commit.
	Probe(ctx context.Context, repo string) (*Meta, error)
	// List returns the files at rev; an empty rev means the default branch.
	List(ctx context.Context, repo, rev string) (*Meta, []File, error)
	// ResolveURL is the download URL for one file.
	ResolveURL(repo, rev, path string) string
}

// ErrNotFound means the hub answered 404: wrong repo name, or the mirror simply
// does not carry it.
var ErrNotFound = errors.New("not found")

// ErrAuthRequired means the repo is private or gated.
var ErrAuthRequired = errors.New("authentication required")

// HTTPError keeps the status code so callers can map it to an exit code.
type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.Status, e.URL)
}

// MapStatus converts a status code into the sentinel errors above.
func MapStatus(status int, url, body string) error {
	switch {
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, url)
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrAuthRequired, url)
	default:
		return &HTTPError{Status: status, URL: url, Body: summarize(body)}
	}
}

func summarize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// EscapePath escapes each segment of a repo-relative path but keeps the
// separators, because mirrors reject pre-escaped slashes.
func EscapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// SortFiles orders files the way the interactive UI shows them.
type SortMode int

const (
	SortName SortMode = iota
	SortSize
	SortQuant
)
