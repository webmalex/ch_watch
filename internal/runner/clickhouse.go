package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ch_watch/internal/model"
)

const (
	canonicalDumpFormat = "TabSeparatedWithNamesAndTypes"
	prettyDumpFormat    = "PrettyCompact"
	markdownDumpFormat  = "Markdown"
)

type ExecFunc func(ctx context.Context, name string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error

type ClickHouseRunner struct {
	stdout   io.Writer
	stderr   io.Writer
	readFile func(path string) ([]byte, error)
	exec     ExecFunc
}

func NewClickHouseRunner(stdout io.Writer, stderr io.Writer) ClickHouseRunner {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	return ClickHouseRunner{
		stdout:   stdout,
		stderr:   stderr,
		readFile: os.ReadFile,
		exec:     defaultExec,
	}
}

func (r ClickHouseRunner) Run(ctx context.Context, request model.RunRequest) model.RunResult {
	started := time.Now()
	result := model.RunResult{
		Path:      request.Path,
		StartedAt: started,
	}

	sql, err := r.readFile(request.Path)
	if err != nil {
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(started)
		return result
	}

	client := request.Client
	if client == "" {
		client = "clickhouse"
	}
	if request.DumpFile {
		return r.runWithDump(ctx, request, client, sql, result)
	}

	var stderrBuf bytes.Buffer
	stderrWriter := io.MultiWriter(r.stderr, &stderrBuf)
	err = r.exec(ctx, client, queryArgs(request, request.Format), bytes.NewReader(sql), r.stdout, stderrWriter)

	result.Err = err
	result.ExitCode = exitCode(err)
	result.Duration = time.Since(started)
	result.Stderr = strings.TrimSpace(stderrBuf.String())
	return result
}

func DumpFilePath(sqlPath string) string {
	return dumpPath(sqlPath, ".tsv")
}

func TextDumpFilePath(sqlPath string) string {
	return dumpPath(sqlPath, ".txt")
}

func MarkdownDumpFilePath(sqlPath string) string {
	return dumpPath(sqlPath, ".md")
}

func dumpPath(sqlPath string, extension string) string {
	ext := filepath.Ext(sqlPath)
	return strings.TrimSuffix(sqlPath, ext) + extension
}

func (r ClickHouseRunner) runWithDump(ctx context.Context, request model.RunRequest, client string, sql []byte, result model.RunResult) model.RunResult {
	var stderrBuf bytes.Buffer
	stderrWriter := io.MultiWriter(r.stderr, &stderrBuf)
	tsvPath := DumpFilePath(request.Path)
	tsvTmpPath := tsvPath + ".tmp"

	tsvFile, err := os.Create(tsvTmpPath)
	if err != nil {
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(result.StartedAt)
		return result
	}

	err = r.exec(ctx, client, queryArgs(request, canonicalDumpFormat), bytes.NewReader(sql), tsvFile, stderrWriter)
	closeErr := tsvFile.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tsvTmpPath)
		result.Err = err
		result.ExitCode = exitCode(err)
		result.Duration = time.Since(result.StartedAt)
		result.Stderr = strings.TrimSpace(stderrBuf.String())
		return result
	}
	if err = os.Rename(tsvTmpPath, tsvPath); err != nil {
		_ = os.Remove(tsvTmpPath)
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(result.StartedAt)
		result.Stderr = strings.TrimSpace(stderrBuf.String())
		return result
	}

	dumpPaths := []string{tsvPath}
	if err = r.renderDump(ctx, client, tsvPath, request.Format, r.stdout, r.stderr); err != nil {
		result.Err = err
		result.ExitCode = exitCode(err)
		result.Duration = time.Since(result.StartedAt)
		result.Stderr = strings.TrimSpace(stderrBuf.String())
		result.DumpPath = tsvPath
		result.DumpPaths = dumpPaths
		return result
	}

	if request.DumpText {
		path, renderErr := r.renderDumpFile(ctx, client, tsvPath, TextDumpFilePath(request.Path), prettyDumpFormat)
		if renderErr != nil {
			result.Err = renderErr
			result.ExitCode = exitCode(renderErr)
			result.Duration = time.Since(result.StartedAt)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
		dumpPaths = append(dumpPaths, path)
	}
	if request.DumpMarkdown {
		path, renderErr := r.renderDumpFile(ctx, client, tsvPath, MarkdownDumpFilePath(request.Path), markdownDumpFormat)
		if renderErr != nil {
			result.Err = renderErr
			result.ExitCode = exitCode(renderErr)
			result.Duration = time.Since(result.StartedAt)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
		dumpPaths = append(dumpPaths, path)
	}

	result.Err = nil
	result.ExitCode = 0
	result.Duration = time.Since(result.StartedAt)
	result.Stderr = strings.TrimSpace(stderrBuf.String())
	result.DumpPath = tsvPath
	result.DumpPaths = dumpPaths
	return result
}

func queryArgs(request model.RunRequest, format string) []string {
	args := make([]string, 0, 5)
	if request.Database != "" {
		args = append(args, "client", "--database", request.Database)
	} else {
		args = append(args, "local")
	}
	if format != "" {
		args = append(args, "--format", format)
	}
	return args
}

func renderArgs(format string) []string {
	return []string{"local", "--input-format", canonicalDumpFormat, "--format", format, "--query", "SELECT * FROM table"}
}

func (r ClickHouseRunner) renderDump(ctx context.Context, client string, tsvPath string, format string, stdout io.Writer, stderr io.Writer) error {
	if format == "" {
		format = prettyDumpFormat
	}
	input, err := os.Open(tsvPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = input.Close()
	}()
	return r.exec(ctx, client, renderArgs(format), input, stdout, stderr)
}

func (r ClickHouseRunner) renderDumpFile(ctx context.Context, client string, tsvPath string, outputPath string, format string) (string, error) {
	tmpPath := outputPath + ".tmp"
	output, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	err = r.renderDump(ctx, client, tsvPath, format, output, r.stderr)
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err = os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return outputPath, nil
}

func defaultExec(ctx context.Context, name string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	type exitCoder interface {
		ExitCode() int
	}
	var coded exitCoder
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return 1
}

func DecodeExitCode(code int) string {
	if code >= 128 {
		sig := syscall.Signal(code - 128)
		return fmt.Sprintf("exit %d (signal: %s)", code, sig)
	}
	return fmt.Sprintf("exit %d", code)
}
