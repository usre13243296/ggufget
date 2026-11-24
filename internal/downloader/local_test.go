package downloader

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ggufget/internal/netx"
)

// TestRunCreatesLayoutDirs proves the whole pipeline works against a local
// server: it creates the two LM Studio directories (<stem>/lmstudio-ai/),
// downloads, and renames the .part file into place.
func TestRunCreatesLayoutDirs(t *testing.T) {
	payload := bytes.Repeat([]byte("GGUF"), 4096) // 16 KiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "m.gguf", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

	root := t.TempDir()
	dest := filepath.Join(root, "model-stem", "lmstudio-ai", "m.gguf")

	opts := &Options{
		Connections: 1,
		Retries:     1,
		Verify:      VerifyOff,
		Client:      netx.New(netx.Options{}),
	}
	res, err := Run(context.Background(), opts, Task{
		URL:      srv.URL + "/m.gguf",
		Dest:     dest,
		Expected: int64(len(payload)),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes != int64(len(payload)) {
		t.Fatalf("bytes = %d, want %d", res.Bytes, len(payload))
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("content mismatch")
	}
	if _, err := os.Stat(filepath.Join(root, "model-stem", "lmstudio-ai")); err != nil {
		t.Fatalf("lmstudio-ai directory was not created: %v", err)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatalf(".part should be gone after success, stat err = %v", err)
	}
}
