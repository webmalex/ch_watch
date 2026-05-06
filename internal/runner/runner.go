package runner

import (
	"context"
	"errors"
	"time"

	"github.com/webmalex/ch_watch/internal/model"
)

var (
	ErrInvalidSQLPath = errors.New("path is not a runnable SQL file")
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
