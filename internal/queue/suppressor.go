package queue

import (
	"time"

	"github.com/webmalex/ch_watch/internal/model"
)

type Suppressor struct {
	window time.Duration
	now    func() time.Time
	recent map[string]time.Time
}

func NewSuppressor(window time.Duration, now func() time.Time) *Suppressor {
	if now == nil {
		now = time.Now
	}
	return &Suppressor{
		window: window,
		now:    now,
		recent: make(map[string]time.Time),
	}
}

func (s *Suppressor) Allow(fingerprint model.FileFingerprint) bool {
	now := s.now()
	s.prune(now)
	key := fingerprint.Key()
	if seenAt, ok := s.recent[key]; ok && now.Sub(seenAt) <= s.window {
		return false
	}
	s.recent[key] = now
	return true
}

func (s *Suppressor) prune(now time.Time) {
	for key, seenAt := range s.recent {
		if now.Sub(seenAt) > s.window {
			delete(s.recent, key)
		}
	}
}
