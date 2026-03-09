package app

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"ch_watch/internal/model"
	"ch_watch/internal/report"
	"ch_watch/internal/runner"
	"ch_watch/internal/watch"
)

type WatchConfig struct {
	Root        string
	Database    string
	Client      string
	Format      string
	Debounce    time.Duration
	Suppress    time.Duration
	PrintEvents bool
	DryRun      bool
}

type RunConfig struct {
	Path     string
	Database string
	Client   string
	Format   string
	DryRun   bool
}

func RunWatch(context.Context, WatchConfig, io.Writer, io.Writer) error {
	return runner.ErrWatchNotReady
}

func RunOnce(ctx context.Context, cfg RunConfig, stdout io.Writer, stderr io.Writer) error {
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return err
	}

	if _, ok, err := watch.SnapshotFile(filepath.Dir(path), path, time.Now()); err != nil {
		return err
	} else if !ok {
		return runner.ErrInvalidSQLPath
	}

	reporter, err := report.NewConsoleReporter("", stdout, stderr)
	if err != nil {
		return err
	}

	request := model.RunRequest{
		Path:     path,
		Database: cfg.Database,
		Client:   cfg.Client,
		Format:   cfg.Format,
		DryRun:   cfg.DryRun,
	}

	reporter.Run(path)
	result := newRunner(cfg, stdout, stderr).Run(ctx, request)
	reporter.Result(result)
	return result.Err
}

func newRunner(cfg RunConfig, stdout io.Writer, stderr io.Writer) runner.Runner {
	if cfg.DryRun {
		return runner.NewDryRunner()
	}
	return runner.NewPendingRunner(stdout, stderr)
}
