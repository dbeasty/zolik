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
	"net"
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
// Behind our nginx (deploy/nginx/zolik-proxy.conf), X-Forwarded-For is set to
// $remote_addr — one entry, the peer nginx saw — replacing whatever the client
// sent. The rightmost entry is read rather than the leftmost all the same: in
// any chain a proxy appends to, the rightmost entry is the one the last proxy
// added, and every entry left of it is the client's own word. Reading the
// leftmost would let a caller pick a fresh key per request by sending a random
// header, and so walk straight past every limit keyed on this.
//
// Without the header — a direct connection, or a test — the TCP peer is used.
// It is still a guard against floods and casual guessing, never a security
// boundary on its own.
func ClientIP(req *http.Request) string {
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.LastIndexByte(fwd, ','); i >= 0 {
			fwd = fwd[i+1:]
		}
		if ip := strings.TrimSpace(fwd); ip != "" {
			return ip
		}
	}
	return remoteHost(req.RemoteAddr)
}

// remoteHost strips the port from a RemoteAddr. net.SplitHostPort, not a cut
// at the first colon, which would turn "[2001:db8::1]:443" into "[2001".
func remoteHost(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
