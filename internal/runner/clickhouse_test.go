package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"ch_watch/internal/model"
)

func TestClickHouseRunnerPassesSQLViaStdin(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 42;\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotName string
	var gotArgs []string
	var gotSQL string

	runner := NewClickHouseRunner(&stdout, &stderr)
	runner.exec = func(_ context.Context, name string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
		body, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		gotName = name
		gotArgs = append([]string(nil), args...)
		gotSQL = string(body)
		_, _ = io.WriteString(stdout, "ok output\n")
		_, _ = io.WriteString(stderr, "warn output\n")
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:     path,
		Database: "demo",
		Client:   "clickhouse-client",
		Format:   "PrettyCompact",
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
	if gotName != "clickhouse-client" {
		t.Fatalf("unexpected client: %q", gotName)
	}
	if len(gotArgs) != 4 || gotArgs[0] != "-d" || gotArgs[1] != "demo" || gotArgs[2] != "-f" || gotArgs[3] != "PrettyCompact" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if gotSQL != "SELECT 42;\n" {
		t.Fatalf("unexpected sql: %q", gotSQL)
	}
	if stdout.String() != "ok output\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.String() != "warn output\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestClickHouseRunnerReturnsExitCode(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")
	runner := NewClickHouseRunner(io.Discard, io.Discard)
	runner.exec = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		return fakeExitError(62)
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:     path,
		Database: "demo",
	})

	if result.Err == nil {
		t.Fatal("expected an error")
	}
	if result.ExitCode != 62 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
}

func TestClickHouseRunnerRequiresDatabase(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")
	runner := NewClickHouseRunner(io.Discard, io.Discard)

	result := runner.Run(context.Background(), model.RunRequest{Path: path})

	if result.Err != ErrDatabaseRequired {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
}

func TestDryRunnerBypassesSubprocessExecution(t *testing.T) {
	t.Parallel()

	result := NewDryRunner().Run(context.Background(), model.RunRequest{Path: "/tmp/query.sql", DryRun: true})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if !result.DryRun {
		t.Fatal("expected dry-run result")
	}
}

type fakeExitError int

func (e fakeExitError) Error() string {
	return "exit error"
}

func (e fakeExitError) ExitCode() int {
	return int(e)
}

func writeSQL(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write sql: %v", err)
	}
	return path
}
