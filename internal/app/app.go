package app

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"ch_watch/internal/model"
	"ch_watch/internal/queue"
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

func RunWatch(ctx context.Context, cfg WatchConfig, stdout io.Writer, stderr io.Writer) error {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return err
	}

	reporter, err := report.NewConsoleReporter("", stdout, stderr)
	if err != nil {
		return err
	}

	watcher, err := watch.NewRecursive(root)
	if err != nil {
		return err
	}

	controller := queue.NewController(queue.ControllerConfig{
		Debounce: cfg.Debounce,
		Suppress: cfg.Suppress,
	}, func(path string, now time.Time) (model.FileFingerprint, bool, error) {
		return watch.SnapshotFile(root, path, now)
	}, newRunner(RunConfig{
		Database: cfg.Database,
		Client:   cfg.Client,
		Format:   cfg.Format,
		DryRun:   cfg.DryRun,
	}, stdout, stderr), reporter)

	go func() {
		_ = controller.Run(ctx)
	}()

	return watcher.Run(ctx, func(event watch.Event) {
		if cfg.PrintEvents {
			reporter.Event(event.Path, event.Op)
		}
		controller.Notify(event.Path)
	})
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
