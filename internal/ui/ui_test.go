package ui

import (
	"bytes"
	"strings"
	"testing"
)

func newTestConsole(in string) (*Console, *bytes.Buffer) {
	var buf bytes.Buffer
	return NewConsole(strings.NewReader(in), &buf), &buf
}

// editLine drives the raw-mode editor with a byte script. In a real terminal
// those bytes come from the console; here they come from a string, which lets
// the key handling be tested without a pty.
func editLine(t *testing.T, input string, hist []string, complete func(string) []string) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	c := &Console{
		Out:      &buf,
		TTY:      true,
		Width:    80,
		History:  hist,
		Complete: complete,
		keys:     newKeyReader(strings.NewReader(input)),
	}
	line, err := c.ReadLine("")
	if err != nil {
		t.Fatalf("ReadLine(%q): %v", input, err)
	}
	return line, &buf
}

func TestEditScripts(t *testing.T) {
	cases := []struct {
		name  string
		input string
		hist  []string
		want  string
	}{
		{"plain", "abc\r", nil, "abc"},
		{"left-insert", "abc\x1b[D\x1b[DX\r", nil, "aXbc"},
		{"home-insert", "abc\x1b[HX\r", nil, "Xabc"},
		{"end-insert", "abc\x1b[FX\r", nil, "abcX"},
		{"ctrl-a", "abc\x01X\r", nil, "Xabc"},
		{"ctrl-e", "abc\x1b[DX\x05Y\r", nil, "abXcY"},
		{"backspace", "abc\x08\r", nil, "ab"},
		{"delete", "abc\x1b[D\x1b[3~\r", nil, "ab"},
		{"ctrl-u", "abc\x15\r", nil, ""},
		{"ctrl-k", "abc\x1b[D\x0b\r", nil, "ab"},
		{"ctrl-w", "foo bar\x17\r", nil, "foo "},
		{"history-up", "\x1b[A\r", []string{"foo"}, "foo"},
		{"history-up-down", "\x1b[A\x1b[Btyped\r", []string{"foo"}, "typed"},
		{"utf8", "模型\r", nil, "模型"},
	}
	for _, c := range cases {
		got, _ := editLine(t, c.input, c.hist, nil)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTabCompletion(t *testing.T) {
	single := func(line string) []string {
		if strings.HasPrefix("help", line) {
			return []string{"help"}
		}
		return nil
	}
	if got, _ := editLine(t, "hel\t\r", nil, single); got != "help" {
		t.Errorf("single candidate: got %q, want %q", got, "help")
	}

	multi := func(string) []string {
		return []string{"Qwen/Qwen2.5-7B", "Qwen/Qwen2.5-0.5B"}
	}
	got, buf := editLine(t, "Qwe\t\r", nil, multi)
	if got != "Qwen/Qwen2.5-" {
		t.Errorf("common prefix: got %q, want %q", got, "Qwen/Qwen2.5-")
	}
	if !strings.Contains(buf.String(), "Qwen/Qwen2.5-7B") {
		t.Errorf("candidates were not printed: %q", buf.String())
	}

	// No candidate leaves the line untouched.
	if got, _ := editLine(t, "zzz\t\r", nil, func(string) []string { return nil }); got != "zzz" {
		t.Errorf("no candidate: got %q, want %q", got, "zzz")
	}

	// A candidate list equal to what is already typed must not add a newline.
	if _, buf := editLine(t, "help\t\r", nil, single); strings.Contains(buf.String(), "help\r\n") {
		t.Errorf("unexpected candidate dump: %q", buf.String())
	}
}

func TestReadPlainAndHistory(t *testing.T) {
	c, _ := newTestConsole("hello\nworld\n")
	for _, want := range []string{"hello", "world"} {
		got, err := c.ReadLine("")
		if err != nil {
			t.Fatalf("ReadLine: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if len(c.History) != 2 || c.History[0] != "hello" {
		t.Errorf("history = %v", c.History)
	}
	// EOF with no data is reported, not turned into an empty line.
	if _, err := c.ReadLine(""); err == nil {
		t.Error("expected EOF")
	}
}

func TestChoose(t *testing.T) {
	c, buf := newTestConsole("nope\n2\n")
	opts := []Option{{Key: "1", Label: "one"}, {Key: "2", Label: "two"}}
	got, err := c.Choose("pick", opts, "")
	if err != nil {
		t.Fatalf("Choose: %v", err)
	}
	if got != "2" {
		t.Errorf("got %q, want 2", got)
	}
	if !strings.Contains(buf.String(), "nope") {
		t.Errorf("bad choice was not echoed: %q", buf.String())
	}

	// The label works as well as the key.
	c2, _ := newTestConsole("two\n")
	if got, err := c2.Choose("pick", opts, ""); err != nil || got != "2" {
		t.Errorf("label: got %q, %v", got, err)
	}

	// q quits.
	c3, _ := newTestConsole("q\n")
	if _, err := c3.Choose("pick", opts, ""); err != ErrQuit {
		t.Errorf("q: err = %v, want ErrQuit", err)
	}
}

func TestPickQuit(t *testing.T) {
	c, _ := newTestConsole("\n")
	if _, err := c.Pick("> "); err != ErrQuit {
		t.Errorf("empty: err = %v, want ErrQuit", err)
	}
	c2, _ := newTestConsole("q\n")
	if _, err := c2.Pick("> "); err != ErrQuit {
		t.Errorf("q: err = %v, want ErrQuit", err)
	}
	c3, _ := newTestConsole("owner/repo\n")
	got, err := c3.Pick("> ")
	if err != nil || got != "owner/repo" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestPageNonTTYPrintsEverything(t *testing.T) {
	c, buf := newTestConsole("")
	c.Page("", "one\ntwo\nthree")
	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("page output missing %q: %q", want, buf.String())
		}
	}
}

func TestConfirmAnswers(t *testing.T) {
	yes := []string{"y\n", "yes\n", "是\n", "oui\n", "sí\n", "да\n"}
	for _, in := range yes {
		c, _ := newTestConsole(in)
		ok, err := c.Confirm("go? ", false)
		if err != nil || !ok {
			t.Errorf("Confirm(%q) = %v, %v; want true", in, ok, err)
		}
	}
	c, _ := newTestConsole("n\n")
	if ok, err := c.Confirm("go? ", true); err != nil || ok {
		t.Errorf("n: got %v, %v; want false", ok, err)
	}
	c2, _ := newTestConsole("\n")
	if ok, err := c2.Confirm("go? ", true); err != nil || !ok {
		t.Errorf("empty with default true: got %v, %v", ok, err)
	}
}

func TestWidthHelpers(t *testing.T) {
	if w := dispWidth("按钮"); w != 4 {
		t.Errorf("dispWidth(按钮) = %d, want 4", w)
	}
	if w := dispWidth("abc"); w != 3 {
		t.Errorf("dispWidth(abc) = %d, want 3", w)
	}
	if got := clipWidth("abcdef", 3); got != "abc" {
		t.Errorf("clipWidth = %q", got)
	}
	if got := commonPrefix([]string{"abc", "abd", "abx"}); got != "ab" {
		t.Errorf("commonPrefix = %q", got)
	}
	if got := dedupe([]string{"b", "a", "b"}); len(got) != 2 {
		t.Errorf("dedupe = %v", got)
	}
}
