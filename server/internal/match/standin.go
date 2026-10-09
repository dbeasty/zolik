package match

import (
	"context"
	"log"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Stand-in bots: a bot plays a person's seat while they are away.
//
// A game that is not drop-in pauses when the seat it is waiting on drops (see
// presence.go). That is still the first thing that happens — the wait — and
// the table's standInAfter option is how long it lasts. Once it is over a bot
// plays the seat, with the module's own bot at the table's strength, until its
// player is heard from again. The seat stays theirs throughout: IsAI is never
// set, and the name, account and avatar never change. Their return ends the
// stand-in at once, and the next decision is theirs.
//
// Three rules keep this honest:
//
//   - Bots never play to an empty room. A stand-in starts only while somebody
//     else is at the table, and plays only while somebody is; when the room
//     empties the table stops, and the reaper treats it exactly as before.
//   - The host decides last. They can put a bot in at once, or take it out to
//     wait for somebody, and a seat they are waiting for is not stood in for
//     again until its player has been back.
//   - The result says so. A seat a bot played for is recorded in StoodIn, and
//     its player gets no win or loss for that match (see stats.ApplyMatch) —
//     otherwise dropping on purpose would let a Hard bot win for you.

// errStandInEnded refuses a stand-in's move that lost the race with its
// player's return. Never shown to anybody: the bot loop moves on to the next
// seat, and the player makes the decision themselves.
var errStandInEnded = module.Error{Code: "STAND_IN_ENDED"}

// SetStandInWait overrides every table's wait, for tests. Zero means the option.
func (m *Manager) SetStandInWait(d time.Duration) { m.standInWait = d }

// standInAfter is how long this table waits before a stand-in, or zero for
// never: the game does not offer one, it is a one-seat game, or the lobby
// chose to wait.
func (m *Manager) standInAfter(match models.Match) time.Duration {
	mod := m.registry.Get(match.ModuleID)
	if !module.DeclaresStandIn(mod) || m.savedGame(match) {
		return 0
	}
	wait := module.MatchConfig{Variation: match.Variation, Options: match.Options}.StandInAfter()
	if wait > 0 && m.standInWait > 0 {
		wait = m.standInWait
	}
	return wait
}

// standInPlaying reports whether the server plays this seat for its player
// right now: a stand-in is in, its player is away, and somebody else is still
// at the table to play for.
func (m *Manager) standInPlaying(match models.Match, p models.Player) bool {
	return p.StandIn != nil && !m.seatHere(match.ID.Hex(), p.ID) && m.someoneElseHere(match, p.ID)
}

// someoneElseHere reports whether a person other than this seat's is at the
// table — playing their own seat, not stood in for.
func (m *Manager) someoneElseHere(match models.Match, except string) bool {
	room := match.ID.Hex()
	for _, p := range match.Players {
		if p.ID == except || p.IsAI || p.StandIn != nil {
			continue
		}
		if m.seatHere(room, p.ID) {
			return true
		}
	}
	return false
}

// markAway notes when a seat at a stand-in table went away, if that is not
// already known. returned clears it.
func (m *Manager) markAway(matchID, playerID string, at time.Time) {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	if m.awaySince == nil {
		m.awaySince = map[string]time.Time{}
	}
	key := matchID + "|" + playerID
	if _, ok := m.awaySince[key]; !ok {
		m.awaySince[key] = at
	}
}

// standInDue is when a stand-in takes this seat over, if one can: the table
// offers stand-ins, the seat is a person's, nobody is playing it yet, its
// player is away, and the host is not holding it for them.
//
// When the absence started is not known — the process restarted, or the seat
// never connected — it starts now, which only ever errs towards waiting.
func (m *Manager) standInDue(match models.Match, p models.Player) (time.Time, bool) {
	wait := m.standInAfter(match)
	if wait == 0 || p.IsAI || p.StandIn != nil {
		return time.Time{}, false
	}
	if match.Status != "active" && match.Status != "suspended" {
		return time.Time{}, false
	}
	room := match.ID.Hex()
	if m.seatHere(room, p.ID) {
		return time.Time{}, false
	}
	key := room + "|" + p.ID
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	if m.standInHeld[key] {
		return time.Time{}, false
	}
	since, ok := m.awaySince[key]
	if !ok {
		since = time.Now()
		if match.SuspendedPlayer == p.ID && match.SuspendedAt != nil {
			since = *match.SuspendedAt
		}
		if m.awaySince == nil {
			m.awaySince = map[string]time.Time{}
		}
		m.awaySince[key] = since
	}
	return since.Add(wait), true
}

// scheduleStandIn wakes checkStandIns when a seat's wait is over, because
// nothing else will: a seat staying away writes nothing for anyone to react
// to. One pending check per seat.
func (m *Manager) scheduleStandIn(matchID, playerID string, at time.Time) {
	key := matchID + "|" + playerID
	m.awayMu.Lock()
	if m.standInTimers == nil {
		m.standInTimers = map[string]bool{}
	}
	if m.standInTimers[key] {
		m.awayMu.Unlock()
		return
	}
	m.standInTimers[key] = true
	m.awayMu.Unlock()
	time.AfterFunc(time.Until(at)+50*time.Millisecond, func() {
		m.awayMu.Lock()
		delete(m.standInTimers, key)
		m.awayMu.Unlock()
		m.checkStandIns(context.Background(), matchID)
	})
}

// checkStandIns puts a stand-in into every seat the table is waiting on whose
// wait is over, un-pauses the table if it was paused for one of them, and
// starts the bots.
func (m *Manager) checkStandIns(ctx context.Context, matchID string) {
	// Held while a phone host cannot reach its remote guests; looked at
	// again when it can (HoldStandIns).
	if m.standInsAreHeld() {
		return
	}
	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return
	}
	match := e.match
	mod := m.registry.Get(match.ModuleID)
	if mod == nil || (match.Status != "active" && match.Status != "suspended") || e.stateErr != nil {
		e.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	skill := standInSkill(match)
	changed := false
	for _, id := range module.AwaitedSeats(mod, module.State(match.State), viewerFor(match), refsOf(match)) {
		p := playerByID(match.Players, id)
		if p == nil {
			continue
		}
		due, ok := m.standInDue(match, *p)
		if !ok {
			continue
		}
		if now.Before(due) {
			m.scheduleStandIn(matchID, id, due)
			continue
		}
		// Nobody left to play for. The table stays as it is, and the reaper
		// decides about it as it always has.
		if !m.someoneElseHere(match, id) {
			continue
		}
		match = withStandIn(match, id, &models.StandIn{Since: now, Skill: string(skill), By: models.StandInTimeout})
		changed = true
	}
	if !changed {
		e.mu.Unlock()
		return
	}
	if match.Status == "suspended" {
		if p := playerByID(match.Players, match.SuspendedPlayer); p == nil || p.StandIn != nil {
			match = unsuspended(match)
		}
	}
	err = m.saveLocked(ctx, e, match)
	saved := e.match
	e.mu.Unlock()
	if err != nil {
		log.Printf("match=%s stand-in failed: %v", matchID, err)
		return
	}
	log.Printf("match=%s stand-in bots playing for away seats", matchID)
	m.Broadcast(saved)
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), matchID)
}

// endStandIn gives a seat back to its player, who is here again.
func (m *Manager) endStandIn(ctx context.Context, matchID, playerID string) {
	m.awayMu.Lock()
	delete(m.standInHeld, matchID+"|"+playerID)
	m.awayMu.Unlock()

	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return
	}
	if p := playerByID(e.match.Players, playerID); p == nil || p.StandIn == nil {
		e.mu.Unlock()
		return
	}
	err = m.saveLocked(ctx, e, withStandIn(e.match, playerID, nil))
	saved := e.match
	e.mu.Unlock()
	if err != nil {
		log.Printf("match=%s player=%s ending stand-in failed: %v", matchID, playerID, err)
		return
	}
	// The bot's turn, if it was mid-way through one, is not the player's: what
	// they do next starts afresh.
	m.turns.forgetSeat(matchID, playerID)
	log.Printf("match=%s player=%s back, stand-in ended", matchID, playerID)
	// Said to them once, so the board they come back to — a hand a bot has
	// been playing — is not a surprise.
	if m.hub != nil {
		m.hub.WriteDirect(matchID, playerID, map[string]any{"type": "stand_in_ended"})
	}
	m.Broadcast(saved)
}

// SetStandIn is the host putting a bot into an away seat, changing how well it
// plays, or taking it out to wait for its player.
func (m *Manager) SetStandIn(ctx context.Context, matchID, callerID, seatID string, on bool, skill string) (models.Match, error) {
	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return models.Match{}, module.Error{Code: "MATCH_NOT_FOUND", Message: matchID}
	}
	match := e.match
	if playerByID(match.Players, callerID) == nil {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "NOT_AT_THIS_TABLE", Message: callerID}
	}
	if match.HostID != callerID {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "NOT_THE_HOST", Message: callerID}
	}
	target := playerByID(match.Players, seatID)
	if target == nil || target.IsAI {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "STAND_IN_NOT_A_PERSON", Message: seatID}
	}
	if !module.DeclaresStandIn(m.registry.Get(match.ModuleID)) || m.savedGame(match) {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "STAND_IN_NOT_OFFERED", Message: match.ModuleID}
	}
	if match.Status != "active" && match.Status != "suspended" {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "MATCH_NOT_ACTIVE"}
	}
	key := match.ID.Hex() + "|" + seatID

	if !on {
		m.awayMu.Lock()
		if m.standInHeld == nil {
			m.standInHeld = map[string]bool{}
		}
		m.standInHeld[key] = true
		m.awayMu.Unlock()
		if target.StandIn == nil {
			e.mu.Unlock()
			return match, nil
		}
		if err := m.saveLocked(ctx, e, withStandIn(match, seatID, nil)); err != nil {
			e.mu.Unlock()
			return models.Match{}, module.Error{Code: "MATCH_MOVED_ON", Message: err.Error()}
		}
		m.turns.forgetSeat(match.ID.Hex(), seatID)
		// Waiting for them is a pause like any other, if it is them the table
		// is waiting on.
		if !m.seatHere(match.ID.Hex(), seatID) {
			m.suspendLocked(ctx, e, seatID)
		}
		saved := e.match
		e.mu.Unlock()
		m.Broadcast(saved)
		return saved, nil
	}

	if m.seatHere(match.ID.Hex(), seatID) {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "STAND_IN_PLAYER_HERE", Message: seatID}
	}
	want := standInSkill(match)
	if skill != "" {
		s, auto := module.ParseSkill(skill)
		if auto {
			e.mu.Unlock()
			return models.Match{}, module.Error{Code: "UNKNOWN_SKILL", Message: skill}
		}
		want = s
	}
	since, by := time.Now().UTC(), models.StandInHost
	if target.StandIn != nil {
		since, by = target.StandIn.Since, target.StandIn.By
	}
	m.awayMu.Lock()
	delete(m.standInHeld, key)
	m.awayMu.Unlock()
	match = withStandIn(match, seatID, &models.StandIn{Since: since, Skill: string(want), By: by})
	if match.Status == "suspended" && match.SuspendedPlayer == seatID {
		match = unsuspended(match)
	}
	if err := m.saveLocked(ctx, e, match); err != nil {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "MATCH_MOVED_ON", Message: err.Error()}
	}
	// A new strength is a new bot, from the next decision on.
	m.turns.forgetSeat(match.ID.Hex(), seatID)
	saved := e.match
	e.mu.Unlock()
	log.Printf("match=%s host=%s stand-in for %s at %s", matchID, callerID, seatID, want)
	m.Broadcast(saved)
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), saved.ID.Hex())
	return saved, nil
}

// withStandIn returns match with seatID's stand-in set to si, or cleared when
// si is nil, and the seat noted in StoodIn the first time one starts. The
// player slice is copied, never written through: it is shared with the match
// the live entry holds.
func withStandIn(match models.Match, seatID string, si *models.StandIn) models.Match {
	players := append([]models.Player(nil), match.Players...)
	for i := range players {
		if players[i].ID == seatID {
			players[i].StandIn = si
		}
	}
	match.Players = players
	if si != nil && !contains(match.StoodIn, seatID) {
		match.StoodIn = append(append([]string(nil), match.StoodIn...), seatID)
	}
	return match
}

// unsuspended is match playing again, with the pause it was in forgotten.
func unsuspended(match models.Match) models.Match {
	match.Status = "active"
	match.SuspendedAt = nil
	match.AbandonAt = nil
	match.SuspendedPlayer = ""
	return match
}

// standInSkill is how well a stand-in plays: as well as the strongest bot the
// table was dealt with, so a person's seat is not handed to something weaker
// than the opponents they chose — and Medium at a table with no bots.
func standInSkill(match models.Match) module.Skill {
	best := module.Skill("")
	for _, p := range match.Players {
		if !p.IsAI {
			continue
		}
		s, auto := module.ParseSkill(p.AIDifficulty)
		if auto {
			continue
		}
		if best == "" || s.Rank() > best.Rank() {
			best = s
		}
	}
	if best == "" {
		return module.SkillMedium
	}
	return best
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
