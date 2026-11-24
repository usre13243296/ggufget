package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func readLog(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestOpenFindsDirFirstAndWrites(t *testing.T) {
	dir := t.TempDir()
	l, attempts := Open(Options{Dir: dir, Level: LevelInfo})
	if l == nil {
		t.Fatalf("no logger, attempts=%v", attempts)
	}
	defer l.Close()
	if want := filepath.Join(dir, "ggufget.log"); l.Path() != want {
		t.Fatalf("path = %q, want %q", l.Path(), want)
	}
	l.Infof("hello %s", "world")
	l.Error("mkdir failed", &os.PathError{Op: "mkdir", Path: `C:\models\x`, Err: syscall.Errno(5)})
	body := readLog(t, l.Path())
	for _, want := range []string{"INFO hello world", "ERROR mkdir failed", "errno=5", `C:\models\x`} {
		if !strings.Contains(body, want) {
			t.Errorf("log missing %q\n%s", want, body)
		}
	}
}

func TestLevelFilters(t *testing.T) {
	dir := t.TempDir()
	l, _ := Open(Options{Dir: dir, Level: LevelWarn})
	if l == nil {
		t.Fatal("no logger")
	}
	defer l.Close()
	l.Infof("invisible")
	l.Debugf("invisible too")
	l.Warnf("visible")
	body := readLog(t, l.Path())
	if strings.Contains(body, "invisible") {
		t.Errorf("info/debug leaked past warn level:\n%s", body)
	}
	if !strings.Contains(body, "WARN visible") {
		t.Errorf("warn missing:\n%s", body)
	}
}

func TestRotatesOnce(t *testing.T) {
	dir := t.TempDir()
	l, _ := Open(Options{Dir: dir, Level: LevelInfo, MaxBytes: 200})
	if l == nil {
		t.Fatal("no logger")
	}
	path := l.Path()
	for i := 0; i < 40; i++ {
		l.Infof("%03d %s", i, strings.Repeat("x", 80))
	}
	l.Close()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rotated file: %v", err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() > 400 {
		t.Fatalf("current log should have restarted small, st=%v err=%v", st, err)
	}
}

func TestRedact(t *testing.T) {
	cases := map[string]string{
		"token hf_abcdefghijklmnopqrst": "token hf_***",
		"Authorization: Bearer abc123":  "Authorization=***",
		"--token=supersecret":           "--token=***",
		"APPDATA=C:\\Users":             "APPDATA=C:\\Users",
	}
	for in, want := range cases {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
	got := RedactArgs([]string{"get", "repo", "--token", "abc", "--token=def"})
	want := []string{"get", "repo", "--token", "***", "--token=***"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("RedactArgs = %v, want %v", got, want)
	}
}

func TestChainErrorPathErrno(t *testing.T) {
	base := &os.PathError{Op: "mkdir", Path: `C:\Users\a\.lmstudio\models\x`, Err: syscall.Errno(5)}
	err := fmt.Errorf("download failed: %w", base)
	if Chain(err) == "" || !strings.Contains(Chain(err), "mkdir") {
		t.Errorf("chain missing mkdir: %q", Chain(err))
	}
	if ErrorPath(err) != `C:\Users\a\.lmstudio\models\x` {
		t.Errorf("ErrorPath = %q", ErrorPath(err))
	}
	if Errno(err) != "5" {
		t.Errorf("Errno = %q, want 5", Errno(err))
	}
}

func TestConcurrentWritesKeepEveryLine(t *testing.T) {
	dir := t.TempDir()
	l, _ := Open(Options{Dir: dir, Level: LevelInfo})
	if l == nil {
		t.Fatal("no logger")
	}
	const workers, each = 20, 20
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				l.Infof("w%02d i%02d", w, i)
			}
		}(w)
	}
	wg.Wait()
	l.Close()
	body := readLog(t, l.Path())
	if n := strings.Count(body, "\n"); n != workers*each {
		t.Fatalf("wrote %d lines, want %d", n, workers*each)
	}
}

func TestDisabledWritesNothing(t *testing.T) {
	l, attempts := Open(Options{Disabled: true, Dir: t.TempDir()})
	if l != nil || len(attempts) != 0 {
		t.Fatalf("disabled logger = %v/attempts=%v", l, attempts)
	}
	if l.Enabled() {
		t.Error("nil logger must not report enabled")
	}
	if l.Path() != "" {
		t.Error("nil logger must have empty path")
	}
}

func TestImplementsIOWriter(t *testing.T) {
	dir := t.TempDir()
	l, _ := Open(Options{Dir: dir, Level: LevelInfo})
	if l == nil {
		t.Fatal("no logger")
	}
	defer l.Close()
	if _, err := l.Write([]byte("single retry 3 after timeout\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if body := readLog(t, l.Path()); !strings.Contains(body, "INFO single retry 3 after timeout") {
		t.Errorf("io.Writer path lost the line:\n%s", body)
	}
}
