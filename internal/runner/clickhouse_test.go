package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webmalex/ch_watch/internal/model"
)

func TestClickHouseRunnerUsesClientModeWhenDatabaseProvided(t *testing.T) {
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
		Format:   "PrettyCompact",
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
	if gotName != "clickhouse" {
		t.Fatalf("unexpected client: %q", gotName)
	}
	if len(gotArgs) != 5 || gotArgs[0] != "client" || gotArgs[1] != "--database" || gotArgs[2] != "demo" || gotArgs[3] != "--format" || gotArgs[4] != "PrettyCompact" {
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

func TestClickHouseRunnerUsesLocalModeWhenDatabaseOmitted(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 7;\n")
	var gotName string
	var gotArgs []string
	var gotSQL string

	runner := NewClickHouseRunner(io.Discard, io.Discard)
	runner.exec = func(_ context.Context, name string, args []string, stdin io.Reader, _ io.Writer, _ io.Writer) error {
		body, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		gotName = name
		gotArgs = append([]string(nil), args...)
		gotSQL = string(body)
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:   path,
		Format: "PrettyCompact",
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
	if gotName != "clickhouse" {
		t.Fatalf("unexpected client: %q", gotName)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "local" || gotArgs[1] != "--format" || gotArgs[2] != "PrettyCompact" {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if gotSQL != "SELECT 7;\n" {
		t.Fatalf("unexpected sql: %q", gotSQL)
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

func TestClickHouseRunnerCapturesStderr(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")
	var stderrBuf bytes.Buffer
	runner := NewClickHouseRunner(io.Discard, &stderrBuf)
	runner.exec = func(_ context.Context, _ string, _ []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
		_, _ = io.WriteString(stderr, "Error: memory limit\n")
		return fakeExitError(144)
	}

	result := runner.Run(context.Background(), model.RunRequest{Path: path})

	if result.Stderr != "Error: memory limit" {
		t.Fatalf("unexpected captured stderr: %q", result.Stderr)
	}
	if stderrBuf.String() != "Error: memory limit\n" {
		t.Fatalf("unexpected stderr output: %q", stderrBuf.String())
	}
	if result.ExitCode != 144 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
}

func TestDecodeExitCodeSignal(t *testing.T) {
	t.Parallel()

	got := DecodeExitCode(144)
	if !strings.Contains(got, "signal") {
		t.Fatalf("expected signal in %q", got)
	}
}

func TestDecodeExitCodeApplication(t *testing.T) {
	t.Parallel()

	got := DecodeExitCode(62)
	if strings.Contains(got, "signal") {
		t.Fatalf("unexpected signal in %q", got)
	}
	if !strings.Contains(got, "62") {
		t.Fatalf("expected 62 in %q", got)
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

func TestDumpDirectWritesPrettyCompactTxt(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")
	var stdout bytes.Buffer
	var calls [][]string

	runner := NewClickHouseRunner(&stdout, io.Discard)
	runner.exec = func(_ context.Context, _ string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		calls = append(calls, append([]string(nil), args...))
		_, _ = io.WriteString(stdout, "pretty output\n")
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:     path,
		Format:   "PrettyCompact",
		DumpFile: true,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	want := TextDumpFilePath(path)
	if result.DumpPath != want {
		t.Fatalf("dump path: got %q, want %q", result.DumpPath, want)
	}
	if stdout.String() != "pretty output\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if len(calls) != 1 {
		t.Fatalf("expected single query call (no render), got %d", len(calls))
	}
	if containsArg(calls[0], "TabSeparatedWithNames") {
		t.Fatalf("--dump should use PrettyCompact, not TSV: %#v", calls[0])
	}

	data, err := os.ReadFile(result.DumpPath)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "pretty output\n") {
		t.Fatalf("unexpected dump content: %q", content)
	}
	if !strings.Contains(content, "-- ") {
		t.Fatalf("dump missing duration comment: %q", content)
	}
}

func TestDumpMarkdownWritesDirectly(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")
	var stdout bytes.Buffer
	var calls [][]string

	runner := NewClickHouseRunner(&stdout, io.Discard)
	runner.exec = func(_ context.Context, _ string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		calls = append(calls, append([]string(nil), args...))
		_, _ = io.WriteString(stdout, "| col |\n|-----|\n| 1   |\n")
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:         path,
		Format:       "PrettyCompact",
		DumpMarkdown: true,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	want := MarkdownDumpFilePath(path)
	if result.DumpPath != want {
		t.Fatalf("dump path: got %q, want %q", result.DumpPath, want)
	}
	if len(calls) != 1 {
		t.Fatalf("expected single query call, got %d", len(calls))
	}
	if !containsArg(calls[0], "Markdown") {
		t.Fatalf("--dump-md should use Markdown format: %#v", calls[0])
	}
	if containsArg(calls[0], "TabSeparatedWithNames") {
		t.Fatalf("--dump-md should be direct, not TSV pipeline: %#v", calls[0])
	}

	data, err := os.ReadFile(result.DumpPath)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "| col |\n") {
		t.Fatalf("unexpected dump content: %q", content)
	}
}

func TestPipeTSVRendersTextAndMarkdown(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")
	runner := NewClickHouseRunner(io.Discard, io.Discard)
	runner.exec = func(_ context.Context, _ string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		switch {
		case !isRenderCall(args) && containsArg(args, "TabSeparatedWithNames"):
			_, _ = io.WriteString(stdout, "id\n1\n")
		case isRenderCall(args) && containsArg(args, "Markdown"):
			_, _ = io.WriteString(stdout, "| id |\n|---|\n| 1 |\n")
		default:
			_, _ = io.WriteString(stdout, "pretty\n")
		}
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:         path,
		Format:       "PrettyCompact",
		PipeText:     true,
		PipeMarkdown: true,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	wantPaths := []string{DumpFilePath(path), TextDumpFilePath(path), MarkdownDumpFilePath(path)}
	if strings.Join(result.DumpPaths, "|") != strings.Join(wantPaths, "|") {
		t.Fatalf("dump paths: got %#v, want %#v", result.DumpPaths, wantPaths)
	}
	txtData, txtErr := os.ReadFile(TextDumpFilePath(path))
	if txtErr != nil {
		t.Fatalf("read txt: %v", txtErr)
	}
	txtContent := string(txtData)
	if !strings.HasPrefix(txtContent, "pretty\n") {
		t.Fatalf("txt content: got %q, want prefix %q", txtContent, "pretty\n")
	}
	if !strings.Contains(txtContent, "-- ") {
		t.Fatalf("txt content missing duration comment: %q", txtContent)
	}
	mdData, mdErr := os.ReadFile(MarkdownDumpFilePath(path))
	if mdErr != nil {
		t.Fatalf("read md: %v", mdErr)
	}
	mdContent := string(mdData)
	if !strings.HasPrefix(mdContent, "| id |\n|---|\n| 1 |\n") {
		t.Fatalf("md content: got %q, want prefix", mdContent)
	}
	if !strings.Contains(mdContent, "-- ") {
		t.Fatalf("md content missing duration comment: %q", mdContent)
	}
}

func TestPipeWithTotalsStripsExtraRowsBeforeRender(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1 GROUP BY WITH TOTALS;\n")
	var renderInputs []string

	runner := NewClickHouseRunner(io.Discard, io.Discard)
	runner.exec = func(_ context.Context, _ string, args []string, stdin io.Reader, stdout io.Writer, _ io.Writer) error {
		if !isRenderCall(args) {
			_, _ = io.WriteString(stdout, "id\tval\n1\ta\n2\tb\n\n0\tab\n")
			return nil
		}
		body, _ := io.ReadAll(stdin)
		renderInputs = append(renderInputs, string(body))
		_, _ = io.WriteString(stdout, "pretty\n")
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:     path,
		Format:   "PrettyCompact",
		PipeText: true,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	tsvData, err := os.ReadFile(DumpFilePath(path))
	if err != nil {
		t.Fatalf("read tsv: %v", err)
	}

	wantTSV := "id\tval\n1\ta\n2\tb\n\n0\tab\n"
	if string(tsvData) != wantTSV {
		t.Fatalf("tsv content: got %q, want %q", string(tsvData), wantTSV)
	}

	if len(renderInputs) != 2 {
		t.Fatalf("expected 2 render calls (console + txt), got %d", len(renderInputs))
	}

	wantRender := "id\tval\n1\ta\n2\tb\n"
	for i, input := range renderInputs {
		if input != wantRender {
			t.Fatalf("render input[%d]: got %q, want %q", i, input, wantRender)
		}
	}
}

func TestDumpFileRemovedOnFailure(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")

	runner := NewClickHouseRunner(io.Discard, io.Discard)
	runner.exec = func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
		return fakeExitError(1)
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:     path,
		Format:   "PrettyCompact",
		DumpFile: true,
	})

	if result.Err == nil {
		t.Fatal("expected error")
	}
	if result.DumpPath != "" {
		t.Fatalf("expected empty dump path on failure, got %q", result.DumpPath)
	}

	dumpPath := TextDumpFilePath(path)
	if _, err := os.Stat(dumpPath); !os.IsNotExist(err) {
		t.Fatal("dump file should not exist after failure")
	}
}

func TestDumpFileNotCreatedWhenDisabled(t *testing.T) {
	t.Parallel()

	path := writeSQL(t, "SELECT 1;\n")

	runner := NewClickHouseRunner(io.Discard, io.Discard)
	runner.exec = func(_ context.Context, _ string, _ []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		_, _ = io.WriteString(stdout, "ok\n")
		return nil
	}

	result := runner.Run(context.Background(), model.RunRequest{
		Path:   path,
		Format: "PrettyCompact",
	})

	if result.DumpPath != "" {
		t.Fatalf("expected empty dump path when dump disabled, got %q", result.DumpPath)
	}

	dumpPath := TextDumpFilePath(path)
	if _, err := os.Stat(dumpPath); !os.IsNotExist(err) {
		t.Fatal("dump file should not exist when dump disabled")
	}
}

func TestDumpFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"/tmp/query.sql", "/tmp/query.tsv"},
		{"/home/user/ch/dev/tmp.sql", "/home/user/ch/dev/tmp.tsv"},
		{"query.sql", "query.tsv"},
	}
	for _, tt := range tests {
		got := DumpFilePath(tt.in)
		if got != tt.want {
			t.Errorf("DumpFilePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTextAndMarkdownDumpFilePaths(t *testing.T) {
	t.Parallel()

	path := "/tmp/query.sql"
	if got := TextDumpFilePath(path); got != "/tmp/query.txt" {
		t.Fatalf("unexpected text path: %q", got)
	}
	if got := MarkdownDumpFilePath(path); got != "/tmp/query.md" {
		t.Fatalf("unexpected markdown path: %q", got)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func isRenderCall(args []string) bool {
	return containsArg(args, "--input-format")
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
