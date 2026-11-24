package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"ggufget/internal/fsutil"
	"ggufget/internal/netx"
)

// single downloads with one connection, resuming from whatever .part already
// holds. This is the path for small files and for mirrors that cannot serve
// ranges.
func (f *fetch) single(ctx context.Context) error {
	file, err := os.OpenFile(fsutil.LongPath(f.part), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	off, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if off > f.total {
		if err := file.Truncate(0); err != nil {
			return err
		}
		off = 0
	}

	attempt := 0
	for {
		attempt++
		if attempt > 1 {
			f.markRetried()
		}
		newOff, err := f.singleAttempt(ctx, file, off)
		off = newOff
		if err == nil && off >= f.total {
			return file.Sync()
		}
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt > f.opts.Retries {
			return err
		}
		f.opts.logf("single retry %d after %v", attempt, err)
		if !f.backoff(ctx, attempt, 0) {
			return ctx.Err()
		}
	}
}

// singleAttempt performs one ranged GET and returns the absolute offset the
// file reached. A server that ignores Range restarts the file from zero.
func (f *fetch) singleAttempt(parent context.Context, file *os.File, off int64) (int64, error) {
	ctx, wd := startWatchdog(parent, f.opts.IdleTimeout)
	defer wd.stop()

	req, err := netx.NewRequest(f.task.URL, map[string]string{
		"Range": fmt.Sprintf("bytes=%d-", off),
	})
	if err != nil {
		return off, err
	}
	resp, err := f.opts.Client.Do(req.WithContext(ctx))
	if err != nil {
		if parent.Err() != nil {
			return off, parent.Err()
		}
		if ctx.Err() != nil {
			return off, errIdle
		}
		return off, err
	}
	defer resp.Body.Close()

	expected := f.total - off
	switch resp.StatusCode {
	case http.StatusPartialContent:
		// Resume as planned.
	case http.StatusOK:
		// The mirror ignored Range and is sending the whole file.
		if err := file.Truncate(0); err != nil {
			return off, err
		}
		off = 0
		expected = f.total
	case http.StatusRequestedRangeNotSatisfiable:
		return off, nil
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return off, &rateLimitError{after: retryAfter(resp.Header)}
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return off, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, f.task.URL, summarize(string(body)))
	}

	buf := make([]byte, 256<<10)
	var written int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.WriteAt(buf[:n], off+written); werr != nil {
				return off + written, werr
			}
			written += int64(n)
			wd.touch()
			f.opts.add(int64(n))
			if expected > 0 && written >= expected {
				break
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if parent.Err() != nil {
				return off + written, parent.Err()
			}
			if ctx.Err() != nil {
				return off + written, errIdle
			}
			return off + written, rerr
		}
	}
	if expected > 0 && written < expected {
		return off + written, io.ErrUnexpectedEOF
	}
	return off + written, nil
}
