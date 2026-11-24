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
	sortMode, ok := provider.ParseSort(a.P.StrOr("sort", "name"))
	if !ok {
		return cli.Badf("--sort %s", a.P.Str("sort"))
	}
	p, meta, files, err := a.fetchListing(ctx, repo, a.P.Str("rev"))
	if err != nil {
		return err
	}
	view := buildView(files, viewOpts{
		any:    a.P.Bool("any"),
		filter: a.P.Str("filter"),
		sort:   sortMode,
		limit:  a.P.IntOr("limit", 0),
	})
	a.printList(repo, p, meta, files, view)
	return nil
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

// resolveSelection turns -f values into files of the repo. Queries are matched
// against the default view first (that is what the numbers refer to) and then
// against the full file list, so an explicit non-gguf filename still works.
func (a *App) resolveSelection(repo string, files []provider.File, queries []string) ([]provider.File, error) {
	view := buildView(files, viewOpts{sort: provider.SortName})
	paths := make([]string, len(view))
	for i, f := range view {
		paths[i] = f.Path
	}
	hits, misses := pick.Resolve(paths, queries)
	chosen := make([]provider.File, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		f := view[h.Index]
		if h.How == pick.HowIndex {
			fmt.Fprintln(a.ErrOut, a.St.Dim(i18n.T("sel.index", h.Query, f.Path)))
		} else {
			fmt.Fprintln(a.ErrOut, a.St.Dim(i18n.T("sel.fuzzy", h.Query, f.Path)))
		}
		if !seen[f.Path] {
			seen[f.Path] = true
			chosen = append(chosen, f)
		}
	}
	if len(misses) > 0 {
		// second chance: match the query against every file in the repo
		all := make([]string, len(files))
		for i, f := range files {
			all[i] = f.Path
		}
		for _, m := range misses {
			idx, ok := pick.Indices(m.Query, len(all))
			if !ok || len(idx) != 1 {
				continue
			}
			f := files[idx[0]]
			if !seen[f.Path] {
				seen[f.Path] = true
				chosen = append(chosen, f)
			}
		}
	}
	var fs []string
	for _, m := range misses {
		if len(m.Candidates) > 0 {
			fs = append(fs, i18n.T("err.selectAmbiguous", m.Query, len(m.Candidates)))
			for _, i := range m.Candidates {
				fs = append(fs, fmt.Sprintf("    %s", paths[i]))
			}
		} else {
			fs = append(fs, i18n.T("err.selectNoMatch", m.Query))
		}
	}
	if len(fs) > 0 {
		fs = append(fs, i18n.T("err.selectHint"))
		return chosen, cli.Badf("%s", strings.Join(fs, "\n"))
	}
	return chosen, nil
}
