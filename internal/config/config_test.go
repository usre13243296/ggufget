package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Default()
	if c.Provider != "auto" {
		t.Errorf("provider = %q", c.Provider)
	}
	if c.HFEndpoint != "https://hf-mirror.com" {
		t.Errorf("hf endpoint = %q", c.HFEndpoint)
	}
	if c.MSEndpoint != "https://modelscope.cn" {
		t.Errorf("modelscope endpoint = %q", c.MSEndpoint)
	}
	if c.Retries != 3 {
		t.Errorf("retries = %d", c.Retries)
	}
	if c.Verify != "auto" {
		t.Errorf("verify = %q", c.Verify)
	}
}

func TestApplyEnv(t *testing.T) {
	t.Setenv("GGUFGET_PROVIDER", "hf-mirror")
	t.Setenv("GGUFGET_MODELS_DIR", "/tmp/models")
	t.Setenv("GGUFGET_CONNECTIONS", "8")
	t.Setenv("GGUFGET_TOKEN", "secret")
	t.Setenv("HF_TOKEN", "hf-secret")

	c := Default()
	c.applyEnv()
	if c.Provider != "hf-mirror" {
		t.Errorf("provider = %q", c.Provider)
	}
	if c.ModelsDir != "/tmp/models" {
		t.Errorf("models dir = %q", c.ModelsDir)
	}
	if c.Connections != 8 {
		t.Errorf("connections = %d", c.Connections)
	}
	// GGUFGET_TOKEN wins over HF_TOKEN
	if c.Token != "secret" {
		t.Errorf("token = %q", c.Token)
	}
}

func TestColorEnabled(t *testing.T) {
	c := Default()
	c.Color = nil
	t.Setenv("NO_COLOR", "1")
	if c.ColorEnabled() {
		t.Error("NO_COLOR must disable colour")
	}
	os.Unsetenv("NO_COLOR")
	off := false
	c.Color = &off
	if c.ColorEnabled() {
		t.Error("an explicit false must disable colour")
	}
	on := true
	c.Color = &on
	if !c.ColorEnabled() {
		t.Error("an explicit true must enable colour")
	}
}

func TestLoadMissingFileIsFine(t *testing.T) {
	dir := t.TempDir()
	c, warn := Load(filepath.Join(dir, "nope.json"), false)
	if c == nil {
		t.Fatal("nil config")
	}
	if warn != "" {
		t.Errorf("a missing file must be silent, got %q", warn)
	}
	if c.Retries != 3 {
		t.Errorf("defaults lost: %+v", c)
	}
}

func TestLoadBadFileWarns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, warn := Load(path, false)
	if c == nil {
		t.Fatal("a broken file must still yield defaults")
	}
	if warn == "" {
		t.Error("a broken file should warn")
	}
}

func TestLoadMerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	body := `{"provider":"modelscope","connections":6,"retries":1,"lang":"en"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, warn := Load(path, false)
	if warn != "" {
		t.Fatalf("unexpected warning: %s", warn)
	}
	if c.Provider != "modelscope" || c.Connections != 6 || c.Retries != 1 || c.Lang != "en" {
		t.Fatalf("merge failed: %+v", c)
	}
	if c.HFEndpoint != "https://hf-mirror.com" {
		t.Errorf("unset keys must keep defaults: %+v", c)
	}
}
