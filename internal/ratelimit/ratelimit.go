package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter is a per-user token bucket rate limiter.
type Limiter struct {
	mu       sync.Mutex
	limiters map[string]*entry
	r        rate.Limit
	burst    int
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New creates a Limiter allowing `requests` per `window` per user.
func New(requests int, window time.Duration) *Limiter {
	l := &Limiter{
		limiters: make(map[string]*entry),
		r:        rate.Every(window / time.Duration(requests)),
		burst:    requests,
	}
	go l.cleanup()
	return l
}

// Allow returns true if the user is within their rate limit.
func (l *Limiter) Allow(userID string) bool {
	l.mu.Lock()
	e, ok := l.limiters[userID]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(l.r, l.burst)}
		l.limiters[userID] = e
	}
	e.lastSeen = time.Now()
	l.mu.Unlock()
	return e.limiter.Allow()
}

// cleanup removes idle entries every 5 minutes.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		for id, e := range l.limiters {
			if time.Since(e.lastSeen) > 10*time.Minute {
				delete(l.limiters, id)
			}
		}
		l.mu.Unlock()
	}
}
