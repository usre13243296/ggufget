package downloader

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"ggufget/internal/netx"
)

// rangeInfo is what a one-byte Range request tells us about the object.
type rangeInfo struct {
	Total         int64
	ETag          string
	AcceptRanges  bool
	Status        int
	ContentLength int64
}

// probe asks for the first byte. A 206 answer proves the server supports
// ranges and reveals the exact size, which is the foundation of every resume
// and segmentation decision.
func probe(ctx context.Context, c *http.Client, url string) (*rangeInfo, error) {
	req, err := netx.NewRequest(url, map[string]string{"Range": "bytes=0-0"})
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Do not read the body: a server that ignores Range would stream the whole
	// model at us.
	io.CopyN(io.Discard, resp.Body, 0)

	info := &rangeInfo{
		ETag:          strings.Trim(resp.Header.Get("ETag"), `"`),
		Status:        resp.StatusCode,
		ContentLength: resp.ContentLength,
	}

	switch resp.StatusCode {
	case http.StatusPartialContent:
		info.AcceptRanges = true
		if _, _, total, ok := parseContentRange(resp.Header.Get("Content-Range")); ok {
			info.Total = total
		}
	case http.StatusOK:
		// Range ignored: the server sent the whole object from byte zero.
		info.AcceptRanges = false
		info.Total = resp.ContentLength
		if info.Total <= 0 {
			if v := resp.Header.Get("Content-Length"); v != "" {
				if n, err := strconv.ParseInt(v, 10, 64); err == nil {
					info.Total = n
				}
			}
		}
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, httpErr(resp.StatusCode, url, string(body))
	}
	return info, nil
}

// parseContentRange understands "bytes 0-0/12345" and "bytes */12345".
func parseContentRange(s string) (start, end, total int64, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "bytes") {
		return 0, 0, 0, false
	}
	s = strings.TrimSpace(strings.TrimPrefix(s, "bytes"))
	rangePart, totalPart, found := strings.Cut(s, "/")
	if !found {
		return 0, 0, 0, false
	}
	if totalPart != "*" {
		n, err := strconv.ParseInt(totalPart, 10, 64)
		if err != nil {
			return 0, 0, 0, false
		}
		total = n
	}
	if rangePart != "*" {
		a, b, found := strings.Cut(rangePart, "-")
		if !found {
			return 0, 0, total, false
		}
		x, err1 := strconv.ParseInt(strings.TrimSpace(a), 10, 64)
		y, err2 := strconv.ParseInt(strings.TrimSpace(b), 10, 64)
		if err1 != nil || err2 != nil {
			return 0, 0, total, false
		}
		return x, y, total, true
	}
	return 0, 0, total, true
}
