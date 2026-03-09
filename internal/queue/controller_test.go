package queue

import (
	"context"
	"testing"
	"time"

	"ch_watch/internal/model"
	"ch_watch/internal/testutil"
)

func TestControllerDeduplicatesWithinBatch(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := testutil.NewFakeRunner(nil)
	controller := NewController(ControllerConfig{
		Debounce: 10 * time.Millisecond,
		Suppress: 50 * time.Millisecond,
	}, snapshotFromMap(map[string]model.FileFingerprint{
		"a.sql": {Path: "a.sql", Size: 1, ModTime: time.Unix(1, 0)},
	}), runner, testutil.NewMemoryReporter())

	go func() { _ = controller.Run(ctx) }()
	controller.Notify("a.sql")
	controller.Notify("a.sql")

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := controller.WaitIdle(waitCtx); err != nil {
		t.Fatalf("wait idle: %v", err)
	}

	runs := runner.Paths()
	if len(runs) != 1 || runs[0] != "a.sql" {
		t.Fatalf("unexpected runs: %#v", runs)
	}
}

func TestControllerQueuesFilesInFirstSeenOrderWhileBusy(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := make(chan struct{})
	runCount := 0
	runner := testutil.NewFakeRunner(func(req model.RunRequest) model.RunResult {
		runCount++
		if req.Path == "a.sql" && runCount == 1 {
			<-block
		}
		return model.RunResult{Path: req.Path}
	})
	controller := NewController(ControllerConfig{
		Debounce: 10 * time.Millisecond,
		Suppress: 20 * time.Millisecond,
	}, snapshotFromMap(map[string]model.FileFingerprint{
		"a.sql": {Path: "a.sql", Size: 1, ModTime: time.Unix(1, 0)},
		"b.sql": {Path: "b.sql", Size: 1, ModTime: time.Unix(2, 0)},
		"c.sql": {Path: "c.sql", Size: 1, ModTime: time.Unix(3, 0)},
	}), runner, testutil.NewMemoryReporter())

	go func() { _ = controller.Run(ctx) }()
	controller.Notify("a.sql")
	runner.WaitForStarts(t, 1, 2*time.Second)
	controller.Notify("b.sql")
	controller.Notify("c.sql")
	close(block)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := controller.WaitIdle(waitCtx); err != nil {
		t.Fatalf("wait idle: %v", err)
	}

	expected := []string{"a.sql", "b.sql", "c.sql"}
	if got := runner.Paths(); !equalStrings(got, expected) {
		t.Fatalf("unexpected run order: got=%v want=%v", got, expected)
	}
}

func TestControllerQueuesSingleRerunForRepeatedBusyFile(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	block := make(chan struct{})
	snapshot := versionedSnapshot()
	runCount := 0
	runner := testutil.NewFakeRunner(func(req model.RunRequest) model.RunResult {
		runCount++
		if req.Path == "a.sql" && runCount == 1 {
			<-block
		}
		return model.RunResult{Path: req.Path}
	})
	controller := NewController(ControllerConfig{
		Debounce: 10 * time.Millisecond,
		Suppress: 20 * time.Millisecond,
	}, snapshot, runner, testutil.NewMemoryReporter())

	go func() { _ = controller.Run(ctx) }()
	controller.Notify("a.sql")
	runner.WaitForStarts(t, 1, 2*time.Second)
	controller.Notify("a.sql")
	controller.Notify("a.sql")
	close(block)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if err := controller.WaitIdle(waitCtx); err != nil {
		t.Fatalf("wait idle: %v", err)
	}

	expected := []string{"a.sql", "a.sql"}
	if got := runner.Paths(); !equalStrings(got, expected) {
		t.Fatalf("unexpected rerun order: got=%v want=%v", got, expected)
	}
}

func snapshotFromMap(values map[string]model.FileFingerprint) SnapshotFunc {
	return func(path string, now time.Time) (model.FileFingerprint, bool, error) {
		fingerprint, ok := values[path]
		if !ok {
			return model.FileFingerprint{}, false, nil
		}
		fingerprint.Recorded = now
		return fingerprint, true, nil
	}
}

func versionedSnapshot() SnapshotFunc {
	versions := map[string]int{}
	return func(path string, now time.Time) (model.FileFingerprint, bool, error) {
		versions[path]++
		version := versions[path]
		return model.FileFingerprint{
			Path:     path,
			Size:     int64(version),
			ModTime:  now.Add(time.Duration(version) * time.Second),
			Recorded: now,
		}, true, nil
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
