package app

import (
	"io"
	"testing"

	"ch_watch/internal/runner"
)

func TestBuildRunnerLiveModeDoesNotRequireDatabase(t *testing.T) {
	t.Parallel()

	r, err := buildRunner(RunConfig{}, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := r.(runner.ClickHouseRunner); !ok {
		t.Fatalf("unexpected runner type: %T", r)
	}
}

func TestNormalizeRunConfigDefaultsClickHouseBinary(t *testing.T) {
	t.Parallel()

	cfg := normalizeRunConfig(RunConfig{})

	if cfg.Client != "clickhouse" {
		t.Fatalf("unexpected client default: %q", cfg.Client)
	}
	if cfg.Format != "PrettyCompact" {
		t.Fatalf("unexpected format default: %q", cfg.Format)
	}
}
