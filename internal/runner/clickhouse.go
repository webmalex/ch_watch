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

	"github.com/webmalex/ch_watch/internal/model"
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

	directDump := request.DumpFile || request.DumpTSV || request.DumpText || request.DumpMarkdown
	needsPipe := request.PipeText || request.PipeMarkdown

	switch {
	case directDump && needsPipe:
		result = r.runDumpDirect(ctx, request, client, sql, result)
		if result.Err != nil {
			return result
		}
		pipeResult := r.runPipe(ctx, request, client, sql)
		result.DumpPaths = append(result.DumpPaths, pipeResult.DumpPaths...)
		if pipeResult.Err != nil {
			result.Err = pipeResult.Err
			result.ExitCode = pipeResult.ExitCode
		}
		result.Duration = time.Since(result.StartedAt)
		return result
	case directDump:
		return r.runDumpDirect(ctx, request, client, sql, result)
	case needsPipe:
		return r.runPipe(ctx, request, client, sql)
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

	dumpFormat := directDumpFormat(request)
	dumpExt := formatExtension(dumpFormat)
	outPath := dumpPath(request.Path, dumpExt)
	tmpPath := outPath + ".tmp"

	dumpFile, err := os.Create(tmpPath)
	if err != nil {
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(result.StartedAt)
		return result
	}

	stdoutWriter := io.MultiWriter(r.stdout, dumpFile)
	err = r.exec(ctx, client, queryArgs(request, dumpFormat), bytes.NewReader(sql), stdoutWriter, stderrWriter)

	result.Err = err
	result.ExitCode = exitCode(err)
	result.Duration = time.Since(result.StartedAt)
	result.Stderr = strings.TrimSpace(stderrBuf.String())

	closeErr := dumpFile.Close()
	if err == nil {
		err = closeErr
		result.Err = err
		result.ExitCode = exitCode(err)
	}

	if err != nil {
		_ = os.Remove(tmpPath)
		return result
	}
	if request.StripTotals {
		if stripErr := stripFileAfterBlankLine(tmpPath); stripErr != nil {
			_ = os.Remove(tmpPath)
			result.Err = stripErr
			result.ExitCode = 1
			return result
		}
	}
	if !request.NoDuration {
		if appendErr := appendDuration(tmpPath, result.Duration); appendErr != nil {
			_ = os.Remove(tmpPath)
			result.Err = appendErr
			result.ExitCode = 1
			return result
		}
	}
	if err = os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		result.Err = err
		result.ExitCode = 1
		return result
	}

	result.DumpPath = outPath
	result.DumpPaths = []string{outPath}
	return result
}

func (r ClickHouseRunner) runPipe(ctx context.Context, request model.RunRequest, client string, sql []byte) model.RunResult {
	started := time.Now()
	result := model.RunResult{
		Path:      request.Path,
		StartedAt: started,
	}
	var stderrBuf bytes.Buffer
	stderrWriter := io.MultiWriter(r.stderr, &stderrBuf)
	tsvPath := DumpFilePath(request.Path)
	tsvTmpPath := tsvPath + ".tmp"

	tsvFile, err := os.Create(tsvTmpPath)
	if err != nil {
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(started)
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
		result.Duration = time.Since(started)
		result.Stderr = strings.TrimSpace(stderrBuf.String())
		return result
	}
	if err = os.Rename(tsvTmpPath, tsvPath); err != nil {
		_ = os.Remove(tsvTmpPath)
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(started)
		result.Stderr = strings.TrimSpace(stderrBuf.String())
		return result
	}

	if request.StripTotals {
		if stripErr := stripFileAfterBlankLine(tsvPath); stripErr != nil {
			_ = os.Remove(tsvPath)
			result.Err = stripErr
			result.ExitCode = 1
			result.Duration = time.Since(started)
			result.Stderr = strings.TrimSpace(stderrBuf.String())
			return result
		}
	}

	dumpPaths := []string{tsvPath}
	if err = r.renderDump(ctx, client, tsvPath, request.Format, r.stdout, r.stderr); err != nil {
		result.Err = err
		result.ExitCode = exitCode(err)
		result.Duration = time.Since(started)
		result.Stderr = strings.TrimSpace(stderrBuf.String())
		result.DumpPath = tsvPath
		result.DumpPaths = dumpPaths
		return result
	}

	if request.PipeText {
		txtPath, renderErr := r.renderDumpFile(ctx, client, tsvPath, TextDumpFilePath(request.Path), prettyDumpFormat)
		if renderErr != nil {
			result.Err = renderErr
			result.ExitCode = exitCode(renderErr)
			result.Duration = time.Since(started)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
		if request.NoDuration {
			dumpPaths = append(dumpPaths, txtPath)
		} else if appendErr := appendDurationComment(txtPath, started); appendErr == nil {
			dumpPaths = append(dumpPaths, txtPath)
		} else {
			result.Err = appendErr
			result.ExitCode = 1
			result.Duration = time.Since(started)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
	}
	if request.PipeMarkdown {
		mdPath, renderErr := r.renderDumpFile(ctx, client, tsvPath, MarkdownDumpFilePath(request.Path), markdownDumpFormat)
		if renderErr != nil {
			result.Err = renderErr
			result.ExitCode = exitCode(renderErr)
			result.Duration = time.Since(started)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
		if request.NoDuration {
			dumpPaths = append(dumpPaths, mdPath)
		} else if appendErr := appendDurationComment(mdPath, started); appendErr == nil {
			dumpPaths = append(dumpPaths, mdPath)
		} else {
			result.Err = appendErr
			result.ExitCode = 1
			result.Duration = time.Since(started)
			result.DumpPath = tsvPath
			result.DumpPaths = dumpPaths
			return result
		}
	}

	result.Err = nil
	result.ExitCode = 0
	result.Duration = time.Since(started)
	result.Stderr = strings.TrimSpace(stderrBuf.String())
	result.DumpPath = tsvPath
	result.DumpPaths = dumpPaths
	return result
}

func DumpFilePath(sqlPath string) string {
	return dumpPath(sqlPath, ".tsv")
}

func TSVDumpFilePath(sqlPath string) string {
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

func directDumpFormat(request model.RunRequest) string {
	if request.DumpTSV {
		return canonicalDumpFormat
	}
	if request.DumpMarkdown {
		return markdownDumpFormat
	}
	if request.DumpText {
		return prettyDumpFormat
	}
	return request.Format
}

func formatExtension(format string) string {
	switch format {
	case markdownDumpFormat:
		return ".md"
	case canonicalDumpFormat, "TabSeparatedWithNamesAndTypes", "TabSeparated":
		return ".tsv"
	default:
		return ".txt"
	}
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
	return bytes.NewReader(stripBytesAfterBlankLine(raw))
}

func stripFileAfterBlankLine(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, stripBytesAfterBlankLine(data), 0o644)
}

func stripBytesAfterBlankLine(data []byte) []byte {
	cut := len(data)
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			cut = i + 1
			break
		}
	}
	return data[:cut]
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
	return appendDuration(path, time.Since(startedAt))
}

func appendDuration(path string, duration time.Duration) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "\n-- %s\n", duration.Round(time.Millisecond))
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
