package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ggufget/internal/cli"
	"ggufget/internal/config"
	"ggufget/internal/fsutil"
	"ggufget/internal/i18n"
)

// cmdConfig shows the effective configuration and where it comes from.
//
// Editing JSON by hand is fine, but only if the tool tells you which file it
// reads and what the merged result is; otherwise people debug a config the
// binary never opened.
func (a *App) cmdConfig(rest []string) error {
	path := config.Path()
	_, statErr := os.Stat(path)
	exists := statErr == nil

	if a.P.Bool("init") {
		if exists {
			fmt.Fprintf(a.Out, "%s\n", i18n.T("cfg.initExists", path))
			return nil
		}
		if err := fsutil.EnsureDir(filepath.Dir(path)); err != nil {
			return err
		}
		body, err := json.MarshalIndent(a.Cfg, "", "  ")
		if err != nil {
			return err
		}
		body = append(body, '\n')
		if err := fsutil.WriteFileAtomic(path, body, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "%s\n", i18n.T("cfg.initDone", path))
		return nil
	}

	if a.JSON {
		a.emitJSON(map[string]any{
			"event":  "config",
			"path":   path,
			"exists": exists,
			"config": map[string]any{
				"provider":    a.Cfg.Provider,
				"hf_endpoint": a.Cfg.HFEndpoint,
				"ms_endpoint": a.Cfg.MSEndpoint,
				"models_dir":  a.Cfg.ModelsDir,
				"layout":      a.Cfg.Layout,
				"connections": a.Cfg.Connections,
				"jobs":        a.Cfg.Jobs,
				"retries":     a.Cfg.Retries,
				"timeout":     a.Cfg.Timeout,
				"verify":      a.Cfg.Verify,
				"lang":        a.Cfg.Lang,
				"log":         a.Cfg.Log,
				"log_level":   a.Cfg.LogLevel,
			},
		})
		return nil
	}

	state := i18n.T("cfg.missing")
	if exists {
		state = i18n.T("cfg.exists")
	}
	fmt.Fprintf(a.Out, "%s %s (%s)\n\n", a.St.Bold(i18n.T("cfg.title")), path, state)
	rows := [][2]string{
		{i18n.T("cfg.provider"), a.Cfg.Provider},
		{i18n.T("cfg.hfEndpoint"), a.Cfg.HFEndpoint},
		{i18n.T("cfg.msEndpoint"), a.Cfg.MSEndpoint},
		{i18n.T("cfg.modelsDir"), a.Cfg.ModelsDir},
		{i18n.T("cfg.layout"), a.Cfg.Layout},
		{i18n.T("cfg.connections"), numberOrAuto(a.Cfg.Connections, i18n.T("cfg.auto"))},
		{i18n.T("cfg.jobs"), numberOrAuto(a.Cfg.Jobs, i18n.T("cfg.auto"))},
		{i18n.T("cfg.retries"), fmt.Sprint(a.Cfg.Retries)},
		{i18n.T("cfg.timeout"), fmt.Sprint(a.Cfg.Timeout)},
		{i18n.T("cfg.verify"), a.Cfg.Verify},
		{i18n.T("cfg.lang"), a.Cfg.Lang},
		{i18n.T("cfg.log"), a.Cfg.Log},
		{i18n.T("cfg.logLevel"), a.Cfg.LogLevel},
	}
	// Labels are Chinese or English depending on --lang, so pad by display
	// width: len() counts a CJK glyph as three bytes and the columns fall apart.
	labelWidth := 0
	for _, r := range rows {
		if w := dispWidth(r[0]); w > labelWidth {
			labelWidth = w
		}
	}
	for _, r := range rows {
		value := r[1]
		if value == "" {
			value = a.St.Dim("-")
		}
		fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(padW(r[0], labelWidth)), value)
	}
	fmt.Fprintf(a.Out, "\n%s\n", a.St.Dim(i18n.T("cfg.hint")))
	return nil
}

// numberOrAuto renders 0 as "auto", because that is what 0 means everywhere
// else in this tool (connections, jobs).
func numberOrAuto(n int, auto string) string {
	if n <= 0 {
		return auto
	}
	return fmt.Sprint(n)
}

// configDefs describes the options that only the config command understands.
func configDefs() []cli.Def {
	return []cli.Def{
		{Name: "init", Kind: cli.Bool, Key: "flag.init"},
	}
}
