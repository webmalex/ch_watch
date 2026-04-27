package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunCommandWiresDryRun(t *testing.T) {
	t.Parallel()

	path := writeSQLFile(t, "query.sql")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := Run(context.Background(), []string{"run", "--dry-run", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "RUNNER") {
		t.Fatalf("expected runner banner in stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "RUN") || !strings.Contains(stdout.String(), "OK") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunCommandRequiresPath(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), []string{"run"}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestRunCommandAllowsFlagsAfterPath(t *testing.T) {
	t.Parallel()

	path := writeSQLFile(t, "query.sql")
	var stdout bytes.Buffer

	if err := Run(context.Background(), []string{"run", path, "--dry-run"}, &stdout, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "RUNNER") {
		t.Fatalf("expected runner banner in stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "RUN") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestParseRunDefaultsUseClickHouseBinary(t *testing.T) {
	t.Parallel()

	path := writeSQLFile(t, "query.sql")
	cfg, err := parseRun([]string{path})
	if err != nil {
		t.Fatalf("parse run: %v", err)
	}

	if cfg.Client != "clickhouse" {
		t.Fatalf("unexpected client default: %q", cfg.Client)
	}
	if cfg.Format != "PrettyCompact" {
		t.Fatalf("unexpected format default: %q", cfg.Format)
	}
}

func TestWatchCommandWiresDryRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	stdout := &lockedBuffer{}
	stderr := &lockedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, []string{"watch", "--root", root, "--dry-run"}, stdout, stderr)
	}()
	time.Sleep(100 * time.Millisecond)

	path := filepath.Join(root, "query.sql")
	if err := os.WriteFile(path, []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for !strings.Contains(stdout.String(), "RUN") {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for watch output: %q %q", stdout.String(), stderr.String())
		case <-time.After(20 * time.Millisecond):
		}
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("watch: %v", err)
	}
	if !strings.Contains(stdout.String(), "WATCH") || !strings.Contains(stdout.String(), "STOP") {
		t.Fatalf("expected watch lifecycle banners in stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func TestHelpShowsCommands(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"--help"}, io.Discard, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"ch_watch", "Commands:", "watch", "run", "version"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q: %q", want, output)
		}
	}
}

func TestWatchHelpShowsFlags(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"watch", "--help"}, io.Discard, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"Usage:", "-root", "-db", "-format", "-dump", "-dry-run", "-debounce", "-suppress"} {
		if !strings.Contains(output, want) {
			t.Fatalf("watch help missing %q: %q", want, output)
		}
	}
}

func TestRunHelpShowsPath(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"run", "--help"}, io.Discard, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "<path>") {
		t.Fatalf("run help missing <path>: %q", output)
	}
	if !strings.Contains(output, "-db") {
		t.Fatalf("run help missing --db: %q", output)
	}
}

func TestUnknownCommandShowsHelp(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"foo"}, io.Discard, &buf)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(buf.String(), "Commands:") {
		t.Fatalf("unknown command should show help: %q", buf.String())
	}
}

func TestWatchHelpAlias(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := Run(context.Background(), []string{"watch", "-h"}, io.Discard, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Fatalf("-h should show help: %q", buf.String())
	}
}

func writeSQLFile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write sql: %v", err)
	}
	return path
}
