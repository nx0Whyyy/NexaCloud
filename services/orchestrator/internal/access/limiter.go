package access

import (
	"sync"
	"time"
)

type rateWindow struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]rateWindow
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{windows: make(map[string]rateWindow), limit: limit, window: window}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.windows) > 4096 {
		for candidate, entry := range l.windows {
			if entry.reset.Before(now) {
				delete(l.windows, candidate)
			}
		}
	}
	entry := l.windows[key]
	if entry.reset.Before(now) {
		entry = rateWindow{reset: now.Add(l.window)}
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.windows[key] = entry
	return true
}
