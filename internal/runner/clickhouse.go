package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
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

	if request.Database == "" {
		result.Err = ErrDatabaseRequired
		result.ExitCode = 1
		result.Duration = time.Since(started)
		return result
	}

	sql, err := r.readFile(request.Path)
	if err != nil {
		result.Err = err
		result.ExitCode = 1
		result.Duration = time.Since(started)
		return result
	}

	args := []string{"-d", request.Database}
	if request.Format != "" {
		args = append(args, "-f", request.Format)
	}
	client := request.Client
	if client == "" {
		client = "clickhouse-client"
	}

	err = r.exec(ctx, client, args, bytes.NewReader(sql), r.stdout, r.stderr)
	result.Err = err
	result.ExitCode = exitCode(err)
	result.Duration = time.Since(started)
	return result
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
