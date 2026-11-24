package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"ggufget/internal/fsutil"
	"ggufget/internal/logx"
	"ggufget/internal/netx"
	"ggufget/internal/units"
)

// rateLimitError carries Retry-After to the retry loop while still matching
// ErrRateLimited with errors.Is.
type rateLimitError struct{ after time.Duration }

func (e *rateLimitError) Error() string { return "mirror rate limited the download" }

func (e *rateLimitError) Is(target error) bool { return target == ErrRateLimited }

func retryAfterOf(err error) time.Duration {
	var rl *rateLimitError
	if errors.As(err, &rl) {
		return rl.after
	}
	return 0
}

// segState is the resume journal for a segmented download. It is written
// atomically every couple of seconds, so a crash costs at most the segment
// bytes in flight.
type segState struct {
	Version  int        `json:"version"`
	Total    int64      `json:"total"`
	ETag     string     `json:"etag"`
	Segments []segSaved `json:"segments"`
}

type segSaved struct {
	Start  int64 `json:"start"`
	Length int64 `json:"length"`
	Done   int64 `json:"done"`
}

type segment struct {
	start  int64
	length int64
	done   atomic.Int64
}

// splitSegments cuts total into n roughly equal ranges; the last one takes the
// remainder so the sum is always exact.
func splitSegments(total int64, n int) []segSaved {
	if n < 1 {
		n = 1
	}
	if total <= 0 {
		return nil
	}
	base := total / int64(n)
	out := make([]segSaved, 0, n)
	var off int64
	for i := 0; i < n; i++ {
		length := base
		if i == n-1 {
			length = total - off
		}
		if length <= 0 {
			break
		}
		out = append(out, segSaved{Start: off, Length: length})
		off += length
	}
	return out
}

func (f *fetch) segmented(parent context.Context, conns int) error {
	file, err := os.OpenFile(fsutil.LongPath(f.part), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	saved := splitSegments(f.total, conns)
	prev, havePrev := loadState(f.statePath)
	switch {
	case havePrev && prev.Total == f.total && prev.ETag == f.etag && len(prev.Segments) > 0:
		saved = prev.Segments
		f.opts.logf("resuming %d segments from journal", len(saved))
	case havePrev:
		// A journal from a different revision is worse than none: it would resume
		// into bytes of another file, so the reason it was dropped is worth a line.
		f.opts.logf("journal ignored total=%d/%d etag=%q/%q segments=%d",
			prev.Total, f.total, prev.ETag, f.etag, len(prev.Segments))
	}
	segs := make([]*segment, 0, len(saved))
	var already int64
	for _, s := range saved {
		seg := &segment{start: s.Start, length: s.Length}
		d := s.Done
		if d < 0 {
			d = 0
		}
		if d > s.Length {
			d = s.Length
		}
		seg.done.Store(d)
		already += d
		segs = append(segs, seg)
	}
	if already > 0 {
		f.resumed = true
		if f.opts.OnResume != nil {
			f.opts.OnResume(already)
		}
		f.opts.setNote("resume " + units.Bytes(already))
	}
	if err := file.Truncate(f.total); err != nil {
		f.opts.logf("truncate failed part=%q size=%d errno=%s chain=%s",
			f.part, f.total, logx.Errno(err), logx.Chain(err))
		return err
	}

	inFlight := conns
	if inFlight > len(segs) {
		inFlight = len(segs)
	}
	g := newGate(inFlight)
	f.opts.setConns(g.current())

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	stopSave := make(chan struct{})
	var saveWG sync.WaitGroup
	saveWG.Add(1)
	go func() {
		defer saveWG.Done()
		saveStateLoop(f.statePath, f.total, f.etag, segs, stopSave)
	}()

	var wg sync.WaitGroup
	errs := make(chan error, len(segs))
	for _, seg := range segs {
		wg.Add(1)
		go func(seg *segment) {
			defer wg.Done()
			if err := f.downloadSegment(ctx, file, seg, g); err != nil {
				errs <- err
				cancel()
			}
		}(seg)
	}
	wg.Wait()
	close(stopSave)
	saveWG.Wait()

	// Persist whatever did arrive, even on the error path.
	saveState(f.statePath, f.total, f.etag, segs)

	select {
	case err := <-errs:
		if parent.Err() != nil {
			return parent.Err()
		}
		return err
	default:
	}
	for i, seg := range segs {
		if got, want := seg.done.Load(), seg.length; got != want {
			return fmt.Errorf("segment %d incomplete: %d/%d", i+1, got, want)
		}
	}
	return file.Sync()
}

func (f *fetch) downloadSegment(ctx context.Context, file *os.File, seg *segment, g *gate) error {
	attempt := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if seg.done.Load() >= seg.length {
			return nil
		}
		if !g.acquire(ctx) {
			return ctx.Err()
		}
		err := f.segAttempt(ctx, file, seg)
		g.release()
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrRateLimited) {
			limit := g.halve()
			f.opts.setConns(limit)
			if f.opts.OnRateLimit != nil {
				f.opts.OnRateLimit(limit)
			}
		}
		if errors.Is(err, context.Canceled) {
			return ctx.Err()
		}
		attempt++
		if attempt > f.opts.Retries {
			return err
		}
		f.markRetried()
		f.opts.logf("segment %d retry %d after %v", seg.start, attempt, err)
		if !f.backoff(ctx, attempt, retryAfterOf(err)) {
			return ctx.Err()
		}
	}
}

func (f *fetch) segAttempt(parent context.Context, file *os.File, seg *segment) error {
	done := seg.done.Load()
	if done >= seg.length {
		return nil
	}
	start := seg.start + done
	end := seg.start + seg.length - 1

	ctx, wd := startWatchdog(parent, f.opts.IdleTimeout)
	defer wd.stop()

	req, err := netx.NewRequest(f.task.URL, map[string]string{
		"Range": fmt.Sprintf("bytes=%d-%d", start, end),
	})
	if err != nil {
		return err
	}
	resp, err := f.opts.Client.Do(req.WithContext(ctx))
	if err != nil {
		if parent.Err() != nil {
			return parent.Err()
		}
		if ctx.Err() != nil {
			return errIdle
		}
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		return fmt.Errorf("mirror ignored the Range request for byte %d", start)
	case http.StatusRequestedRangeNotSatisfiable:
		// The segment is already complete on the server side.
		seg.done.Store(seg.length)
		return nil
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return &rateLimitError{after: retryAfter(resp.Header)}
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, f.task.URL, summarize(string(body)))
	}

	want := seg.length - done
	buf := make([]byte, 256<<10)
	var got int64
	for got < want {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.WriteAt(buf[:n], start+got); werr != nil {
				return werr
			}
			got += int64(n)
			seg.done.Store(done + got)
			wd.touch()
			f.opts.add(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if parent.Err() != nil {
				return parent.Err()
			}
			if ctx.Err() != nil {
				return errIdle
			}
			return rerr
		}
	}
	if got < want {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func saveState(path string, total int64, etag string, segs []*segment) {
	if path == "" {
		return
	}
	st := segState{Version: 1, Total: total, ETag: etag, Segments: make([]segSaved, 0, len(segs))}
	for _, s := range segs {
		st.Segments = append(st.Segments, segSaved{
			Start:  s.start,
			Length: s.length,
			Done:   s.done.Load(),
		})
	}
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = fsutil.WriteFileAtomic(path, b, 0o644)
}

func saveStateLoop(path string, total int64, etag string, segs []*segment, stop <-chan struct{}) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			saveState(path, total, etag, segs)
		}
	}
}

func loadState(path string) (*segState, bool) {
	b, err := os.ReadFile(fsutil.LongPath(path))
	if err != nil || len(b) == 0 {
		return nil, false
	}
	var st segState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, false
	}
	if st.Total <= 0 || len(st.Segments) == 0 {
		return nil, false
	}
	return &st, true
}
