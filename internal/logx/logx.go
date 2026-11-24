// Package logx writes the diagnostic log ggufget keeps next to its binary.
//
// The log is on by default: when a download fails, the log must already
// contain enough context (paths, status codes, errno values) that the failure
// can be explained without guessing. It never blocks a command: if no
// candidate path is writable the program keeps running without a log.
package logx

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Level orders messages from least to most verbose.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

var levelNames = [...]string{"ERROR", "WARN", "INFO", "DEBUG"}

func (l Level) String() string {
	if int(l) < 0 || int(l) >= len(levelNames) {
		return "INFO"
	}
	return levelNames[l]
}

// ParseLevel accepts the names used on the command line.
func ParseLevel(s string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error", "err":
		return LevelError, true
	case "warn", "warning":
		return LevelWarn, true
	case "info", "":
		return LevelInfo, true
	case "debug", "trace":
		return LevelDebug, true
	}
	return LevelInfo, false
}

// DefaultMaxBytes is the size at which the log is rotated once.
const DefaultMaxBytes = 4 << 20

// Options controls where and how the log is written.
type Options struct {
	// Path forces one exact file. When empty the candidate chain is used.
	Path string
	// Dir is the binary's directory, the first candidate of the chain.
	Dir string
	// Level drops messages more verbose than this.
	Level Level
	// MaxBytes rotates the log once it would grow past this size.
	MaxBytes int64
	// Disabled turns logging off entirely.
	Disabled bool
}

// Attempt records one candidate path that could not be opened. Callers surface
// these when no log could be created, so "why is there no log" has an answer.
type Attempt struct {
	Path string
	Err  error
}

// Logger is a small, concurrency-safe, line oriented file logger.
type Logger struct {
	mu     sync.Mutex
	w      *os.File
	path   string
	level  Level
	max    int64
	size   int64
	closed bool
}

// Enabled reports whether l is usable.
func (l *Logger) Enabled() bool { return l != nil && !l.closed }

// Path returns the file being written, or "" when logging is off.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Open creates the log, trying each candidate in turn. A nil logger with no
// attempts means logging was disabled; a nil logger with attempts means every
// location failed (the caller may print them).
func Open(o Options) (*Logger, []Attempt) {
	if o.Disabled {
		return nil, nil
	}
	if o.Level < LevelError || o.Level > LevelDebug {
		o.Level = LevelInfo
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = DefaultMaxBytes
	}

	var candidates []string
	if p := strings.TrimSpace(o.Path); p != "" {
		candidates = []string{expand(p)}
	} else {
		candidates = defaults(o.Dir)
	}

	var attempts []Attempt
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		f, err := openLogFile(cand)
		if err != nil {
			attempts = append(attempts, Attempt{Path: cand, Err: err})
			continue
		}
		st, _ := f.Stat()
		l := &Logger{w: f, path: cand, level: o.Level, max: o.MaxBytes}
		if st != nil {
			l.size = st.Size()
		}
		return l, attempts
	}
	return nil, attempts
}

// defaults is the candidate chain: next to the binary first (so the file is
// easy to find), then the per-user cache directory, then the temp directory.
func defaults(dir string) []string {
	var out []string
	if dir == "" {
		if exe, err := os.Executable(); err == nil {
			dir = filepath.Dir(exe)
		}
	}
	if dir != "" {
		out = append(out, filepath.Join(dir, "ggufget.log"))
	}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		out = append(out, filepath.Join(cache, "ggufget", "logs", "ggufget.log"))
	}
	out = append(out, filepath.Join(os.TempDir(), "ggufget.log"))
	return out
}

func openLogFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func expand(p string) string {
	if strings.HasPrefix(p, "~"+string(os.PathSeparator)) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Debugf logs at debug level (dropped unless --log-level=debug or --verbose).
func (l *Logger) Debugf(format string, a ...any) { l.emit(LevelDebug, format, a...) }

// Infof logs at info level.
func (l *Logger) Infof(format string, a ...any) { l.emit(LevelInfo, format, a...) }

// Warnf logs at warn level.
func (l *Logger) Warnf(format string, a ...any) { l.emit(LevelWarn, format, a...) }

// Errorf logs at error level.
func (l *Logger) Errorf(format string, a ...any) { l.emit(LevelError, format, a...) }

// Error logs a note plus the full unwrap chain: wrapping with %w keeps the
// cause, but only the chain shows which layer produced "Access is denied".
func (l *Logger) Error(note string, err error) {
	if l == nil || !l.Enabled() {
		return
	}
	l.emit(LevelError, "%s on=%q errno=%s chain=%s", note, ErrorPath(err), Errno(err), Chain(err))
}

func (l *Logger) emit(lv Level, format string, a ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || lv > l.level {
		return
	}
	msg := fmt.Sprintf(format, a...)
	msg = strings.ReplaceAll(msg, "\r", "")
	msg = strings.ReplaceAll(msg, "\n", " | ")
	line := time.Now().Format("2006-01-02T15:04:05.000-07:00") + " " + lv.String() + " " + msg + "\n"
	l.writeLocked(line)
}

// Write makes Logger an io.Writer so it can be handed to the downloader, which
// already knows how to log retries and resume decisions.
func (l *Logger) Write(p []byte) (int, error) {
	if l == nil {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return len(p), nil
	}
	text := strings.TrimRight(string(p), "\r\n")
	if text == "" || LevelInfo > l.level {
		return len(p), nil
	}
	l.writeLocked(time.Now().Format("2006-01-02T15:04:05.000-07:00") + " " + LevelInfo.String() + " " + text + "\n")
	return len(p), nil
}

func (l *Logger) writeLocked(line string) {
	if l.w == nil {
		return
	}
	if l.max > 0 && l.size > 0 && l.size+int64(len(line)) > l.max {
		l.rotateLocked()
	}
	n, err := io.WriteString(l.w, line)
	if err != nil {
		return
	}
	l.size += int64(n)
}

func (l *Logger) rotateLocked() {
	if l.w == nil {
		return
	}
	_ = l.w.Close()
	old := l.path + ".1"
	_ = os.Remove(old)
	_ = os.Rename(l.path, old)
	f, err := openLogFile(l.path)
	if err != nil {
		l.w = nil
		return
	}
	l.w = f
	l.size = 0
}

// Close flushes and closes the file. It is safe to call more than once.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	if l.w == nil {
		return nil
	}
	err := l.w.Close()
	l.w = nil
	return err
}

// Chain renders every layer of a wrapped error, outermost first, so the log
// shows both the operation that failed and the OS call underneath it.
func Chain(err error) string {
	if err == nil {
		return ""
	}
	var parts []string
	seen := map[string]bool{}
	for i := 0; err != nil && i < 8; i++ {
		s := fmt.Sprintf("%T: %v", err, err)
		if seen[s] {
			break
		}
		seen[s] = true
		parts = append(parts, s)
		next := errors.Unwrap(err)
		if next == nil {
			break
		}
		err = next
	}
	return strings.Join(parts, " <- ")
}

// ErrorPath returns the filesystem path carried by a wrapped *os.PathError.
func ErrorPath(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) && pe.Path != "" {
		return pe.Path
	}
	return ""
}

// Errno returns the numeric OS error (5 on Windows = access denied) or "".
func Errno(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) && errno != 0 {
		return strconv.FormatUint(uint64(errno), 10)
	}
	return ""
}

var (
	tokenPattern  = regexp.MustCompile(`hf_[A-Za-z0-9]{10,}`)
	secretPattern = regexp.MustCompile(`(?i)\b(token|api[_-]?key|authorization|bearer)\b\s*[=:]\s*(?:(?:bearer|basic|token)\s+)?("[^"]*"|\S+)`)
)

// Redact removes credentials from text that will be written to disk.
func Redact(s string) string {
	s = tokenPattern.ReplaceAllString(s, "hf_***")
	return secretPattern.ReplaceAllString(s, "$1=***")
}

// RedactArgs rewrites a command line, blanking --token values.
func RedactArgs(args []string) []string {
	out := make([]string, 0, len(args))
	blankNext := false
	for _, a := range args {
		switch {
		case blankNext:
			out = append(out, "***")
			blankNext = false
		case a == "--token" || a == "-token":
			out = append(out, a)
			blankNext = true
		case strings.HasPrefix(a, "--token="):
			out = append(out, "--token=***")
		default:
			out = append(out, Redact(a))
		}
	}
	return out
}
