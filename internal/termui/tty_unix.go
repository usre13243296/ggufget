//go:build unix

package termui

import (
	"os"
	"strconv"
)

// IsTTY reports whether f is an interactive terminal.
func IsTTY(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// EnableANSI reports whether ANSI escapes are safe to emit. On Unix that is
// simply "is it a terminal and not a dumb one".
func EnableANSI(f *os.File) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return IsTTY(f)
}

// Width returns the terminal width in columns.
func Width() int {
	if v := os.Getenv("COLUMNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 20 {
			return n
		}
	}
	return 80
}

// NoColor honours the NO_COLOR convention and dumb terminals.
func NoColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return os.Getenv("TERM") == "dumb"
}
