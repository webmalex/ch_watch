package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/webmalex/ch_watch/internal/depsaccept"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ch_watch_maint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	pr := fs.Int("pr", 0, "specific PR number; 0 means latest open Dependabot PR")
	dryRun := fs.Bool("dry-run", false, "print planned actions without merge, commit, push, tag, or release")
	versionFile := fs.String("version-file", "VERSION", "path to VERSION file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}

	return depsaccept.Run(ctx, depsaccept.Config{
		PR:          *pr,
		DryRun:      *dryRun,
		VersionFile: *versionFile,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
}
