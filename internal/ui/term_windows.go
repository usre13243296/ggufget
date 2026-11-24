//go:build windows

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	modeProcessedInput  = 0x0001
	modeLineInput       = 0x0002
	modeEchoInput       = 0x0004
	modeVTInput         = 0x0200
	modeProcessedOutput = 0x0001
	modeVTOutput        = 0x0004
)

func consoleMode(h uintptr) (uint32, bool) {
	var m uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&m)))
	return m, r != 0
}

func setConsoleMode(h uintptr, m uint32) {
	_, _, _ = procSetConsoleMode.Call(h, uintptr(m))
}

// enableRaw disables line buffering and echo so the line editor sees single
// keypresses. It returns nil when stdin/stdout are not both consoles (pipes,
// CI, redirected input) and the caller falls back to plain line reads.
func enableRaw() func() {
	in := os.Stdin.Fd()
	out := os.Stdout.Fd()
	inMode, ok := consoleMode(in)
	if !ok {
		return nil
	}
	outMode, ok := consoleMode(out)
	if !ok {
		return nil
	}
	setConsoleMode(in, inMode&^(modeLineInput|modeEchoInput)|modeVTInput)
	setConsoleMode(out, outMode|modeVTOutput|modeProcessedOutput)
	return func() {
		setConsoleMode(in, inMode)
		setConsoleMode(out, outMode)
	}
}
