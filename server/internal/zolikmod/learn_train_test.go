package zolikmod

import (
	"math"
	"reflect"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// The privileged vector is the hidden cards: it holds every other seat's hand
// and the stock card for card, and moving only the hidden cards moves it.
func TestPrivilegedIsTheHiddenCards(t *testing.T) {
	g := learnGame{}
	changed, checked := 0, 0
	for _, tb := range []struct {
		variation string
		seats     int
	}{{varFloor35, 4}, {varClassic, 2}, {varClassic, 3}} {
		w := newLearnWalker(t, tb.variation, tb.seats, 900+int64(tb.seats))
		for n := 0; n < 300; n++ {
			seat := w.actor()
			if seat == "" {
				w.deal()
				continue
			}
			if n%7 == 0 {
				p, _ := g.Position(w.state)
				v, err := g.PrivilegedFor(p, seat)
				if err != nil {
					t.Fatal(err)
				}
				if len(v) != g.PrivDim() || !learnFinite(v) {
					t.Fatalf("privileged vector %d wide (PrivDim %d) or not finite", len(v), g.PrivDim())
				}
				s, _ := decode(w.state)
				if s.Rules.Status == rules.StatusActive && !s.Break.Open {
					order := seatsFrom(s.Rules.TurnOrder, seat)
					for i, id := range order[1:] {
						var held float32
						for k := 0; k < numCardSlot; k++ {
							held += v[privOthers+i*numCardSlot+k] * 2
						}
						if int(math.Round(float64(held))) != len(s.Rules.Hands[id]) {
							t.Fatalf("privileged block %d holds %v cards, %s holds %d", i, held, id, len(s.Rules.Hands[id]))
						}
					}
					if got := int(math.Round(float64(v[privStock] * 100))); got != len(s.Rules.DrawPile) {
						t.Fatalf("stock %d, privileged says %d", len(s.Rules.DrawPile), got)
					}
					q, _ := g.Position(scrambleHidden(t, w.state, seat, w.rnd))
					u, _ := g.PrivilegedFor(q, seat)
					if !reflect.DeepEqual(u, v) {
						changed++
					}
					checked++
				}
			}
			offers := learnOffers(t, w.m, w.state, seat)
			cands, _ := g.Candidates(w.state, seat, offers)
			if !w.move(seat, cands) {
				w.deal()
			}
		}
	}
	if checked < 50 || changed < checked/2 {
		t.Fatalf("hidden cards moved the privileged vector at %d of %d positions", changed, checked)
	}
}

// The match score follows the sheet: before deal one everyone is at zero,
// each step adds that deal's penalties, and the last entry of a finished match
// is its result, the engine's winner the one seat that won.
func TestMatchHistoryFollowsTheSheet(t *testing.T) {
	g := learnGame{}
	for _, seats := range []int{2, 4} {
		players := learn.Players(seats)
		cfg := g.Config(seats, varFloor35)
		hard := learn.Contender{Name: "hard", Bot: heuristicBot{}, Skill: module.SkillHard}
		final, st, err := learn.PlayOut(g.Module(), cfg, players, 77, 50_000, func(string) (module.Bot, module.Skill) { return hard.Bot, hard.Skill })
		if err != nil || st.Stalled {
			t.Fatalf("%d seats: %v stalled=%v", seats, err, st.Stalled)
		}
		s, _ := decode(final)
		wins := 0.0
		for _, p := range players {
			h, err := g.MatchHistory(final, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if h[0].Own != 0 || h[0].Best != 0 || h[0].Deal != 0 || h[0].Target != 200 || h[0].Seats != seats {
				t.Fatalf("before deal one: %+v", h[0])
			}
			last := h[len(h)-1]
			if !last.Over || last.Own != s.Rules.TotalScores[p.ID] || last.Deal != len(s.Rules.GameScores[p.ID]) {
				t.Fatalf("%s: last %+v, total %d over %d deals", p.ID, last, s.Rules.TotalScores[p.ID], len(s.Rules.GameScores[p.ID]))
			}
			if last.Top < 200 {
				t.Fatalf("the match ended with nobody at the target: %+v", last)
			}
			if (last.Won == 1) != (s.Rules.WinnerID == p.ID) {
				t.Fatalf("%s won %v, winner %q", p.ID, last.Won, s.Rules.WinnerID)
			}
			wins += last.Won
			for i := 1; i < len(h)-1; i++ {
				if h[i].Over || h[i].Own < h[i-1].Own {
					t.Fatalf("%s deal %d: %+v after %+v", p.ID, i, h[i], h[i-1])
				}
			}
			p0, _ := g.Position(final)
			now, err := g.MatchScoreFor(p0, p.ID)
			if err != nil || now != last {
				t.Fatalf("MatchScoreFor %+v, history ends %+v (%v)", now, last, err)
			}
		}
		if wins != 1 {
			t.Fatalf("%d seats: %v wins shared out", seats, wins)
		}
	}
}

// v2's encoder is a prefix of this one, and a narrowed game hands it exactly
// that prefix.
func TestNarrowedToV2IsThePrefix(t *testing.T) {
	g := learnGame{}
	ng, err := learn.Narrowed(g, 900, 62)
	if err != nil {
		t.Fatal(err)
	}
	if offInfer != 900 || fInferNext != 62 {
		t.Fatalf("v2 prefix moved: %d/%d", offInfer, fInferNext)
	}
	w := newLearnWalker(t, varFloor35, 4, 31)
	for n := 0; n < 60; n++ {
		seat := w.actor()
		if seat == "" {
			w.deal()
			continue
		}
		full, _ := g.Encode(w.state, seat)
		cut, _ := ng.Encode(w.state, seat)
		if !reflect.DeepEqual(full[:900], cut) {
			t.Fatal("narrowed encoding is not the prefix")
		}
		offers := learnOffers(t, w.m, w.state, seat)
		cands, _ := g.Candidates(w.state, seat, offers)
		nc, _ := ng.Candidates(w.state, seat, offers)
		for i := range nc {
			if len(nc[i].Features) != 62 || !reflect.DeepEqual(nc[i].Features, cands[i].Features[:62]) {
				t.Fatal("narrowed candidate is not the prefix")
			}
		}
		if !w.move(seat, cands) {
			w.deal()
		}
	}
}
