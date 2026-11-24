package ui

import (
	"strings"
	"unicode"
)

// runeWidth is the number of terminal cells a rune occupies. Only the ranges we
// actually print need to be exact: CJK ideographs, kana, Hangul, fullwidth forms
// and emoji are double width.
func runeWidth(r rune) int {
	switch {
	case r == 0 || r < 32 || r == 0x7f:
		return 0
	case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cf, r):
		return 0
	case isWide(r):
		return 2
	}
	return 1
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f,
		r >= 0x2e80 && r <= 0x303e,
		r >= 0x3041 && r <= 0x33ff,
		r >= 0x3400 && r <= 0x4dbf,
		r >= 0x4e00 && r <= 0x9fff,
		r >= 0xa000 && r <= 0xa4cf,
		r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff,
		r >= 0xfe30 && r <= 0xfe6f,
		r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f300 && r <= 0x1f64f,
		r >= 0x1f900 && r <= 0x1f9ff,
		r >= 0x20000 && r <= 0x3fffd:
		return true
	}
	return false
}

// dispWidth is the width of s in terminal cells.
func dispWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// clipWidth cuts s to at most width cells, never splitting a wide character.
func clipWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := 0
	for i, r := range s {
		rw := runeWidth(r)
		if w+rw > width {
			return s[:i]
		}
		w += rw
	}
	return s
}

// padWidth right-pads s with spaces to exactly width cells.
func padWidth(s string, width int) string {
	if d := width - dispWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, it := range items {
		if seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	return out
}

func commonPrefix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	pre := items[0]
	for _, it := range items[1:] {
		for !strings.HasPrefix(it, pre) {
			if pre == "" {
				return ""
			}
			pre = pre[:len(pre)-1]
		}
	}
	return pre
}
