package report

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"ch_watch/internal/model"
)

type Reporter interface {
	Run(path string)
	Result(result model.RunResult)
	Event(path string, op string)
}

type ConsoleReporter struct {
	base   string
	stdout io.Writer
	stderr io.Writer
	mu     sync.Mutex
}

func NewConsoleReporter(base string, stdout io.Writer, stderr io.Writer) (*ConsoleReporter, error) {
	if base == "" {
		cwd, err := filepath.Abs(".")
		if err != nil {
			return nil, err
		}
		base = cwd
	}
	return &ConsoleReporter{base: base, stdout: stdout, stderr: stderr}, nil
}

func (r *ConsoleReporter) Run(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.stdout, "[%s] RUN %s\n", timestamp(), r.display(path))
}

func (r *ConsoleReporter) Result(result model.RunResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := "OK"
	writer := r.stdout
	if result.Err != nil {
		status = "FAIL"
		writer = r.stderr
		_, _ = fmt.Fprintf(writer, "[%s] %s %s (exit %d, %s)\n", timestamp(), status, r.display(result.Path), result.ExitCode, result.Duration.Round(time.Millisecond))
		return
	}
	_, _ = fmt.Fprintf(writer, "[%s] %s %s (%s)\n", timestamp(), status, r.display(result.Path), result.Duration.Round(time.Millisecond))
}

func (r *ConsoleReporter) Event(path string, op string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.stdout, "[%s] EVENT %s %s\n", timestamp(), op, r.display(path))
}

func (r *ConsoleReporter) display(path string) string {
	rel, err := filepath.Rel(r.base, path)
	if err == nil && rel != "" && rel != "." {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
