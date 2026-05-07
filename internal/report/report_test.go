package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/webmalex/ch_watch/internal/model"
)

func TestConsoleReporterUsesColorAndEmojiForRunLifecycle(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 30, 11, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Run("/repo/demo/ch/dev/tmp.sql")
	reporter.Result(model.RunResult{Path: "/repo/demo/ch/dev/tmp.sql", Duration: 138 * time.Millisecond})

	output := stdout.String()
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected ansi colors in output: %q", output)
	}
	plain := stripANSI(output)
	if !strings.Contains(plain, "🚀 RUN demo/ch/dev/tmp.sql") {
		t.Fatalf("expected colored run label: %q", output)
	}
	if !strings.Contains(output, "\x1b[1;32mdemo/ch/dev/tmp.sql\x1b[0m") {
		t.Fatalf("expected green file path in output: %q", output)
	}
	if !strings.Contains(plain, "✅ OK demo/ch/dev/tmp.sql (138ms)") {
		t.Fatalf("expected colored success label: %q", output)
	}
	if !strings.Contains(plain, "[12:30:11]") {
		t.Fatalf("expected fixed timestamp: %q", output)
	}
}

func TestConsoleReporterWritesSystemSeparator(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 30, 11, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.System("👀 WATCH", "root=demo/ch | mode=dry-run")

	output := stdout.String()
	if !strings.Contains(output, "=== 👀 WATCH") {
		t.Fatalf("expected separator header: %q", output)
	}
	if !strings.Contains(output, "root=demo/ch | mode=dry-run") {
		t.Fatalf("expected system details: %q", output)
	}
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected ansi colors in system output: %q", output)
	}
}

func TestConsoleReporterWritesFailuresToStderr(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &stderr, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 30, 12, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Result(model.RunResult{Path: "/repo/demo/ch/dev/tmp.sql", Duration: 842 * time.Millisecond, ExitCode: 62, Err: errors.New("boom")})

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output on failure: %q", stdout.String())
	}
	output := stderr.String()
	if !strings.Contains(stripANSI(output), "❌ FAIL demo/ch/dev/tmp.sql (exit 62, 842ms)") {
		t.Fatalf("expected colored fail line: %q", output)
	}
}

func stripANSI(input string) string {
	replacer := strings.NewReplacer(
		"\x1b[0m", "",
		"\x1b[90m", "",
		"\x1b[1;96m", "",
		"\x1b[1;92m", "",
		"\x1b[1;91m", "",
		"\x1b[1;32m", "",
		"\x1b[1;97;44m", "",
		"\x1b[1;36m", "",
		"\x1b[1;35m", "",
		"\x1b[1;33m", "",
		"\x1b[1;93m", "",
	)
	return replacer.Replace(input)
}

func TestNewConsoleReporterWithNonEmptyBase(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	r, err := NewConsoleReporter("/repo", &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil reporter")
	}
}

func TestNewConsoleReporterWithEmptyBase(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	r, err := NewConsoleReporter("", &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil reporter")
	}
	if r.base == "" {
		t.Fatal("expected base to be resolved to cwd")
	}
}

func TestNewConsoleReporterNilNowDefaultsToTimeNow(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	r, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.now == nil {
		t.Fatal("expected now to be defaulted to time.Now")
	}
	ts := r.timestamp()
	if ts == "" {
		t.Fatal("expected non-empty timestamp from default now func")
	}
}

func TestEventOutputsOperationAndPath(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 14, 5, 30, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Event("/repo/demo/ch/dev/tmp.sql", "CREATE")

	output := stripANSI(stdout.String())
	if !strings.Contains(output, "🔎 EVENT") {
		t.Fatalf("expected event label: %q", output)
	}
	if !strings.Contains(output, "CREATE") {
		t.Fatalf("expected operation in output: %q", output)
	}
	if !strings.Contains(output, "demo/ch/dev/tmp.sql") {
		t.Fatalf("expected relative path in output: %q", output)
	}
	if !strings.Contains(output, "[14:05:30]") {
		t.Fatalf("expected timestamp in output: %q", output)
	}
}

func TestResultWithErrorAndStderr(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &stderr, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Result(model.RunResult{
		Path:     "/repo/demo/ch/dev/tmp.sql",
		Duration: 100 * time.Millisecond,
		ExitCode: 1,
		Err:      errors.New("syntax error"),
		Stderr:   "Code: 62. DB::Exception: syntax error",
	})

	stderrPlain := stripANSI(stderr.String())
	if !strings.Contains(stderrPlain, "⚠️ STDERR") {
		t.Fatalf("expected stderr label in output: %q", stderrPlain)
	}
	if !strings.Contains(stderrPlain, "Code: 62. DB::Exception: syntax error") {
		t.Fatalf("expected stderr content in output: %q", stderrPlain)
	}
	if !strings.Contains(stderrPlain, "❌ FAIL") {
		t.Fatalf("expected fail label in output: %q", stderrPlain)
	}
}

func TestResultWithLegacyDumpPath(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Result(model.RunResult{
		Path:     "/repo/demo/ch/dev/tmp.sql",
		Duration: 200 * time.Millisecond,
		DumpPath: "/repo/demo/ch/dev/tmp.tsv",
	})

	output := stripANSI(stdout.String())
	if !strings.Contains(output, "💾 DUMP") {
		t.Fatalf("expected dump label in output: %q", output)
	}
	if !strings.Contains(output, "demo/ch/dev/tmp.tsv") {
		t.Fatalf("expected dump path in output: %q", output)
	}
}

func TestResultWithMultipleDumpPaths(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Result(model.RunResult{
		Path:      "/repo/demo/ch/dev/tmp.sql",
		Duration:  200 * time.Millisecond,
		DumpPaths: []string{"/repo/demo/ch/dev/tmp.tsv", "/repo/demo/ch/dev/tmp.txt", "/repo/demo/ch/dev/tmp.md"},
	})

	output := stripANSI(stdout.String())
	count := strings.Count(output, "💾 DUMP")
	if count != 3 {
		t.Fatalf("expected 3 dump lines, got %d: %q", count, output)
	}
	if !strings.Contains(output, "demo/ch/dev/tmp.tsv") {
		t.Fatalf("expected tsv path: %q", output)
	}
	if !strings.Contains(output, "demo/ch/dev/tmp.txt") {
		t.Fatalf("expected txt path: %q", output)
	}
	if !strings.Contains(output, "demo/ch/dev/tmp.md") {
		t.Fatalf("expected md path: %q", output)
	}
}

func TestResultDumpPathsTakesPrecedenceOverDumpPath(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Result(model.RunResult{
		Path:      "/repo/demo/ch/dev/tmp.sql",
		Duration:  200 * time.Millisecond,
		DumpPath:  "/repo/demo/ch/dev/legacy.tsv",
		DumpPaths: []string{"/repo/demo/ch/dev/multi.tsv"},
	})

	output := stripANSI(stdout.String())
	if strings.Contains(output, "legacy.tsv") {
		t.Fatalf("legacy DumpPath should not appear when DumpPaths is set: %q", output)
	}
	if !strings.Contains(output, "multi.tsv") {
		t.Fatalf("expected DumpPaths entry: %q", output)
	}
	count := strings.Count(output, "💾 DUMP")
	if count != 1 {
		t.Fatalf("expected exactly 1 dump line, got %d: %q", count, output)
	}
}

func TestDisplayNonRelativePathReturnsAbsolutePath(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	reporter, err := newConsoleReporter("/repo", &stdout, &bytes.Buffer{}, func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("new reporter: %v", err)
	}

	reporter.Run("/completely/different/path/query.sql")

	output := stripANSI(stdout.String())
	if !strings.Contains(output, "/completely/different/path/query.sql") {
		t.Fatalf("expected absolute path in output: %q", output)
	}
}

func TestSeparatorWithLongLabel(t *testing.T) {
	t.Parallel()

	longLabel := strings.Repeat("X", 70)
	result := separator(longLabel)

	header := "=== " + longLabel + " "
	if result != header {
		t.Fatalf("expected header without padding for long label, got: %q", result)
	}
}

func TestSeparatorWithShortLabel(t *testing.T) {
	t.Parallel()

	result := separator("HI")
	if len(result) != 72 {
		t.Fatalf("expected 72-char separator, got %d: %q", len(result), result)
	}
	if !strings.HasPrefix(result, "=== HI ") {
		t.Fatalf("expected prefix: %q", result)
	}
	if !strings.HasSuffix(result, "=") {
		t.Fatalf("expected trailing '=': %q", result)
	}
}

func TestEmojiForFail(t *testing.T) {
	t.Parallel()

	if emojiFor("FAIL") != "❌" {
		t.Fatalf(`expected "❌" for FAIL`)
	}
	if emojiFor("OK") != "✅" {
		t.Fatalf(`expected "✅" for OK`)
	}
	if emojiFor("anything") != "✅" {
		t.Fatalf(`expected "✅" for non-FAIL status`)
	}
}
