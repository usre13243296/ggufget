package ui

import (
	"fmt"
	"strings"

	"ggufget/internal/i18n"
)

// Option is one line in a menu. Key is what the user types ("1", "q").
type Option struct {
	Key   string
	Label string
	Hint  string
}

// Choose prints the options and reads one choice. Typing the key or the exact
// label selects; q or an empty line returns ErrQuit.
func (c *Console) Choose(prompt string, opts []Option, extra string) (string, error) {
	widest := 0
	for _, o := range opts {
		if w := dispWidth(o.Key); w > widest {
			widest = w
		}
	}
	for _, o := range opts {
		fmt.Fprintf(c.Out, "  %s  %s", c.Cyan(padWidth(o.Key, widest)), o.Label)
		if o.Hint != "" {
			fmt.Fprintf(c.Out, "  %s", c.Dim(o.Hint))
		}
		fmt.Fprint(c.Out, "\n")
	}
	if extra != "" {
		fmt.Fprintf(c.Out, "  %s\n", c.Dim(extra))
	}
	for {
		line, err := c.ReadLine(prompt)
		if err != nil {
			return "", err
		}
		s := strings.TrimSpace(line)
		if s == "" || strings.EqualFold(s, "q") || strings.EqualFold(s, "quit") {
			return "", ErrQuit
		}
		for _, o := range opts {
			if strings.EqualFold(o.Key, s) {
				return o.Key, nil
			}
		}
		for _, o := range opts {
			if strings.EqualFold(strings.TrimSpace(o.Label), s) {
				return o.Key, nil
			}
		}
		fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("ui.badChoice", s)))
	}
}

// Pick reads a free-form answer, returning ErrQuit for q or an empty line.
func (c *Console) Pick(prompt string) (string, error) {
	line, err := c.ReadLine(prompt)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(line)
	if s == "" || strings.EqualFold(s, "q") || strings.EqualFold(s, "quit") {
		return "", ErrQuit
	}
	return s, nil
}

const pageHeight = 18

// Page shows long text with a built-in pager: space for the next screen, q to
// return. It deliberately does not shell out to less/more, which mangle UTF-8
// in the Windows console.
func (c *Console) Page(title, body string) error {
	if title != "" {
		fmt.Fprintf(c.Out, "\n%s\n", c.Bold(title))
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if !c.TTY || c.keys == nil {
		for _, ln := range lines {
			fmt.Fprintln(c.Out, ln)
		}
		return nil
	}
	for i := 0; i < len(lines); i += pageHeight {
		end := i + pageHeight
		if end > len(lines) {
			end = len(lines)
		}
		for _, ln := range lines[i:end] {
			fmt.Fprintln(c.Out, ln)
		}
		if end >= len(lines) {
			break
		}
		fmt.Fprint(c.Out, c.Dim("  "+i18n.T("ui.more")))
		for {
			k, err := c.keys.Read()
			if err != nil {
				fmt.Fprint(c.Out, "\r\n")
				return err
			}
			if k.Code == Char && (k.R == 'q' || k.R == 'Q') {
				fmt.Fprint(c.Out, "\r\n")
				return nil
			}
			if k.Code == Enter || (k.Code == Char && k.R == ' ') {
				fmt.Fprint(c.Out, "\r\n")
				break
			}
		}
	}
	return nil
}
