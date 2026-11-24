package app

import (
	"context"
	"fmt"
	"strings"

	"ggufget/internal/cli"
	"ggufget/internal/i18n"
	"ggufget/internal/pick"
	"ggufget/internal/provider"
	"ggufget/internal/units"
)

// viewOpts is the shared "how the list looks" struct. Index numbers shown by
// `list` and accepted by `get -f <n>` come from exactly this view.
type viewOpts struct {
	any    bool
	filter string
	sort   provider.SortMode
	limit  int
}

func buildView(files []provider.File, o viewOpts) []provider.File {
	out := files
	if !o.any {
		gguf := make([]provider.File, 0, len(out))
		for _, f := range out {
			if f.IsGGUF() {
				gguf = append(gguf, f)
			}
		}
		out = gguf
	}
	out = provider.FilterFiles(out, o.filter)
	out = provider.SortFiles(out, o.sort)
	if o.limit > 0 && len(out) > o.limit {
		out = out[:o.limit]
	}
	return out
}

func (a *App) cmdList(ctx context.Context, rest []string) error {
	repo, err := a.repoArg(rest)
	if err != nil {
		return err
	}
	view, err := a.listView()
	if err != nil {
		return err
	}
	p, meta, files, err := a.fetchListing(ctx, repo, a.P.Str("rev"))
	if err != nil {
		return err
	}
	a.printList(repo, p, meta, files, buildView(files, view))
	return nil
}

// listView reads the shared --any/--filter/--sort/--limit options. Both list
// and get use it, so an index means the same file in both commands.
func (a *App) listView() (viewOpts, error) {
	sortMode, ok := provider.ParseSort(a.P.StrOr("sort", "name"))
	if !ok {
		return viewOpts{}, cli.Badf("--sort %s", a.P.Str("sort"))
	}
	return viewOpts{
		any:    a.P.Bool("any"),
		filter: a.P.Str("filter"),
		sort:   sortMode,
		limit:  a.P.IntOr("limit", 0),
	}, nil
}

func (a *App) printList(repo string, p provider.Provider, meta *provider.Meta, all, view []provider.File) {
	if a.JSON {
		for i, f := range view {
			a.emitJSON(map[string]any{
				"event":    "file",
				"index":    i + 1,
				"file":     f.Path,
				"size":     f.Size,
				"quant":    units.Quant(f.Path),
				"sha256":   f.SHA256,
				"provider": p.Name(),
				"repo":     repo,
				"commit":   meta.Commit,
			})
		}
		return
	}
	if len(view) == 0 {
		fmt.Fprintf(a.Out, "%s\n", i18n.T("list.empty"))
		return
	}
	var total int64
	nameW := dispWidth(i18n.T("list.file"))
	for _, f := range view {
		total += f.Size
		if n := dispWidth(f.Path); n > nameW {
			nameW = n
		}
	}
	if nameW > 56 {
		nameW = 56
	}
	const wIdx, wSize = 4, 9
	title := "list.title"
	if !a.P.Bool("any") && countNonGGUF(all) > 0 {
		title = "list.titleGGUF"
	}
	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T(title, p.Name(), len(view), units.Bytes(total))))
	fmt.Fprintf(a.Out, "%s  %s  %s  %s\n",
		padW(i18n.T("list.index"), wIdx),
		padW(i18n.T("list.file"), nameW),
		padLeftW(i18n.T("list.size"), wSize),
		i18n.T("list.quant"))
	fmt.Fprintf(a.Out, "%s\n", a.St.Dim(strings.Repeat("-", wIdx+2+nameW+2+wSize+2+8)))
	for i, f := range view {
		fmt.Fprintf(a.Out, "%s  %s  %s  %s\n",
			a.St.Dim(padW(fmt.Sprintf("%d", i+1), wIdx)),
			padW(truncateW(f.Path, nameW), nameW),
			padLeftW(units.Bytes(f.Size), wSize),
			units.Quant(f.Path))
	}
	fmt.Fprintf(a.Out, "\n%s\n", a.St.Dim(i18n.T("list.pickHint", repo)))
}

func countNonGGUF(files []provider.File) int {
	n := 0
	for _, f := range files {
		if !f.IsGGUF() {
			n++
		}
	}
	return n
}

// dispWidth counts terminal cells: CJK and other wide runes take two.
func dispWidth(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
			(r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe30 && r <= 0xfe6f) ||
			(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
			(r >= 0x20000 && r <= 0x3fffd)) {
			w += 2
			continue
		}
		w++
	}
	return w
}

func padW(s string, w int) string {
	if d := dispWidth(s); d < w {
		return s + strings.Repeat(" ", w-d)
	}
	return s
}

func padLeftW(s string, w int) string {
	if d := dispWidth(s); d < w {
		return strings.Repeat(" ", w-d) + s
	}
	return s
}

func pad(s string, w int) string { return padW(s, w) }

// truncateW shortens s to at most w cells, keeping the tail (the quant is the
// interesting part of a long filename).
func truncateW(s string, w int) string {
	if dispWidth(s) <= w {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && dispWidth(string(runes)) > w {
		runes = runes[1:]
	}
	if w > 1 {
		return "…" + string(runes)
	}
	return string(runes)
}

// resolveSelection turns -f values into repository files.
//
// Index expressions are interpreted in the same view `list` shows, so
// `get --sort size -f 2` selects the second row of `list --sort size`.
// Filenames and keywords are matched against the whole repository, so a
// non-gguf file can be named exactly without --any. An out-of-range index is an
// error: it must never fall back to a position in the full file list.
func (a *App) resolveSelection(files []provider.File, queries []string, view viewOpts) ([]provider.File, error) {
	viewFiles := buildView(files, view)
	allPaths := make([]string, len(files))
	for i, f := range files {
		allPaths[i] = f.Path
	}

	chosen := make([]provider.File, 0, len(queries))
	seen := map[string]bool{}
	add := func(f provider.File) {
		if seen[f.Path] {
			return
		}
		seen[f.Path] = true
		chosen = append(chosen, f)
	}
	var problems []string

	for _, raw := range queries {
		q := pick.Clean(raw)
		if q == "" {
			continue
		}
		if pick.IsIndexExpr(q) {
			idxs, ok := pick.Indices(q, len(viewFiles))
			if !ok {
				problems = append(problems, i18n.T("err.indexRange", raw, len(viewFiles)))
				continue
			}
			for _, k := range idxs {
				f := viewFiles[k-1]
				fmt.Fprintln(a.ErrOut, a.St.Dim(i18n.T("sel.index", raw, f.Path)))
				add(f)
			}
			continue
		}

		hits, misses := pick.Resolve(allPaths, []string{q})
		if len(hits) == 1 {
			f := files[hits[0].Index]
			fmt.Fprintln(a.ErrOut, a.St.Dim(i18n.T("sel.fuzzy", raw, f.Path)))
			add(f)
			continue
		}
		if len(misses) > 0 && len(misses[0].Candidates) > 0 {
			problems = append(problems, i18n.T("err.selectAmbiguous", raw, len(misses[0].Candidates)))
			for _, ci := range misses[0].Candidates {
				problems = append(problems, fmt.Sprintf("    %s", allPaths[ci]))
			}
			continue
		}
		problems = append(problems, i18n.T("err.selectNoMatch", raw))
	}

	if len(problems) > 0 {
		problems = append(problems, i18n.T("err.selectHint"))
		return chosen, cli.Badf("%s", strings.Join(problems, "\n"))
	}
	return chosen, nil
}
