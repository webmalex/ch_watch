package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"ch_watch/internal/app"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError(stderr)
	}

	switch args[0] {
	case "watch":
		cfg, err := parseWatch(args[1:])
		if err != nil {
			return err
		}
		return app.RunWatch(ctx, cfg, stdout, stderr)
	case "run":
		cfg, err := parseRun(args[1:])
		if err != nil {
			return err
		}
		return app.RunOnce(ctx, cfg, stdout, stderr)
	case "help", "-h", "--help":
		return usageError(stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parseWatch(args []string) (app.WatchConfig, error) {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := app.WatchConfig{}
	fs.StringVar(&cfg.Root, "root", "./ch", "watch root")
	fs.StringVar(&cfg.Database, "db", "", "ClickHouse database")
	fs.StringVar(&cfg.Client, "client", "clickhouse-client", "client path")
	fs.StringVar(&cfg.Format, "format", "PrettyCompact", "output format")
	fs.DurationVar(&cfg.Debounce, "debounce", 75*time.Millisecond, "debounce window")
	fs.DurationVar(&cfg.Suppress, "suppress", 250*time.Millisecond, "suppression window")
	fs.BoolVar(&cfg.PrintEvents, "print-events", false, "print normalized events")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print what would run")

	if err := fs.Parse(args); err != nil {
		return app.WatchConfig{}, err
	}
	if fs.NArg() != 0 {
		return app.WatchConfig{}, errors.New("watch does not accept positional arguments")
	}
	return cfg, nil
}

func parseRun(args []string) (app.RunConfig, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := app.RunConfig{}
	fs.StringVar(&cfg.Database, "db", "", "ClickHouse database")
	fs.StringVar(&cfg.Client, "client", "clickhouse-client", "client path")
	fs.StringVar(&cfg.Format, "format", "PrettyCompact", "output format")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print what would run")

	if err := fs.Parse(args); err != nil {
		return app.RunConfig{}, err
	}
	if fs.NArg() != 1 {
		return app.RunConfig{}, errors.New("run requires exactly one SQL file path")
	}
	cfg.Path = fs.Arg(0)
	return cfg, nil
}

func usageError(w io.Writer) error {
	_, _ = fmt.Fprintln(w, "usage: ch_watch <watch|run> [options]")
	return errors.New("missing command")
}
