package app

import (
	"context"
	"fmt"
	"strings"

	"ggufget/internal/cli"
	"ggufget/internal/i18n"
	"ggufget/internal/provider"
)

func (a *App) cmdSearch(ctx context.Context, rest []string) error {
	q := strings.TrimSpace(strings.Join(rest, " "))
	if q == "" {
		return cli.Badf("search <keyword>")
	}
	if err := a.checkProviderFlag(); err != nil {
		return err
	}
	limit := a.P.IntOr("limit", 10)
	base := a.hfBase()
	hits, err := provider.Search(ctx, a.Client, base, q, limit)
	if err != nil && base != "https://huggingface.co" {
		// hf-mirror occasionally answers 5xx on its API; ask upstream instead.
		if alt, altErr := provider.Search(ctx, a.Client, "https://huggingface.co", q, limit); altErr == nil {
			hits, err = alt, nil
		}
	}
	if err != nil {
		fmt.Fprintf(a.ErrOut, "%s\n", a.St.Dim(i18n.T("search.failHint")))
		return err
	}
	if a.JSON {
		for _, h := range hits {
			a.emitJSON(map[string]any{"event": "repo", "repo": h.ID, "downloads": h.Downloads})
		}
		return nil
	}
	if len(hits) == 0 {
		fmt.Fprintf(a.Out, "%s\n", i18n.T("search.none", q))
		fmt.Fprintf(a.Out, "%s\n", a.St.Dim(i18n.T("search.note")))
		return nil
	}
	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T("search.title", q, len(hits))))
	for i, h := range hits {
		fmt.Fprintf(a.Out, "  %s  %s  %s\n",
			a.St.Dim(padW(fmt.Sprintf("%d", i+1), 3)),
			padW(clip(h.ID, 56), 56),
			a.St.Dim(humanCount(h.Downloads)))
	}
	fmt.Fprintf(a.Out, "\n%s\n", a.St.Dim(i18n.T("search.note")))
	fmt.Fprintf(a.Out, "%s\n", a.St.Dim(i18n.T("list.pickHint", hits[0].ID)))
	return nil
}

// hfBase picks the Hugging Face compatible endpoint for search.
func (a *App) hfBase() string {
	switch strings.ToLower(strings.TrimSpace(a.providerKind())) {
	case "hf", "huggingface":
		return "https://huggingface.co"
	}
	return a.Cfg.HFEndpoint
}

func humanCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// clip shortens an ASCII repo id from the left, which is what `search` needs.
func clip(s string, w int) string {
	if len(s) <= w {
		return s
	}
	if w <= 1 {
		return s[:w]
	}
	return s[:w-1] + "…"
}
