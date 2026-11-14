package prsi

import (
	"fmt"
	"math/rand"
	"testing"

	"zolik/server/internal/module"
)

// The position a 2-player sim reached on seed 2, step 67: the draw pile is
// empty, the discard pile is down to its top card, and the player to move
// holds nothing that fits. Every action used to be refused, forever.
func stuckPosition(t *testing.T, mut func(*GameState)) module.State {
	return withState(t, func(s *GameState) {
		s.DrawPile = nil
		s.DiscardPile = []string{"7H"}
		s.Hands = map[string][]string{"p1": {"9D"}, "p2": {"8C", "KS"}}
		if mut != nil {
			mut(s)
		}
	})
}

func enabledOffer(t *testing.T, raw module.State, player, id string) bool {
	t.Helper()
	offers, err := New().LegalActions(raw, player)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	for _, o := range offers {
		if o.ID == id {
			return o.Enabled
		}
	}
	return false
}

func TestDraw_NothingToDrawAndNothingToPlayEndsTheTurnEmptyHanded(t *testing.T) {
	m := New()
	raw := stuckPosition(t, nil)
	if !enabledOffer(t, raw, "p1", OfferDraw) {
		t.Fatal("stuck player has no enabled draw: the table is dead")
	}
	next, events, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatalf("empty draw refused: %v", err)
	}
	s := stateOf(t, next)
	if s.Current != "p2" || s.Status != "active" {
		t.Fatalf("turn should pass to p2 with the game on, got current=%q status=%q", s.Current, s.Status)
	}
	if len(s.Hands["p1"]) != 1 || s.BlankDraws != 1 {
		t.Fatalf("hand=%v blankDraws=%d, want one card and one blank draw", s.Hands["p1"], s.BlankDraws)
	}
	if n := intOf(events[0].Data["count"]); n != 0 {
		t.Fatalf("drew %d, want 0", n)
	}
	mv, ok := m.NarrateEvent(next, events[0])
	if !ok || mv.Fact.LabelKey != "prsi.move.nothingToDraw" {
		t.Fatalf("empty draw narrated as %+v (%v)", mv, ok)
	}
}

func TestDraw_EmptyPileIsRefusedToAPlayerWhoCanPlay(t *testing.T) {
	m := New()
	raw := stuckPosition(t, func(s *GameState) { s.Hands["p1"] = []string{"9H"} })
	_, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if module.CodeOf(err) != ErrNothingToDraw {
		t.Fatalf("got %v, want %s — an empty draw must not be a way to stall", err, ErrNothingToDraw)
	}
	if !enabledOffer(t, raw, "p1", OfferPlay) {
		t.Fatal("play should be the enabled way out")
	}
	if ids := m.ExplainRefusal(module.MatchConfig{}, ErrNothingToDraw); len(ids) == 0 {
		t.Fatal("refusal points at no written rule")
	}
}

func TestBlocked_FullCircleOfEmptyDrawsEndsOnFewestCards(t *testing.T) {
	m := New()
	raw := stuckPosition(t, nil)
	raw, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatal(err)
	}
	raw, events, err := m.Apply(raw, "p2", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatal(err)
	}
	s := stateOf(t, raw)
	if s.Status != "completed" {
		t.Fatalf("status %q after a full circle of empty draws, want completed", s.Status)
	}
	done, winners, _ := m.Finished(raw)
	if !done || len(winners) != 1 || winners[0] != "p1" {
		t.Fatalf("Finished = %v %v, want p1 (one card to two)", done, winners)
	}
	if events[len(events)-1].Type != "game_ended" {
		t.Fatalf("last event %q, want game_ended", events[len(events)-1].Type)
	}
	vm, err := m.View(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(vm.Status) == 0 || vm.Status[0].Value != "p1" {
		t.Fatalf("status %+v, want p1 named the winner", vm.Status)
	}
}

func TestBlocked_TieOnFewestCardsSharesTheWin(t *testing.T) {
	m := New()
	raw := stuckPosition(t, func(s *GameState) { s.Hands["p2"] = []string{"8C"} })
	raw, _, _ = m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	raw, _, err := m.Apply(raw, "p2", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatal(err)
	}
	done, winners, _ := m.Finished(raw)
	if !done || len(winners) != 2 {
		t.Fatalf("Finished = %v %v, want p1 and p2 sharing", done, winners)
	}
	vm, _ := m.View(raw, "")
	if got, _ := vm.Status[0].Params["winners"].([]string); len(got) != 2 {
		t.Fatalf("status winners %v, want both", vm.Status[0].Params["winners"])
	}
}

// A blank draw that paid a seven's debt is not a free turn: the payer has not
// yet had the chance to play on this pile, so it does not count towards the
// circle that ends the match.
func TestBlocked_PayingADebtWithNothingDoesNotCountTowardsTheCircle(t *testing.T) {
	m := New()
	raw := stuckPosition(t, func(s *GameState) {
		s.PendingDraw = 2
		s.Hands["p1"] = []string{"9H"} // fits once the debt is paid
	})
	raw, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatalf("taking a debt from an empty pile refused: %v", err)
	}
	if s := stateOf(t, raw); s.PendingDraw != 0 || s.BlankDraws != 0 || s.Current != "p2" {
		t.Fatalf("after paying nothing: pending=%d blank=%d current=%q", s.PendingDraw, s.BlankDraws, s.Current)
	}
	raw, _, err = m.Apply(raw, "p2", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatal(err)
	}
	if s := stateOf(t, raw); s.Status != "active" {
		t.Fatal("match ended before p1 had a free turn")
	}
	if _, _, err := m.Apply(raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{"9H"}}); err != nil {
		t.Fatalf("p1 should get to play: %v", err)
	}
}

func TestSeven_ShortPilePaysWhatItHasAndEndsTheTurn(t *testing.T) {
	m := New()
	raw := stuckPosition(t, func(s *GameState) {
		s.PendingDraw = 4
		s.DrawPile = []string{"TC"}
	})
	raw, events, err := m.Apply(raw, "p1", module.Action{Verb: VerbDraw})
	if err != nil {
		t.Fatal(err)
	}
	s := stateOf(t, raw)
	if n := intOf(events[0].Data["count"]); n != 1 || len(s.Hands["p1"]) != 2 {
		t.Fatalf("drew %d, hand %v; want the one card there was", n, s.Hands["p1"])
	}
	if s.PendingDraw != 0 || s.Current != "p2" {
		t.Fatalf("pending=%d current=%q, want the debt cleared and the turn passed", s.PendingDraw, s.Current)
	}
}

func TestNewMatch_RefusesADealTheDeckCannotCover(t *testing.T) {
	m := New()
	cfg := func(n int) module.MatchConfig {
		return module.MatchConfig{Options: module.Options{OptHandSize: n}}
	}
	ids := []string{"a", "b", "c", "d", "e", "f"}
	_, err := m.NewMatch(cfg(6), refs(ids...), 1)
	if module.CodeOf(err) != ErrDealTooLarge {
		t.Fatalf("6 players x 6 cards: got %v, want %s", err, ErrDealTooLarge)
	}
	if got := m.ExplainRefusal(cfg(6), ErrDealTooLarge); len(got) != 1 || got[0] != "prsi.rules.dealLimit" {
		t.Fatalf("refusal points at %v, want prsi.rules.dealLimit", got)
	}
	// Everything the lobby can otherwise offer deals, and leaves a draw pile.
	for seats := 2; seats <= 6; seats++ {
		for _, hand := range []int{4, 5, 6} {
			if seats == 6 && hand == 6 {
				continue
			}
			raw, err := m.NewMatch(cfg(hand), refs(ids[:seats]...), 1)
			if err != nil {
				t.Fatalf("%d x %d: %v", seats, hand, err)
			}
			if s := stateOf(t, raw); len(s.DrawPile) < 1 || len(s.DiscardPile) != 1 {
				t.Fatalf("%d x %d: draw=%d discard=%d", seats, hand, len(s.DrawPile), len(s.DiscardPile))
			}
		}
	}
}

// Six at five leaves two cards; when both are queens the bury-the-wild loop
// used to rotate them forever. Some seed in this range deals exactly that.
func TestNewMatch_ShortTailOfQueensStillTurnsACard(t *testing.T) {
	m := New()
	cfg := module.MatchConfig{Options: module.Options{OptHandSize: 5}}
	players := refs("a", "b", "c", "d", "e", "f")
	queenUp := 0
	for seed := int64(1); seed <= 3000; seed++ {
		raw, err := m.NewMatch(cfg, players, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if rankOf(stateOf(t, raw).top()) == rankWild {
			queenUp++
		}
	}
	if queenUp == 0 {
		t.Log("no seed dealt a tail of queens; the bound is untested by this sweep")
	}
}

// The house rule, swept: whenever the game is on, the player to move has an
// enabled offer (every Prší verb ends the turn), and the game ends. One seat
// hoards — it draws whenever the engine lets it — which is the play that used
// to empty the pile and strand the table; on odd seeds every seat mostly does,
// which drives tables into full circles of empty draws.
func TestSim_AlwaysALegalTurnEndingActionAndTheGameEnds(t *testing.T) {
	m := New()
	ids := []string{"a", "b", "c", "d", "e", "f"}
	games, blocked, emptyDraws := 0, 0, 0
	for seats := 2; seats <= 6; seats++ {
		for _, hand := range []int{4, 5, 6} {
			if !dealFits(seats, hand) {
				continue
			}
			cfg := module.MatchConfig{Options: module.Options{OptHandSize: hand}}
			for seed := int64(1); seed <= 60; seed++ {
				name := fmt.Sprintf("%d seats x %d, seed %d", seats, hand, seed)
				raw, err := m.NewMatch(cfg, refs(ids[:seats]...), seed)
				if err != nil {
					t.Fatalf("%s: NewMatch: %v", name, err)
				}
				rng := rand.New(rand.NewSource(seed * 31))
				hoarder := ids[int(seed)%seats]
				allHoard := seed%2 == 1
				ended := false
				for step := 0; step < 20000; step++ {
					s := stateOf(t, raw)
					if s.Status != "active" {
						ended = true
						break
					}
					offers, err := m.LegalActions(raw, s.Current)
					if err != nil {
						t.Fatalf("%s: LegalActions: %v", name, err)
					}
					// Strict hoarding by every seat can cycle for ever (one plays,
					// the next draws it back) — a choice of the players, not a
					// dead position — so a table of hoarders sometimes plays.
					hoards := s.Current == hoarder || (allHoard && rng.Intn(5) != 0)
					a, ok := simPick(rng, offers, hoards)
					if !ok {
						t.Fatalf("%s step %d: %s has no enabled offer (hand %v, top %s, draw %d, discard %d)",
							name, step, s.Current, s.Hands[s.Current], s.top(), len(s.DrawPile), len(s.DiscardPile))
					}
					next, events, err := m.Apply(raw, s.Current, a)
					if err != nil {
						t.Fatalf("%s step %d: offered %v refused: %v", name, step, a, err)
					}
					if a.Verb == VerbDraw && intOf(events[0].Data["count"]) == 0 {
						emptyDraws++
					}
					raw = next
				}
				if !ended {
					t.Fatalf("%s: did not end in 20000 actions", name)
				}
				done, winners, _ := m.Finished(raw)
				if !done || len(winners) == 0 {
					t.Fatalf("%s: finished=%v winners=%v", name, done, winners)
				}
				if s := stateOf(t, raw); len(s.Hands[winners[0]]) != 0 {
					blocked++
				}
				games++
			}
		}
	}
	t.Logf("%d games, %d ended blocked, %d empty draws", games, blocked, emptyDraws)
	// Blocked endings are rare in play (every seat must be out of the suit
	// at once); the Blocked tests above construct them. Empty draws are the
	// position that used to strand the table, and the sweep must reach them.
	if emptyDraws == 0 {
		t.Fatal("the sweep never reached an empty pile: it tests nothing")
	}
}

// simPick chooses among enabled offers: a hoarder draws whenever it can,
// everyone else plays when they can. Cards and suits are random.
func simPick(rng *rand.Rand, offers []module.ActionOffer, hoarder bool) (module.Action, bool) {
	byID := map[string]module.ActionOffer{}
	for _, o := range offers {
		if o.Enabled {
			byID[o.ID] = o
		}
	}
	order := []string{OfferPlay, OfferPass, OfferDraw}
	if hoarder {
		order = []string{OfferDraw, OfferPass, OfferPlay}
	}
	for _, id := range order {
		o, ok := byID[id]
		if !ok {
			continue
		}
		a := module.Action{Verb: o.Verb}
		if o.ID == OfferPlay {
			cards := o.Source.Cards
			a.Cards = []string{cards[rng.Intn(len(cards))]}
			if rankOf(a.Cards[0]) == rankWild {
				a.Params = map[string]string{"suit": suits[rng.Intn(len(suits))]}
			}
		}
		return a, true
	}
	return module.Action{}, false
}
