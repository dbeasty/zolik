package match

import (
	"encoding/json"
	"strings"
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/zolikmod"
)

// The runtime's own tests. Everything needing Mongo is covered by the e2e
// suite (e2e/tests/match-runtime.spec.ts); what is worth pinning here is the
// projection, because it is the one place a runtime bug could leak cards.

func registry() *module.Registry {
	return module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New())
}

func dealt(t *testing.T, moduleID string, playerIDs ...string) (*Manager, models.Match) {
	t.Helper()
	reg := registry()
	mod := reg.Get(moduleID)
	if mod == nil {
		t.Fatalf("no module %q", moduleID)
	}
	players := make([]models.Player, 0, len(playerIDs))
	refs := make([]module.PlayerRef, 0, len(playerIDs))
	for _, id := range playerIDs {
		players = append(players, models.Player{ID: id, Name: id})
		refs = append(refs, module.PlayerRef{ID: id, Name: id})
	}
	state, err := mod.NewMatch(module.MatchConfig{}, refs, 5)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	return &Manager{registry: reg}, models.Match{
		ModuleID: moduleID, Status: "active", Players: players, State: state,
	}
}

func TestBuildStateMsg_ProjectsPerViewer(t *testing.T) {
	// The security-relevant term, checked for every hosted module rather than
	// trusted per game: a viewer must never receive another player's cards.
	for _, moduleID := range []string{"zolik", "prsi", "canasta"} {
		t.Run(moduleID, func(t *testing.T) {
			m, match := dealt(t, moduleID, "p1", "p2")

			for _, viewer := range []string{"p1", "p2"} {
				msg := m.BuildStateMsg(match, viewer)

				if msg.ModuleID != moduleID {
					t.Errorf("moduleId = %q, want %q", msg.ModuleID, moduleID)
				}
				if len(msg.View.Zones) == 0 {
					t.Fatalf("%s got an empty board", viewer)
				}

				sawOwnHand := false
				for _, z := range msg.View.Zones {
					if z.Kind != module.ZoneHand {
						continue
					}
					if z.OwnerID == viewer {
						sawOwnHand = true
						if len(z.Cards) == 0 {
							t.Errorf("%s cannot see their own hand", viewer)
						}
						continue
					}
					if len(z.Cards) > 0 {
						t.Errorf("%s can see %s's cards: %v", viewer, z.OwnerID, z.Cards)
					}
					if z.Count == 0 {
						t.Errorf("%s cannot see how many cards %s holds", viewer, z.OwnerID)
					}
				}
				if !sawOwnHand {
					t.Errorf("%s was sent no hand of their own", viewer)
				}
			}
		})
	}
}

func TestBuildStateMsg_LegalActionsIsNeverNull(t *testing.T) {
	// A nil slice serialises to JSON `null`, which every client then has to
	// guard before indexing — the exact bug that crashed the game screen once
	// already when a discard pile came back nil.
	m, match := dealt(t, "prsi", "p1", "p2")
	raw, err := json.Marshal(m.BuildStateMsg(match, "p1"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["legalActions"] == nil {
		t.Error("legalActions serialised as null rather than an empty array")
	}
}

func TestBuildStateMsg_LobbyHasNoBoardYet(t *testing.T) {
	// A match exists before its game has done anything. The runtime must be
	// able to describe it without asking a module to decode empty state —
	// which is also a small demonstration that the envelope stands alone.
	m := &Manager{registry: registry()}
	msg := m.BuildStateMsg(models.Match{
		ModuleID: "prsi", Status: "lobby",
		Players: []models.Player{{ID: "p1", Name: "p1"}},
	}, "p1")

	if msg.Status != "lobby" {
		t.Errorf("status = %q, want lobby", msg.Status)
	}
	if len(msg.View.Zones) != 0 {
		t.Errorf("a lobby has no board yet, got %d zones", len(msg.View.Zones))
	}
	if msg.LegalActions == nil {
		t.Error("legalActions should be an empty list, not nil")
	}
	if len(msg.Players) != 1 {
		t.Errorf("players = %v", msg.Players)
	}
}

func TestBuildStateMsg_CarriesTheLobbysOptions(t *testing.T) {
	// A "see the rules" screen reached from an in-progress match has to ask
	// for the table's actual settings, not a variation's defaults — which
	// means the options a lobby chose have to survive onto the wire.
	m := &Manager{registry: registry()}
	msg := m.BuildStateMsg(models.Match{
		ModuleID: "zolik", Status: "lobby",
		Options: map[string]int{"initialMeldMinimum": 50},
		Players: []models.Player{{ID: "p1", Name: "p1"}},
	}, "p1")

	if msg.Options["initialMeldMinimum"] != 50 {
		t.Errorf("options = %v, want initialMeldMinimum=50", msg.Options)
	}
}

func TestBuildStateMsg_UnknownModuleDegradesRatherThanPanics(t *testing.T) {
	// A document naming a module this build does not have — a rollback, or a
	// module removed — must render as an inert match rather than crash the
	// broadcast for everyone else in it.
	m := &Manager{registry: registry()}
	msg := m.BuildStateMsg(models.Match{
		ModuleID: "marias", Status: "active",
		Players: []models.Player{{ID: "p1"}},
		State:   json.RawMessage(`{"anything":1}`),
	}, "p1")

	if len(msg.View.Zones) != 0 || len(msg.LegalActions) != 0 {
		t.Errorf("an unknown module should render nothing playable, got %+v", msg)
	}
	if msg.ModuleID != "marias" {
		t.Errorf("the match should still say what it is: %q", msg.ModuleID)
	}
}

// TestBuildStateMsgCarriesTheRoundLog — the history rides on the state message
// rather than on an event, which is what makes it survive a reconnection and a
// page reload. An event-fed table is empty for anyone who refreshed.
func TestBuildStateMsgCarriesTheRoundLog(t *testing.T) {
	m, match := dealt(t, "zolik", "p1", "p2")
	msg := m.BuildStateMsg(match, "p1")

	if msg.Rounds == nil {
		t.Fatal("a game with rounds sent no round log")
	}
	if msg.Rounds.LabelKey == "" {
		t.Error("the log does not say what a round is called here")
	}
	if msg.Rounds.Rounds == nil {
		t.Error("a fresh match sent a null round list; empty and absent are different answers")
	}
	if len(msg.Rounds.Rounds) != 0 {
		t.Errorf("a freshly dealt match reports %d finished rounds", len(msg.Rounds.Rounds))
	}
}

// TestBuildStateMsgOmitsRoundsForAGameWithoutThem — Prší is one deal per match,
// so the key must be absent rather than an empty object a client has to
// interpret.
func TestBuildStateMsgOmitsRoundsForAGameWithoutThem(t *testing.T) {
	m, match := dealt(t, "prsi", "p1", "p2")
	msg := m.BuildStateMsg(match, "p1")

	if msg.Rounds != nil {
		t.Errorf("Prší sent a round log: %+v", msg.Rounds)
	}
	blob, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), `"rounds"`) {
		t.Error(`the wire carries a "rounds" key for a game with no rounds`)
	}
}
