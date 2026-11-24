// Package units formats byte counts and durations for humans.
package units

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	kb = 1024
	mb = kb * 1024
	gb = mb * 1024
	tb = gb * 1024
)

// Bytes renders n using binary units (1024 based). The long form is used in
// plans and listings.
func Bytes(n int64) string {
	switch {
	case n < 0:
		return "?"
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < mb:
		return fmt.Sprintf("%.1f KiB", float64(n)/kb)
	case n < gb:
		return fmt.Sprintf("%.1f MiB", float64(n)/mb)
	case n < tb:
		return fmt.Sprintf("%.2f GiB", float64(n)/gb)
	default:
		return fmt.Sprintf("%.2f TiB", float64(n)/tb)
	}
}

// BytesShort keeps the unit but drops digits for the compact progress line.
func BytesShort(n int64) string {
	switch {
	case n < 0:
		return "?"
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < mb:
		return fmt.Sprintf("%.0f K", float64(n)/kb)
	case n < gb:
		return fmt.Sprintf("%.1f M", float64(n)/mb)
	case n < tb:
		return fmt.Sprintf("%.2f G", float64(n)/gb)
	default:
		return fmt.Sprintf("%.2f T", float64(n)/tb)
	}
}

// Rate renders a byte-per-second speed.
func Rate(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "-"
	}
	return Bytes(int64(bytesPerSec)) + "/s"
}

// MiBps converts bytes per second into mebibytes per second.
func MiBps(bytesPerSec float64) float64 { return bytesPerSec / mb }

// ETA renders a remaining duration as 1h02m, 3m12s or 45s.
func ETA(d time.Duration) string {
	if d <= 0 {
		return "--"
	}
	if d > 100*time.Hour {
		return ">100h"
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%02ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// Duration is a friendlier form for elapsed times.
func Duration(d time.Duration) string {
	if d < time.Second {
		return d.Round(10 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

// quantRe finds a gguf quantization label such as Q4_K_M or IQ2_XXS.
// The prefix must start at a separator (or at the beginning) so that model
// names like "Qwen2.5" are not mistaken for quantizations.
var quantRe = regexp.MustCompile(`(?i)(?:^|[_\-.])((?:iq|q)\d+[a-z0-9]*(?:_(?:xxs|xss|xs|s|m|l|xl|k|km|ks|n|nl|[0-9]))*)`)

// Quant extracts a quantization label from a file name, or "" when the name
// carries none. It is cosmetic: it only feeds the listing table.
func Quant(name string) string {
	if m := quantRe.FindStringSubmatch(name); m != nil {
		return strings.ToUpper(m[1])
	}
	up := strings.ToUpper(name)
	for _, tag := range []string{"BF16", "F16", "F32"} {
		if strings.Contains(up, "_"+tag) || strings.Contains(up, "-"+tag) {
			return tag
		}
	}
	return ""
}
