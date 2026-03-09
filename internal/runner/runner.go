package runner

import (
	"context"
	"errors"
	"io"
	"time"

	"ch_watch/internal/model"
)

var (
	ErrInvalidSQLPath = errors.New("path is not a runnable SQL file")
	ErrWatchNotReady  = errors.New("watch command is not implemented yet")
)

type Runner interface {
	Run(ctx context.Context, request model.RunRequest) model.RunResult
}

type DryRunner struct{}

func NewDryRunner() DryRunner {
	return DryRunner{}
}

func (DryRunner) Run(_ context.Context, request model.RunRequest) model.RunResult {
	started := time.Now()
	return model.RunResult{
		Path:      request.Path,
		StartedAt: started,
		Duration:  time.Since(started),
		DryRun:    true,
	}
}

type PendingRunner struct {
	stdout io.Writer
	stderr io.Writer
}

func NewPendingRunner(stdout io.Writer, stderr io.Writer) PendingRunner {
	return PendingRunner{stdout: stdout, stderr: stderr}
}

func (PendingRunner) Run(_ context.Context, request model.RunRequest) model.RunResult {
	started := time.Now()
	return model.RunResult{
		Path:      request.Path,
		StartedAt: started,
		Duration:  time.Since(started),
		Err:       errors.New("clickhouse runner is not implemented yet"),
		ExitCode:  1,
	}
}
