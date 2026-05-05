package report

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ch_watch/internal/model"
	"ch_watch/internal/runner"
)

type Reporter interface {
	System(label string, message string)
	Run(path string)
	Result(result model.RunResult)
	Event(path string, op string)
}

type ConsoleReporter struct {
	base   string
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
	mu     sync.Mutex
}

func NewConsoleReporter(base string, stdout io.Writer, stderr io.Writer) (*ConsoleReporter, error) {
	return newConsoleReporter(base, stdout, stderr, time.Now)
}

func newConsoleReporter(base string, stdout io.Writer, stderr io.Writer, now func() time.Time) (*ConsoleReporter, error) {
	if base == "" {
		cwd, err := filepath.Abs(".")
		if err != nil {
			return nil, err
		}
		base = cwd
	}
	if now == nil {
		now = time.Now
	}
	return &ConsoleReporter{base: base, stdout: stdout, stderr: stderr, now: now}, nil
}

func (r *ConsoleReporter) System(label string, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.stdout, "%s\n", colorize(systemHeaderStyle, separator(label)))
	_, _ = fmt.Fprintf(r.stdout, "%s\n", colorize(systemTextStyle, fmt.Sprintf("[%s] %s", r.timestamp(), message)))
}

func (r *ConsoleReporter) Run(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.stdout, "%s\n", r.line(runStyle, "🚀 RUN", r.pathText(r.display(path))))
}

func (r *ConsoleReporter) Result(result model.RunResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := "OK"
	writer := r.stdout
	style := okStyle
	details := fmt.Sprintf("%s (%s)", r.pathText(r.display(result.Path)), result.Duration.Round(time.Millisecond))
	if result.Err != nil {
		status = "FAIL"
		writer = r.stderr
		style = failStyle
		details = fmt.Sprintf("%s (%s, %s)", r.pathText(r.display(result.Path)), runner.DecodeExitCode(result.ExitCode), result.Duration.Round(time.Millisecond))
	}
	_, _ = fmt.Fprintf(writer, "%s\n", r.line(style, emojiFor(status)+" "+status, details))
	if result.Err != nil && result.Stderr != "" {
		_, _ = fmt.Fprintf(r.stderr, "%s\n", r.line(stderrStyle, "⚠️ STDERR", result.Stderr))
	}
	dumpPaths := result.DumpPaths
	if len(dumpPaths) == 0 && result.DumpPath != "" {
		dumpPaths = []string{result.DumpPath}
	}
	for _, path := range dumpPaths {
		_, _ = fmt.Fprintf(r.stdout, "%s\n", r.line(dumpStyle, "💾 DUMP", r.pathText(r.display(path))))
	}
}

func (r *ConsoleReporter) Event(path string, op string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.stdout, "%s\n", r.line(eventStyle, "🔎 EVENT", op+" "+r.pathText(r.display(path))))
}

func (r *ConsoleReporter) display(path string) string {
	rel, err := filepath.Rel(r.base, path)
	if err == nil && rel != "" && rel != "." {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func (r *ConsoleReporter) line(style string, label string, details string) string {
	return fmt.Sprintf("%s %s %s", colorize(timestampStyle, "["+r.timestamp()+"]"), colorize(style, label), details)
}

func (r *ConsoleReporter) pathText(path string) string {
	return colorize(pathStyle, path)
}

func (r *ConsoleReporter) timestamp() string {
	return r.now().Format("15:04:05")
}

func separator(label string) string {
	header := "=== " + label + " "
	if len(header) >= 72 {
		return header
	}
	return header + strings.Repeat("=", 72-len(header))
}

func emojiFor(status string) string {
	if status == "FAIL" {
		return "❌"
	}
	return "✅"
}

func colorize(style string, text string) string {
	return style + text + ansiReset
}

const (
	ansiReset         = "\033[0m"
	timestampStyle    = "\033[90m"
	runStyle          = "\033[1;96m"
	okStyle           = "\033[1;92m"
	failStyle         = "\033[1;91m"
	eventStyle        = "\033[1;93m"
	pathStyle         = "\033[1;32m"
	systemHeaderStyle = "\033[1;97;44m"
	systemTextStyle   = "\033[1;36m"
	dumpStyle         = "\033[1;35m"
	stderrStyle       = "\033[1;33m"
)
