// Package config holds the user's saved preferences and the rules that merge
// them with the environment and the command line.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ggufget/internal/fsutil"
)

// Config mirrors the JSON file documented in the manual.
type Config struct {
	Provider    string `json:"provider"`
	HFEndpoint  string `json:"hf_endpoint"`
	MSEndpoint  string `json:"ms_endpoint"`
	ModelsDir   string `json:"models_dir"`
	Layout      string `json:"layout"`
	Connections int    `json:"connections"`
	Jobs        int    `json:"jobs"`
	Retries     int    `json:"retries"`
	Timeout     int    `json:"timeout"`
	Verify      string `json:"verify"`
	Proxy       string `json:"proxy"`
	Lang        string `json:"lang"`
	Color       *bool  `json:"color"`
	Token       string `json:"token"`
	// Log is the diagnostic log file ("" means "next to the binary").
	Log      string `json:"log"`
	LogLevel string `json:"log_level"`

	// Interactive-shell state, written back on exit: the repositories you have
	// used (TAB completion) and the last mirror measurements (shown at start).
	History   []string           `json:"history,omitempty"`
	Favorites []string           `json:"favorites,omitempty"`
	Speeds    map[string]float64 `json:"speeds,omitempty"`
	SpeedsAt  *time.Time         `json:"speeds_at,omitempty"`
}

// Default returns the built-in configuration.
func Default() *Config {
	on := true
	return &Config{
		Provider:   "auto",
		HFEndpoint: "https://hf-mirror.com",
		MSEndpoint: "https://modelscope.cn",
		Layout:     "lmstudio",
		Retries:    3,
		Timeout:    30,
		Verify:     "auto",
		Lang:       "",
		Color:      &on,
		LogLevel:   "info",
	}
}

// Path returns the platform config file location.
func Path() string {
	if p := strings.TrimSpace(os.Getenv("GGUFGET_CONFIG")); p != "" {
		return fsutil.ExpandHome(p)
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "ggufget", "config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ggufget", "config.json")
}

// Load reads the config file (unless skip is set) and overlays the environment.
// The returned warning is non-fatal: a broken file must never stop a download.
func Load(path string, skip bool) (*Config, string) {
	c := Default()
	warn := ""
	if !skip {
		if path == "" {
			path = Path()
		}
		b, err := os.ReadFile(fsutil.ExpandHome(path))
		switch {
		case err == nil && len(strings.TrimSpace(string(b))) > 0:
			var raw Config
			if err := json.Unmarshal(b, &raw); err != nil {
				warn = err.Error()
			} else {
				c.merge(&raw)
			}
		case err != nil && !os.IsNotExist(err):
			warn = err.Error()
		}
	}
	c.applyEnv()
	return c, warn
}

// merge copies only the fields the user actually set.
func (c *Config) merge(o *Config) {
	if o.Provider != "" {
		c.Provider = o.Provider
	}
	if o.HFEndpoint != "" {
		c.HFEndpoint = o.HFEndpoint
	}
	if o.MSEndpoint != "" {
		c.MSEndpoint = o.MSEndpoint
	}
	if o.ModelsDir != "" {
		c.ModelsDir = o.ModelsDir
	}
	if o.Layout != "" {
		c.Layout = o.Layout
	}
	if o.Connections != 0 {
		c.Connections = o.Connections
	}
	if o.Jobs != 0 {
		c.Jobs = o.Jobs
	}
	if o.Retries != 0 {
		c.Retries = o.Retries
	}
	if o.Timeout != 0 {
		c.Timeout = o.Timeout
	}
	if o.Verify != "" {
		c.Verify = o.Verify
	}
	if o.Proxy != "" {
		c.Proxy = o.Proxy
	}
	if o.Lang != "" {
		c.Lang = o.Lang
	}
	if o.Color != nil {
		c.Color = o.Color
	}
	if o.Token != "" {
		c.Token = o.Token
	}
	if o.Log != "" {
		c.Log = o.Log
	}
	if o.LogLevel != "" {
		c.LogLevel = o.LogLevel
	}
	if len(o.History) > 0 {
		c.History = o.History
	}
	if len(o.Favorites) > 0 {
		c.Favorites = o.Favorites
	}
	if len(o.Speeds) > 0 {
		c.Speeds = o.Speeds
	}
	if o.SpeedsAt != nil {
		c.SpeedsAt = o.SpeedsAt
	}
}

// Save writes the configuration atomically, creating its directory first.
func Save(path string, c *Config) error {
	if path == "" {
		path = Path()
	}
	if err := fsutil.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return fsutil.WriteFileAtomic(path, body, 0o644)
}

func (c *Config) applyEnv() {
	if v := os.Getenv("GGUFGET_PROVIDER"); v != "" {
		c.Provider = v
	}
	if v := os.Getenv("GGUFGET_HF_ENDPOINT"); v != "" {
		c.HFEndpoint = v
	}
	if v := os.Getenv("GGUFGET_MS_ENDPOINT"); v != "" {
		c.MSEndpoint = v
	}
	if v := os.Getenv("GGUFGET_MODELS_DIR"); v != "" {
		c.ModelsDir = v
	}
	if v := os.Getenv("GGUFGET_LAYOUT"); v != "" {
		c.Layout = v
	}
	if v := os.Getenv("GGUFGET_CONNECTIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Connections = n
		}
	}
	if v := os.Getenv("GGUFGET_JOBS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Jobs = n
		}
	}
	if v := os.Getenv("GGUFGET_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Retries = n
		}
	}
	if v := os.Getenv("GGUFGET_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Timeout = n
		}
	}
	if v := os.Getenv("GGUFGET_VERIFY"); v != "" {
		c.Verify = v
	}
	if v := os.Getenv("GGUFGET_LANG"); v != "" {
		c.Lang = v
	}
	if v := os.Getenv("GGUFGET_LOG"); v != "" {
		c.Log = v
	}
	if v := os.Getenv("GGUFGET_LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("HF_TOKEN"); v != "" && c.Token == "" {
		c.Token = v
	}
	if v := os.Getenv("GGUFGET_TOKEN"); v != "" {
		c.Token = v
	}
	if c.Proxy == "" {
		for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
			if v := os.Getenv(k); v != "" {
				c.Proxy = v
				break
			}
		}
	}
}

// ColorEnabled resolves the tri-state color setting with NO_COLOR support.
func (c *Config) ColorEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if c.Color == nil {
		return true
	}
	return *c.Color
}
