package match

import (
	"context"
	"log"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Getting up from a cash table for a player who has gone.
//
// A drop-in game plays on past an absent seat (see agents.go): poker checks
// or folds for it, Blackjack stakes the minimum and stands. That is right for
// a lost connection and wrong for a phone left on a train, which would post
// blinds and lose minimum stakes all evening. So at a game that lets players
// get up (module.DeclaresLeave), a seat away for longer than the table's
// leaveAfterAway is cashed out on its player's behalf — the one move the
// runtime ever makes for a person by name, and only the one the rules say.
// The module decides whether it is allowed: a poker tournament refuses it,
// and the seat simply stays sat out, blinded off as a tournament seat is.

// SetLeaveWait overrides every table's wait, for tests. Zero means the option.
func (m *Manager) SetLeaveWait(d time.Duration) { m.leaveWait = d }

// leaveAfter is how long a seat at this table may be away before it is
// cashed out, or zero for never.
func (m *Manager) leaveAfter(match models.Match) time.Duration {
	mod := m.registry.Get(match.ModuleID)
	if !module.DeclaresLeave(mod) {
		return 0
	}
	wait := module.MatchConfig{Variation: match.Variation, Options: match.Options}.LeaveAfterAway()
	if wait > 0 && m.leaveWait > 0 {
		wait = m.leaveWait
	}
	return wait
}

// scheduleLeave wakes checkLeave when a seat's wait is over. One pending
// check per seat.
func (m *Manager) scheduleLeave(matchID, playerID string, at time.Time) {
	key := matchID + "|" + playerID
	m.awayMu.Lock()
	if m.leaveTimers == nil {
		m.leaveTimers = map[string]bool{}
	}
	if m.leaveTimers[key] {
		m.awayMu.Unlock()
		return
	}
	m.leaveTimers[key] = true
	m.awayMu.Unlock()
	time.AfterFunc(time.Until(at)+50*time.Millisecond, func() {
		m.awayMu.Lock()
		delete(m.leaveTimers, key)
		m.awayMu.Unlock()
		m.checkLeave(context.Background(), matchID, playerID)
	})
}

// awayFrom is since when a seat has been away, if it is.
func (m *Manager) awayFrom(matchID, playerID string) (time.Time, bool) {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	since, ok := m.awaySince[matchID+"|"+playerID]
	return since, ok
}

// checkLeave gets a seat up from the table if its player has been away long
// enough, and books another look if not yet.
func (m *Manager) checkLeave(ctx context.Context, matchID, playerID string) {
	match, err := m.current(ctx, matchID)
	if err != nil || match.Status != "active" {
		return
	}
	wait := m.leaveAfter(match)
	if wait == 0 || m.seatHere(matchID, playerID) {
		return
	}
	if p := playerByID(match.Players, playerID); p == nil || p.IsAI {
		return
	}
	since, ok := m.awayFrom(matchID, playerID)
	if !ok {
		return // back since, and gone again: that absence has its own timer
	}
	// Nobody is cashed out while a phone host cannot reach them: see
	// hostpause.go. The clock starts again when it can.
	if m.standInsAreHeld() {
		m.scheduleLeave(matchID, playerID, time.Now().Add(wait))
		return
	}
	if due := since.Add(wait); time.Now().Before(due) {
		m.scheduleLeave(matchID, playerID, due)
		return
	}
	err = m.HandleAction(ctx, matchID, playerID, module.Action{OfferID: module.OfferLeave, Verb: module.VerbLeave})
	switch {
	case err == nil:
		log.Printf("match=%s player=%s away %s, left the table with their chips", matchID, playerID, time.Since(since).Round(time.Second))
	default:
		// A tournament, or a seat already gone: it stays as it is.
		log.Printf("match=%s player=%s away, not cashed out: %v", matchID, playerID, err)
	}
}

// noteAwayToLeave starts the clock on a seat at a table that cashes out absent
// players. Called when a person's connection goes, and by the bot loop for a
// seat it finds sat out, which is how a clock lost to a restart is wound again.
func (m *Manager) noteAwayToLeave(match models.Match, playerID string) {
	wait := m.leaveAfter(match)
	if wait == 0 {
		return
	}
	if p := playerByID(match.Players, playerID); p == nil || p.IsAI {
		return
	}
	id := match.ID.Hex()
	m.markAway(id, playerID, time.Now())
	if since, ok := m.awayFrom(id, playerID); ok {
		m.scheduleLeave(id, playerID, since.Add(wait))
	}
}
