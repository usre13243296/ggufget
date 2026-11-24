package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ggufget/internal/config"
	"ggufget/internal/i18n"
	"ggufget/internal/lmstudio"
	"ggufget/internal/netx"
	"ggufget/internal/provider"
	"ggufget/internal/ui"
)

// interactive is the shell you get when ggufget is started without arguments.
// It mirrors what a classic console downloader does: show the mirrors and their
// measured speed, then let one screen do listing and downloading together.
func (a *App) interactive(ctx context.Context) int {
	c := a.Console
	if c == nil {
		return 1
	}
	if len(a.Cfg.History) > 0 {
		c.History = append(c.History, a.Cfg.History...)
	}
	c.Complete = a.completer()
	defer a.persistShell(c.History)

	for {
		if ctx.Err() != nil {
			return 130
		}
		a.printHeader(c)
		key, err := c.Choose(i18n.T("ui.prompt"), mainOptions(), i18n.T("ui.mainHint"))
		switch {
		case err == nil:
		case errors.Is(err, ui.ErrQuit), errors.Is(err, io.EOF):
			fmt.Fprintln(a.Out, i18n.T("ui.bye"))
			return 0
		case errors.Is(err, ui.ErrInterrupt), errors.Is(err, context.Canceled):
			return 130
		default:
			return a.run(ctx, err)
		}
		switch key {
		case "1":
			a.menuDownload(ctx, c)
		case "2":
			a.menuSearch(ctx, c)
		case "3":
			a.menuSpeed(ctx, c)
		case "4":
			a.menuSettings(ctx, c)
		case "h":
			a.showHelp(c, "")
		}
	}
}

func mainOptions() []ui.Option {
	return []ui.Option{
		{Key: "1", Label: i18n.T("ui.cmdDownload"), Hint: i18n.T("ui.cmdDownloadHint")},
		{Key: "2", Label: i18n.T("ui.cmdSearch"), Hint: i18n.T("ui.cmdSearchHint")},
		{Key: "3", Label: i18n.T("ui.cmdSpeed"), Hint: i18n.T("ui.cmdSpeedHint")},
		{Key: "4", Label: i18n.T("ui.cmdSettings"), Hint: i18n.T("ui.cmdSettingsHint")},
		{Key: "h", Label: i18n.T("ui.cmdHelp"), Hint: i18n.T("ui.cmdHelpHint")},
		{Key: "q", Label: i18n.T("ui.cmdQuit")},
	}
}

func (a *App) printHeader(c *ui.Console) {
	fmt.Fprintln(c.Out)
	fmt.Fprintf(c.Out, "  %s\n", c.Bold("ggufget "+netx.Version))
	rows := [][2]string{
		{i18n.T("ui.modelsDir"), a.modelsDirLabel()},
		{i18n.T("ui.mirrors"), a.speedLine()},
		{i18n.T("ui.lang"), i18n.Name(i18n.Cur())},
	}
	for _, r := range rows {
		fmt.Fprintf(c.Out, "  %s  %s\n", c.Dim(padW(r[0], 8)), r[1])
	}
}

func (a *App) modelsDirLabel() string {
	dir, err := lmstudio.ResolveDir("", a.Cfg.ModelsDir)
	if err != nil {
		return i18n.T("ui.modelsDirNone")
	}
	return dir
}

// speedLine shows the two fastest mirrors, which is the whole point of the
// start screen: the user should never have to guess which source is quick today.
func (a *App) speedLine() string {
	if len(a.Cfg.Speeds) == 0 {
		return i18n.T("ui.speedNone")
	}
	type row struct {
		name string
		mbps float64
	}
	var rows []row
	for _, name := range []string{"modelscope", "hf-mirror", "hf"} {
		if v, ok := a.Cfg.Speeds[name]; ok {
			rows = append(rows, row{name, v})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].mbps > rows[j].mbps })
	parts := make([]string, 0, 2)
	for i, r := range rows {
		if i == 2 {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %.1f MiB/s", r.name, r.mbps))
	}
	if len(parts) == 0 {
		return i18n.T("ui.speedNone")
	}
	return strings.Join(parts, "  ·  ")
}

func (a *App) persistShell(history []string) {
	if len(history) > 50 {
		history = history[len(history)-50:]
	}
	a.Cfg.History = append([]string{}, history...)
	if _, err := a.saveConfig(); err != nil {
		a.Log.Warnf("history not saved: %v", err)
	}
}

// saveConfig writes the config file. When the default location is not writable
// (a security product blocking an unsigned binary, a locked-down profile, a
// read-only home) it falls back to a portable file next to the executable, so
// the settings the user just changed are not silently lost. It returns the
// fallback path when that happened, and an error only when nothing was written.
func (a *App) saveConfig() (string, error) {
	err := config.Save(a.CfgPath, a.Cfg)
	if err == nil {
		return "", nil
	}
	a.Log.Error(fmt.Sprintf("config save failed path=%q", a.CfgPath), err)
	if a.CfgPathExplicit {
		return "", err
	}
	alt := filepath.Join(filepath.Dir(executablePath()), portableConfigName)
	if alt == a.CfgPath {
		return "", err
	}
	if altErr := config.Save(alt, a.Cfg); altErr != nil {
		a.Log.Error(fmt.Sprintf("portable config save failed path=%q", alt), altErr)
		return "", err
	}
	a.Log.Warnf("config saved to portable path=%q (default location not writable)", alt)
	a.CfgPath = alt
	return alt, nil
}

// portableConfigName is the file written next to the executable when the
// normal config location cannot be created.
const portableConfigName = "ggufget.config.json"

func (a *App) completer() func(string) []string {
	return func(line string) []string {
		low := strings.ToLower(strings.TrimSpace(line))
		pool := make([]string, 0, len(a.Cfg.History)+len(a.Cfg.Favorites))
		pool = append(pool, a.Cfg.History...)
		pool = append(pool, a.Cfg.Favorites...)
		var out []string
		for _, s := range pool {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(s), low) {
				out = append(out, s)
			}
		}
		return out
	}
}

func (a *App) menuDownload(ctx context.Context, c *ui.Console) {
	repo, err := a.promptRepo(c)
	if err != nil {
		return
	}
	a.downloadRepo(ctx, c, repo)
}

func (a *App) promptRepo(c *ui.Console) (string, error) {
	c.Complete = a.completer()
	defer func() { c.Complete = nil }()
	for {
		line, err := c.Pick(i18n.T("ui.repoPrompt"))
		if err != nil {
			return "", err
		}
		repo, err := provider.NormalizeRepo(line)
		if err != nil {
			fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("err.repoFormat")))
			continue
		}
		return repo, nil
	}
}

// downloadRepo lists the files, then keeps asking for a selection: typing a
// number downloads, "a" downloads everything, q goes back. Listing and
// downloading are deliberately the same screen.
func (a *App) downloadRepo(ctx context.Context, c *ui.Console, repo string) {
	p, meta, files, err := a.fetchListing(ctx, repo, "")
	if err != nil {
		a.run(ctx, err)
		return
	}
	if len(files) == 0 {
		fmt.Fprintf(c.Out, "  %s\n", i18n.T("list.empty"))
		return
	}
	// The shell shows the same default view `list` does: GGUF files only, so
	// the index the user types means the same thing in both places.
	view := buildView(files, viewOpts{})
	if len(view) == 0 {
		fmt.Fprintf(c.Out, "  %s\n", i18n.T("list.empty"))
		return
	}
	a.printList(repo, p, meta, files, view)
	for attempt := 0; attempt < 5; attempt++ {
		sel, err := c.Pick(i18n.T("ui.selectPrompt"))
		if err != nil {
			return
		}
		low := strings.ToLower(strings.TrimSpace(sel))
		var code int
		switch low {
		case "a", "all", "*":
			code = a.runArgs(ctx, "get", repo, "--all")
		default:
			code = a.runArgs(ctx, "get", repo, "-f", sel)
		}
		if code == 0 || code != 2 {
			return
		}
		fmt.Fprintf(c.Out, "  %s\n", c.Dim(i18n.T("ui.selectRetry")))
	}
}

func (a *App) menuSearch(ctx context.Context, c *ui.Console) {
	kw, err := c.Pick(i18n.T("ui.searchPrompt"))
	if err != nil {
		return
	}
	hits, err := provider.Search(ctx, a.Client, a.hfBase(), kw, 20)
	if err != nil {
		a.run(ctx, err)
		return
	}
	if len(hits) == 0 {
		fmt.Fprintf(c.Out, "  %s\n", i18n.T("search.none"))
		return
	}
	fmt.Fprintln(c.Out)
	for i, h := range hits {
		fmt.Fprintf(c.Out, "  %2d. %s  %s\n", i+1, padW(h.ID, 56), c.Dim(humanCount(h.Downloads)))
	}
	for try := 0; try < 5; try++ {
		sel, err := c.Pick(i18n.T("ui.searchPick"))
		if err != nil {
			return
		}
		idx, ok := pickIndex(sel, len(hits))
		if !ok {
			fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("ui.badChoice", sel)))
			continue
		}
		a.downloadRepo(ctx, c, hits[idx].ID)
		return
	}
}

func pickIndex(sel string, n int) (int, bool) {
	s := strings.TrimSpace(sel)
	if s == "" {
		return 0, false
	}
	v := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		v = v*10 + int(r-'0')
		if v > 1<<20 {
			return 0, false
		}
	}
	if v < 1 || v > n {
		return 0, false
	}
	return v - 1, true
}

func (a *App) menuSpeed(ctx context.Context, c *ui.Console) {
	fmt.Fprintf(c.Out, "\n  %s\n", i18n.T("ui.speedRunning"))
	res := a.speedTest(ctx)
	sort.SliceStable(res, func(i, j int) bool {
		if (res[i].err == nil) != (res[j].err == nil) {
			return res[i].err == nil
		}
		return res[i].mbps > res[j].mbps
	})
	speeds := map[string]float64{}
	for _, r := range res {
		if r.err != nil {
			fmt.Fprintf(c.Out, "  %-10s %s\n", r.name, c.Dim(i18n.T("ui.speedFail")))
			continue
		}
		fmt.Fprintf(c.Out, "  %-10s %s\n", r.name, c.Green(fmt.Sprintf("%6.1f MiB/s", r.mbps)))
		speeds[r.name] = r.mbps
	}
	if len(speeds) > 0 {
		a.Cfg.Speeds = speeds
		now := time.Now()
		a.Cfg.SpeedsAt = &now
		fallback, err := a.saveConfig()
		if err != nil {
			fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("ui.saveFailed", err.Error())))
			return
		}
		if fallback != "" {
			fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("ui.saveFallback", fallback)))
		}
		fmt.Fprintf(c.Out, "  %s\n", c.Dim(i18n.T("ui.speedSaved")))
	}
}

func (a *App) menuSettings(ctx context.Context, c *ui.Console) {
	for {
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintln(c.Out)
		fmt.Fprintf(c.Out, "  %s\n", c.Bold(i18n.T("ui.setTitle")))
		opts := []ui.Option{
			{Key: "1", Label: fmt.Sprintf("%s: %s", i18n.T("ui.setLang"), i18n.Name(i18n.Cur()))},
			{Key: "2", Label: fmt.Sprintf("%s: %s", i18n.T("ui.setLayout"), a.Cfg.Layout)},
			{Key: "3", Label: fmt.Sprintf("%s: %s", i18n.T("ui.setProvider"), a.providerKind())},
			{Key: "4", Label: fmt.Sprintf("%s: %s", i18n.T("ui.setModelsDir"), a.modelsDirLabel())},
			{Key: "q", Label: i18n.T("ui.back")},
		}
		key, err := c.Choose(i18n.T("ui.prompt"), opts, "")
		if err != nil {
			return
		}
		switch key {
		case "1":
			a.setLanguage(c)
		case "2":
			a.setLayout(c)
		case "3":
			a.setProvider(c)
		case "4":
			a.setModelsDir(c)
		}
	}
}

func (a *App) setLanguage(c *ui.Console) {
	langs := i18n.Langs()
	opts := make([]ui.Option, 0, len(langs))
	for i, l := range langs {
		opts = append(opts, ui.Option{Key: fmt.Sprint(i + 1), Label: i18n.Name(l), Hint: string(l)})
	}
	opts = append(opts, ui.Option{Key: "q", Label: i18n.T("ui.back")})
	key, err := c.Choose(i18n.T("ui.setLangPrompt"), opts, "")
	if err != nil {
		return
	}
	n, ok := pickIndex(key, len(langs))
	if !ok {
		return
	}
	i18n.Set(string(langs[n]))
	a.Cfg.Lang = string(langs[n])
	a.afterConfigChange(c)
}

func (a *App) setLayout(c *ui.Console) {
	opts := []ui.Option{
		{Key: "1", Label: string(lmstudio.LayoutLMStudio), Hint: i18n.T("ui.layoutLMSHint")},
		{Key: "2", Label: string(lmstudio.LayoutRepo), Hint: i18n.T("ui.layoutRepoHint")},
		{Key: "3", Label: string(lmstudio.LayoutFlat), Hint: i18n.T("ui.layoutFlatHint")},
		{Key: "q", Label: i18n.T("ui.back")},
	}
	key, err := c.Choose(i18n.T("ui.setLayoutPrompt"), opts, "")
	if err != nil {
		return
	}
	switch key {
	case "1":
		a.Cfg.Layout = string(lmstudio.LayoutLMStudio)
	case "2":
		a.Cfg.Layout = string(lmstudio.LayoutRepo)
	case "3":
		a.Cfg.Layout = string(lmstudio.LayoutFlat)
	default:
		return
	}
	a.afterConfigChange(c)
}

func (a *App) setProvider(c *ui.Console) {
	opts := []ui.Option{
		{Key: "1", Label: "auto", Hint: i18n.T("ui.providerAutoHint")},
		{Key: "2", Label: "modelscope", Hint: i18n.T("ui.providerMSHint")},
		{Key: "3", Label: "hf-mirror", Hint: i18n.T("ui.providerHFMHint")},
		{Key: "4", Label: "hf", Hint: i18n.T("ui.providerHFHint")},
		{Key: "q", Label: i18n.T("ui.back")},
	}
	key, err := c.Choose(i18n.T("ui.setProviderPrompt"), opts, "")
	if err != nil {
		return
	}
	switch key {
	case "1":
		a.Cfg.Provider = "auto"
	case "2":
		a.Cfg.Provider = "modelscope"
	case "3":
		a.Cfg.Provider = "hf-mirror"
	case "4":
		a.Cfg.Provider = "hf"
	default:
		return
	}
	a.afterConfigChange(c)
}

func (a *App) setModelsDir(c *ui.Console) {
	line, err := c.ReadLine(i18n.T("ui.setDirPrompt"))
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	a.Cfg.ModelsDir = line
	a.afterConfigChange(c)
}

func (a *App) afterConfigChange(c *ui.Console) {
	fallback, err := a.saveConfig()
	if err != nil {
		fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("ui.saveFailed", err.Error())))
		return
	}
	if fallback != "" {
		fmt.Fprintf(c.Out, "  %s\n", c.Yellow(i18n.T("ui.saveFallback", fallback)))
	}
	fmt.Fprintf(c.Out, "  %s\n", c.Green(i18n.T("ui.saved")))
}
