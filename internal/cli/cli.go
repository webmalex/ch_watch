package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/webmalex/ch_watch/internal/app"
	"github.com/webmalex/ch_watch/internal/version"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		writeHelp(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "watch":
		if isHelp(args[1:]) {
			fs, _ := newWatchFlags()
			writeCommandHelp(stderr, "watch", "watch a directory and rerun SQL on changes", fs)
			return nil
		}
		cfg, err := parseWatch(args[1:])
		if err != nil {
			return err
		}
		return app.RunWatch(ctx, cfg, stdout, stderr)
	case "run":
		if isHelp(args[1:]) {
			fs, _ := newRunFlags()
			writeCommandHelp(stderr, "run", "execute SQL file(s); pass a directory to run all .sql files recursively", fs)
			return nil
		}
		cfg, err := parseRun(args[1:])
		if err != nil {
			return err
		}
		return app.RunOnce(ctx, cfg, stdout, stderr)
	case "version", "-v", "--version":
		_, _ = fmt.Fprintf(stdout, "ch_watch %s\n", version.Current())
		return nil
	case "help", "-h", "--help":
		writeHelp(stderr)
		return nil
	default:
		writeHelp(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func newWatchFlags() (*flag.FlagSet, *app.WatchConfig) {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := &app.WatchConfig{}
	fs.StringVar(&cfg.Root, "root", "./ch", "watch root directory (default: ./ch; positional arg takes precedence)")
	fs.StringVar(&cfg.Database, "db", "", "ClickHouse database (client mode); env: CH_DB")
	fs.StringVar(&cfg.Client, "client", "clickhouse", "clickhouse binary path")
	fs.StringVar(&cfg.Format, "format", "PrettyCompact", "output format")
	fs.DurationVar(&cfg.Debounce, "debounce", 75*time.Millisecond, "debounce window")
	fs.DurationVar(&cfg.Suppress, "suppress", 250*time.Millisecond, "suppression window")
	fs.BoolVar(&cfg.PrintEvents, "print-events", false, "print normalized events")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print what would run")
	fs.BoolVar(&cfg.DumpFile, "dump", false, "dump query result directly to file in --format (default PrettyCompact .txt)")
	fs.BoolVar(&cfg.DumpText, "dump-txt", false, "shorthand for --dump with PrettyCompact .txt")
	fs.BoolVar(&cfg.DumpMarkdown, "dump-md", false, "shorthand for --dump with Markdown .md")
	fs.BoolVar(&cfg.PipeText, "pipe-txt", false, "TSV pipeline: render PrettyCompact .txt from canonical .tsv")
	fs.BoolVar(&cfg.PipeMarkdown, "pipe-md", false, "TSV pipeline: render Markdown .md from canonical .tsv")
	return fs, cfg
}

func parseWatch(args []string) (app.WatchConfig, error) {
	fs, cfg := newWatchFlags()

	if err := fs.Parse(args); err != nil {
		return app.WatchConfig{}, err
	}
	if fs.NArg() > 1 {
		return app.WatchConfig{}, errors.New("watch accepts at most one positional argument (root directory)")
	}
	if fs.NArg() == 1 {
		cfg.Root = fs.Arg(0)
	}
	return *cfg, nil
}

func newRunFlags() (*flag.FlagSet, *app.RunConfig) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	cfg := &app.RunConfig{}
	fs.StringVar(&cfg.Database, "db", "", "ClickHouse database (client mode); env: CH_DB")
	fs.StringVar(&cfg.Client, "client", "clickhouse", "clickhouse binary path")
	fs.StringVar(&cfg.Format, "format", "PrettyCompact", "output format")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "print what would run")
	fs.BoolVar(&cfg.DumpFile, "dump", false, "dump query result directly to file in --format (default PrettyCompact .txt)")
	fs.BoolVar(&cfg.DumpText, "dump-txt", false, "shorthand for --dump with PrettyCompact .txt")
	fs.BoolVar(&cfg.DumpMarkdown, "dump-md", false, "shorthand for --dump with Markdown .md")
	fs.BoolVar(&cfg.PipeText, "pipe-txt", false, "TSV pipeline: render PrettyCompact .txt from canonical .tsv")
	fs.BoolVar(&cfg.PipeMarkdown, "pipe-md", false, "TSV pipeline: render Markdown .md from canonical .tsv")
	return fs, cfg
}

func parseRun(args []string) (app.RunConfig, error) {
	fs, cfg := newRunFlags()

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
	return *cfg, nil
}

func reorderRunArgs(args []string) ([]string, error) {
	flagArgs := make([]string, 0, len(args))
	positional := make([]string, 0, 1)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run" || arg == "--dump" || arg == "--dump-txt" || arg == "--dump-md" || arg == "--pipe-txt" || arg == "--pipe-md":
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

func isHelp(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return args[0] == "-h" || args[0] == "--help" || args[0] == "help"
}

func writeHelp(w io.Writer) {
	_, _ = fmt.Fprintf(w, "ch_watch %s\n\n", version.Current())
	_, _ = fmt.Fprintln(w, "SQL file watcher for ClickHouse debug workflows.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  watch    watch a directory and rerun SQL on changes")
	_, _ = fmt.Fprintln(w, "  run      execute SQL file(s)")
	_, _ = fmt.Fprintln(w, "  version  print version")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Global flags:")
	_, _ = fmt.Fprintln(w, "  -h, --help       show help")
	_, _ = fmt.Fprintln(w, "  -v, --version    print version")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Use \"ch_watch <command> --help\" for more information about a command.")
}

func writeCommandHelp(w io.Writer, name string, desc string, fs *flag.FlagSet) {
	_, _ = fmt.Fprintf(w, "Usage: ch_watch %s [options]", name)
	switch name {
	case "run":
		_, _ = fmt.Fprint(w, " <path>")
	case "watch":
		_, _ = fmt.Fprint(w, " [root]")
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "\n%s\n\n", desc)
	_, _ = fmt.Fprintln(w, "Options:")
	fs.SetOutput(w)
	fs.PrintDefaults()
}
