package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ggufget/internal/cli"
	"ggufget/internal/i18n"
	"ggufget/internal/netx"
	"ggufget/internal/ui"
)

func (a *App) printVersion() {
	fmt.Fprintf(a.Out, "ggufget %s (%s/%s, %s)\n",
		netx.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// printHelp shows the command overview, or the detailed help for one command.
func (a *App) printHelp(pos []string) {
	if len(pos) == 0 {
		a.printOverview()
		return
	}
	cmd := strings.ToLower(strings.TrimSpace(pos[0]))
	if cmd == "h" {
		cmd = "help"
	}
	if !knownCommand(cmd) {
		if s := cli.Closest(cmd, commandList); s != "" {
			fmt.Fprintf(a.Out, "%s\n\n", i18n.T("err.suggestCmd", s))
		}
		fmt.Fprintf(a.Out, "%s\n\n", i18n.T("err.unknownCmd", cmd))
		a.printOverview()
		return
	}
	a.printCommandHelp(cmd)
}

// printOverview is the screen `h` and `ggufget help` show: every command, what
// it does and how to call it, then the global options. The long-form manual is
// shipped as a separate document, not embedded in the binary.
func (a *App) printOverview() {
	fmt.Fprintf(a.Out, "%s %s\n", a.St.Bold("ggufget"), netx.Version)
	fmt.Fprintf(a.Out, "%s\n\n", a.St.Dim(i18n.T("app.tagline")))

	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T("help.commands")))
	width := 0
	for _, c := range commandList {
		if n := len("ggufget " + usageOf(c)); n > width {
			width = n
		}
	}
	for _, c := range commandList {
		usage := "ggufget " + usageOf(c)
		fmt.Fprintf(a.Out, "  %s  %s\n", padW(usage, width), i18n.T("cmd."+c))
	}

	fmt.Fprintf(a.Out, "\n%s\n", a.St.Bold(i18n.T("help.globalOptions")))
	a.printFlagTable(globalDefs)

	fmt.Fprintf(a.Out, "\n%s\n", a.St.Bold(i18n.T("help.tips")))
	for _, k := range []string{"help.tipHelp", "help.tipTab", "help.tipDocs"} {
		fmt.Fprintf(a.Out, "  %s\n", i18n.T(k))
	}
	a.printDocsPointer()
	fmt.Fprintf(a.Out, "\n%s\n", a.St.Dim(i18n.T("help.exitcodes")))
}

func (a *App) printCommandHelp(cmd string) {
	fmt.Fprintf(a.Out, "%s\n\n", a.St.Bold("ggufget "+usageOf(cmd)))
	fmt.Fprintf(a.Out, "%s\n\n", i18n.T("cmd."+cmd))
	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T("help.options")))
	a.printFlagTable(commandDefs(cmd))
	if cmd == "get" {
		fmt.Fprintf(a.Out, "\n%s\n", i18n.T("help.getIndexRule"))
	}
	if ex := i18n.T("help.ex." + cmd); ex != "" && !strings.HasPrefix(ex, "help.") {
		fmt.Fprintf(a.Out, "\n%s\n%s\n", a.St.Bold(i18n.T("help.examples")), ex)
	}
	fmt.Fprintf(a.Out, "\n%s\n", a.St.Dim(i18n.T("help.exitcodes")))
	a.printDocsPointer()
}

// printDocsPointer names the external manual and, when the document happens to
// sit next to the binary, prints its absolute path so it can be opened directly.
func (a *App) printDocsPointer() {
	fmt.Fprintf(a.Out, "\n%s\n", a.St.Bold(i18n.T("help.docs")))
	if p := findDoc("MANUAL.md"); p != "" {
		fmt.Fprintf(a.Out, "  %s\n", p)
	}
	fmt.Fprintf(a.Out, "  %s\n", i18n.T("help.docsWhere"))
}

func findDoc(name string) string {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, name), filepath.Join(dir, "docs", name))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, name))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// showHelp renders the same text into the built-in pager, so `h` inside the
// interactive shell never shells out to less/more (they mangle UTF-8 in the
// Windows console).
func (a *App) showHelp(c *ui.Console, topic string) {
	var b strings.Builder
	old := a.Out
	a.Out = &b
	if topic == "" {
		a.printOverview()
	} else {
		a.printHelp([]string{topic})
	}
	a.Out = old
	_ = c.Page("", b.String())
}

// commandList is the set of subcommands, in help order.
var commandList = []string{"list", "get", "info", "search", "doctor", "config", "version", "help"}

func knownCommand(cmd string) bool {
	for _, c := range commandList {
		if c == cmd {
			return true
		}
	}
	return false
}

func usageOf(cmd string) string {
	switch cmd {
	case "list":
		return "list <repo> [options]"
	case "get":
		return "get <repo> [options]"
	case "info":
		return "info <repo>"
	case "search":
		return "search <keyword>"
	case "doctor":
		return "doctor"
	case "config":
		return "config [options]"
	}
	return cmd
}

func commandDefs(cmd string) []cli.Def {
	out := append([]cli.Def{}, globalDefs...)
	switch cmd {
	case "list":
		out = append(out, listDefs...)
	case "get":
		out = append(out, getDefs...)
		out = append(out, listDefs...)
	case "config":
		out = append(out, configDefs()...)
	}
	return out
}

// printFlagTable aligns the left column so the descriptions line up.
func (a *App) printFlagTable(defs []cli.Def) {
	type row struct{ left, desc string }
	rows := make([]row, 0, len(defs))
	width := 0
	for _, d := range defs {
		left := "    --" + d.Name
		if d.Short != "" {
			left = "  -" + d.Short + ", --" + d.Name
		}
		if d.Kind != cli.Bool {
			left += " <" + i18n.T("help.value") + ">"
		}
		desc := i18n.T(d.Key)
		if strings.HasPrefix(desc, "flag.") {
			desc = ""
		}
		rows = append(rows, row{left, desc})
		if len(left) > width {
			width = len(left)
		}
	}
	for _, r := range rows {
		fmt.Fprintf(a.Out, "%-*s  %s\n", width, r.left, r.desc)
	}
}
