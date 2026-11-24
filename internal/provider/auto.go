package provider

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Auto probes every configured mirror at once and returns the first one, in
// priority order, that actually carries the repository.
//
// Priority encodes measured reality: ModelScope is roughly ten times faster
// than hf-mirror from mainland China, which in turn is far faster than
// huggingface.co. Probing concurrently keeps the cost to about one round trip.
type Auto struct {
	providers []Provider
	timeout   time.Duration
}

// NewAuto builds the selector. The order of ps is the priority order.
func NewAuto(timeout time.Duration, ps ...Provider) *Auto {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	return &Auto{providers: ps, timeout: timeout}
}

// Providers exposes the priority list, e.g. for doctor.
func (a *Auto) Providers() []Provider { return a.providers }

// MirrorFail records why one mirror could not serve a repository.
type MirrorFail struct {
	Name string
	Err  error
}

// MirrorsError reports that every mirror failed, keeping each reason instead of
// hiding all but the first one. A tool that silently reports one broken mirror
// when three are broken sends people debugging the wrong thing.
type MirrorsError struct {
	Repo  string
	Fails []MirrorFail
}

func (e *MirrorsError) Error() string {
	parts := make([]string, 0, len(e.Fails))
	for _, f := range e.Fails {
		parts = append(parts, f.Name+": "+f.Err.Error())
	}
	return strings.Join(parts, "; ")
}

// Is reports ErrNotFound only when every mirror agreed the repo is missing.
// One 404 next to two timeouts is not proof of anything.
func (e *MirrorsError) Is(target error) bool {
	if target == ErrNotFound {
		if len(e.Fails) == 0 {
			return false
		}
		for _, f := range e.Fails {
			if !errors.Is(f.Err, ErrNotFound) {
				return false
			}
		}
		return true
	}
	for _, f := range e.Fails {
		if errors.Is(f.Err, target) {
			return true
		}
	}
	return false
}

// As lets errors.As dig a net.Error or an HTTPError out of the failure list so
// the caller can still pick the right exit code.
func (e *MirrorsError) As(target any) bool {
	for _, f := range e.Fails {
		if errors.As(f.Err, target) {
			return true
		}
	}
	return false
}

type probeResult struct {
	p    Provider
	meta *Meta
	err  error
}

// Pick returns the first provider that has repo, along with its metadata.
func (a *Auto) Pick(ctx context.Context, repo string) (Provider, *Meta, error) {
	if len(a.providers) == 0 {
		return nil, nil, errors.New("no mirrors configured")
	}
	results := make([]probeResult, len(a.providers))
	ch := make(chan struct {
		idx int
		res probeResult
	}, len(a.providers))

	for i, p := range a.providers {
		go func(i int, p Provider) {
			c, cancel := context.WithTimeout(ctx, a.timeout)
			defer cancel()
			m, err := p.Probe(c, repo)
			ch <- struct {
				idx int
				res probeResult
			}{i, probeResult{p: p, meta: m, err: err}}
		}(i, p)
	}
	for range a.providers {
		r := <-ch
		results[r.idx] = r.res
	}

	fails := make([]MirrorFail, 0, len(a.providers))
	for i, p := range a.providers {
		r := results[i]
		if r.err == nil {
			return p, r.meta, nil
		}
		fails = append(fails, MirrorFail{Name: p.Name(), Err: r.err})
	}
	return nil, nil, &MirrorsError{Repo: repo, Fails: fails}
}

// List picks a mirror and lists the files, returning which mirror was used.
func (a *Auto) List(ctx context.Context, repo, rev string) (Provider, *Meta, []File, error) {
	p, _, err := a.Pick(ctx, repo)
	if err != nil {
		return nil, nil, nil, err
	}
	meta, files, err := p.List(ctx, repo, rev)
	return p, meta, files, err
}
