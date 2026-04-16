package alertmanager

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu        sync.Mutex
	window    time.Duration
	maxEvents int
	now       func() time.Time
	events    []time.Time
}

func newRateLimiter(window time.Duration, maxEvents int) *rateLimiter {
	return &rateLimiter{
		window:    window,
		maxEvents: maxEvents,
		now:       time.Now,
		events:    make([]time.Time, 0, maxEvents),
	}
}

func (l *rateLimiter) Allow() bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	kept := l.events[:0]
	for _, ts := range l.events {
		if !ts.Before(cutoff) {
			kept = append(kept, ts)
		}
	}
	l.events = kept

	if len(l.events) >= l.maxEvents {
		return false
	}

	l.events = append(l.events, now)
	return true
}
