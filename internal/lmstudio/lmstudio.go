// Package lmstudio finds where LM Studio keeps its models and asks its CLI
// whether a downloaded file became visible.
package lmstudio

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ggufget/internal/fsutil"
)

// Home returns the LM Studio data directory (~/.lmstudio).
func Home() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".lmstudio")
}

// SettingsPath returns the settings.json that stores the models folder.
func SettingsPath() string {
	h := Home()
	if h == "" {
		return ""
	}
	return filepath.Join(h, "settings.json")
}

type settings struct {
	DownloadsFolder string `json:"downloadsFolder"`
}

// ModelsDir reads downloadsFolder out of settings.json.
func ModelsDir() (string, bool) {
	p := SettingsPath()
	if p == "" {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	var s settings
	if err := json.Unmarshal(b, &s); err != nil {
		return "", false
	}
	dir := strings.TrimSpace(s.DownloadsFolder)
	if dir == "" {
		return "", false
	}
	return fsutil.ExpandHome(dir), true
}

// ResolveDir implements the lookup order from SPEC section 6.
func ResolveDir(out, cfgDir string) (string, error) {
	for _, cand := range []string{out, cfgDir} {
		if strings.TrimSpace(cand) != "" {
			return fsutil.ExpandHome(cand), nil
		}
	}
	if dir, ok := ModelsDir(); ok {
		return dir, nil
	}
	if h := Home(); h != "" {
		fallback := filepath.Join(h, "models")
		if st, err := os.Stat(fallback); err == nil && st.IsDir() {
			return fallback, nil
		}
		return fallback, nil
	}
	return "", os.ErrNotExist
}

// CLI locates the lms executable that ships with LM Studio.
func CLI() (string, bool) {
	h := Home()
	names := []string{"lms"}
	if runtime.GOOS == "windows" {
		names = []string{"lms.exe", "lms"}
	}
	if h != "" {
		for _, n := range names {
			p := filepath.Join(h, "bin", n)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}
	if p, err := exec.LookPath("lms"); err == nil {
		return p, true
	}
	return "", false
}

type lsEntry struct {
	Path     string `json:"path"`
	ModelKey string `json:"modelKey"`
}

// Indexed returns the model paths LM Studio currently knows about, relative to
// the models directory ("publisher/repo/file.gguf").
func Indexed(ctx context.Context) ([]string, error) {
	bin, ok := CLI()
	if !ok {
		return nil, os.ErrNotExist
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "ls", "--json")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseLS(out), nil
}

func parseLS(out []byte) []string {
	out = []byte(strings.TrimSpace(string(out)))
	if len(out) == 0 || out[0] != '[' {
		return nil
	}
	var entries []lsEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Path != "" {
			paths = append(paths, filepath.ToSlash(e.Path))
		}
	}
	return paths
}

// Visible reports whether rel (slash-separated, relative to the models dir) is
// already indexed. It accepts a match on the full relative path, or — because
// LM Studio rewrites the path of imported models — on the file name alone.
func Visible(ctx context.Context, rel string) (bool, error) {
	paths, err := Indexed(ctx)
	if err != nil {
		return false, err
	}
	want := strings.ToLower(strings.TrimPrefix(filepath.ToSlash(rel), "/"))
	base := path.Base(want)
	for _, p := range paths {
		low := strings.ToLower(p)
		if low == want || path.Base(low) == base {
			return true, nil
		}
	}
	return false, nil
}
