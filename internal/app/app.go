package app

import (
	"context"
	"io"
	"path/filepath"
	"strings"
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
	DumpFile    bool
}

type RunConfig struct {
	Path     string
	Database string
	Client   string
	Format   string
	DryRun   bool
	DumpFile bool
}

func RunWatch(ctx context.Context, cfg WatchConfig, stdout io.Writer, stderr io.Writer) error {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return err
	}
	runCfg := normalizeRunConfig(RunConfig{
		Database: cfg.Database,
		Client:   cfg.Client,
		Format:   cfg.Format,
		DryRun:   cfg.DryRun,
		DumpFile: cfg.DumpFile,
	})
	run, err := buildRunner(runCfg, stdout, stderr)
	if err != nil {
		return err
	}

	reporter, err := report.NewConsoleReporter("", stdout, stderr)
	if err != nil {
		return err
	}
	reporter.System("👀 WATCH", watchSystemMessage(root, runCfg, cfg.PrintEvents))
	defer reporter.System("🛑 STOP", "watch loop stopped")

	watcher, err := watch.NewRecursive(root)
	if err != nil {
		return err
	}

	controller := queue.NewController(queue.ControllerConfig{
		Debounce: cfg.Debounce,
		Suppress: cfg.Suppress,
		Request: model.RunRequest{
			Database: cfg.Database,
			Client:   runCfg.Client,
			Format:   runCfg.Format,
			DryRun:   cfg.DryRun,
			DumpFile: cfg.DumpFile,
		},
	}, func(path string, now time.Time) (model.FileFingerprint, bool, error) {
		return watch.SnapshotFile(root, path, now)
	}, run, reporter)

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
	cfg = normalizeRunConfig(cfg)
	run, err := buildRunner(cfg, stdout, stderr)
	if err != nil {
		return err
	}

	reporter, err := report.NewConsoleReporter("", stdout, stderr)
	if err != nil {
		return err
	}
	reporter.System("⚙️ RUNNER", runSystemMessage(path, cfg))

	request := model.RunRequest{
		Path:     path,
		Database: cfg.Database,
		Client:   cfg.Client,
		Format:   cfg.Format,
		DryRun:   cfg.DryRun,
		DumpFile: cfg.DumpFile,
	}

	reporter.Run(path)
	result := run.Run(ctx, request)
	reporter.Result(result)
	return result.Err
}

func buildRunner(cfg RunConfig, stdout io.Writer, stderr io.Writer) (runner.Runner, error) {
	if cfg.DryRun {
		return runner.NewDryRunner(), nil
	}
	if cfg.Client == "" {
		cfg.Client = "clickhouse"
	}
	if cfg.Format == "" {
		cfg.Format = "PrettyCompact"
	}
	return runner.NewClickHouseRunner(stdout, stderr), nil
}

func normalizeRunConfig(cfg RunConfig) RunConfig {
	if cfg.Client == "" {
		cfg.Client = "clickhouse"
	}
	if cfg.Format == "" {
		cfg.Format = "PrettyCompact"
	}
	return cfg
}

func watchSystemMessage(root string, cfg RunConfig, printEvents bool) string {
	parts := []string{
		"root=" + displayPath(root),
		"mode=" + modeLabel(cfg.DryRun),
		"client=" + cfg.Client,
		"format=" + cfg.Format,
	}
	if cfg.Database != "" {
		parts = append(parts, "db="+cfg.Database)
	}
	if printEvents {
		parts = append(parts, "events=on")
	}
	return strings.Join(parts, " | ")
}

func runSystemMessage(path string, cfg RunConfig) string {
	parts := []string{
		"path=" + displayPath(path),
		"mode=" + modeLabel(cfg.DryRun),
		"client=" + cfg.Client,
		"format=" + cfg.Format,
	}
	if cfg.Database != "" {
		parts = append(parts, "db="+cfg.Database)
	}
	return strings.Join(parts, " | ")
}

func modeLabel(dryRun bool) string {
	if dryRun {
		return "dry-run"
	}
	return "live"
}

func displayPath(path string) string {
	cwd, err := filepath.Abs(".")
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || rel == "" || rel == "." {
		return filepath.ToSlash(path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
