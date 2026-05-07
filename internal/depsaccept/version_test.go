package depsaccept

import (
	"path/filepath"
	"testing"
)

func TestBumpPatch(t *testing.T) {
	t.Parallel()

	got, err := bumpPatch("0.7.5")
	if err != nil {
		t.Fatalf("bumpPatch returned error: %v", err)
	}
	if got != "0.7.6" {
		t.Fatalf("bumpPatch() = %q, want %q", got, "0.7.6")
	}
}

func TestBumpPatchWithVPrefix(t *testing.T) {
	t.Parallel()

	got, err := bumpPatch("v1.2.3")
	if err != nil {
		t.Fatalf("bumpPatch returned error: %v", err)
	}
	if got != "1.2.4" {
		t.Fatalf("bumpPatch() = %q, want %q", got, "1.2.4")
	}
}

func TestBumpPatchRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	if _, err := bumpPatch("0.7"); err == nil {
		t.Fatal("bumpPatch returned nil error for invalid version")
	}
}

func TestReadWriteVersion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "VERSION")
	if err := writeVersion(path, "2.3.4"); err != nil {
		t.Fatalf("writeVersion returned error: %v", err)
	}
	got, err := readVersion(path)
	if err != nil {
		t.Fatalf("readVersion returned error: %v", err)
	}
	if got != "2.3.4" {
		t.Fatalf("readVersion() = %q, want %q", got, "2.3.4")
	}
}
