package testutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/webmalex/ch_watch/internal/model"
)

type FakeRunner struct {
	mu      sync.Mutex
	paths   []string
	results []model.RunResult
	started chan struct{}
	run     func(model.RunRequest) model.RunResult
}

func NewFakeRunner(run func(model.RunRequest) model.RunResult) *FakeRunner {
	return &FakeRunner{
		started: make(chan struct{}, 32),
		run:     run,
	}
}

func (r *FakeRunner) Run(_ context.Context, request model.RunRequest) model.RunResult {
	r.mu.Lock()
	r.paths = append(r.paths, request.Path)
	r.mu.Unlock()
	r.started <- struct{}{}

	if r.run != nil {
		result := r.run(request)
		r.mu.Lock()
		r.results = append(r.results, result)
		r.mu.Unlock()
		return result
	}
	result := model.RunResult{Path: request.Path}
	r.mu.Lock()
	r.results = append(r.results, result)
	r.mu.Unlock()
	return result
}

func (r *FakeRunner) Paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

func (r *FakeRunner) WaitForStarts(t *testing.T, count int, timeout time.Duration) {
	t.Helper()
	for i := 0; i < count; i++ {
		select {
		case <-r.started:
		case <-time.After(timeout):
			t.Fatalf("timed out waiting for run %d", i+1)
		}
	}
}
