// Package fsutil wraps the filesystem details that differ between Windows and
// Unix: long paths, atomic replacement, free space and home expansion.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome turns a leading "~" into the user's home directory.
func ExpandHome(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if len(p) <= 2 {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

// EnsureDir creates dir and every missing parent.
func EnsureDir(dir string) error {
	if dir == "" {
		return errors.New("empty directory")
	}
	if err := os.MkdirAll(LongPath(dir), 0o755); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	return nil
}

// ProbeWritable answers "could we create a file under dir" and leaves nothing
// behind. It uses the nearest existing ancestor, because the directory itself
// may legitimately not exist yet, and a create attempt is the only answer that
// also covers antivirus/ACL interlocks that a permission read cannot see.
func ProbeWritable(dir string) error {
	if dir == "" {
		return errors.New("empty directory")
	}
	base := NearestExistingDir(dir)
	f, err := os.CreateTemp(LongPath(base), ".ggufget-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return nil
}

// Size returns the size of path and whether it exists. A missing file yields
// (0, false, nil) so callers can treat it as "not started".
func Size(path string) (int64, bool, error) {
	st, err := os.Stat(LongPath(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if st.IsDir() {
		return 0, false, fmt.Errorf("%s is a directory", path)
	}
	return st.Size(), true, nil
}

// WriteFileAtomic writes data to path through a temporary sibling, so a crash
// never leaves a half-written file in place.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(LongPath(dir), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return Rename(tmpName, LongPath(path))
}

// NearestExistingDir walks up from dir until it finds an existing directory,
// which is what free-space queries need.
func NearestExistingDir(dir string) string {
	dir = filepath.Clean(dir)
	for {
		if st, err := os.Stat(LongPath(dir)); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "" {
			return dir
		}
		dir = parent
	}
}

// HumanishError turns a raw OS error into a shorter message.
func HumanishError(err error) string {
	if err == nil {
		return ""
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
