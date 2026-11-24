// Package termui renders progress and status lines that behave sensibly in a
// terminal, in a pipe and in NDJSON mode.
package termui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"ggufget/internal/units"
)

// Mode selects how events are rendered.
type Mode int

const (
	// ModeText is the human default: an animated line on a TTY, plain periodic
	// lines when redirected.
	ModeText Mode = iota
	// ModeJSON writes one JSON object per line, for scripts.
	ModeJSON
	// ModeQuiet prints nothing but final errors.
	ModeQuiet
)

// Reporter renders what the downloader is doing.
type Reporter struct {
	Out io.Writer
	Err io.Writer

	Mode  Mode
	TTY   bool
	Color bool
	Width int

	mu       sync.Mutex
	label    string
	total    int64
	done     int64
	conns    int
	note     string
	started  time.Time
	lastAt   time.Time
	lastDone int64
	speed    float64
	stop     chan struct{}
	wg       sync.WaitGroup
	lineOpen bool
}

// NewReporter builds a reporter for the given streams.
func NewReporter(out, err io.Writer, mode Mode) *Reporter {
	r := &Reporter{Out: out, Err: err, Mode: mode, Width: Width()}
	if f, ok := out.(*os.File); ok {
		r.TTY = IsTTY(f)
		r.Color = r.TTY && !NoColor() && EnableANSI(f)
	}
	if os.Getenv("NO_COLOR") != "" {
		r.Color = false
	}
	return r
}

// BeginFile starts the per-file progress display.
func (r *Reporter) BeginFile(idx, count int, name string, total int64, note string) {
	r.mu.Lock()
	r.label = fmt.Sprintf("[%d/%d] %s", idx, count, name)
	r.total = total
	r.done = 0
	r.conns = 0
	r.note = note
	r.started = time.Now()
	r.lastAt = time.Now()
	r.lastDone = 0
	r.speed = 0
	r.mu.Unlock()

	if r.Mode == ModeJSON {
		r.emit(map[string]any{
			"event": "start", "file": name, "index": idx, "count": count, "total": total,
		})
		return
	}
	if r.Mode == ModeQuiet {
		return
	}
	r.mu.Lock()
	if r.TTY {
		fmt.Fprintf(r.Err, "%s  %s\n", r.label, units.Bytes(total))
	}
	r.mu.Unlock()

	interval := 10 * time.Second
	if r.TTY {
		interval = 200 * time.Millisecond
	}
	r.stop = make(chan struct{})
	r.wg.Add(1)
	go r.loop(interval)
}

func (r *Reporter) loop(interval time.Duration) {
	defer r.wg.Done()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
			r.render()
		}
	}
}

// Add advances the byte counter by n.
func (r *Reporter) Add(n int64) {
	r.mu.Lock()
	r.done += n
	r.mu.Unlock()
}

// SetConns records how many connections are in use.
func (r *Reporter) SetConns(n int) {
	r.mu.Lock()
	r.conns = n
	r.mu.Unlock()
}

// SetNote replaces the trailing note printed after the progress bar.
func (r *Reporter) SetNote(s string) {
	r.mu.Lock()
	r.note = s
	r.mu.Unlock()
}

func (r *Reporter) render() {
	r.mu.Lock()
	done, total := r.done, r.total
	now := time.Now()
	if dt := now.Sub(r.lastAt); dt > 0 {
		inst := float64(done-r.lastDone) / dt.Seconds()
		if inst > 0 {
			if r.speed == 0 {
				r.speed = inst
			} else {
				r.speed = 0.75*r.speed + 0.25*inst
			}
		}
	}
	r.lastAt, r.lastDone = now, done
	label, speed, conns, note := r.label, r.speed, r.conns, r.note
	r.mu.Unlock()

	if r.Mode == ModeJSON {
		r.emit(map[string]any{
			"event": "progress", "done": done, "total": total,
			"speed": int64(speed), "conns": conns,
		})
		return
	}
	if r.Mode == ModeQuiet {
		return
	}

	var eta string
	if speed > 0 && total > done {
		eta = units.ETA(time.Duration(float64(total-done)/speed) * time.Second)
	}
	if !r.TTY {
		fmt.Fprintf(r.Err, "  %s / %s  %s  %s%s\n",
			units.Bytes(done), units.Bytes(total), units.Rate(speed), label, suffix(conns, note))
		return
	}

	pct := 0.0
	if total > 0 {
		pct = float64(done) / float64(total) * 100
	}
	bar := r.bar(pct)
	line := fmt.Sprintf("  %s / %s  %5.1f%%  %s  %8s  %s%s",
		units.BytesShort(done), units.BytesShort(total), pct, bar, units.Rate(speed), eta, suffix(conns, note))

	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.Err, "\r%s%s", line, r.clearToEnd())
	r.lineOpen = true
}

func suffix(conns int, note string) string {
	var b strings.Builder
	if conns > 1 {
		fmt.Fprintf(&b, " · %d conns", conns)
	}
	if note != "" {
		fmt.Fprintf(&b, " · %s", note)
	}
	return b.String()
}

// bar renders a bar sized to the terminal, leaving room for the numbers.
func (r *Reporter) bar(pct float64) string {
	w := r.Width - 62
	if w < 8 {
		w = 8
	}
	if w > 40 {
		w = 40
	}
	filled := int(pct / 100 * float64(w))
	if filled > w {
		filled = w
	}
	if filled < 0 {
		filled = 0
	}
	f := strings.Repeat("█", filled)
	e := strings.Repeat("░", w-filled)
	if !r.Color {
		return "[" + strings.Repeat("#", filled) + strings.Repeat(".", w-filled) + "]"
	}
	return "[" + f + e + "]"
}

func (r *Reporter) clearToEnd() string {
	if r.Color {
		return "\x1b[K"
	}
	return "                    "
}

// EndFile closes the per-file display.
func (r *Reporter) EndFile(name string, total int64, err error) {
	r.mu.Lock()
	stop, started := r.stop, r.started
	r.stop = nil
	lineOpen := r.lineOpen
	r.lineOpen = false
	r.mu.Unlock()

	if stop != nil {
		close(stop)
		r.wg.Wait()
	}

	elapsed := time.Since(started)
	if r.Mode == ModeJSON {
		ev := map[string]any{"event": "done", "file": name, "elapsed": elapsed.Seconds()}
		if err != nil {
			ev["event"] = "error"
			ev["error"] = err.Error()
		}
		r.emit(ev)
		return
	}
	if r.Mode == ModeQuiet {
		return
	}
	r.mu.Lock()
	if lineOpen {
		fmt.Fprint(r.Err, "\r"+strings.Repeat(" ", max(r.Width-1, 10))+"\r")
	} else if r.TTY {
		fmt.Fprintln(r.Err)
	}
	r.mu.Unlock()
}

// Finish prints the closing summary line for one file.
func (r *Reporter) Finish(dest string) {
	if r.Mode == ModeQuiet {
		return
	}
	if r.Mode == ModeJSON {
		r.emit(map[string]any{"event": "saved", "path": dest})
		return
	}
	fmt.Fprintf(r.Out, "  -> %s\n", dest)
}

// Printf writes a normal status line, respecting quiet mode.
func (r *Reporter) Printf(format string, a ...any) {
	if r.Mode == ModeQuiet {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lineOpen {
		fmt.Fprint(r.Out, "\n")
		r.lineOpen = false
	}
	fmt.Fprintf(r.Out, format, a...)
}

// Errorf writes an error, always.
func (r *Reporter) Errorf(format string, a ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lineOpen {
		fmt.Fprint(r.Out, "\n")
		r.lineOpen = false
	}
	fmt.Fprintf(r.Err, format, a...)
}

func (r *Reporter) emit(v map[string]any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Out.Write(append(b, '\n'))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
