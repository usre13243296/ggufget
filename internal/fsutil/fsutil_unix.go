//go:build unix

package fsutil

import (
	"os"
	"syscall"
)

// LongPath is a no-op outside Windows.
func LongPath(p string) string { return p }

// Rename replaces dst atomically; os.Rename already does this on Unix.
func Rename(src, dst string) error { return os.Rename(src, dst) }

// DiskFree reports the bytes available to the calling user on the filesystem
// that holds dir.
func DiskFree(dir string) (uint64, error) {
	dir = NearestExistingDir(dir)
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	// Bavail is signed on some platforms and unsigned on others; the cast works
	// for both.
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
