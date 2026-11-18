package sietemedio

import (
	"encoding/json"
	"testing"

	"zolik/server/internal/module"
)

// narrated is every line the events of one action read as, the events having
// been through JSON the way the runtime's stored log hands them back.
func narrated(t *testing.T, s *GameState, playerID string, a module.Action) ([]module.Move, *GameState) {
	t.Helper()
	next, events, err := New().Apply(rawOf(t, s), playerID, a)
	if err != nil {
		t.Fatalf("Apply %s: %v", a.Verb, err)
	}
	blob, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var stored []module.Event
	if err := json.Unmarshal(blob, &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var out []module.Move
	for _, ev := range stored {
		if mv, ok := New().NarrateEvent(next, ev); ok {
			out = append(out, mv)
		}
	}
	return out, stateOf(t, next)
}

func keysOf(moves []module.Move) []string {
	out := make([]string, 0, len(moves))
	for _, m := range moves {
		out = append(out, m.Fact.LabelKey)
	}
	return out
}

func TestADrawNamesTheCardAndAStandIsALine(t *testing.T) {
	s := withTable(t, func(*GameState) {})
	moves, s := narrated(t, s, "p2", module.Action{OfferID: VerbHit, Verb: VerbHit})
	if len(moves) != 1 || moves[0].Fact.LabelKey != "sietemedio.move.drew" {
		t.Fatalf("a draw under 7½ read %v", keysOf(moves))
	}
	if moves[0].PlayerID != "p2" || moves[0].Fact.Params["card"] != "2C" || moves[0].Notable {
		t.Fatalf("draw line = %+v", moves[0])
	}

	moves, _ = narrated(t, s, "p2", module.Action{OfferID: VerbStand, Verb: VerbStand})
	if len(moves) != 1 || moves[0].Fact.LabelKey != "sietemedio.move.stood" || moves[0].Fact.Params["player"] != "p2" {
		t.Fatalf("a stand read %+v", moves)
	}
}

func TestADrawOverSevenAndAHalfSaysSoAndStandsOut(t *testing.T) {
	// 5 + 2 is 7; the 3 after it goes over.
	s := withTable(t, func(*GameState) {})
	s = mustApply(t, s, "p2", VerbHit)
	moves, _ := narrated(t, s, "p2", module.Action{OfferID: VerbHit, Verb: VerbHit})
	if len(moves) == 0 || moves[0].Fact.LabelKey != "sietemedio.move.drewBust" {
		t.Fatalf("a bust read %v", keysOf(moves))
	}
	if moves[0].Fact.Params["card"] != "3S" || !moves[0].Notable {
		t.Fatalf("bust line = %+v", moves[0])
	}
}

func TestTheBankTurnAndSettlementAreNarrated(t *testing.T) {
	// p2 and p3 stand; the bank, on 4, draws the 2 and stands on 6.
	s := withTable(t, func(*GameState) {})
	s = mustApply(t, s, "p2", VerbStand)
	moves, s := narrated(t, s, "p3", module.Action{OfferID: VerbStand, Verb: VerbStand})
	want := []string{"sietemedio.move.stood", "sietemedio.move.bankTurn"}
	if got := keysOf(moves); len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("the last stand read %v, want %v", got, want)
	}
	if moves[1].PlayerID != "p1" {
		t.Fatalf("bank turn belongs to %q", moves[1].PlayerID)
	}
	s = mustApply(t, s, "p1", VerbHit)
	moves, _ = narrated(t, s, "p1", module.Action{OfferID: VerbStand, Verb: VerbStand})
	var settled *module.Move
	for i := range moves {
		if moves[i].Fact.LabelKey == "sietemedio.move.bankStands" {
			settled = &moves[i]
		}
	}
	if settled == nil || settled.Fact.Params["total"] != "6" {
		t.Fatalf("the settlement read %+v", moves)
	}
}

func TestEveryEventTheEngineSendsReads(t *testing.T) {
	cases := []struct {
		ev   module.Event
		want string
	}{
		{module.Event{Type: "stake_placed", Data: map[string]any{"playerId": "p2", "amount": float64(20)}}, "sietemedio.move.staked"},
		{module.Event{Type: "no_room", Data: map[string]any{"playerId": "p3"}}, "sietemedio.move.noRoom"},
		{module.Event{Type: "leaving", Data: map[string]any{"playerId": "p2"}}, "sietemedio.move.leaving"},
		{module.Event{Type: "left", Data: map[string]any{"playerId": "p2", "chips": 480}}, "sietemedio.move.left"},
		{module.Event{Type: "round_settled", Data: map[string]any{"round": 1, "bankTotal": float64(16)}}, "sietemedio.move.bankBust"},
		{module.Event{Type: "round_settled", Data: map[string]any{"round": 1, "bankTotal": 15}}, "sietemedio.move.bankStands"},
	}
	for _, c := range cases {
		mv, ok := New().NarrateEvent(nil, c.ev)
		if !ok || mv.Fact.LabelKey != c.want {
			t.Errorf("%s read %q (%v), want %q", c.ev.Type, mv.Fact.LabelKey, ok, c.want)
		}
	}
	if mv, _ := New().NarrateEvent(nil, cases[0].ev); mv.Fact.Params["n"] != 20 {
		t.Errorf("stake amount = %v", mv.Fact.Params["n"])
	}
	if mv, _ := New().NarrateEvent(nil, cases[5].ev); mv.Fact.Params["total"] != "7½" {
		t.Errorf("bank total = %v", mv.Fact.Params["total"])
	}
	// The deal and the round's own start have the table's notes, not a line.
	for _, typ := range []string{"cards_dealt", "round_started", "match_ended"} {
		if _, ok := New().NarrateEvent(nil, module.Event{Type: typ, Data: map[string]any{}}); ok {
			t.Errorf("%s was narrated", typ)
		}
	}
}

// TestAFaceDownCardIsNeverNarrated plays whole matches with the first card
// down and checks no line shown to a seat names another seat's hole card while
// that hand is still face down.
func TestAFaceDownCardIsNeverNarrated(t *testing.T) {
	m := New()
	players := seats("p1", "p2", "p3")
	for seed := int64(1); seed <= 5; seed++ {
		raw, err := m.NewMatch(module.MatchConfig{Variation: VarMexicana, Options: module.Options{OptRounds: 6, OptFirstCardDown: 1}}, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		if !stateOf(t, raw).FirstCardDown {
			t.Fatalf("first card is not down")
		}
		lines := 0
		_, _, err = module.PlayWithOffers(m, raw, players, module.DriverOptions{
			MaxActions: 600,
			OnEvents: func(_ string, events []module.Event, before, after module.State) {
				b := stateOf(t, before)
				for _, viewer := range players {
					for _, ev := range events {
						seen, ok := module.ProjectEvent(m, ev, viewer.ID)
						if !ok {
							continue
						}
						mv, ok := m.NarrateEvent(after, seen)
						if !ok {
							continue
						}
						lines++
						card, _ := mv.Fact.Params["card"].(string)
						for _, st := range b.Seats {
							if st.PlayerID != viewer.ID && !st.Shown && len(st.Cards) > 0 && card == st.Cards[0] {
								t.Errorf("seed %d: %s read %s's face-down %s", seed, viewer.ID, st.PlayerID, card)
							}
						}
					}
				}
			},
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if lines == 0 {
			t.Fatalf("seed %d: nothing was narrated", seed)
		}
	}
}
