package canasta

import (
	"encoding/json"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// TestViewHidesWhatItShould is the one contract term with a security
// consequence, and the reason View is a module method: only this package knows
// what is secret in this game.
//
// Canasta's answer is unusual enough to be worth pinning. Hands are private and
// the stock is face down, as everywhere — but melds and red threes are public
// to *both* partnerships, and the discard pile shows only its top card even
// though everyone at a real table has watched it being built.
func TestViewHidesWhatItShould(t *testing.T) {
	m := New()
	players := refs("p1", "p2", "p3", "p4")
	states := collectStates(t, module.MatchConfig{Options: module.Options{OptTargetScore: 500}}, players, 3, 300)

	for i, state := range states {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, viewer := range players {
			vm, err := m.View(state, viewer.ID)
			if err != nil {
				t.Fatalf("View: %v", err)
			}

			var sawOwnHand bool
			for _, z := range vm.Zones {
				switch {
				case z.Kind == module.ZoneHand && z.OwnerID == viewer.ID:
					sawOwnHand = true
					if len(z.Cards) != len(s.Hands[viewer.ID]) {
						t.Errorf("state %d: %s sees %d of their own %d cards",
							i, viewer.ID, len(z.Cards), len(s.Hands[viewer.ID]))
					}
				case z.Kind == module.ZoneHand:
					// Somebody else's hand: a count, never a card.
					if len(z.Cards) != 0 {
						t.Fatalf("state %d: %s can see %s's cards %v",
							i, viewer.ID, z.OwnerID, z.Cards)
					}
					if z.Count != len(s.Hands[z.OwnerID]) {
						t.Errorf("state %d: %s's count is %d, want %d",
							i, z.OwnerID, z.Count, len(s.Hands[z.OwnerID]))
					}
				case z.ID == "draw":
					if len(z.Cards) != 0 {
						t.Fatalf("state %d: the stock leaked %v", i, z.Cards)
					}
					if z.Count != len(s.DrawPile) {
						t.Errorf("state %d: stock count %d, want %d", i, z.Count, len(s.DrawPile))
					}
				case z.ID == "discard":
					// The top card, and nothing buried under it.
					if len(z.Cards) > 1 {
						t.Fatalf("state %d: the pile showed %v, not just its top", i, z.Cards)
					}
					if len(z.Cards) == 1 && z.Cards[0].Card != s.top() {
						t.Errorf("state %d: pile shows %q, top is %q", i, z.Cards[0].Card, s.top())
					}
					if z.Count != len(s.DiscardPile) {
						t.Errorf("state %d: pile count %d, want %d", i, z.Count, len(s.DiscardPile))
					}
				}
			}
			if !sawOwnHand {
				t.Errorf("state %d: %s was not shown their own hand", i, viewer.ID)
			}
		}
	}
}

// TestNoHiddenCardSurvivesSerialisation is the blunt version of the check
// above, and the one that would catch a leak through a field nobody thought
// about: encode the whole view and look for a card the viewer cannot see.
func TestNoHiddenCardSurvivesSerialisation(t *testing.T) {
	m := New()
	players := refs("p1", "p2")
	states := collectStates(t, module.MatchConfig{Options: module.Options{OptTargetScore: 500}}, players, 3, 200)

	for i, state := range states {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, viewer := range players {
			vm, err := m.View(state, viewer.ID)
			if err != nil {
				t.Fatalf("View: %v", err)
			}
			blob, err := json.Marshal(vm)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			wire := string(blob)

			// Anything the viewer legitimately sees is excluded first, so what
			// is left really is secret.
			visible := map[string]bool{}
			for _, c := range s.Hands[viewer.ID] {
				visible[c] = true
			}
			for _, mm := range s.allMelds() {
				for _, c := range mm.Cards {
					visible[c] = true
				}
			}
			for j := range s.Teams {
				for _, c := range s.Teams[j].RedThrees {
					visible[c] = true
				}
			}
			if top := s.top(); top != "" {
				visible[top] = true
			}

			for _, p := range players {
				if p.ID == viewer.ID {
					continue
				}
				for _, c := range s.Hands[p.ID] {
					if visible[c] {
						continue // an identical card the viewer legitimately sees
					}
					if strings.Contains(wire, `"`+c+`"`) {
						t.Fatalf("state %d: %s's view contains %q, which is in %s's hand",
							i, viewer.ID, c, p.ID)
					}
				}
			}
		}
	}
}

// TestViewCarriesTheScoreboard checks the compensation for the one place the
// module interface fits Canasta imperfectly: Finished names a single winner,
// so standings have to arrive some other way.
func TestViewCarriesTheScoreboard(t *testing.T) {
	raw := fourHanded(func(s *GameState) {
		s.Teams[0].Score = 1200
		s.Teams[1].Score = 800
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KD", "KS", "KC", "KH", "KD", "KS"}}}
		s.Teams[0].RedThrees = []string{"3H"}
		s.Hands["p1"] = []string{"8C", "9C"}
	})

	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}

	scores := map[int]string{}
	for _, f := range vm.Status {
		if f.LabelKey != "status.teamScore" {
			continue
		}
		team, _ := f.Params["team"].(int)
		scores[team] = f.Value
	}
	if scores[0] != "1200" || scores[1] != "800" {
		t.Errorf("standings are %v, want team 0 on 1200 and team 1 on 800", scores)
	}

	// The canasta is badged, so a client renders "canasta" without knowing
	// that seven is the number.
	var badged bool
	for _, z := range vm.Zones {
		for _, g := range z.Groups {
			for _, b := range g.BadgeKeys {
				if b == "badge.naturalCanasta" {
					badged = true
				}
			}
		}
	}
	if !badged {
		t.Error("a seven-card wild-free meld should be badged as a natural canasta")
	}

	// Red threes are their own public zone.
	var sawRedThrees bool
	for _, z := range vm.Zones {
		if strings.HasPrefix(z.ID, "redThrees:") && len(z.Cards) == 1 {
			sawRedThrees = true
		}
	}
	if !sawRedThrees {
		t.Error("the red three should be visible on the table")
	}
}

// TestPromptsExplainWhatIsMissing — the offers say what a player may do; the
// prompts say what they are working toward, which is the part a player cannot
// infer from a greyed-out button.
func TestPromptsExplainWhatIsMissing(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].Score = 1600 // a 90-point floor
		s.Hands["p1"] = []string{"8C", "9C", "TC"}
	})
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}

	var minimum any
	var needCanastas any
	for _, p := range vm.Prompts {
		switch p.LabelKey {
		case "prompt.initialMeld":
			minimum = p.Params["n"]
		case "prompt.canastasNeeded":
			needCanastas = p.Params["n"]
		}
	}
	if minimum != 90 {
		t.Errorf("initial meld prompt says %v, want 90", minimum)
	}
	if needCanastas != 1 {
		t.Errorf("canastas-needed prompt says %v, want 1", needCanastas)
	}
}
