package feedback

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// anonLimiter throttles reports that arrive with no session at all.
//
// It exists because the database-backed throttle cannot cover them: that one
// counts a reporter's recent reports by account or guest id, and an anonymous
// report has neither. Without this, the *open* path — the one anybody can
// reach — was the only unthrottled one, which is exactly backwards.
//
// In memory, and therefore per-process, deliberately. The alternative is
// storing something derived from the caller's IP address on every report,
// which means keeping a record of where players are for the sake of a counter.
// Per-process is a weaker ceiling but costs no privacy, and the thing being
// defended against — a stuck client retrying in a loop, or casual abuse — is
// stopped by either.
type anonLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	window time.Duration
	limit  int
}

// maxTrackedKeys bounds the map so a spray of forged X-Forwarded-For values
// cannot grow it without limit. Past this, the oldest-quiet keys are dropped;
// dropping one only forgives its history, which is the safe direction to fail.
const maxTrackedKeys = 10000

func newAnonLimiter(window time.Duration, limit int) *anonLimiter {
	return &anonLimiter{hits: map[string][]time.Time{}, window: window, limit: limit}
}

// allow records an attempt and reports whether it is under the ceiling.
func (l *anonLimiter) allow(key string, now time.Time) bool {
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

// sweepLocked drops keys whose every hit has aged out. The caller holds the
// lock.
func (l *anonLimiter) sweepLocked(cutoff time.Time) {
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

// clientIP identifies the caller as well as it can.
//
// X-Forwarded-For is trusted because behind a proxy it is the only thing that
// distinguishes callers at all — but it is client-supplied and so forgeable,
// which means this throttle slows an honest client down and merely
// inconveniences a determined one. That is the intended scope: it is a guard
// against accidental floods, not a security boundary. The message cap, the
// per-account throttle and the console's delete are what carry the rest.
func clientIP(req *http.Request) string {
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
