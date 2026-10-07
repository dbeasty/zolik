package klondike

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// hiddenCards is every card the player has not seen: face-down in a column,
// or still in the stock.
func hiddenCards(s *GameState) []string {
	var out []string
	for _, c := range s.Cols {
		out = append(out, c.Down...)
	}
	return append(out, s.Stock...)
}

// TestNothingEverNamesAFaceDownCard is the secrecy term: across many deals and
// at every step, neither the view nor any offer carries a card the player has
// not turned up.
func TestNothingEverNamesAFaceDownCard(t *testing.T) {
	m := New()
	for seed := int64(1); seed <= 8; seed++ {
		for _, variation := range []string{"classic", "draw3", "vegas"} {
			raw, _ := m.NewMatch(module.MatchConfig{Variation: variation}, solo, seed)
			for step := 0; step < 400; step++ {
				s, _ := decode(raw)
				vm, _ := m.View(raw, "p1")
				offers, _ := m.LegalActions(raw, "p1")
				blob, _ := json.Marshal(struct {
					V module.ViewModel
					O []module.ActionOffer
				}{vm, offers})
				for _, c := range hiddenCards(s) {
					if strings.Contains(string(blob), strconv.Quote(c)) {
						t.Fatalf("seed %d %s step %d: face-down %s is on the wire", seed, variation, step, c)
					}
				}
				checkCounts(t, s, vm)

				a, ok := module.ChooseAction(offers, []string{"autofinish", "move", "draw", "recycle"})
				if !ok {
					break
				}
				next, _, err := m.Apply(raw, "p1", a)
				if err != nil {
					t.Fatalf("seed %d: offered %+v refused: %v", seed, a, err)
				}
				raw = next
				if done, _, _ := m.Finished(raw); done {
					break
				}
			}
		}
	}
}

// checkCounts: what the view counts adds up to 52, and a column's Hidden is
// exactly its face-down cards.
func checkCounts(t *testing.T, s *GameState, vm module.ViewModel) {
	t.Helper()
	total := 0
	for _, z := range vm.Zones {
		switch z.ID {
		case zoneStock, zoneWaste:
			total += z.Count
		case zoneFoundations:
			for _, g := range z.Groups {
				total += len(g.Cards)
			}
		case zoneTableau:
			for i, g := range z.Groups {
				if g.Hidden != len(s.Cols[i].Down) {
					t.Fatalf("%s hides %d, has %d down", g.ID, g.Hidden, len(s.Cols[i].Down))
				}
				total += g.Hidden + len(g.Cards)
			}
		}
	}
	if total != 52 {
		t.Fatalf("the view accounts for %d cards", total)
	}
}

func TestOpenViewShowsEverything(t *testing.T) {
	raw, _ := New().NewMatch(module.MatchConfig{}, solo, 8)
	vm, _ := New().OpenView(raw)
	blob, _ := json.Marshal(vm)
	s, _ := decode(raw)
	for _, c := range hiddenCards(s) {
		if !strings.Contains(string(blob), strconv.Quote(c)) {
			t.Fatalf("open view leaves out %s", c)
		}
	}
}

func TestDrawThreeFansTheWaste(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(module.MatchConfig{Variation: "draw3"}, solo, 8)
	raw, _, _ = m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	vm, _ := m.View(raw, "p1")
	for _, z := range vm.Zones {
		if z.ID == zoneWaste && (z.Fan != 3 || len(z.Cards) != 3 || z.Count != 3) {
			t.Fatalf("waste %+v", z)
		}
	}
}
