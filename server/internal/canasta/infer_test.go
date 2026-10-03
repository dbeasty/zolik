package canasta

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"

	"zolik/server/internal/cardinfer"
	"zolik/server/internal/learn"
	"zolik/server/internal/module"
)

// watchPositions plays matches with Hard and calls see at every position a
// bot is asked to act in.
func watchPositions(t *testing.T, variation string, seats int, seeds int64, see func(s *GameState)) {
	t.Helper()
	cfg := module.MatchConfig{Variation: variation, Options: module.Options{OptTargetScore: 1500}}
	for seed := int64(1); seed <= seeds; seed++ {
		_, st, err := learn.PlayOut(New(), cfg, learn.Players(seats), 9100+seed, 50_000, func(string) (module.Bot, module.Skill) {
			return botFunc(func(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
				if s, err := decode(raw); err == nil && s.Status == "active" && !s.Break.Open && s.Current == seat.PlayerID {
					see(s)
				}
				return bot{}.Act(raw, seat, offers)
			}), module.SkillHard
		})
		if err != nil || st.Illegal > 0 || st.Stalled {
			t.Fatalf("seed %d: %v illegal %d stalled %v", seed, err, st.Illegal, st.Stalled)
		}
	}
}

// TestSeenHeldIsInTheHand: every card the record says a seat holds is in that
// seat's hand — the record follows captures, melds, lay-offs and discards
// exactly.
func TestSeenHeldIsInTheHand(t *testing.T) {
	checked := 0
	for _, tb := range []struct {
		v     string
		seats int
	}{{"classic", 2}, {"classic", 4}, {"samba", 4}} {
		watchPositions(t, tb.v, tb.seats, 2, func(s *GameState) {
			seen := s.seenThisDeal()
			for id, held := range seen.Held {
				if !hasCards(s.Hands[id], held) {
					t.Fatalf("%s deal %d: %s is recorded holding %v, hand %v", tb.v, s.DealNumber, id, held, s.Hands[id])
				}
				checked += len(held)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no capture was ever recorded")
	}
}

// TestInferDoesNotPeek: scrambling every hand but the observer's and the
// stock changes nothing the inference says.
func TestInferDoesNotPeek(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	n := 0
	watchPositions(t, "classic", 4, 2, func(s *GameState) {
		n++
		if n%7 != 0 {
			return
		}
		me := s.Current
		want := infer(s, me)
		raw, _ := json.Marshal(s)
		for k := 0; k < 2; k++ {
			var sc GameState
			_ = json.Unmarshal(raw, &sc)
			var pool []string
			for _, id := range sc.TurnOrder {
				if id != me {
					pool = append(pool, sc.Hands[id]...)
				}
			}
			pool = append(pool, sc.DrawPile...)
			rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
			for _, id := range sc.TurnOrder {
				if id != me {
					m := len(sc.Hands[id])
					sc.Hands[id], pool = pool[:m], pool[m:]
				}
			}
			sc.DrawPile = pool
			got := infer(&sc, me)
			if want.est != got.est {
				t.Fatalf("deal %d: %s's estimate moved when hidden cards did", s.DealNumber, me)
			}
			for _, c := range sc.Hands[me] {
				if want.captureRisk(s, c) != got.captureRisk(&sc, c) {
					t.Fatalf("capture risk of %s moved", c)
				}
			}
		}
	})
}

// TestInferNormalisedAndInformative: every seat's expected cards are its hand
// size, and the estimate puts more weight on cards actually held than the
// pool does.
func TestInferNormalisedAndInformative(t *testing.T) {
	hit, base := 0.0, 0.0
	watchPositions(t, "classic", 2, 3, func(s *GameState) {
		me := s.Current
		inf := infer(s, me)
		pool := 0.0
		for _, u := range inf.est.Unseen {
			pool += u
		}
		for i, id := range inf.seats {
			sum := 0.0
			for _, x := range inf.est.Hold[i] {
				sum += x
			}
			if math.Abs(sum-float64(len(s.Hands[id]))) > 1e-6 {
				t.Fatalf("%s: expected %.4f cards, holds %d", id, sum, len(s.Hands[id]))
			}
			for _, c := range s.Hands[id] {
				k := cardinfer.Slot(c)
				hit += inf.est.Hold[i][k] / float64(len(s.Hands[id]))
				base += inf.est.Unseen[k] / pool
			}
		}
	})
	t.Logf("estimate on held cards %.2f vs prior %.2f", hit, base)
	if hit <= base {
		t.Errorf("no better than the prior: %.2f <= %.2f", hit, base)
	}
}

// TestCaptureRiskReadsTheRecord: the next seat captured a pile holding two
// kings, so a king discarded now hands it the pile; a card it has no record
// of does not.
func TestCaptureRiskReadsTheRecord(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Hands["p1"] = []string{"KS", "9H", "9C"}
		s.Hands["p2"] = []string{"KH", "KC", "5D", "6D", "7S", "8S", "TC", "JC", "QH", "AH", "4H"}
		s.DiscardPile = []string{"5C", "6C", "7C", "8C", "9C", "TD", "JD"}
		s.Frozen = true
		s.Teams[1].HasMelded = true
		s.Seen = &Seen{Deal: s.DealNumber, Held: map[string][]string{"p2": {"KH", "KC"}}}
	})
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	inf := infer(s, "p1")
	king, nine := inf.captureRisk(s, "KS"), inf.captureRisk(s, "9H")
	t.Logf("capture risk: KS %.2f, 9H %.2f", king, nine)
	if king < 0.9 || nine > 0.3 {
		t.Fatalf("KS %.2f, 9H %.2f", king, nine)
	}
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillHard}, offers)
	if !ok || len(a.Cards) != 1 || a.Cards[0] == "KS" {
		t.Fatalf("Hard discarded %v, handing over the pile", a)
	}
	t.Logf("hard discards %v", a.Cards)
}

// botFunc adapts a function to module.Bot.
type botFunc func(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool)

func (f botFunc) Act(s module.State, seat module.BotSeat, o []module.ActionOffer) (module.Action, bool) {
	return f(s, seat, o)
}
