package match_test

import (
	"sync"
	"testing"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
)

// agentsHere is an AgentPresence whose answer the test sets.
type agentsHere struct {
	mu   sync.Mutex
	here map[string]bool
}

func (a *agentsHere) set(id string, here bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.here == nil {
		a.here = map[string]bool{}
	}
	a.here[id] = here
}

func (a *agentsHere) Present(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.here[id]
}

// agentTable deals a person and an agent a Prší game the agent leads, with the
// agent tracked at the lobby the way the MCP join does and atLobby run before
// the deal. Who leads is the deal's, so tables are dealt until one fits.
func agentTable(t *testing.T, atLobby func(m *match.Manager)) (m *match.Manager, repo match.Repository, presence *agentsHere, id string) {
	t.Helper()
	ctx := t.Context()
	m, repo, _ = newReaperHarness(t)
	presence = &agentsHere{}
	presence.set("agent", true)
	m.SetAgentPresence(presence)
	for range 40 {
		created, err := m.Create(ctx, "prsi", module.MatchConfig{}, models.Player{ID: "p1", Name: "Ada"})
		if err != nil {
			t.Fatal(err)
		}
		id = created.ID.Hex()
		if _, err := m.Join(ctx, id, models.Player{ID: "agent", Name: "Claude", IsAgent: true}); err != nil {
			t.Fatal(err)
		}
		m.TrackAgent(id, "agent")
		if atLobby != nil {
			atLobby(m)
		}
		if _, err := m.Start(ctx, id); err != nil {
			t.Fatal(err)
		}
		if m.Awaits(ctx, id, "agent") {
			// It goes quiet on its turn.
			presence.set("agent", false)
			return m, repo, presence, id
		}
		if err := m.DeleteAsHost(ctx, id, "p1"); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("no deal led with the agent")
	return
}

// The sweep used to let go of an agent seen at a lobby, and nothing tracked it
// again when the game was dealt: an agent that went quiet after the deal left
// its table waiting on it for good, never suspended, never swept.
func TestAnAgentTrackedInTheLobbyIsWatchedOnceDealt(t *testing.T) {
	ctx := t.Context()
	m, _, _, id := agentTable(t, func(m *match.Manager) { m.SweepAgents(ctx) })
	m.SweepAgents(ctx)
	if cur, _ := m.Current(ctx, id); cur.Status != "suspended" {
		t.Fatalf("status %q with the agent on turn and silent, want suspended", cur.Status)
	}
}

// A restart forgets every tracked agent. The one that went quiet across it
// never calls again to be tracked, so the table has to say who its agents are
// when it is next loaded.
func TestAnAgentIsWatchedAgainAfterARestart(t *testing.T) {
	ctx := t.Context()
	_, repo, presence, id := agentTable(t, nil)

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	restarted := match.NewManager(repo, module.NewRegistry(prsi.New()), hub)
	restarted.SetAgentPresence(presence)
	// Something loads the table: the person opening it, the bot loop, a list.
	if _, err := restarted.Current(ctx, id); err != nil {
		t.Fatal(err)
	}
	restarted.SweepAgents(ctx)
	if cur, _ := restarted.Current(ctx, id); cur.Status != "suspended" {
		t.Fatalf("status %q after a restart with the agent silent on turn, want suspended", cur.Status)
	}
}
