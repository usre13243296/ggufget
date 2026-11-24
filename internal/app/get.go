package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ggufget/internal/cli"
	"ggufget/internal/downloader"
	"ggufget/internal/fsutil"
	"ggufget/internal/i18n"
	"ggufget/internal/lmstudio"
	"ggufget/internal/provider"
	"ggufget/internal/termui"
	"ggufget/internal/units"

	"os/signal"
	"syscall"
)

type dlTask struct {
	idx  int
	f    provider.File
	url  string
	dest string
	rel  string
	sha  string
}

func (a *App) cmdGet(ctx context.Context, rest []string) error {
	repo, err := a.repoArg(rest)
	if err != nil {
		return err
	}
	if a.P.Bool("all") && a.P.Has("file") {
		return cli.Badf("%s", i18n.T("err.fileAndAll"))
	}
	p, meta, files, err := a.fetchListing(ctx, repo, a.P.Str("rev"))
	if err != nil {
		return err
	}
	rev := meta.Commit
	if r := a.P.Str("rev"); r != "" {
		rev = r
	}

	var chosen []provider.File
	if a.P.Bool("all") {
		for _, f := range files {
			if f.IsGGUF() {
				chosen = append(chosen, f)
			}
		}
	} else {
		qs := a.P.Strs("file")
		if len(qs) == 0 {
			return cli.Badf("%s", i18n.T("err.needFile"))
		}
		chosen, err = a.resolveSelection(repo, files, qs)
		if err != nil {
			return err
		}
	}
	if len(chosen) == 0 {
		return fmt.Errorf("%w: %s", provider.ErrNotFound, i18n.T("list.empty"))
	}

	modelsDir, err := lmstudio.ResolveDir(a.P.Str("out"), a.Cfg.ModelsDir)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.noModelsDir"), err)
	}

	layoutName := a.P.StrOr("layout", a.Cfg.Layout)
	layout, ok := lmstudio.ParseLayout(layoutName)
	if !ok {
		return cli.Badf("%s", i18n.T("err.badLayout", layoutName))
	}
	if a.P.Bool("no-lmstudio") {
		layout = lmstudio.LayoutFlat
	}
	owner, name := provider.SplitRepo(repo)

	wantSHA := strings.ToLower(strings.TrimSpace(a.P.Str("sha256")))
	if wantSHA != "" && len(chosen) > 1 {
		return cli.Badf("--sha256 只能配合单个文件使用")
	}

	tasks := make([]dlTask, 0, len(chosen))
	seen := make(map[string]string, len(chosen))
	var total int64
	for i, f := range chosen {
		sha := f.SHA256
		if i == 0 && wantSHA != "" {
			sha = wantSHA
		}
		rel := lmstudio.Rel(layout, owner, name, f.Path)
		dest := filepath.Join(modelsDir, filepath.FromSlash(rel))
		if prev, dup := seen[strings.ToLower(dest)]; dup {
			return cli.Badf("%s", i18n.T("err.destCollision", f.Path, prev))
		}
		seen[strings.ToLower(dest)] = f.Path
		tasks = append(tasks, dlTask{
			idx:  i + 1,
			f:    f,
			url:  p.ResolveURL(repo, rev, f.Path),
			dest: dest,
			rel:  rel,
			sha:  sha,
		})
		total += f.Size
	}

	if err := a.checkWritable(modelsDir); err != nil {
		return err
	}
	a.printPlan(repo, p, rev, modelsDir, layout, tasks, total)
	a.logGet(repo, p, rev, modelsDir, layout, tasks)
	if a.P.Bool("dry-run") {
		a.Rep.Printf("%s\n", i18n.T("dl.dryrun"))
		return nil
	}
	if !a.P.Bool("yes") && !a.JSON && termui.IsTTY(os.Stdin) && !a.confirm(i18n.T("plan.confirm")) {
		a.Rep.Printf("%s\n", i18n.T("plan.cancelled"))
		return nil
	}

	return a.runDownloads(ctx, tasks, total)
}

// reportedError marks an error that was already printed next to the file it
// happened on, so run() does not print the same line a second time.
type reportedError struct{ err error }

func (e *reportedError) Error() string { return e.err.Error() }
func (e *reportedError) Unwrap() error { return e.err }

// permError says what to do about an unwritable directory without dragging the
// raw "mkdir …: Access is denied." into a sentence the user cannot act on.
type permError struct {
	dir string
	err error
}

func (e *permError) Error() string { return i18n.T("err.permission", e.dir) }
func (e *permError) Unwrap() error { return e.err }

// checkWritable fails before the plan is printed: a download that cannot create
// its directory is a configuration problem, and discovering it after the plan
// and a progress line makes it look like the tool lost the time.
func (a *App) checkWritable(modelsDir string) error {
	if a.P.Bool("dry-run") {
		return nil
	}
	if err := fsutil.ProbeWritable(modelsDir); err != nil {
		a.Log.Error("models dir not writable dir="+modelsDir, err)
		return &permError{dir: modelsDir, err: err}
	}
	a.Log.Infof("write test ok dir=%q", modelsDir)
	return nil
}

// logGet records where every file is meant to land. When a download fails on
// somebody else's machine, the first question is "where did it try to write",
// and the plan has scrolled out of the console by then.
func (a *App) logGet(repo string, p provider.Provider, rev, modelsDir string, layout lmstudio.Layout, tasks []dlTask) {
	a.Log.Infof("get repo=%q provider=%s rev=%s files=%d layout=%s out_flag=%q config_models_dir=%q models_dir=%q",
		repo, p.Name(), short(rev), len(tasks), string(layout), a.P.Str("out"), a.Cfg.ModelsDir, modelsDir)
	for _, t := range tasks {
		a.Log.Infof("target %d/%d file=%q size=%d sha=%q dest=%q rel=%q",
			t.idx, len(tasks), t.f.Path, t.f.Size, t.sha, t.dest, t.rel)
	}
}

func (a *App) printPlan(repo string, p provider.Provider, rev, modelsDir string, layout lmstudio.Layout, tasks []dlTask, total int64) {
	if a.JSON {
		items := make([]map[string]any, 0, len(tasks))
		for _, t := range tasks {
			items = append(items, map[string]any{
				"file": t.f.Path, "size": t.f.Size, "dest": t.dest, "rel": t.rel,
			})
		}
		a.emitJSON(map[string]any{
			"event": "plan", "repo": repo, "provider": p.Name(), "revision": rev,
			"dest": modelsDir, "layout": string(layout), "files": items, "total": total,
		})
		return
	}
	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T("plan.title")))
	fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(i18n.T("plan.repo")), repo)
	fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(i18n.T("plan.provider")), p.Name())
	if rev != "" {
		fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(i18n.T("plan.rev")), short(rev))
	}
	fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(i18n.T("plan.dest")), modelsDir)
	fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(i18n.T("plan.layout")), layoutHint(layout))
	fmt.Fprintf(a.Out, "  %s  %d 个，%s\n", a.St.Dim(i18n.T("plan.files")), len(tasks), units.Bytes(total))
	for _, t := range tasks {
		fmt.Fprintf(a.Out, "  %d. %s  %s\n", t.idx, t.f.Path, a.St.Dim(units.Bytes(t.f.Size)))
		if t.rel != t.f.Path {
			fmt.Fprintf(a.Out, "     %s\n", a.St.Dim("-> "+t.rel))
		}
	}
	fmt.Fprintf(a.Out, "\n%s\n\n", a.St.Dim(i18n.T("plan.noSpeed")))
}

// layoutHint spells out the directory structure the chosen layout creates.
func layoutHint(l lmstudio.Layout) string {
	switch l {
	case lmstudio.LayoutRepo:
		return "repo: <发布者>/<仓库>/<文件>.gguf"
	case lmstudio.LayoutFlat:
		return "flat: <文件>.gguf"
	default:
		return "lmstudio: <文件名>/" + lmstudio.VendorDir + "/<文件名>.gguf"
	}
}

func short(rev string) string {
	if len(rev) > 12 && !strings.ContainsAny(rev, "/.") {
		return rev[:12]
	}
	return rev
}

func (a *App) confirm(prompt string) bool {
	fmt.Fprint(a.Out, prompt)
	line, err := a.readLine()
	if err != nil {
		return true
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "" || line == "y" || line == "yes"
}

func (a *App) runDownloads(ctx context.Context, tasks []dlTask, total int64) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	verify, ok := downloader.ParseVerify(a.P.StrOr("verify", a.Cfg.Verify))
	if !ok {
		return cli.Badf("--verify %s", a.P.Str("verify"))
	}
	conns := a.P.IntOr("connections", a.Cfg.Connections)
	retries := a.P.IntOr("retries", a.Cfg.Retries)
	jobs := a.P.IntOr("jobs", a.Cfg.Jobs)
	if jobs < 1 {
		jobs = 1
	}
	if jobs > len(tasks) {
		jobs = len(tasks)
	}

	start := time.Now()
	var mu sync.Mutex
	var okBytes int64
	var okCount, failCount int
	var firstErr error

	var wg sync.WaitGroup
	sem := make(chan struct{}, jobs)
	for _, t := range tasks {
		sem <- struct{}{}
		wg.Add(1)
		go func(t dlTask) {
			defer wg.Done()
			defer func() { <-sem }()

			res, err := a.downloadOne(ctx, t, len(tasks), verify, conns, retries, jobs)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failCount++
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			if res != nil && !res.Skipped {
				okCount++
				okBytes += res.Bytes
			}
		}(t)
	}
	wg.Wait()

	if failCount > 0 {
		// One file out of one: the per-file line above already said everything,
		// and a "0 succeeded, 1 failed" tail plus a repeated error is noise.
		if len(tasks) > 1 || failCount > 1 {
			a.Rep.Printf("%s\n", i18n.T("err.partial", okCount, failCount))
		}
		return &reportedError{err: firstErr}
	}
	a.Rep.Printf("%s\n", a.St.OK(i18n.T("dl.summary", okCount, units.Bytes(okBytes), units.Duration(time.Since(start)))))
	a.hintLMStudio(ctx, tasks)
	return nil
}

func (a *App) downloadOne(ctx context.Context, t dlTask, count int, verify downloader.VerifyMode, conns, retries, jobs int) (*downloader.Result, error) {
	// A live progress bar only makes sense for one file at a time.
	var rep *termui.Reporter
	if jobs == 1 {
		rep = a.Rep
		rep.BeginFile(t.idx, count, t.f.Path, t.f.Size, "")
	}

	opts := &downloader.Options{
		Connections: conns,
		Retries:     retries,
		Verify:      verify,
		Overwrite:   a.P.Bool("overwrite"),
		Client:      a.Client,
		Progress:    rep,
		Verbose:     a.Verbose,
		Log:         a.Log,
		OnResume: func(have int64) {
			if rep != nil {
				rep.SetNote(i18n.T("dl.resume", units.Bytes(have)))
			}
			a.emitJSON(map[string]any{"event": "resume", "file": t.f.Path, "have": have})
		},
		OnRetry: func(n int, wait time.Duration, err error) {
			if rep != nil {
				rep.SetNote(i18n.T("dl.retry", n, units.Duration(wait), err))
			}
			a.emitJSON(map[string]any{"event": "retry", "file": t.f.Path, "attempt": n, "error": err.Error()})
		},
		OnRateLimit: func(c int) {
			if rep != nil {
				rep.SetNote(i18n.T("dl.ratelimit", c))
			}
			a.emitJSON(map[string]any{"event": "ratelimit", "file": t.f.Path, "connections": c})
		},
		OnVerify: func() {
			if rep != nil {
				rep.SetNote(i18n.T("dl.verify"))
			}
			a.emitJSON(map[string]any{"event": "verify", "file": t.f.Path})
		},
	}

	res, err := downloader.Run(ctx, opts, downloader.Task{
		URL:      t.url,
		Dest:     t.dest,
		Expected: t.f.Size,
		SHA256:   t.sha,
		Display:  t.f.Path,
	})

	if rep != nil {
		rep.EndFile(t.f.Path, t.f.Size, err)
	}
	if err != nil {
		a.emitJSON(map[string]any{"event": "error", "file": t.f.Path, "error": err.Error()})
		if jobs == 1 {
			a.Rep.Errorf("%s\n", a.St.Bad(err.Error()))
		} else {
			a.Rep.Errorf("  %s %s\n", a.St.Bad("x"), err.Error())
		}
		return nil, err
	}
	if res.Skipped {
		a.Rep.Printf("%s\n", a.St.Dim(i18n.T("dl.skip", res.Path)))
	} else {
		a.Rep.Printf("%s %s  %s\n", a.St.OK("v"), t.f.Path,
			a.St.Dim(fmt.Sprintf("%s / %s", units.Bytes(res.Bytes), units.Duration(res.Elapsed))))
	}
	a.Rep.Finish(res.Path)
	return res, nil
}

// hintLMStudio checks the lms index once and tells the user where the model is.
func (a *App) hintLMStudio(ctx context.Context, tasks []dlTask) {
	if a.P.Bool("no-lmstudio") || len(tasks) == 0 {
		return
	}
	if _, ok := lmstudio.CLI(); !ok {
		a.Rep.Printf("%s\n", a.St.Dim(i18n.T("lm.notRunning")))
		a.Rep.Printf("%s\n", a.St.Dim(i18n.T("lm.hint")))
		return
	}
	if vis, err := lmstudio.Visible(ctx, tasks[0].rel); err == nil && vis {
		a.Rep.Printf("%s\n", a.St.Dim(i18n.T("lm.visible")))
		return
	}
	a.Rep.Printf("%s\n", a.St.Dim(i18n.T("lm.hint")))
}
