// Package ratelimit is a small in-memory ceiling on how often something may
// be done, keyed by whatever the caller can identify the doer by.
//
// In memory, and therefore per-process, deliberately. The alternative — a
// counter in the database — would mean writing a record of who did what and
// when for the sake of a throttle, and the things this guards (a stuck client
// retrying in a loop, someone guessing a password) are stopped by a
// per-process ceiling just as well. It is not a defence against a distributed
// attacker, and nothing here should be the only thing standing between an
// attacker and anything.
package ratelimit

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxTrackedKeys bounds the map so a spray of forged keys cannot grow it
// without limit. Past this, keys whose history has entirely aged out are
// dropped; dropping one only forgives its history, which is the safe
// direction to fail.
const maxTrackedKeys = 10000

// Limiter allows at most Limit attempts per key within Window.
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	window time.Duration
	limit  int
}

func New(window time.Duration, limit int) *Limiter {
	return &Limiter{hits: map[string][]time.Time{}, window: window, limit: limit}
}

// Allow records an attempt and reports whether it is under the ceiling.
func (l *Limiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, at := range l.hits[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)

	if len(l.hits) > maxTrackedKeys {
		l.sweepLocked(cutoff)
	}
	return true
}

// Reset forgets a key's history — what a successful sign-in calls, so that a
// few wrong guesses before the right one do not count against the next
// attempt.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

// sweepLocked drops keys whose every attempt has aged out. The caller holds
// the lock.
func (l *Limiter) sweepLocked(cutoff time.Time) {
	for key, times := range l.hits {
		stale := true
		for _, at := range times {
			if at.After(cutoff) {
				stale = false
				break
			}
		}
		if stale {
			delete(l.hits, key)
		}
	}
}

// ClientIP identifies the caller as well as it can.
//
// X-Forwarded-For is trusted because behind a proxy it is the only thing that
// distinguishes callers at all — but it is client-supplied and so forgeable,
// which means a ceiling keyed on this slows an honest client and merely
// inconveniences a determined one. Keep that in mind when choosing what to
// hang off it: it is a guard against floods and casual guessing, never a
// security boundary on its own.
func ClientIP(req *http.Request) string {
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		first, _, ok := strings.Cut(fwd, ",")
		if !ok {
			first = fwd
		}
		return strings.TrimSpace(first)
	}
	host, _, ok := strings.Cut(req.RemoteAddr, ":")
	if !ok {
		return req.RemoteAddr
	}
	return host
}
