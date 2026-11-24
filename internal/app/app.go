// Package app wires the pieces together: it turns argv into a reporter, a
// configuration and a set of mirrors, then dispatches to one command.
package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ggufget/internal/cli"
	"ggufget/internal/config"
	"ggufget/internal/downloader"
	"ggufget/internal/fsutil"
	"ggufget/internal/i18n"
	"ggufget/internal/logx"
	"ggufget/internal/netx"
	"ggufget/internal/provider"
	"ggufget/internal/termui"
	"ggufget/internal/ui"
)

// App carries everything a command needs.
type App struct {
	Cfg             *config.Config
	CfgPath         string
	CfgPathExplicit bool
	Out             io.Writer
	ErrOut          io.Writer
	Rep             *termui.Reporter
	St              termui.Style
	Client          *http.Client
	P               *cli.Parsed
	Console         *ui.Console
	Verbose         bool
	JSON            bool
	Log             *logx.Logger
	in              *bufio.Reader
}

// readLine reads one line from stdin. Interactive prompts go through the
// console (so TAB and arrow keys work); the buffered fallback stays for code
// paths that run before the console exists.
func (a *App) readLine() (string, error) {
	if a.Console != nil {
		return a.Console.ReadLine("")
	}
	if a.in == nil {
		a.in = bufio.NewReader(os.Stdin)
	}
	line, err := a.in.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

var globalDefs = []cli.Def{
	{Name: "help", Short: "h", Kind: cli.Bool, Key: "flag.help"},
	{Name: "version", Short: "V", Kind: cli.Bool, Key: "flag.version"},
	{Name: "quiet", Short: "q", Kind: cli.Bool, Key: "flag.quiet"},
	{Name: "verbose", Short: "v", Kind: cli.Bool, Key: "flag.verbose"},
	{Name: "json", Kind: cli.Bool, Key: "flag.json"},
	{Name: "no-color", Kind: cli.Bool, Key: "flag.nocolor"},
	{Name: "no-progress", Kind: cli.Bool, Key: "flag.noprogress"},
	{Name: "lang", Kind: cli.Str, Key: "flag.lang"},
	{Name: "config", Kind: cli.Str, Key: "flag.config"},
	{Name: "no-config", Kind: cli.Bool, Key: "flag.noconfig"},
	{Name: "log", Kind: cli.Str, Key: "flag.log"},
	{Name: "no-log", Kind: cli.Bool, Key: "flag.nolog"},
	{Name: "log-level", Kind: cli.Str, Key: "flag.loglevel"},
	{Name: "proxy", Kind: cli.Str, Key: "flag.proxy"},
	{Name: "timeout", Kind: cli.Int, Key: "flag.timeout"},
}

var listDefs = []cli.Def{
	{Name: "any", Kind: cli.Bool, Key: "flag.any"},
	{Name: "filter", Kind: cli.Str, Key: "flag.filter"},
	{Name: "sort", Kind: cli.Str, Key: "flag.sort"},
	{Name: "limit", Kind: cli.Int, Key: "flag.limit"},
}

var getDefs = []cli.Def{
	{Name: "rev", Short: "b", Kind: cli.Str, Key: "flag.rev"},
	{Name: "file", Short: "f", Kind: cli.Str, Key: "flag.file"},
	{Name: "all", Short: "a", Kind: cli.Bool, Key: "flag.all"},
	{Name: "provider", Short: "p", Kind: cli.Str, Key: "flag.provider"},
	{Name: "out", Short: "o", Kind: cli.Str, Key: "flag.out"},
	{Name: "jobs", Short: "j", Kind: cli.Int, Key: "flag.jobs"},
	{Name: "connections", Short: "c", Kind: cli.Int, Key: "flag.conns"},
	{Name: "retries", Kind: cli.Int, Key: "flag.retries"},
	{Name: "verify", Kind: cli.Str, Key: "flag.verify"},
	{Name: "overwrite", Kind: cli.Bool, Key: "flag.overwrite"},
	{Name: "yes", Short: "y", Kind: cli.Bool, Key: "flag.yes"},
	{Name: "dry-run", Kind: cli.Bool, Key: "flag.dryrun"},
	{Name: "keep-part", Kind: cli.Bool, Key: "flag.keeppart"},
	{Name: "token", Kind: cli.Str, Key: "flag.token"},
	{Name: "no-lmstudio", Kind: cli.Bool, Key: "flag.nolmstudio"},
	{Name: "layout", Kind: cli.Str, Key: "flag.layout"},
	{Name: "sha256", Kind: cli.Str, Key: "flag.sha256"},
}

func allDefs() []cli.Def {
	out := make([]cli.Def, 0, len(globalDefs)+len(listDefs)+len(getDefs)+len(configDefs()))
	out = append(out, globalDefs...)
	out = append(out, listDefs...)
	out = append(out, getDefs...)
	out = append(out, configDefs()...)
	return out
}

// Run is the process entry point; it returns the exit code.
func Run(args []string) int {
	a := &App{Out: os.Stdout, ErrOut: os.Stderr}
	p, err := cli.Parse(args, allDefs())
	if err != nil {
		a.reportArgError(err)
		return 2
	}
	a.P = p

	cfgPath, cfgExplicit := resolveConfigPath(p)
	cfg, warn := config.Load(cfgPath, p.Bool("no-config"))
	a.Cfg = cfg
	a.CfgPath = cfgPath
	a.CfgPathExplicit = cfgExplicit

	lang := p.StrOr("lang", cfg.Lang)
	if lang != "" {
		i18n.Set(lang)
	} else {
		i18n.FromEnv()
	}
	if warn != "" {
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("cfg.bad", warn))
	}

	mode := termui.ModeText
	switch {
	case p.Bool("json"):
		mode = termui.ModeJSON
	case p.Bool("quiet"):
		mode = termui.ModeQuiet
	}
	a.JSON = mode == termui.ModeJSON
	a.Rep = termui.NewReporter(a.Out, a.ErrOut, mode)
	a.St = termui.NewStyle(a.ErrOut)
	if p.Bool("no-color") || !cfg.ColorEnabled() {
		a.St.Enabled = false
		a.Rep.Color = false
	}
	a.Verbose = p.Bool("verbose")

	a.Console = ui.NewConsole(os.Stdin, a.Out)
	defer a.Console.Close()
	if p.Bool("no-color") || !cfg.ColorEnabled() {
		a.Console.Color = false
	}

	a.openLog(p, cfg)
	defer a.Log.Close()
	if a.Log.Enabled() {
		a.Log.Infof("start version=%s go=%s os=%s/%s pid=%d exe=%q cwd=%q args=%q",
			netx.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, os.Getpid(),
			executablePath(), workingDir(), logx.RedactArgs(os.Args[1:]))
		a.Log.Infof("config path=%q models_dir=%q layout=%q provider=%q hf_endpoint=%q ms_endpoint=%q token=%q config_warn=%q",
			a.CfgPath, a.Cfg.ModelsDir, a.Cfg.Layout, a.Cfg.Provider,
			a.Cfg.HFEndpoint, a.Cfg.MSEndpoint, maskSecret(a.Cfg.Token), warn)
	}

	timeout := p.IntOr("timeout", cfg.Timeout)
	if timeout <= 0 {
		timeout = 30
	}
	a.Client = netx.New(netx.Options{
		HeaderTimeout: time.Duration(timeout) * time.Second,
		Proxy:         p.StrOr("proxy", cfg.Proxy),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rc := a.exec(ctx, p)
	a.Log.Infof("session end exit=%d", rc)
	return rc
}

// openLog resolves the level and destination, then opens the log. Failure is
// never fatal: the command still runs, and every rejected path is reported so
// "why is there no log" has an answer.
func (a *App) openLog(p *cli.Parsed, cfg *config.Config) {
	path := pickLogPath(p.Str("log"), os.Getenv("GGUFGET_LOG"), cfg.Log)
	levelName := p.Str("log-level")
	if levelName == "" {
		levelName = os.Getenv("GGUFGET_LOG_LEVEL")
	}
	if levelName == "" {
		levelName = cfg.LogLevel
	}
	level, ok := logx.ParseLevel(levelName)
	if !ok && strings.TrimSpace(levelName) != "" {
		fmt.Fprintf(a.ErrOut, "%s\n", a.St.Dim(i18n.T("err.badLoglevel", levelName)))
	}
	if a.Verbose {
		level = logx.LevelDebug
	}
	l, attempts := logx.Open(logx.Options{
		Path:     path,
		Level:    level,
		Disabled: p.Bool("no-log"),
	})
	a.Log = l
	if l == nil && len(attempts) > 0 {
		for _, at := range attempts {
			fmt.Fprintf(a.ErrOut, "%s\n", a.St.Dim(i18n.T("err.logDisabled", at.Path, shortText(at.Err.Error()))))
		}
	}
}

// pickLogPath applies the documented log destination precedence: --log, then
// GGUFGET_LOG, then the config file's log field. Empty means "use the default
// candidate chain next to the binary".
func pickLogPath(flagPath, envPath, cfgPath string) string {
	for _, p := range []string{flagPath, envPath, cfgPath} {
		if v := strings.TrimSpace(p); v != "" {
			return v
		}
	}
	return ""
}

// resolveConfigPath decides which config file to use: an explicit --config,
// then GGUFGET_CONFIG, then a portable ggufget.config.json sitting next to the
// executable (written when the default location turned out not to be writable),
// then the per-user default.
func resolveConfigPath(p *cli.Parsed) (string, bool) {
	if custom := p.Str("config"); custom != "" {
		return fsutil.ExpandHome(custom), true
	}
	if strings.TrimSpace(os.Getenv("GGUFGET_CONFIG")) != "" {
		return config.Path(), true
	}
	if exe := executablePath(); exe != "" {
		portable := filepath.Join(filepath.Dir(exe), portableConfigName)
		if _, ok, err := fsutil.Size(portable); err == nil && ok {
			return portable, false
		}
	}
	return config.Path(), false
}

func executablePath() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return ""
}

func workingDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// maskSecret keeps a token out of the log while still showing whether one was
// configured.
func maskSecret(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return "***"
}

// exec dispatches one already-parsed command line. The interactive menu calls
// it through runArgs, so a menu choice uses exactly the same code path as argv.
func (a *App) exec(ctx context.Context, p *cli.Parsed) int {
	a.P = p
	if p.Has("version") {
		a.printVersion()
		return 0
	}
	if p.Has("help") {
		a.printHelp(p.Pos)
		return 0
	}
	if len(p.Pos) == 0 {
		return a.interactive(ctx)
	}

	cmd, rest := p.Pos[0], p.Pos[1:]
	switch cmd {
	case "list":
		return a.run(ctx, a.cmdList(ctx, rest))
	case "get":
		return a.run(ctx, a.cmdGet(ctx, rest))
	case "info":
		return a.run(ctx, a.cmdInfo(ctx, rest))
	case "search":
		return a.run(ctx, a.cmdSearch(ctx, rest))
	case "doctor":
		return a.run(ctx, a.cmdDoctor(ctx, rest))
	case "config":
		return a.run(ctx, a.cmdConfig(rest))
	case "version":
		a.printVersion()
		return 0
	case "h", "help":
		a.printHelp(rest)
		return 0
	default:
		if s := cli.Closest(cmd, commandList); s != "" {
			fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.suggestCmd", s))
		}
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.unknownCmd", cmd))
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.tryHelp"))
		return 2
	}
}

// runArgs re-parses a synthesized command line and runs it; used by the menu.
func (a *App) runArgs(ctx context.Context, args ...string) int {
	p, err := cli.Parse(args, allDefs())
	if err != nil {
		a.reportArgError(err)
		return 2
	}
	return a.exec(ctx, p)
}

func (a *App) run(ctx context.Context, err error) int {
	if err == nil {
		a.settle()
		return 0
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.interrupt", ".part"))
		a.Log.Warnf("interrupted by the user")
		a.reportLog()
		return 130
	}
	// Every mirror failing deserves one line per mirror; a single concatenated
	// line reads like an internal Go error and hides which mirror to blame.
	var me *provider.MirrorsError
	if errors.As(err, &me) {
		if len(me.Fails) == 1 {
			// Only one mirror was asked (--provider), so there is no list to show.
			err = me.Fails[0].Err
		} else {
			fmt.Fprintf(a.ErrOut, "%s\n", a.St.Bad(i18n.T("err.mirrorsFailed", me.Repo)))
			for _, f := range me.Fails {
				fmt.Fprintf(a.ErrOut, "  %-10s %s\n", f.Name, a.St.Dim(shortReason(f.Err)))
			}
			fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.mirrorsHint"))
			return a.endError(err)
		}
	}
	switch {
	case errors.Is(err, provider.ErrNotFound):
		fmt.Fprintf(a.ErrOut, "%s\n", a.St.Bad(i18n.T("err.notFoundText")))
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.notFoundHint"))
	case errors.Is(err, provider.ErrAuthRequired):
		fmt.Fprintf(a.ErrOut, "%s\n", a.St.Bad(i18n.T("err.private")))
	default:
		var re *reportedError
		if errors.As(err, &re) {
			// Already printed next to the file; repeat it and the console shows
			// the same failure twice, which reads like two separate problems.
			return a.endError(err)
		}
		fmt.Fprintf(a.ErrOut, "%s\n", a.St.Bad(err.Error()))
	}
	return a.endError(err)
}

// endError records the failure and points the user at the log, which is the
// only place that carries the full errno/status detail.
func (a *App) endError(err error) int {
	a.Log.Error("command failed", err)
	a.reportLog()
	a.settle()
	return exitCode(err)
}

func (a *App) reportLog() {
	if a.Log.Enabled() {
		fmt.Fprintf(a.ErrOut, "%s\n", a.St.Dim(i18n.T("err.logPath", a.Log.Path())))
	}
}

// shortReason turns a transport error into the one phrase that tells the user
// what to do: a status code, "timeout", or a trimmed cause. Printing the raw
// Go error wraps three terminal lines and buries the useful word.
func shortReason(err error) string {
	switch {
	case errors.Is(err, provider.ErrNotFound):
		return i18n.T("err.http404")
	case errors.Is(err, provider.ErrAuthRequired):
		return i18n.T("err.authShort")
	}
	var he *provider.HTTPError
	if errors.As(err, &he) {
		return fmt.Sprintf("HTTP %d", he.Status)
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		if ue.Timeout() {
			return i18n.T("err.timeout")
		}
		return shortText(ue.Err.Error())
	}
	return shortText(err.Error())
}

func shortText(s string) string {
	if len(s) > 90 {
		return s[:87] + "..."
	}
	return s
}

// settle holds a console opened by double-click so the user can read the last
// lines instead of watching the window vanish.
func (a *App) settle() {
	if !doubleClickConsole() {
		return
	}
	fmt.Fprint(a.ErrOut, "\n"+i18n.T("ui.back")+" ")
	buf := make([]byte, 1)
	_, _ = os.Stdin.Read(buf)
}

func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, context.Canceled):
		return 130
	case errors.Is(err, downloader.ErrDiskFull):
		return 5
	case errors.Is(err, downloader.ErrVerify), errors.Is(err, downloader.ErrNoHash):
		return 4
	case errors.Is(err, provider.ErrNotFound):
		return 6
	case errors.Is(err, os.ErrPermission):
		return 7
	case errors.Is(err, cli.ErrBadValue):
		return 2
	case errors.Is(err, os.ErrNotExist):
		return 7
	}
	var uf *cli.UnknownFlagError
	if errors.As(err, &uf) {
		return 2
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return 3
	}
	var he *provider.HTTPError
	if errors.As(err, &he) {
		return 3
	}
	if errors.Is(err, downloader.ErrRateLimited) || errors.Is(err, downloader.ErrInterrupted) {
		return 3
	}
	return 1
}

func (a *App) reportArgError(err error) {
	var uf *cli.UnknownFlagError
	if errors.As(err, &uf) {
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.unknownFlag", uf.Flag))
		if uf.Suggest != "" {
			fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.suggestFlag", uf.Suggest))
		}
		fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.tryHelp"))
		return
	}
	fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.usage", err.Error()))
	fmt.Fprintf(a.ErrOut, "%s\n", i18n.T("err.tryHelp"))
}

// token returns the access token from the flag, the config or the environment.
func (a *App) token() string {
	if t := a.P.Str("token"); t != "" {
		return t
	}
	return a.Cfg.Token
}

// buildProviders returns mirrors in the order the user asked for.
func (a *App) buildProviders(kind string) []provider.Provider {
	tok := a.token()
	ms := provider.NewMS(a.Cfg.MSEndpoint, a.Client, tok)
	hfm := provider.NewHF("hf-mirror", a.Cfg.HFEndpoint, a.Client, tok)
	hf := provider.NewHF("hf", "https://huggingface.co", a.Client, tok)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "modelscope", "ms":
		return []provider.Provider{ms}
	case "hf-mirror", "mirror":
		return []provider.Provider{hfm}
	case "hf", "huggingface":
		return []provider.Provider{hf}
	}
	return []provider.Provider{ms, hfm, hf}
}

func (a *App) providerKind() string {
	return a.P.StrOr("provider", a.Cfg.Provider)
}

func (a *App) checkProviderFlag() error {
	kind := strings.ToLower(a.providerKind())
	switch kind {
	case "", "auto", "modelscope", "ms", "hf-mirror", "mirror", "hf", "huggingface":
		return nil
	}
	return cli.Badf("--provider %s", kind)
}

// logf writes one line to the diagnostic log.
func (a *App) logf(format string, args ...any) {
	if a.Log == nil {
		return
	}
	a.Log.Infof(format, args...)
}

// emitJSON writes one NDJSON event when --json is active.
func (a *App) emitJSON(v map[string]any) {
	if !a.JSON {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintf(a.Out, "%s\n", b)
}

// repoArg normalises the repo positional argument.
func (a *App) repoArg(rest []string) (string, error) {
	if len(rest) == 0 || strings.TrimSpace(rest[0]) == "" {
		return "", cli.Badf("%s", i18n.T("err.repoRequired"))
	}
	repo, err := provider.NormalizeRepo(rest[0])
	if err != nil {
		return "", cli.Badf("%s（%s）", i18n.T("err.repoFormat"), rest[0])
	}
	return repo, nil
}

// fetchListing picks a mirror and lists the repo through it.
func (a *App) fetchListing(ctx context.Context, repo, rev string) (provider.Provider, *provider.Meta, []provider.File, error) {
	if err := a.checkProviderFlag(); err != nil {
		return nil, nil, nil, err
	}
	provs := a.buildProviders(a.providerKind())
	auto := provider.NewAuto(3*time.Second, provs...)
	return auto.List(ctx, repo, rev)
}
