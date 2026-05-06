package testutil

import (
	"sync"

	"github.com/webmalex/ch_watch/internal/model"
)

type MemoryReporter struct {
	mu      sync.Mutex
	runs    []string
	results []model.RunResult
}

func NewMemoryReporter() *MemoryReporter {
	return &MemoryReporter{}
}

func (r *MemoryReporter) System(string, string) {}

func (r *MemoryReporter) Run(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, path)
}

func (r *MemoryReporter) Result(result model.RunResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, result)
}

func (r *MemoryReporter) Event(string, string) {}

func (r *MemoryReporter) Runs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.runs...)
}
