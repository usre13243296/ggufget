package termui

import (
	"io"
	"os"
)

const (
	escReset  = "\x1b[0m"
	escBold   = "\x1b[1m"
	escDim    = "\x1b[2m"
	escRed    = "\x1b[31m"
	escGreen  = "\x1b[32m"
	escYellow = "\x1b[33m"
	escCyan   = "\x1b[36m"
)

// Style wraps text in ANSI escapes when enabled. When disabled every method is
// the identity, so call sites never need to branch.
type Style struct{ Enabled bool }

// NewStyle decides whether to colour output written to w.
func NewStyle(w io.Writer) Style {
	f, ok := w.(*os.File)
	if !ok || !IsTTY(f) {
		return Style{}
	}
	return Style{Enabled: !NoColor() && EnableANSI(f)}
}

func (s Style) wrap(code, x string) string {
	if !s.Enabled || x == "" {
		return x
	}
	return code + x + escReset
}

func (s Style) Bold(x string) string   { return s.wrap(escBold, x) }
func (s Style) Dim(x string) string    { return s.wrap(escDim, x) }
func (s Style) Red(x string) string    { return s.wrap(escRed, x) }
func (s Style) Green(x string) string  { return s.wrap(escGreen, x) }
func (s Style) Yellow(x string) string { return s.wrap(escYellow, x) }
func (s Style) Cyan(x string) string   { return s.wrap(escCyan, x) }

// OK renders a check mark, green when colour is on.
func (s Style) OK(x string) string { return s.Green("✓ " + x) }

// Bad renders a failure mark.
func (s Style) Bad(x string) string { return s.Red("✗ " + x) }
