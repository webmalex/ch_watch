package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	if !strings.Contains(stdout.String(), "RUN") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestWatchCommandWiresDryRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(ctx, []string{"watch", "--root", root, "--dry-run"}, &stdout, &stderr)
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
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
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
