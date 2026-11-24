package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"ggufget/internal/config"
	"ggufget/internal/fsutil"
	"ggufget/internal/i18n"
	"ggufget/internal/lmstudio"
	"ggufget/internal/logx"
	"ggufget/internal/netx"
	"ggufget/internal/provider"
	"ggufget/internal/units"
)

// The speed test downloads the first 8 MiB of the smallest gguf of a repo that
// exists on every mirror we support.
const (
	probeRepo  = "lmstudio-community/Qwen2.5-7B-Instruct-GGUF"
	probeBytes = 4 << 20
)

type speedRes struct {
	name string
	mbps float64
	err  error
}

func (a *App) cmdDoctor(ctx context.Context, rest []string) error {
	fmt.Fprintf(a.Out, "%s\n", a.St.Bold(i18n.T("doc.title")))
	row := func(k, v string) {
		fmt.Fprintf(a.Out, "  %s  %s\n", a.St.Dim(pad(k, 10)), v)
	}
	row(i18n.T("doc.os"), fmt.Sprintf("%s/%s  %s", runtime.GOOS, runtime.GOARCH, runtime.Version()))
	row(i18n.T("doc.go"), netx.Version)

	modelsDir, mErr := lmstudio.ResolveDir(a.P.Str("out"), a.Cfg.ModelsDir)
	if mErr != nil {
		row(i18n.T("doc.modelsdir"), a.St.Bad(i18n.T("err.noModelsDir")))
	} else {
		v := modelsDir
		if perr := fsutil.ProbeWritable(modelsDir); perr != nil {
			v += "  " + a.St.Bad(i18n.T("doc.logBad"))
			a.Log.Warnf("models dir not writable dir=%q errno=%s chain=%s",
				modelsDir, logx.Errno(perr), logx.Chain(perr))
		} else {
			v += "  " + a.St.Dim(i18n.T("doc.logOK"))
		}
		row(i18n.T("doc.modelsdir"), v)
		if free, err := fsutil.DiskFree(fsutil.NearestExistingDir(modelsDir)); err == nil {
			row(i18n.T("doc.disk"), units.Bytes(int64(free)))
		} else {
			row(i18n.T("doc.disk"), a.St.Bad(err.Error()))
		}
	}
	if lms, ok := lmstudio.CLI(); ok {
		row(i18n.T("doc.lms"), lms)
	} else {
		row(i18n.T("doc.lms"), a.St.Dim(i18n.T("doc.bad")))
	}
	if proxy := a.P.StrOr("proxy", a.Cfg.Proxy); proxy != "" {
		row(i18n.T("doc.proxy"), proxy)
	} else if env := proxyFromEnv(); env != "" {
		row(i18n.T("doc.proxy"), env)
	} else {
		row(i18n.T("doc.proxy"), a.St.Dim(i18n.T("doc.proxyNone")))
	}
	cfgRow := a.CfgPath
	if cfgRow == "" {
		cfgRow = config.Path()
	}
	if err := fsutil.ProbeWritable(filepath.Dir(cfgRow)); err != nil {
		cfgRow += "  " + a.St.Bad(i18n.T("doc.logBad"))
	} else {
		cfgRow += "  " + a.St.Dim(i18n.T("doc.logOK"))
	}
	row(i18n.T("doc.configPath"), cfgRow)
	if a.Log != nil && a.Log.Enabled() {
		row(i18n.T("doc.logPath"), a.Log.Path()+"  "+a.St.Dim(i18n.T("doc.logOK")))
	} else if a.P.Bool("no-log") {
		row(i18n.T("doc.logPath"), a.St.Dim(i18n.T("doc.logOff")))
	} else {
		row(i18n.T("doc.logPath"), a.St.Bad(i18n.T("doc.logBad")))
	}

	if a.P.Bool("dry-run") {
		return nil
	}

	fmt.Fprintf(a.Out, "\n  %s\n", i18n.T("doc.test"))
	results := a.speedTest(ctx)
	if len(results) == 0 {
		return nil
	}
	res := make([]speedRes, len(results))
	copy(res, results)
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].err != nil {
			return false
		}
		if res[j].err != nil {
			return true
		}
		return res[i].mbps > res[j].mbps
	})
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(a.Out, "    %s\n", i18n.T("doc.speedFail", r.name, r.err))
			continue
		}
		fmt.Fprintf(a.Out, "    %s\n", i18n.T("doc.speed", r.name, r.mbps))
	}
	best := res[0]
	if best.err != nil {
		fmt.Fprintf(a.Out, "\n  %s\n", a.St.Yellow(i18n.T("doc.noMirror")))
		return nil
	}
	fmt.Fprintf(a.Out, "\n  %s\n", a.St.OK(i18n.T("doc.recommend", best.name, best.mbps)))
	return nil
}

func proxyFromEnv() string {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// speedTest measures each mirror one after another. Running them in parallel
// would make the three numbers fight over the same link and rank nothing.
func (a *App) speedTest(ctx context.Context) []speedRes {
	provs := a.buildProviders("auto")
	res := make([]speedRes, 0, len(provs))
	for _, p := range provs {
		mbps, err := a.measure(ctx, p)
		res = append(res, speedRes{name: p.Name(), mbps: mbps, err: err})
	}
	return res
}

func (a *App) measure(parent context.Context, p provider.Provider) (float64, error) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()

	meta, files, err := p.List(ctx, probeRepo, "")
	if err != nil {
		return 0, err
	}
	var pick provider.File
	found := false
	for _, f := range files {
		if f.IsGGUF() && (!found || f.Size < pick.Size) {
			pick, found = f, true
		}
	}
	if !found {
		return 0, errors.New("no gguf found")
	}
	url := p.ResolveURL(probeRepo, meta.Commit, pick.Path)
	req, err := netx.NewRequest(url, map[string]string{
		"Range": fmt.Sprintf("bytes=0-%d", probeBytes-1),
	})
	if err != nil {
		return 0, err
	}
	req = req.WithContext(ctx)

	start := time.Now()
	resp, err := a.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return 0, provider.MapStatus(resp.StatusCode, url, "")
	}
	n, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, probeBytes))
	elapsed := time.Since(start).Seconds()
	if n <= 0 || elapsed <= 0 {
		if copyErr != nil {
			return 0, copyErr
		}
		return 0, errors.New("no data")
	}
	// A slow mirror is cut off by the deadline; the bytes seen so far still
	// describe its speed, which is exactly what the user needs to know.
	return float64(n) / elapsed / (1 << 20), nil
}
