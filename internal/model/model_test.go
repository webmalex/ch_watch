package model

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestFileFingerprint_Key(t *testing.T) {
	t.Parallel()

	ts := time.Date(2025, 3, 14, 15, 9, 26, 530000000, time.FixedZone("MSK", 3*3600))

	fp := FileFingerprint{
		Path:    "/sql/query.sql",
		Size:    1024,
		ModTime: ts,
	}

	got := fp.Key()
	utcNano := ts.UTC().Format(time.RFC3339Nano)
	want := "/sql/query.sql|" + utcNano + "|1024"

	if got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}

	if got != fmt.Sprintf("%s|%s|%d", fp.Path, fp.ModTime.UTC().Format(time.RFC3339Nano), fp.Size) {
		t.Errorf("Key() format mismatch: %q", got)
	}
}

func TestFileFingerprint_Key_zeroValues(t *testing.T) {
	t.Parallel()

	fp := FileFingerprint{}
	got := fp.Key()

	if got != "|0001-01-01T00:00:00Z|0" {
		t.Errorf("Key() with zero values = %q, want %q", got, "|0001-01-01T00:00:00Z|0")
	}
}

func TestFileFingerprint_Key_utcConversion(t *testing.T) {
	t.Parallel()

	localTS := time.Date(2025, 6, 1, 12, 0, 0, 0, time.FixedZone("EST", -5*3600))
	fp := FileFingerprint{Path: "a.sql", Size: 7, ModTime: localTS}

	got := fp.Key()
	want := "a.sql|2025-06-01T17:00:00Z|7"

	if got != want {
		t.Errorf("Key() did not convert to UTC: got %q, want %q", got, want)
	}
}

func TestRunResult_Success_true(t *testing.T) {
	t.Parallel()

	r := RunResult{Path: "ok.sql", ExitCode: 0}
	if !r.Success() {
		t.Error("Success() = false with Err==nil, want true")
	}
}

func TestRunResult_Success_false(t *testing.T) {
	t.Parallel()

	r := RunResult{Path: "fail.sql", Err: errors.New("boom")}
	if r.Success() {
		t.Error("Success() = true with Err!=nil, want false")
	}
}

func TestRunResult_Success_nonZeroExitButNoErr(t *testing.T) {
	t.Parallel()

	r := RunResult{ExitCode: 1}
	if !r.Success() {
		t.Error("Success() should return true when Err==nil regardless of ExitCode")
	}
}
