package queue

import (
	"testing"
	"time"

	"ch_watch/internal/model"
)

func TestSuppressorIgnoresRecentDuplicateFingerprint(t *testing.T) {
	t.Parallel()

	now := time.Unix(1710000000, 0)
	clock := now
	suppressor := NewSuppressor(50*time.Millisecond, func() time.Time { return clock })
	fingerprint := model.FileFingerprint{Path: "a.sql", Size: 12, ModTime: now}

	if !suppressor.Allow(fingerprint) {
		t.Fatal("expected first fingerprint to pass")
	}
	if suppressor.Allow(fingerprint) {
		t.Fatal("expected duplicate fingerprint to be suppressed")
	}

	clock = clock.Add(60 * time.Millisecond)
	if !suppressor.Allow(fingerprint) {
		t.Fatal("expected old fingerprint to pass after suppression window")
	}

	clock = clock.Add(10 * time.Millisecond)
	fingerprint.ModTime = fingerprint.ModTime.Add(time.Second)
	if !suppressor.Allow(fingerprint) {
		t.Fatal("expected changed fingerprint to pass")
	}
}
