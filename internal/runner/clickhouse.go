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
	canonicalDumpFormat = "TabSeparatedWithNames"
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

	needsTSVPipeline := request.DumpText || request.DumpMarkdown

	switch {
	case request.DumpFile && needsTSVPipeline:
		return r.runDumpWithRender(ctx, request, client, sql, result)
	case request.DumpFile:
		return r.runDumpDirect(ctx, request, client, sql, result)
	case needsTSVPipeline:
		request.DumpFile = true
		return r.runDumpWithRender(ctx, request, client, sql, result)
	default:
		return r.runPlain(ctx, request, client, sql, result)
	}
}

func (r ClickHouseRunner) runPlain(ctx context.Context, request model.RunRequest, client string, sql []byte, result model.RunResult) model.RunResult {
	var stderrBuf bytes.Buffer
	stderrWriter := io.MultiWriter(r.stderr, &stderrBuf)
	err := r.exec(ctx, client, queryArgs(request, request.Format), bytes.NewReader(sql), r.stdout, stderrWriter)

	result.Err = err
	result.ExitCode = exitCode(err)
	result.Duration = time.Since(result.StartedAt)
	result.Stderr = strings.TrimSpace(stderrBuf.String())
	return result
}

func (r ClickHouseRunner) runDumpDirect(ctx context.Context, request model.RunRequest, client string, sql []byte, result model.RunResult) model.RunResult {
	var stderrBuf bytes.Buffer
	stderrWriter := io.MultiWriter(r.stderr, &stderrBuf)

	txtPath := TextDumpFilePath(request.Path)
	txtTmpPath := txtPath + ".tmp"

	dumpFile, err := os.Create(txtTmpPath)
	if err != nil {
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(result.StartedAt)
		return result
	}

	stdoutWriter := io.MultiWriter(r.stdout, dumpFile)
	err = r.exec(ctx, client, queryArgs(request, request.Format), bytes.NewReader(sql), stdoutWriter, stderrWriter)

	result.Err = err
	result.ExitCode = exitCode(err)
	result.Duration = time.Since(result.StartedAt)
	result.Stderr = strings.TrimSpace(stderrBuf.String())

	if err == nil {
		_, _ = fmt.Fprintf(dumpFile, "\n-- %s\n", result.Duration.Round(time.Millisecond))
	}
	_ = dumpFile.Close()

	if err != nil {
		_ = os.Remove(txtTmpPath)
		return result
	}
	if err = os.Rename(txtTmpPath, txtPath); err != nil {
		_ = os.Remove(txtTmpPath)
		result.Err = err
		result.ExitCode = 1
		return result
	}

	result.DumpPath = txtPath
	result.DumpPaths = []string{txtPath}
	return result
}

func (r ClickHouseRunner) runDumpWithRender(ctx context.Context, request model.RunRequest, client string, sql []byte, result model.RunResult) model.RunResult {
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
		txtPath, renderErr := r.renderDumpFile(ctx, client, tsvPath, TextDumpFilePath(request.Path), prettyDumpFormat)
		if renderErr != nil {
			result.Err = renderErr
			result.ExitCode = exitCode(renderErr)
			result.Duration = time.Since(result.StartedAt)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
		if appendErr := appendDurationComment(txtPath, result.StartedAt); appendErr == nil {
			dumpPaths = append(dumpPaths, txtPath)
		}
	}
	if request.DumpMarkdown {
		mdPath, renderErr := r.renderDumpFile(ctx, client, tsvPath, MarkdownDumpFilePath(request.Path), markdownDumpFormat)
		if renderErr != nil {
			result.Err = renderErr
			result.ExitCode = exitCode(renderErr)
			result.Duration = time.Since(result.StartedAt)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
		if appendErr := appendDurationComment(mdPath, result.StartedAt); appendErr == nil {
			dumpPaths = append(dumpPaths, mdPath)
		}
	}

	result.Err = nil
	result.ExitCode = 0
	result.Duration = time.Since(result.StartedAt)
	result.Stderr = strings.TrimSpace(stderrBuf.String())
	result.DumpPath = tsvPath
	result.DumpPaths = dumpPaths
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
	dataOnly := stripAfterBlankLine(input)
	return r.exec(ctx, client, renderArgs(format), dataOnly, stdout, stderr)
}

func stripAfterBlankLine(r io.Reader) *bytes.Reader {
	raw, _ := io.ReadAll(r)
	cut := len(raw)
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] == '\n' && raw[i+1] == '\n' {
			cut = i + 1
			break
		}
	}
	return bytes.NewReader(raw[:cut])
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

func appendDurationComment(path string, startedAt time.Time) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "\n-- %s\n", time.Since(startedAt).Round(time.Millisecond))
	_ = f.Close()
	return err
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
