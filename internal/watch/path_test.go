package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsSQLFile(t *testing.T) {
	t.Parallel()

	if !IsSQLFile("query.sql") {
		t.Fatal("expected .sql file to match")
	}
	if !IsSQLFile("QUERY.SQL") {
		t.Fatal("expected case-insensitive match")
	}
	if IsSQLFile("query.sql.tmp") {
		t.Fatal("did not expect temp file to match")
	}
	if IsSQLFile("query.txt") {
		t.Fatal("did not expect txt file to match")
	}
}

func TestIsWithinRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inside := filepath.Join(root, "a", "query.sql")
	outside := filepath.Join(filepath.Dir(root), "query.sql")

	if !IsWithinRoot(root, inside) {
		t.Fatal("expected inside path to match root")
	}
	if IsWithinRoot(root, outside) {
		t.Fatal("did not expect outside path to match root")
	}
}

func TestSnapshotFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "dev", "query.sql")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("SELECT 1;\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	now := time.Unix(1710000000, 0)
	fingerprint, ok, err := SnapshotFile(root, target, now)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected file to be runnable")
	}
	if fingerprint.Path != target {
		t.Fatalf("unexpected path: %s", fingerprint.Path)
	}
	if fingerprint.Size == 0 {
		t.Fatal("expected non-zero file size")
	}
	if !fingerprint.Recorded.Equal(now) {
		t.Fatalf("unexpected recorded time: %s", fingerprint.Recorded)
	}

	missing := filepath.Join(root, "missing.sql")
	if _, ok, err := SnapshotFile(root, missing, now); err != nil || ok {
		t.Fatalf("expected missing file to be ignored, ok=%v err=%v", ok, err)
	}

	other := filepath.Join(root, "note.txt")
	if err := os.WriteFile(other, []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if _, ok, err := SnapshotFile(root, other, now); err != nil || ok {
		t.Fatalf("expected non-sql file to be ignored, ok=%v err=%v", ok, err)
	}

	outside := filepath.Join(filepath.Dir(root), "outside.sql")
	if err := os.WriteFile(outside, []byte("SELECT 2;\n"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if _, ok, err := SnapshotFile(root, outside, now); err != nil || ok {
		t.Fatalf("expected outside file to be ignored, ok=%v err=%v", ok, err)
	}
}

func TestSnapshotFileNonRegular(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	link := filepath.Join(root, "link.sql")
	if err := os.Symlink("/nonexistent/target", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, ok, err := SnapshotFile(root, link, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected symlink to be skipped (ok=false)")
	}
}

func TestSnapshotFileNonRegularDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "subdir.sql")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, ok, err := SnapshotFile(root, dir, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected directory to be skipped (ok=false)")
	}
}

func TestSnapshotFileNotExist(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	missing := filepath.Join(root, "nope.sql")

	_, ok, err := SnapshotFile(root, missing, time.Now())
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false for missing file")
	}
}

func TestSnapshotFileStatError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	blockFile := filepath.Join(root, "block")
	if err := os.WriteFile(blockFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write block: %v", err)
	}
	blocked := filepath.Join(blockFile, "nested.sql")

	_, ok, err := SnapshotFile(root, blocked, time.Now())
	if err == nil {
		t.Fatal("expected stat error when parent is a file, not a directory")
	}
	if ok {
		t.Fatal("expected ok=false on stat error")
	}
}

func TestIsWithinRootPathEqualsRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if !IsWithinRoot(root, root) {
		t.Fatal("expected root to be within itself")
	}
}

func TestIsWithinRootPathIsParentOfRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := filepath.Dir(root)
	if IsWithinRoot(root, parent) {
		t.Fatal("expected parent of root to be outside root")
	}
}

func TestIsWithinRootPathOutsideViaDotDot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	escaped := filepath.Join(root, "..", "..", "etc", "passwd")
	if IsWithinRoot(root, escaped) {
		t.Fatal("expected path with .. prefix to be outside root")
	}
}
