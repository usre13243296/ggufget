//go:build linux || darwin

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

func ioctlTermios(fd, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// enableRaw turns off canonical mode and echo on the terminal; Ctrl+C keeps
// raising SIGINT because ISIG is deliberately left alone.
func enableRaw() func() {
	fd := os.Stdin.Fd()
	var old syscall.Termios
	if err := ioctlTermios(fd, ioctlGetTermios, unsafe.Pointer(&old)); err != nil {
		return nil
	}
	raw := old
	raw.Lflag &^= syscall.ECHO | syscall.ICANON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctlTermios(fd, ioctlSetTermios, unsafe.Pointer(&raw)); err != nil {
		return nil
	}
	return func() { _ = ioctlTermios(fd, ioctlSetTermios, unsafe.Pointer(&old)) }
}
