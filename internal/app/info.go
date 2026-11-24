package app

import (
	"context"
	"fmt"
	"strings"

	"ggufget/internal/i18n"
	"ggufget/internal/provider"
	"ggufget/internal/units"
)

func (a *App) cmdInfo(ctx context.Context, rest []string) error {
	repo, err := a.repoArg(rest)
	if err != nil {
		return err
	}
	p, meta, files, err := a.fetchListing(ctx, repo, a.P.Str("rev"))
	if err != nil {
		return err
	}

	var ggufs []provider.File
	var total int64
	for _, f := range files {
		if f.IsGGUF() {
			ggufs = append(ggufs, f)
			total += f.Size
		}
	}
	var small, large provider.File
	if len(ggufs) > 0 {
		small, large = ggufs[0], ggufs[0]
		for _, f := range ggufs {
			if f.Size < small.Size {
				small = f
			}
			if f.Size > large.Size {
				large = f
			}
		}
	}
	quants := quantList(ggufs)

	if a.JSON {
		a.emitJSON(map[string]any{
			"event":    "info",
			"repo":     repo,
			"provider": p.Name(),
			"commit":   meta.Commit,
			"license":  meta.License,
			"gated":    meta.Gated,
			"private":  meta.Private,
			"gguf":     len(ggufs),
			"total":    total,
			"quants":   quants,
		})
		return nil
	}

	row := func(k, v string) {
		fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(pad(k, 8)), v)
	}
	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T("info.title")))
	row(i18n.T("plan.repo"), repo)
	row(i18n.T("plan.provider"), p.Name())
	if meta.Commit != "" {
		row("commit", short(meta.Commit))
	}
	if meta.License != "" {
		row(i18n.T("info.license"), meta.License)
	}
	if meta.Private {
		row("", a.St.Yellow(i18n.T("info.private")))
	}
	if meta.Gated {
		row("", a.St.Yellow(i18n.T("info.gated")))
		row("", i18n.T("info.tokenHint"))
	}
	if len(quants) > 0 {
		row(i18n.T("info.quant"), strings.Join(quants, ", "))
	}
	fmt.Fprintf(a.Out, "\n  %s\n", i18n.T("info.gguf", len(ggufs), units.Bytes(total)))
	if len(ggufs) > 0 {
		fmt.Fprintf(a.Out, "  %s  %s  %s\n", a.St.Dim(i18n.T("info.smallest")), small.Path, units.Bytes(small.Size))
		fmt.Fprintf(a.Out, "  %s  %s  %s\n", a.St.Dim(i18n.T("info.largest")), large.Path, units.Bytes(large.Size))
	}
	return nil
}

func quantList(files []provider.File) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		q := units.Quant(f.Path)
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		out = append(out, q)
	}
	return out
}
