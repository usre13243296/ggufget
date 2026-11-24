//go:build !linux && !darwin && !windows

package ui

// enableRaw is a no-op on platforms without a supported raw-mode API; the
// console then falls back to plain line reads.
func enableRaw() func() { return nil }
