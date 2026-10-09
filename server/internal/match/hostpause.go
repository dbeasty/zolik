package match

import (
	"context"
	"strings"
	"time"
)

// When the server itself was away.
//
// On a phone the server is the phone. When it is put away — the app sent to
// the background, the screen locked — it stops, and every player at the table
// is "away" from it for as long as that lasts. When it comes back, the clocks
// that measure a player's absence must not have been running against the
// guests for the host's absence: a bot standing in, a seat cashed out or a
// table abandoned the moment the host returns would punish everybody else for
// the host's lock screen. And while a hybrid table's link to its remote guests
// is down, they cannot be here however much they want to be, so no bot takes
// a seat on that account.

// HostResumed is the server coming back from having been away. Every away
// clock starts again from now, and the reaper gives every table a full window
// before it abandons anything.
func (m *Manager) HostResumed() {
	now := time.Now()
	m.awayMu.Lock()
	m.resumedAt = now
	for k := range m.awaySince {
		m.awaySince[k] = now
	}
	m.awayMu.Unlock()
	m.recheckStandIns()
}

// HoldStandIns stops stand-ins starting while on, and on release gives every
// away clock a fresh start and looks again. For a phone whose relay to remote
// guests is down.
func (m *Manager) HoldStandIns(on bool) {
	m.awayMu.Lock()
	was := m.standInsHeld
	m.standInsHeld = on
	m.awayMu.Unlock()
	if was && !on {
		m.HostResumed()
	}
}

// standInsAreHeld reports HoldStandIns.
func (m *Manager) standInsAreHeld() bool {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	return m.standInsHeld
}

// justResumed reports whether the server came back recently enough that a
// table abandoned now would be abandoned for the server's absence.
func (m *Manager) justResumed(now time.Time, window time.Duration) bool {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	return !m.resumedAt.IsZero() && now.Sub(m.resumedAt) < window
}

// recheckStandIns looks again at every table with somebody away.
func (m *Manager) recheckStandIns() {
	m.awayMu.Lock()
	ids := map[string]bool{}
	for k := range m.awaySince {
		if id, _, ok := strings.Cut(k, "|"); ok {
			ids[id] = true
		}
	}
	m.awayMu.Unlock()
	for id := range ids {
		go m.checkStandIns(context.Background(), id)
	}
}
