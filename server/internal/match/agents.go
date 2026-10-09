package match

import (
	"context"
	"log"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// AI agents at the table, and seats that are paused out.
//
// An agent (see internal/mcp) is a seat like a person's: it is not driven by
// the server's bot loop, it can be slow, and it can disappear. What it lacks is
// a WebSocket, so "is this seat here" cannot be read off the connection
// registry alone — AgentPresence answers it for those seats, and seatHere is
// the one question every presence rule now asks.
//
// What happens when a seat is not here depends on the game (see
// module.DropIn). A table that needs everyone pauses, as it always has. A
// drop-in table — poker, blackjack — does not: past SitOutGrace the seat is
// played passively by the same loop that plays bots, until its player returns.

// SitOutGrace is how long a seat at a drop-in table may be away before the
// table stops waiting for it. Long enough to survive a tunnel or a page
// reload, short enough that four people are not held up by one dropped phone.
const SitOutGrace = 20 * time.Second

// AgentPresence says whether an agent seat's client has been heard from
// recently. Implemented by internal/mcp's registry; optional.
type AgentPresence interface {
	Present(playerID string) bool
}

// SetAgentPresence attaches the source of agent presence. Optional: without
// it an agent seat reads as away, which is the honest answer for a server that
// has no MCP endpoint.
func (m *Manager) SetAgentPresence(p AgentPresence) { m.agentPresence = p }

// SetSitOutGrace overrides SitOutGrace. For tests; zero means the default.
func (m *Manager) SetSitOutGrace(d time.Duration) { m.sitOutGrace = d }

func (m *Manager) graceFor() time.Duration {
	if m.sitOutGrace > 0 {
		return m.sitOutGrace
	}
	return SitOutGrace
}

func playerMsg(p models.Player) PlayerMsg {
	msg := PlayerMsg{ID: p.ID, Name: p.Name, IsAI: p.IsAI, Avatar: p.Avatar, IsAgent: p.IsAgent, AgentLabel: p.AgentLabel}
	if p.IsAI {
		msg.Skill = p.AIDifficulty
	}
	return msg
}

// seatHere reports whether the person or agent in a seat is connected now.
func (m *Manager) seatHere(room, playerID string) bool {
	if m.hub != nil && m.hub.Registry().Has(room, playerID) {
		return true
	}
	return m.agentPresence != nil && m.agentPresence.Present(playerID)
}

// satOut reports whether the table is playing past this seat: a drop-in game,
// a seat somebody else is meant to be sitting at, and that somebody away for
// longer than the grace period.
func (m *Manager) satOut(match models.Match, p models.Player) bool {
	if p.IsAI || match.Status != string(rules.StatusActive) {
		return false
	}
	mod := m.registry.Get(match.ModuleID)
	if _, ok := module.SitOutVerbs(mod); !ok {
		return false
	}
	room := match.ID.Hex()
	key := room + "|" + p.ID
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	if m.seatHere(room, p.ID) {
		delete(m.awaySince, key)
		return false
	}
	if m.awaySince == nil {
		m.awaySince = map[string]time.Time{}
	}
	since, ok := m.awaySince[key]
	if !ok {
		since = time.Now()
		m.awaySince[key] = since
	}
	return time.Since(since) >= m.graceFor()
}

// scheduleSitOut wakes the bot loop once the grace period is over, because
// nothing else will: a seat going quiet writes nothing for the loop to react
// to. The loop itself decides whether the seat is by then sat out.
//
// One timer per match at a time: the loop asks again every time it finds the
// table waiting on a seat that is away, and the answer does not change.
func (m *Manager) scheduleSitOut(matchID string) {
	m.awayMu.Lock()
	if m.sitOutTimers == nil {
		m.sitOutTimers = map[string]bool{}
	}
	if m.sitOutTimers[matchID] {
		m.awayMu.Unlock()
		return
	}
	m.sitOutTimers[matchID] = true
	m.awayMu.Unlock()
	time.AfterFunc(m.graceFor()+50*time.Millisecond, func() {
		m.awayMu.Lock()
		delete(m.sitOutTimers, matchID)
		m.awayMu.Unlock()
		m.RunBotsIfNeeded(context.Background(), matchID)
	})
}

// returned notes that a seat is back. When it had been sat out, the table is
// told, so the "paused" mark comes off every screen.
func (m *Manager) returned(ctx context.Context, matchID, playerID string) {
	key := matchID + "|" + playerID
	m.awayMu.Lock()
	since, was := m.awaySince[key]
	delete(m.awaySince, key)
	m.awayMu.Unlock()
	if !was || time.Since(since) < m.graceFor() {
		return
	}
	if cur, err := m.current(ctx, matchID); err == nil {
		m.Broadcast(cur)
	}
}

// drivenSeat picks the first awaited seat the server plays: a bot's, a
// stand-in's (see standin.go), or a sat-out seat's. passive is true for the
// last, which is played by the module's sit-out verbs rather than its bot.
func (m *Manager) drivenSeat(match models.Match, mod module.GameModule, awaited []string) (actor string, passive bool) {
	if id := firstBot(awaited, match.Players); id != "" {
		return id, false
	}
	for _, id := range awaited {
		p := playerByID(match.Players, id)
		if p == nil {
			continue
		}
		if m.standInPlaying(match, *p) {
			return id, false
		}
		if m.satOut(match, *p) {
			return id, true
		}
		// Away, but not for long enough yet: come back when it is.
		if !p.IsAI && !m.seatHere(match.ID.Hex(), id) {
			if _, dropIn := module.SitOutVerbs(mod); dropIn {
				m.scheduleSitOut(match.ID.Hex())
			} else if due, ok := m.standInDue(match, *p); ok {
				m.scheduleStandIn(match.ID.Hex(), id, due)
			}
		}
	}
	return "", false
}

// TrackAgent remembers that an agent holds a seat at a match, so the sweep
// below can watch it. In memory only: an agent's next call re-registers it
// after a restart.
func (m *Manager) TrackAgent(matchID, playerID string) {
	m.awayMu.Lock()
	defer m.awayMu.Unlock()
	if m.agentTables == nil {
		m.agentTables = map[string]map[string]bool{}
	}
	if m.agentTables[matchID] == nil {
		m.agentTables[matchID] = map[string]bool{}
	}
	m.agentTables[matchID][playerID] = true
}

// SweepAgents acts on agent seats that have gone quiet. A drop-in table gets
// its bot loop woken, which will play a seat that is past its grace period; any
// other table is paused for that seat exactly as for a person whose socket
// closed.
func (m *Manager) SweepAgents(ctx context.Context) {
	m.awayMu.Lock()
	tables := make(map[string][]string, len(m.agentTables))
	for id, seats := range m.agentTables {
		for pid := range seats {
			tables[id] = append(tables[id], pid)
		}
	}
	m.awayMu.Unlock()

	for matchID, seats := range tables {
		match, err := m.current(ctx, matchID)
		if err != nil || (match.Status != "active" && match.Status != "suspended") {
			m.awayMu.Lock()
			delete(m.agentTables, matchID)
			m.awayMu.Unlock()
			continue
		}
		for _, pid := range seats {
			if m.seatHere(matchID, pid) {
				continue
			}
			if _, dropIn := module.SitOutVerbs(m.registry.Get(match.ModuleID)); dropIn {
				m.RunBotsIfNeeded(context.WithoutCancel(ctx), matchID)
			} else {
				m.SuspendOnDisconnect(ctx, matchID, pid, "agent silent")
			}
		}
	}
}

// StartAgentSweep runs SweepAgents until ctx ends.
func (m *Manager) StartAgentSweep(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.SweepAgents(ctx)
			}
		}
	}()
	log.Printf("agent presence sweep every %s", every)
}
