// Package downloader moves one model file from a mirror to disk, with resume,
// segmentation, retries and integrity checks.
//
// Two invariants hold at all times:
//   - the destination path never exists in a partially written state;
//   - a resumed download never mixes bytes from two different file versions.
package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ggufget/internal/fsutil"
	"ggufget/internal/logx"
	"ggufget/internal/netx"
	"ggufget/internal/termui"
	"ggufget/internal/units"
)

// VerifyMode selects how much integrity checking to do.
type VerifyMode int

const (
	// VerifyAuto hashes small files and any download that was resumed or retried.
	VerifyAuto VerifyMode = iota
	VerifySize
	VerifySHA256
	VerifyOff
)

// ParseVerify parses the --verify value.
func ParseVerify(s string) (VerifyMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return VerifyAuto, true
	case "size":
		return VerifySize, true
	case "sha256", "sha":
		return VerifySHA256, true
	case "off", "none", "no":
		return VerifyOff, true
	}
	return VerifyAuto, false
}

// Sentinel errors the CLI maps onto exit codes.
var (
	ErrRateLimited = errors.New("rate limited")
	ErrInterrupted = errors.New("interrupted")
	ErrDiskFull    = errors.New("not enough disk space")
	ErrExists      = errors.New("destination exists")
	ErrNoHash      = errors.New("no expected sha256")
	ErrVerify      = errors.New("verification failed")
	errIdle        = errors.New("connection stalled")
)

// Options tunes a download.
type Options struct {
	// Connections is the number of parallel range requests; 0 means auto.
	Connections int
	Retries     int
	Verify      VerifyMode
	Overwrite   bool
	Client      *http.Client
	Progress    *termui.Reporter
	Verbose     bool
	Log         io.Writer
	// SegMinSize is the smallest file worth splitting; bigger does not help.
	SegMinSize int64
	// IdleTimeout aborts a connection that stops producing bytes.
	IdleTimeout time.Duration

	OnResume    func(haveByte int64)
	OnRetry     func(attempt int, wait time.Duration, err error)
	OnRateLimit func(conns int)
	OnVerify    func()
}

// Task is one file to fetch.
type Task struct {
	URL      string
	Dest     string
	Expected int64
	SHA256   string
	Display  string
}

// Result reports what happened.
type Result struct {
	Path    string
	Bytes   int64
	Skipped bool
	Resumed bool
	Elapsed time.Duration
}

func (o *Options) add(n int64) {
	if o.Progress != nil && n > 0 {
		o.Progress.Add(n)
	}
}

func (o *Options) setConns(n int) {
	if o.Progress != nil {
		o.Progress.SetConns(n)
	}
}

func (o *Options) setNote(s string) {
	if o.Progress != nil {
		o.Progress.SetNote(s)
	}
}

func (o *Options) logf(format string, a ...any) {
	if o.Log != nil {
		fmt.Fprintf(o.Log, format+"\n", a...)
	}
}

type fetch struct {
	opts      *Options
	task      Task
	part      string
	statePath string
	total     int64
	etag      string
	accept    bool
	resumed   bool
	retried   atomic.Bool
}

// markRetried records that bytes from more than one response may end up in the
// same file, which is what makes a final hash check worth its cost.
func (f *fetch) markRetried() { f.retried.Store(true) }

// Run downloads one file.
func Run(ctx context.Context, opts *Options, t Task) (*Result, error) {
	start := time.Now()
	if opts == nil {
		opts = &Options{}
	}
	if opts.Client == nil {
		opts.Client = netx.New(netx.Options{})
	}
	if opts.Retries < 0 {
		opts.Retries = 0
	}
	if opts.Connections < 0 {
		opts.Connections = 0
	}
	if opts.SegMinSize <= 0 {
		opts.SegMinSize = 64 << 20
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 15 * time.Second
	}

	f := &fetch{
		opts:      opts,
		task:      t,
		part:      t.Dest + ".part",
		statePath: t.Dest + ".part.seg",
	}
	dir := filepath.Dir(t.Dest)
	if err := fsutil.EnsureDir(dir); err != nil {
		// This is the error that made "mkdir …: Access is denied." impossible to
		// diagnose: the terminal shows the message, but not which directory,
		// which errno, or what else was in the error chain.
		opts.logf("mkdir failed dir=%q errno=%s chain=%s", dir, logx.Errno(err), logx.Chain(err))
		return nil, err
	}

	info, err := probe(ctx, opts.Client, t.URL)
	if err != nil {
		opts.logf("probe failed url=%q errno=%s chain=%s", t.URL, logx.Errno(err), logx.Chain(err))
		return nil, err
	}
	f.total, f.etag, f.accept = info.Total, info.ETag, info.AcceptRanges
	opts.logf("probe url=%q status=%d total=%d etag=%q ranges=%v content_length=%d",
		t.URL, info.Status, info.Total, f.etag, f.accept, info.ContentLength)
	if f.total <= 0 {
		f.total = t.Expected
	}
	if f.total <= 0 {
		return nil, errors.New("mirror did not report a file size")
	}

	// Already downloaded?
	if size, ok, statErr := fsutil.Size(t.Dest); statErr == nil && ok {
		if opts.Overwrite {
			os.Remove(fsutil.LongPath(t.Dest))
		} else if size == f.total {
			if err := verifyExisting(ctx, opts, t.Dest, f.total, t.SHA256); err != nil {
				return nil, err
			}
			return &Result{Path: t.Dest, Bytes: size, Skipped: true, Elapsed: time.Since(start)}, nil
		} else {
			return nil, fmt.Errorf("%w: %s (%s, expected %s)", ErrExists, t.Dest,
				units.Bytes(size), units.Bytes(f.total))
		}
	}

	have := int64(0)
	if st, statErr := os.Stat(fsutil.LongPath(f.part)); statErr == nil {
		have = st.Size()
	}
	if have > f.total {
		os.Truncate(fsutil.LongPath(f.part), 0)
		have = 0
	}
	if have > 0 {
		f.resumed = true
		opts.logf("resume part=%q have=%d total=%d", f.part, have, f.total)
		if opts.OnResume != nil {
			opts.OnResume(have)
		}
		opts.setNote("resume " + units.Bytes(have))
	}

	if free, freeErr := fsutil.DiskFree(filepath.Dir(t.Dest)); freeErr == nil {
		need := uint64(f.total-have) + 64<<20
		opts.logf("disk dir=%q free=%d need=%d", filepath.Dir(t.Dest), free, need)
		if free < need {
			return nil, fmt.Errorf("%w: need %s, free %s", ErrDiskFull,
				units.Bytes(int64(need)), units.Bytes(int64(free)))
		}
	}

	conns := opts.Connections
	if conns == 0 {
		conns = 4
	}
	if !f.accept || f.total < opts.SegMinSize || conns < 2 {
		conns = 1
	}
	opts.setConns(conns)
	opts.logf("download conns=%d total=%d ranges=%v", conns, f.total, f.accept)

	if conns > 1 {
		err = f.segmented(ctx, conns)
	} else {
		err = f.single(ctx)
	}
	if err != nil {
		opts.logf("download failed errno=%s chain=%s", logx.Errno(err), logx.Chain(err))
		if ctx.Err() != nil {
			return nil, ErrInterrupted
		}
		return nil, err
	}
	return f.finish(ctx, start)
}

// finish verifies the assembled file and moves it into place.
func (f *fetch) finish(ctx context.Context, start time.Time) (*Result, error) {
	opts := f.opts
	st, err := os.Stat(fsutil.LongPath(f.part))
	if err != nil {
		opts.logf("finish stat failed part=%q errno=%s chain=%s", f.part, logx.Errno(err), logx.Chain(err))
		return nil, err
	}
	if st.Size() != f.total {
		return nil, fmt.Errorf("size mismatch after download: %d != %d", st.Size(), f.total)
	}
	if opts.OnVerify != nil {
		opts.OnVerify()
	}
	if err := verifyPart(ctx, opts, f.part, f.total, f.task.SHA256, f.resumed || f.retried.Load()); err != nil {
		opts.logf("verify failed part=%q errno=%s chain=%s", f.part, logx.Errno(err), logx.Chain(err))
		return nil, err
	}
	if err := fsutil.Rename(fsutil.LongPath(f.part), fsutil.LongPath(f.task.Dest)); err != nil {
		opts.logf("rename failed from=%q to=%q errno=%s chain=%s",
			f.part, f.task.Dest, logx.Errno(err), logx.Chain(err))
		return nil, err
	}
	opts.logf("done dest=%q bytes=%d resumed=%v elapsed=%s", f.task.Dest, f.total, f.resumed, time.Since(start).Round(time.Millisecond))
	os.Remove(fsutil.LongPath(f.statePath))
	return &Result{
		Path:    f.task.Dest,
		Bytes:   f.total,
		Resumed: f.resumed,
		Elapsed: time.Since(start),
	}, nil
}

// wantsHash decides whether the integrity check is worth a second full read.
//
// A completed range download whose length already matches is strong evidence on
// its own; hashing matters for small files and whenever a resume or a retry
// means bytes from more than one response ended up in the same file.
func wantsHash(mode VerifyMode, size int64, expect string, risky bool) bool {
	switch mode {
	case VerifyOff, VerifySize:
		return false
	case VerifySHA256:
		return true
	}
	if expect == "" {
		return false
	}
	return size <= 4<<30 || risky
}

func verifyPart(ctx context.Context, opts *Options, path string, size int64, expect string, risky bool) error {
	if !wantsHash(opts.Verify, size, expect, risky) {
		return nil
	}
	if expect == "" {
		if opts.Verify == VerifySHA256 {
			return ErrNoHash
		}
		return nil
	}
	got, err := hashFile(ctx, path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, expect) {
		os.Remove(fsutil.LongPath(path))
		return fmt.Errorf("%w: sha256 mismatch: expected %s, got %s", ErrVerify, expect, got)
	}
	return nil
}

func verifyExisting(ctx context.Context, opts *Options, path string, size int64, expect string) error {
	if expect == "" || opts.Verify == VerifyOff || opts.Verify == VerifySize {
		return nil
	}
	got, err := hashFile(ctx, path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, expect) {
		return fmt.Errorf("%w: %s has sha256 %s, expected %s", ErrVerify, path, got, expect)
	}
	return nil
}

// hashFile streams the file so a 10 GiB model never lands in memory.
func hashFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(fsutil.LongPath(path))
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// backoff implements exponential backoff with jitter, honouring Retry-After.
func (f *fetch) backoff(ctx context.Context, attempt int, retryAfter time.Duration) bool {
	d := retryAfter
	if d <= 0 {
		shift := attempt - 1
		if shift > 5 {
			shift = 5
		}
		d = time.Duration(500<<uint(shift)) * time.Millisecond
		jitter := time.Duration(rand.Int63n(int64(d)/2 + 1))
		d += jitter
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	if f.opts.OnRetry != nil {
		f.opts.OnRetry(attempt, d, nil)
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// sleep waits for d and reports whether the context is still alive.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// httpErr maps a status code onto the package's sentinels or a readable error.
func httpErr(status int, url, body string) error {
	switch status {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return ErrRateLimited
	}
	return fmt.Errorf("HTTP %d from %s: %s", status, url, summarize(body))
}

func summarize(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

// retryAfter reads the Retry-After header in seconds form.
func retryAfter(h http.Header) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	var secs int
	if _, err := fmt.Sscanf(v, "%d", &secs); err == nil && secs > 0 && secs <= 300 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// watchdog cancels a request whose body stops producing bytes. Without it a
// half-open TCP connection looks exactly like a very slow mirror.
type watchdog struct {
	cancel context.CancelFunc
	last   atomic.Int64
	done   chan struct{}
	once   sync.Once
}

func startWatchdog(parent context.Context, idle time.Duration) (context.Context, *watchdog) {
	if idle <= 0 {
		idle = 15 * time.Second
	}
	ctx, cancel := context.WithCancel(parent)
	w := &watchdog{cancel: cancel, done: make(chan struct{})}
	w.last.Store(time.Now().UnixNano())
	go func() {
		t := time.NewTicker(idle / 3)
		defer t.Stop()
		for {
			select {
			case <-w.done:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if time.Since(time.Unix(0, w.last.Load())) > idle {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, w
}

func (w *watchdog) touch() { w.last.Store(time.Now().UnixNano()) }

func (w *watchdog) stop() {
	w.once.Do(func() {
		close(w.done)
		w.cancel()
	})
}
