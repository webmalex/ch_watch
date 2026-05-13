package model

import (
	"fmt"
	"time"
)

type FileFingerprint struct {
	Path     string
	Size     int64
	ModTime  time.Time
	Recorded time.Time
}

func (f FileFingerprint) Key() string {
	return f.Path + "|" + f.ModTime.UTC().Format(time.RFC3339Nano) + "|" + fmt.Sprintf("%d", f.Size)
}

type RunRequest struct {
	Path         string
	Database     string
	Client       string
	Format       string
	DryRun       bool
	DumpFile     bool
	DumpTSV      bool
	DumpText     bool
	DumpMarkdown bool
	PipeText     bool
	PipeMarkdown bool
}

type RunResult struct {
	Path      string
	StartedAt time.Time
	Duration  time.Duration
	ExitCode  int
	Err       error
	DryRun    bool
	DumpPath  string
	DumpPaths []string
	Stderr    string
}

func (r RunResult) Success() bool {
	return r.Err == nil
}
