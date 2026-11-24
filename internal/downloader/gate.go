package downloader

import (
	"context"
	"sync"
	"time"
)

// gate limits how many segment bodies may be in flight and can shrink itself
// when a mirror starts answering 429. A buffered channel cannot change size, so
// this is a small polling semaphore; with a handful of goroutines the 20 ms
// poll costs nothing.
type gate struct {
	mu     sync.Mutex
	limit  int
	active int
}

func newGate(n int) *gate {
	if n < 1 {
		n = 1
	}
	return &gate{limit: n}
}

func (g *gate) acquire(ctx context.Context) bool {
	for {
		g.mu.Lock()
		if g.active < g.limit {
			g.active++
			g.mu.Unlock()
			return true
		}
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return false
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (g *gate) release() {
	g.mu.Lock()
	if g.active > 0 {
		g.active--
	}
	g.mu.Unlock()
}

// halve reduces concurrency after a rate-limit response and reports the new
// limit so the UI can say what happened.
func (g *gate) halve() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.limit > 1 {
		g.limit /= 2
	}
	return g.limit
}

func (g *gate) current() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.limit
}
