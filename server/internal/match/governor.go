package match

import (
	"context"
	"sync"
	"time"

	"zolik/server/internal/botgov"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Which engine plays a bot seat, and keeping that choice still for a turn.
//
// Two things can change the bot behind a seat while a match is in play: the
// operator's switch that puts a game's Hard seats on its trained model
// (learn.HardModel, read on every call to Bot()), and the governor
// (internal/botgov) moving a seat to a cheaper class when CPU runs short.
// Either landing in the middle of a multi-action turn would hand half a plan —
// a meld begun by the model — to a bot that never made it. So both are read
// only when a seat starts a turn, and the bot chosen then plays the turn out.

// SetGovernor attaches the bot governor. Optional: without one every seat
// plays what it asked for, exactly as before.
func (m *Manager) SetGovernor(g *botgov.Governor) { m.governor = g }

// Governor is the governor SetGovernor attached, or nil.
func (m *Manager) Governor() *botgov.Governor { return m.governor }

type seatRef struct{ match, seat string }

type turnBot struct {
	bot    module.Bot
	seat   module.BotSeat
	engine string
	// want is what the seat asked for, which the governor keys its lease
	// by — not what it was given, which may be the class below.
	want botgov.Class
	seen time.Time
}

// turnTracker remembers who moved last in each match and the bot each seat
// started its current turn with.
type turnTracker struct {
	mu    sync.Mutex
	moved map[string]string
	bots  map[seatRef]turnBot
}

func (t *turnTracker) noteMove(matchID, playerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.moved == nil {
		t.moved = map[string]string{}
	}
	t.moved[matchID] = playerID
}

// midTurn reports whether a seat made its match's last move — so the move it
// is about to make continues that turn — and the bot it started it with.
func (t *turnTracker) midTurn(matchID, seatID string) (turnBot, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.moved[matchID] != seatID {
		return turnBot{}, false
	}
	b, ok := t.bots[seatRef{matchID, seatID}]
	return b, ok
}

func (t *turnTracker) remember(matchID, seatID string, b turnBot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.bots == nil {
		t.bots = map[seatRef]turnBot{}
	}
	t.bots[seatRef{matchID, seatID}] = b
}

func (t *turnTracker) forget(matchID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.moved, matchID)
	for k := range t.bots {
		if k.match == matchID {
			delete(t.bots, k)
		}
	}
}

// forgetSeat drops the bot a seat started its turn with, so the next move it
// makes is not taken for the rest of that turn. For a stand-in ending or
// changing strength: the seat's next decision is somebody else's.
func (t *turnTracker) forgetSeat(matchID, seatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.bots, seatRef{matchID, seatID})
	if t.moved[matchID] == seatID {
		delete(t.moved, matchID)
	}
}

func (t *turnTracker) sweep(now time.Time, idle time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	live := map[string]bool{}
	for k, b := range t.bots {
		if now.Sub(b.seen) >= idle {
			delete(t.bots, k)
			continue
		}
		live[k.match] = true
	}
	for m := range t.moved {
		if !live[m] {
			delete(t.moved, m)
		}
	}
}

// noteMove is called for every accepted action.
func (m *Manager) noteMove(matchID, playerID string) {
	m.turns.noteMove(matchID, playerID)
	m.governor.Moved(matchID, playerID)
}

// matchStopped lets go of everything held for a match's bot seats.
func (m *Manager) matchStopped(matchID string) {
	m.turns.forget(matchID)
	m.governor.ReleaseMatch(matchID)
}

// seatBot is the bot, seat and engine that play this seat's next move.
func (m *Manager) seatBot(match models.Match, mod module.GameModule, actor string) turnBot {
	id := match.ID.Hex()
	now := time.Now()
	if b, ok := m.turns.midTurn(id, actor); ok {
		b.seen = now
		m.turns.remember(id, actor, b)
		// The governor is still told, so its seat stays seen; mid-turn it
		// changes nothing.
		m.decide(match, mod, actor, b.want, now)
		return b
	}

	bot := module.BotFor(mod)
	seat := playableSeat(bot, botSeatFor(match, actor))
	engine := engineOf(bot, seat)
	want := botgov.Class{Module: match.ModuleID, Skill: string(seat.Skill), Engine: engine}
	if d, ok := m.decide(match, mod, actor, want, now); ok && d.Reduced {
		if d.Class.Engine == botgov.EngineRule {
			if layered, ok := bot.(interface{ Heuristic() module.Bot }); ok {
				bot = layered.Heuristic()
			}
		}
		seat.Skill = module.Skill(d.Class.Skill)
		engine = d.Class.Engine
	}
	b := turnBot{bot: bot, seat: seat, engine: engine, want: want, seen: now}
	m.turns.remember(id, actor, b)
	return b
}

// decide asks the governor, when there is one doing anything.
func (m *Manager) decide(match models.Match, mod module.GameModule, actor string, want botgov.Class, now time.Time) (botgov.Decision, bool) {
	if m.governor.Mode() == botgov.Off || want.Skill == "" {
		return botgov.Decision{}, false
	}
	round := 0
	if r := module.RoundsFor(mod, module.State(match.State)); r != nil {
		round = len(r.Rounds)
	}
	return m.governor.Decide(match.ID.Hex(), actor, want, round, hasHuman(match), now), true
}

func hasHuman(match models.Match) bool {
	for _, p := range match.Players {
		if !p.IsAI {
			return true
		}
	}
	return false
}

// StartGovernorSweep releases, every interval, leases and turn records held
// for seats nobody has asked about in a while — a table that ended or moved
// away without the bot loop seeing it go.
func (m *Manager) StartGovernorSweep(ctx context.Context, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				m.governor.Sweep(now)
				m.turns.sweep(now, 10*time.Minute)
			}
		}
	}()
}
