package app

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotImplemented = errors.New("not implemented yet")

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
	return ErrNotImplemented
}

func RunOnce(context.Context, RunConfig, io.Writer, io.Writer) error {
	return ErrNotImplemented
}
