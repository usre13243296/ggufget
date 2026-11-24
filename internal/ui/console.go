package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"ggufget/internal/termui"
)

// ErrInterrupt is returned when the user presses Ctrl+C at a prompt.
var ErrInterrupt = errors.New("interrupted")

// ErrQuit is returned by menus when the user asks to go back or quit.
var ErrQuit = errors.New("quit")

// Console owns the terminal: line editing, completion, history and colour.
// On a real console it switches stdin to raw mode so single keypresses arrive;
// everywhere else (pipes, CI, redirected files) it falls back to plain lines.
type Console struct {
	In    io.Reader
	Out   io.Writer
	TTY   bool
	Color bool
	Width int

	// History is offered with the up/down arrows and by TAB completion.
	History []string
	// Complete turns the current line into candidates; nil disables TAB.
	Complete func(line string) []string

	keys    *keyReader
	restore func()
	br      *bufio.Reader
	closed  bool
}

// NewConsole prepares the terminal. Call Close before the process exits.
func NewConsole(in io.Reader, out io.Writer) *Console {
	c := &Console{In: in, Out: out, Width: 80}
	if f, ok := in.(*os.File); ok && termui.IsTTY(f) {
		c.TTY = true
	}
	if c.TTY {
		c.Width = termui.Width()
		c.keys = newKeyReader(in)
		c.restore = enableRaw()
		if c.restore == nil {
			c.TTY = false
			c.keys = nil
		}
	}
	c.Color = c.TTY && !termui.NoColor()
	return c
}

// Close restores the console mode. Safe to call twice.
func (c *Console) Close() {
	if c.restore != nil && !c.closed {
		c.restore()
		c.closed = true
	}
}

// Printf writes to stdout.
func (c *Console) Printf(format string, a ...any) { fmt.Fprintf(c.Out, format, a...) }

// Println writes one line to stdout.
func (c *Console) Println(a ...any) { fmt.Fprintln(c.Out, a...) }

// Style helpers; they are no-ops when colour is off or the build has none.
func (c *Console) wrap(code, s string) string {
	if !c.Color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (c *Console) Bold(s string) string   { return c.wrap("1", s) }
func (c *Console) Dim(s string) string    { return c.wrap("2", s) }
func (c *Console) Red(s string) string    { return c.wrap("31", s) }
func (c *Console) Green(s string) string  { return c.wrap("32", s) }
func (c *Console) Yellow(s string) string { return c.wrap("33", s) }
func (c *Console) Cyan(s string) string   { return c.wrap("36", s) }

// ReadLine reads one edited line. The returned error is io.EOF on Ctrl+D with
// an empty line, or ErrInterrupt on Ctrl+C.
func (c *Console) ReadLine(prompt string) (string, error) {
	if !c.TTY || c.keys == nil {
		return c.readPlain(prompt)
	}
	line, err := c.edit(prompt)
	if err != nil {
		return line, err
	}
	c.pushHistory(line)
	return line, nil
}

func (c *Console) pushHistory(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	if n := len(c.History); n > 0 && c.History[n-1] == line {
		return
	}
	c.History = append(c.History, line)
	if len(c.History) > 200 {
		c.History = c.History[len(c.History)-200:]
	}
}

func (c *Console) readPlain(prompt string) (string, error) {
	if c.br == nil {
		c.br = bufio.NewReader(c.In)
	}
	if prompt != "" {
		fmt.Fprint(c.Out, prompt)
	}
	s, err := c.br.ReadString('\n')
	s = strings.TrimSpace(strings.TrimRight(s, "\r\n"))
	if err != nil && s == "" {
		return "", err
	}
	c.pushHistory(s)
	return s, nil
}

func (c *Console) edit(prompt string) (string, error) {
	var runes []rune
	cursor := 0
	histIdx := len(c.History)
	draft := ""
	draw := func() { c.drawLine(prompt, runes, cursor) }
	draw()
	for {
		key, err := c.keys.Read()
		if err != nil {
			fmt.Fprint(c.Out, "\r\n")
			return string(runes), err
		}
		switch key.Code {
		case Enter:
			fmt.Fprint(c.Out, "\r\n")
			return string(runes), nil
		case CtrlC:
			fmt.Fprint(c.Out, "^C\r\n")
			return "", ErrInterrupt
		case CtrlD:
			if len(runes) == 0 {
				fmt.Fprint(c.Out, "\r\n")
				return "", io.EOF
			}
			if cursor < len(runes) {
				runes = append(runes[:cursor], runes[cursor+1:]...)
			}
		case Tab:
			c.completeLine(&runes, &cursor)
		case Backspace:
			if cursor > 0 {
				runes = append(runes[:cursor-1], runes[cursor:]...)
				cursor--
			}
		case Delete:
			if cursor < len(runes) {
				runes = append(runes[:cursor], runes[cursor+1:]...)
			}
		case Left:
			if cursor > 0 {
				cursor--
			}
		case Right:
			if cursor < len(runes) {
				cursor++
			}
		case Home, CtrlA:
			cursor = 0
		case End, CtrlE:
			cursor = len(runes)
		case CtrlU:
			runes = runes[cursor:]
			cursor = 0
		case CtrlK:
			runes = runes[:cursor]
		case CtrlW:
			i := cursor
			for i > 0 && unicode.IsSpace(runes[i-1]) {
				i--
			}
			for i > 0 && !unicode.IsSpace(runes[i-1]) {
				i--
			}
			runes = append(runes[:i], runes[cursor:]...)
			cursor = i
		case Up:
			if histIdx > 0 {
				if histIdx == len(c.History) {
					draft = string(runes)
				}
				histIdx--
				runes = []rune(c.History[histIdx])
				cursor = len(runes)
			}
		case Down:
			if histIdx < len(c.History) {
				histIdx++
				if histIdx == len(c.History) {
					runes = []rune(draft)
				} else {
					runes = []rune(c.History[histIdx])
				}
				cursor = len(runes)
			}
		case Char:
			runes = append(runes, 0)
			copy(runes[cursor+1:], runes[cursor:])
			runes[cursor] = key.R
			cursor++
		}
		draw()
	}
}

// drawLine repaints the prompt and the visible slice of the edit buffer, then
// parks the cursor at the right column. \r + clear-to-end-of-line is used
// instead of \b loops because wide (CJK) characters occupy two cells.
func (c *Console) drawLine(prompt string, runes []rune, cursor int) {
	avail := c.Width - dispWidth(prompt) - 1
	if avail < 8 {
		avail = 8
	}
	start := 0
	if cursor > avail {
		start = cursor - avail
	}
	shown := clipWidth(string(runes[start:]), avail)
	col := dispWidth(prompt) + dispWidth(string(runes[start:cursor]))
	fmt.Fprint(c.Out, "\r\x1b[K")
	fmt.Fprint(c.Out, prompt)
	fmt.Fprint(c.Out, shown)
	fmt.Fprintf(c.Out, "\r\x1b[%dC", col)
}

func (c *Console) completeLine(runes *[]rune, cursor *int) {
	if c.Complete == nil {
		return
	}
	text := string(*runes)
	cands := dedupe(c.Complete(text))
	switch len(cands) {
	case 0:
		return
	case 1:
		*runes = []rune(cands[0])
		*cursor = len(*runes)
		return
	}
	pre := commonPrefix(cands)
	if len(pre) > len(text) {
		*runes = []rune(pre)
		*cursor = len(*runes)
	}
	fmt.Fprint(c.Out, "\r\n")
	c.printColumns(cands)
}

func (c *Console) printColumns(items []string) {
	if len(items) > 60 {
		items = items[:60]
	}
	widest := 0
	for _, it := range items {
		if w := dispWidth(it); w > widest {
			widest = w
		}
	}
	colW := widest + 2
	cols := (c.Width - 1) / colW
	if cols < 1 {
		cols = 1
	}
	for i, it := range items {
		pad := colW - dispWidth(it)
		fmt.Fprint(c.Out, it, strings.Repeat(" ", pad))
		if (i+1)%cols == 0 || i == len(items)-1 {
			fmt.Fprint(c.Out, "\r\n")
		}
	}
}

// yesWords accepts the affirmative answer in every language the UI ships.
var yesWords = map[string]bool{
	"y": true, "yes": true, // English
	"是": true, "好": true, "对": true, // Chinese
	"oui": true,             // French
	"sí":  true, "si": true, // Spanish
	"да": true, // Russian
}

// Confirm asks a yes/no question; the default is used for an empty answer.
func (c *Console) Confirm(prompt string, def bool) (bool, error) {
	hint := " [y/N] "
	if def {
		hint = " [Y/n] "
	}
	line, err := c.ReadLine(prompt + hint)
	if err != nil {
		return false, err
	}
	s := strings.ToLower(strings.TrimSpace(line))
	if s == "" {
		return def, nil
	}
	return yesWords[s], nil
}
