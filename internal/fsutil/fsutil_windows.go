//go:build windows

package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procMoveFileExW       = kernel32.NewProc("MoveFileExW")
	procGetDiskFreeSpaceW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

const moveFileReplaceExisting = 0x1

// LongPath adds the \\?\ prefix when a path is long enough to hit the classic
// MAX_PATH limit. Model paths routinely exceed 260 characters, and that limit
// is the single most common Windows-only breakage in tools like this one.
func LongPath(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\\.\`) {
		return p
	}
	abs := p
	if !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		} else {
			return p
		}
	}
	if len(abs) < 240 {
		return abs
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + abs[2:]
	}
	return `\\?\` + abs
}

// Rename replaces dst atomically. os.Rename fails on Windows when dst exists,
// hence the MoveFileExW call.
func Rename(src, dst string) error {
	s, err := syscall.UTF16PtrFromString(LongPath(src))
	if err != nil {
		return err
	}
	d, err := syscall.UTF16PtrFromString(LongPath(dst))
	if err != nil {
		return err
	}
	r, _, e := procMoveFileExW.Call(
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(d)),
		uintptr(moveFileReplaceExisting),
	)
	if r == 0 {
		if e != syscall.Errno(0) {
			return &os.LinkError{Op: "rename", Old: src, New: dst, Err: e}
		}
		return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.EINVAL}
	}
	return nil
}

// DiskFree reports the bytes available to the calling user on the volume that
// holds dir.
func DiskFree(dir string) (uint64, error) {
	dir = NearestExistingDir(dir)
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var freeToCaller, total, totalFree uint64
	r, _, e := procGetDiskFreeSpaceW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeToCaller)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		return 0, e
	}
	return freeToCaller, nil
}
