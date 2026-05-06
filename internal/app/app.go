package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ch_watch/internal/model"
	"ch_watch/internal/queue"
	"ch_watch/internal/report"
	"ch_watch/internal/runner"
	"ch_watch/internal/watch"
)

type WatchConfig struct {
	Root         string
	Database     string
	Client       string
	Format       string
	Debounce     time.Duration
	Suppress     time.Duration
	PrintEvents  bool
	DryRun       bool
	DumpFile     bool
	DumpText     bool
	DumpMarkdown bool
	PipeText     bool
	PipeMarkdown bool
}

type RunConfig struct {
	Path         string
	Database     string
	Client       string
	Format       string
	DryRun       bool
	DumpFile     bool
	DumpText     bool
	DumpMarkdown bool
	PipeText     bool
	PipeMarkdown bool
}

func RunWatch(ctx context.Context, cfg WatchConfig, stdout io.Writer, stderr io.Writer) error {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return err
	}
	runCfg := normalizeRunConfig(RunConfig{
		Database:     cfg.Database,
		Client:       cfg.Client,
		Format:       cfg.Format,
		DryRun:       cfg.DryRun,
		DumpFile:     cfg.DumpFile,
		DumpText:     cfg.DumpText,
		DumpMarkdown: cfg.DumpMarkdown,
		PipeText:     cfg.PipeText,
		PipeMarkdown: cfg.PipeMarkdown,
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
			Database:     runCfg.Database,
			Client:       runCfg.Client,
			Format:       runCfg.Format,
			DryRun:       cfg.DryRun,
			DumpFile:     runCfg.DumpFile,
			DumpText:     runCfg.DumpText,
			DumpMarkdown: runCfg.DumpMarkdown,
			PipeText:     runCfg.PipeText,
			PipeMarkdown: runCfg.PipeMarkdown,
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

	info, err := os.Stat(path)
	if err != nil {
		return err
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

	if info.IsDir() {
		return runDir(ctx, path, cfg, run, reporter)
	}
	return runFile(ctx, path, cfg, run, reporter)
}

func runFile(ctx context.Context, path string, cfg RunConfig, run runner.Runner, reporter report.Reporter) error {
	if _, ok, err := watch.SnapshotFile(filepath.Dir(path), path, time.Now()); err != nil {
		return err
	} else if !ok {
		return runner.ErrInvalidSQLPath
	}

	reporter.System("⚙️ RUNNER", runSystemMessage(path, cfg))
	request := model.RunRequest{
		Path:         path,
		Database:     cfg.Database,
		Client:       cfg.Client,
		Format:       cfg.Format,
		DryRun:       cfg.DryRun,
		DumpFile:     cfg.DumpFile,
		DumpText:     cfg.DumpText,
		DumpMarkdown: cfg.DumpMarkdown,
		PipeText:     cfg.PipeText,
		PipeMarkdown: cfg.PipeMarkdown,
	}

	reporter.Run(path)
	result := run.Run(ctx, request)
	reporter.Result(result)
	return result.Err
}

func runDir(ctx context.Context, root string, cfg RunConfig, run runner.Runner, reporter report.Reporter) error {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !watch.IsSQLFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return ErrNoSQLFiles
	}
	sort.Strings(files)

	reporter.System("⚙️ RUNNER", dirSystemMessage(root, len(files), cfg))

	var firstErr error
	for _, f := range files {
		request := model.RunRequest{
			Path:         f,
			Database:     cfg.Database,
			Client:       cfg.Client,
			Format:       cfg.Format,
			DryRun:       cfg.DryRun,
			DumpFile:     cfg.DumpFile,
			DumpText:     cfg.DumpText,
			DumpMarkdown: cfg.DumpMarkdown,
			PipeText:     cfg.PipeText,
			PipeMarkdown: cfg.PipeMarkdown,
		}
		reporter.Run(f)
		result := run.Run(ctx, request)
		reporter.Result(result)
		if result.Err != nil && firstErr == nil {
			firstErr = result.Err
		}
	}
	return firstErr
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

var ErrNoSQLFiles = errors.New("no .sql files found in directory")

func normalizeRunConfig(cfg RunConfig) RunConfig {
	if cfg.Client == "" {
		cfg.Client = "clickhouse"
	}
	if cfg.Database == "" {
		cfg.Database = os.Getenv("CH_DB")
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

func dirSystemMessage(root string, count int, cfg RunConfig) string {
	parts := []string{
		"root=" + displayPath(root),
		fmt.Sprintf("files=%d", count),
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
