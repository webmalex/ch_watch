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
	"time"

	"ch_watch/internal/model"
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

	args := make([]string, 0, 5)
	if request.Database != "" {
		args = append(args, "client", "--database", request.Database)
	} else {
		args = append(args, "local")
	}
	if request.Format != "" {
		args = append(args, "--format", request.Format)
	}
	client := request.Client
	if client == "" {
		client = "clickhouse"
	}

	stdoutWriter := r.stdout
	var dumpFile *os.File
	dumpPath := ""

	if request.DumpFile {
		dumpPath = DumpFilePath(request.Path)
		f, fErr := os.Create(dumpPath)
		if fErr != nil {
			dumpPath = ""
		} else {
			dumpFile = f
			stdoutWriter = io.MultiWriter(r.stdout, f)
		}
	}

	err = r.exec(ctx, client, args, bytes.NewReader(sql), stdoutWriter, r.stderr)

	result.Err = err
	result.ExitCode = exitCode(err)
	result.Duration = time.Since(started)

	if dumpFile != nil {
		if err == nil {
			_, _ = fmt.Fprintf(dumpFile, "\n-- %s\n", result.Duration.Round(time.Millisecond))
		}
		_ = dumpFile.Close()
		if err != nil {
			_ = os.Remove(dumpPath)
			dumpPath = ""
		}
	}

	result.DumpPath = dumpPath
	return result
}

func DumpFilePath(sqlPath string) string {
	ext := filepath.Ext(sqlPath)
	return strings.TrimSuffix(sqlPath, ext) + ".txt"
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
