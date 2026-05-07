package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/webmalex/ch_watch/internal/model"
	"github.com/webmalex/ch_watch/internal/queue"
	"github.com/webmalex/ch_watch/internal/testutil"
)

func TestRecursiveWatcherRunsChangedSQLFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := mustWriteSQL(t, root, "dev/query.sql", "SELECT 1;\n")
	h := startWatcherHarness(t, root, testutil.NewFakeRunner(nil))

	mustWriteSQL(t, root, "dev/query.sql", "SELECT 2;\n")
	h.waitRuns(t, 1)
	h.waitIdle(t)

	assertPaths(t, h.runner.Paths(), []string{target})
}

func TestRecursiveWatcherAddsNewDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	h := startWatcherHarness(t, root, testutil.NewFakeRunner(nil))

	newDir := filepath.Join(root, "new", "nested")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir all: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	target := mustWriteSQL(t, root, "new/nested/query.sql", "SELECT 1;\n")
	h.waitRuns(t, 1)
	h.waitIdle(t)

	assertPaths(t, h.runner.Paths(), []string{target})
}

func TestRecursiveWatcherDeduplicatesDuplicateWrites(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := mustWriteSQL(t, root, "dev/query.sql", "SELECT 1;\n")
	h := startWatcherHarness(t, root, testutil.NewFakeRunner(nil))

	mustWriteSQL(t, root, "dev/query.sql", "SELECT 2;\n")
	mustWriteSQL(t, root, "dev/query.sql", "SELECT 3;\n")
	h.waitRuns(t, 1)
	h.waitIdle(t)

	assertPaths(t, h.runner.Paths(), []string{target})
}

func TestRecursiveWatcherQueuesBusyFilesSequentially(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	first := mustWriteSQL(t, root, "dev/first.sql", "SELECT 1;\n")
	second := mustWriteSQL(t, root, "dev/second.sql", "SELECT 2;\n")
	block := make(chan struct{})
	runCount := 0
	runner := testutil.NewFakeRunner(func(req model.RunRequest) model.RunResult {
		runCount++
		if runCount == 1 {
			<-block
		}
		return model.RunResult{Path: req.Path}
	})
	h := startWatcherHarness(t, root, runner)

	mustWriteSQL(t, root, "dev/first.sql", "SELECT 10;\n")
	runner.WaitForStarts(t, 1, 2*time.Second)
	mustWriteSQL(t, root, "dev/second.sql", "SELECT 20;\n")
	close(block)
	h.waitRuns(t, 2)
	h.waitIdle(t)

	assertPaths(t, h.runner.Paths(), []string{first, second})
}

func TestRecursiveWatcherSurvivesRenameNoise(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := mustWriteSQL(t, root, "dev/query.sql", "SELECT 1;\n")
	h := startWatcherHarness(t, root, testutil.NewFakeRunner(nil))

	tmpPath := filepath.Join(root, "dev", "scratch.tmp")
	if err := os.WriteFile(tmpPath, []byte("tmp\n"), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	renamed := filepath.Join(root, "dev", "scratch.bak")
	if err := os.Rename(tmpPath, renamed); err != nil {
		t.Fatalf("rename tmp: %v", err)
	}
	mustWriteSQL(t, root, "dev/query.sql", "SELECT 2;\n")
	h.waitRuns(t, 1)
	h.waitIdle(t)

	select {
	case err := <-h.errCh:
		t.Fatalf("watcher returned error: %v", err)
	default:
	}
	assertPaths(t, h.runner.Paths(), []string{target})
}

type watcherHarness struct {
	controller *queue.Controller
	runner     *testutil.FakeRunner
	errCh      chan error
	cancel     context.CancelFunc
}

func startWatcherHarness(t *testing.T, root string, runner *testutil.FakeRunner) watcherHarness {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	controller := queue.NewController(queue.ControllerConfig{
		Debounce: 20 * time.Millisecond,
		Suppress: 50 * time.Millisecond,
	}, func(path string, now time.Time) (model.FileFingerprint, bool, error) {
		return SnapshotFile(root, path, now)
	}, runner, testutil.NewMemoryReporter())
	go func() { _ = controller.Run(ctx) }()

	watcher, err := NewRecursive(root)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- watcher.Run(ctx, func(event Event) {
			controller.Notify(event.Path)
		})
	}()

	return watcherHarness{controller: controller, runner: runner, errCh: errCh, cancel: cancel}
}

func (h watcherHarness) waitIdle(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.controller.WaitIdle(ctx); err != nil {
		t.Fatalf("wait idle: %v", err)
	}
}

func (h watcherHarness) waitRuns(t *testing.T, want int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if len(h.runner.Paths()) >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d runs, got %v", want, h.runner.Paths())
		case err := <-h.errCh:
			t.Fatalf("watcher returned error: %v", err)
		case <-ticker.C:
		}
	}
}

func mustWriteSQL(t *testing.T, root string, relative string, contents string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestOpNameNoMatch(t *testing.T) {
	t.Parallel()

	got := opName(fsnotify.Op(0))
	if got == "" {
		t.Fatal("expected non-empty fallback string for zero op")
	}
}

func TestOpNameCombined(t *testing.T) {
	t.Parallel()

	got := opName(fsnotify.Create | fsnotify.Write)
	if got != "CREATE+WRITE" {
		t.Fatalf("expected CREATE+WRITE, got %q", got)
	}
}

func TestOpNameAllOps(t *testing.T) {
	t.Parallel()

	got := opName(fsnotify.Create | fsnotify.Write | fsnotify.Chmod | fsnotify.Rename | fsnotify.Remove)
	if got != "CREATE+WRITE+CHMOD+RENAME+REMOVE" {
		t.Fatalf("unexpected combined op name: %q", got)
	}
}

func TestHandleEventNonSQLFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	r, err := NewRecursive(root)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	called := false
	ev := fsnotify.Event{Name: filepath.Join(root, "notes.txt"), Op: fsnotify.Write}
	if err := r.handleEvent(ev, func(Event) { called = true }); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}
	if called {
		t.Fatal("expected no callback for non-SQL file")
	}
}

func TestHandleEventFileOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.sql")
	if err := os.WriteFile(outside, []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	r, err := NewRecursive(root)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	called := false
	ev := fsnotify.Event{Name: outside, Op: fsnotify.Write}
	if err := r.handleEvent(ev, func(Event) { called = true }); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}
	if called {
		t.Fatal("expected no callback for file outside root")
	}
}

func TestHandleEventRemoveDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	subDir := filepath.Join(root, "dev")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	r, err := NewRecursive(root)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	subDirClean := NormalizePath(subDir)
	r.mu.Lock()
	if _, ok := r.paths[subDirClean]; !ok {
		r.mu.Unlock()
		t.Fatalf("expected %q in paths before remove", subDirClean)
	}
	before := len(r.paths)
	r.mu.Unlock()

	called := false
	ev := fsnotify.Event{Name: subDir, Op: fsnotify.Remove}
	if err := r.handleEvent(ev, func(Event) { called = true }); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}
	if called {
		t.Fatal("expected no callback for Remove op on directory")
	}

	r.mu.Lock()
	after := len(r.paths)
	r.mu.Unlock()
	if after >= before {
		t.Fatalf("expected path removed from watcher.paths, before=%d after=%d", before, after)
	}
}

func assertPaths(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected paths length: got=%v want=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected path order: got=%v want=%v", got, want)
		}
	}
}
