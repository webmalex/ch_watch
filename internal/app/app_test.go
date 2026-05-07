package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webmalex/ch_watch/internal/report"
	"github.com/webmalex/ch_watch/internal/runner"
)

func TestBuildRunnerLiveModeDoesNotRequireDatabase(t *testing.T) {
	t.Parallel()

	r, err := buildRunner(RunConfig{}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := r.(runner.ClickHouseRunner); !ok {
		t.Fatalf("unexpected runner type: %T", r)
	}
}

func TestNormalizeRunConfigDefaultsClickHouseBinary(t *testing.T) {
	t.Parallel()

	cfg := normalizeRunConfig(RunConfig{})

	if cfg.Client != "clickhouse" {
		t.Fatalf("unexpected client default: %q", cfg.Client)
	}
	if cfg.Format != "PrettyCompact" {
		t.Fatalf("unexpected format default: %q", cfg.Format)
	}
}

func TestNormalizeRunConfigFallsBackToEnvDB(t *testing.T) {
	t.Setenv("CH_DB", "testdb")

	cfg := normalizeRunConfig(RunConfig{})
	if cfg.Database != "testdb" {
		t.Fatalf("expected database from env, got %q", cfg.Database)
	}
}

func TestNormalizeRunConfigFlagOverridesEnvDB(t *testing.T) {
	t.Setenv("CH_DB", "envdb")

	cfg := normalizeRunConfig(RunConfig{Database: "flagdb"})
	if cfg.Database != "flagdb" {
		t.Fatalf("expected flag to override env, got %q", cfg.Database)
	}
}

func TestBuildRunnerDryMode(t *testing.T) {
	t.Parallel()

	r, err := buildRunner(RunConfig{DryRun: true}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := r.(runner.DryRunner); !ok {
		t.Fatalf("expected DryRunner, got %T", r)
	}
}

func TestRunOnceSingleFileDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(sqlPath, []byte("SELECT 1"), 0o644); err != nil {
		t.Fatalf("write sql: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := RunOnce(context.Background(), RunConfig{
		Path:   sqlPath,
		DryRun: true,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunOnceDirectoryDryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, name := range []string{"a.sql", "b.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	var stdout, stderr bytes.Buffer
	err := RunOnce(context.Background(), RunConfig{
		Path:   dir,
		DryRun: true,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunOnceDirectoryNoSQLFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := RunOnce(context.Background(), RunConfig{
		Path:   dir,
		DryRun: true,
	}, &stdout, &stderr)
	if err != ErrNoSQLFiles {
		t.Fatalf("expected ErrNoSQLFiles, got: %v", err)
	}
}

func TestRunOnceNonExistentPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := RunOnce(context.Background(), RunConfig{
		Path:   "/no/such/path/query.sql",
		DryRun: true,
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestRunFileNonSQLExtension(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	txtPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dr := runner.NewDryRunner()
	reporter, err := newTestReporter()
	if err != nil {
		t.Fatalf("reporter: %v", err)
	}

	err = runFile(context.Background(), txtPath, RunConfig{DryRun: true}, dr, reporter)
	if err != runner.ErrInvalidSQLPath {
		t.Fatalf("expected ErrInvalidSQLPath, got: %v", err)
	}
}

func TestRunDirMixedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("SELECT 1"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("skip me"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.sql"), []byte("SELECT 2"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dr := runner.NewDryRunner()
	reporter, err := newTestReporter()
	if err != nil {
		t.Fatalf("reporter: %v", err)
	}

	err = runDir(context.Background(), dir, RunConfig{DryRun: true}, dr, reporter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDirSubdirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.sql"), []byte("SELECT 1"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dr := runner.NewDryRunner()
	reporter, err := newTestReporter()
	if err != nil {
		t.Fatalf("reporter: %v", err)
	}

	err = runDir(context.Background(), dir, RunConfig{DryRun: true}, dr, reporter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModeLabel(t *testing.T) {
	t.Parallel()

	if got := modeLabel(true); got != "dry-run" {
		t.Fatalf("expected dry-run, got %q", got)
	}
	if got := modeLabel(false); got != "live" {
		t.Fatalf("expected live, got %q", got)
	}
}

func TestDisplayPathRelative(t *testing.T) {
	t.Parallel()

	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	rel := filepath.Join(cwd, "sub", "file.sql")
	got := displayPath(rel)
	want := filepath.ToSlash(filepath.Join("sub", "file.sql"))
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDisplayPathAbsoluteOutsideCWD(t *testing.T) {
	t.Parallel()

	got := displayPath("/tmp/outside/file.sql")
	want := "/tmp/outside/file.sql"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestDisplayPathParentDotDot(t *testing.T) {
	t.Parallel()

	parent := filepath.Join("..", "other", "file.sql")
	abs, err := filepath.Abs(parent)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	got := displayPath(abs)
	want := filepath.ToSlash(abs)
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWatchSystemMessageWithDatabase(t *testing.T) {
	t.Parallel()

	msg := watchSystemMessage("/root", RunConfig{
		DryRun:   false,
		Client:   "clickhouse",
		Format:   "PrettyCompact",
		Database: "mydb",
	}, true)

	if !strings.Contains(msg, "mode=live") {
		t.Fatal("expected mode=live")
	}
	if !strings.Contains(msg, "db=mydb") {
		t.Fatal("expected db=mydb")
	}
	if !strings.Contains(msg, "events=on") {
		t.Fatal("expected events=on")
	}
}

func TestWatchSystemMessageWithoutDatabaseNoEvents(t *testing.T) {
	t.Parallel()

	msg := watchSystemMessage("/root", RunConfig{
		DryRun: true,
		Client: "clickhouse",
		Format: "PrettyCompact",
	}, false)

	if !strings.Contains(msg, "mode=dry-run") {
		t.Fatal("expected mode=dry-run")
	}
	if strings.Contains(msg, "db=") {
		t.Fatal("did not expect db=")
	}
	if strings.Contains(msg, "events=") {
		t.Fatal("did not expect events=")
	}
}

func TestRunSystemMessageWithDatabase(t *testing.T) {
	t.Parallel()

	msg := runSystemMessage("/path/to/query.sql", RunConfig{
		DryRun:   false,
		Client:   "clickhouse",
		Format:   "PrettyCompact",
		Database: "testdb",
	})

	if !strings.Contains(msg, "mode=live") {
		t.Fatal("expected mode=live")
	}
	if !strings.Contains(msg, "db=testdb") {
		t.Fatal("expected db=testdb")
	}
}

func TestRunSystemMessageWithoutDatabase(t *testing.T) {
	t.Parallel()

	msg := runSystemMessage("/path/to/query.sql", RunConfig{
		DryRun: true,
		Client: "clickhouse",
		Format: "PrettyCompact",
	})

	if !strings.Contains(msg, "mode=dry-run") {
		t.Fatal("expected mode=dry-run")
	}
	if strings.Contains(msg, "db=") {
		t.Fatal("did not expect db=")
	}
}

func TestDirSystemMessageWithDatabase(t *testing.T) {
	t.Parallel()

	msg := dirSystemMessage("/root", 3, RunConfig{
		DryRun:   false,
		Client:   "clickhouse",
		Format:   "PrettyCompact",
		Database: "mydb",
	})

	if !strings.Contains(msg, "files=3") {
		t.Fatal("expected files=3")
	}
	if !strings.Contains(msg, "db=mydb") {
		t.Fatal("expected db=mydb")
	}
	if !strings.Contains(msg, "mode=live") {
		t.Fatal("expected mode=live")
	}
}

func TestDirSystemMessageWithoutDatabase(t *testing.T) {
	t.Parallel()

	msg := dirSystemMessage("/root", 1, RunConfig{
		DryRun: true,
		Client: "clickhouse",
		Format: "PrettyCompact",
	})

	if !strings.Contains(msg, "files=1") {
		t.Fatal("expected files=1")
	}
	if !strings.Contains(msg, "mode=dry-run") {
		t.Fatal("expected mode=dry-run")
	}
	if strings.Contains(msg, "db=") {
		t.Fatal("did not expect db=")
	}
}

func TestDisplayPathCWDItself(t *testing.T) {
	t.Parallel()

	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	got := displayPath(cwd)
	if got != filepath.ToSlash(cwd) {
		t.Fatalf("expected absolute path for cwd itself, got %q", got)
	}
}

func TestErrNoSQLFilesSentinel(t *testing.T) {
	t.Parallel()

	if ErrNoSQLFiles.Error() != "no .sql files found in directory" {
		t.Fatalf("unexpected error message: %q", ErrNoSQLFiles.Error())
	}
}

func newTestReporter() (*report.ConsoleReporter, error) {
	return report.NewConsoleReporter("", io.Discard, io.Discard)
}
