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
	)
	return replacer.Replace(input)
}
