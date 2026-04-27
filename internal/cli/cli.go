package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"ch_watch/internal/app"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		writeUsage(stderr)
		return errors.New("missing command")
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
		writeUsage(stderr)
		return nil
	default:
		writeUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parseWatch(args []string) (app.WatchConfig, error) {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := app.WatchConfig{}
	fs.StringVar(&cfg.Root, "root", "./ch", "watch root")
	fs.StringVar(&cfg.Database, "db", "", "ClickHouse database")
	fs.StringVar(&cfg.Client, "client", "clickhouse", "clickhouse binary path")
	fs.StringVar(&cfg.Format, "format", "PrettyCompact", "output format")
	fs.DurationVar(&cfg.Debounce, "debounce", 75*time.Millisecond, "debounce window")
	fs.DurationVar(&cfg.Suppress, "suppress", 250*time.Millisecond, "suppression window")
	fs.BoolVar(&cfg.PrintEvents, "print-events", false, "print normalized events")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print what would run")
	fs.BoolVar(&cfg.DumpFile, "dump", false, "dump query result to .txt file next to the SQL file")

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
	fs.StringVar(&cfg.Client, "client", "clickhouse", "clickhouse binary path")
	fs.StringVar(&cfg.Format, "format", "PrettyCompact", "output format")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print what would run")
	fs.BoolVar(&cfg.DumpFile, "dump", false, "dump query result to .txt file next to the SQL file")

	reordered, err := reorderRunArgs(args)
	if err != nil {
		return app.RunConfig{}, err
	}
	if err := fs.Parse(reordered); err != nil {
		return app.RunConfig{}, err
	}
	if fs.NArg() != 1 {
		return app.RunConfig{}, errors.New("run requires exactly one SQL file path")
	}
	cfg.Path = fs.Arg(0)
	return cfg, nil
}

func reorderRunArgs(args []string) ([]string, error) {
	flagArgs := make([]string, 0, len(args))
	positional := make([]string, 0, 1)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			flagArgs = append(flagArgs, arg)
		case arg == "--db" || arg == "--client" || arg == "--format":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs value: %s", arg)
			}
			flagArgs = append(flagArgs, arg, args[i+1])
			i++
		case strings.HasPrefix(arg, "--db=") || strings.HasPrefix(arg, "--client=") || strings.HasPrefix(arg, "--format="):
			flagArgs = append(flagArgs, arg)
		case strings.HasPrefix(arg, "-"):
			flagArgs = append(flagArgs, arg)
		default:
			positional = append(positional, arg)
		}
	}

	return append(flagArgs, positional...), nil
}

func writeUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: ch_watch <watch|run> [options]")
}
