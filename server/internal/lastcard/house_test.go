package lastcard

import (
	"testing"

	"zolik/server/internal/module"
)

// --- stacking ---------------------------------------------------------------------

func TestStackingPassesTheDrawOn(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Stacking = stackTwos
		s.Hands["p1"] = []string{"T-D", "A-7"}
		s.Hands["p2"] = []string{"C-D", "V-3"}
	})
	raw = apply(t, raw, "p1", playCard("T-D"))
	s := stateOf(t, raw)
	if s.PendingDraw != 2 || s.Current != "p2" || len(s.Hands["p2"]) != 2 {
		t.Fatalf("pending %d, %s to move, p2 holds %d", s.PendingDraw, s.Current, len(s.Hands["p2"]))
	}
	refused(t, raw, "p2", playCard("V-3"), ErrMustAnswerDraw)
	if o := offerIDs(t, raw, "p2")[OfferDraw]; o.LabelKey != "lastcard.offer.takeStack" {
		t.Errorf("the draw offer reads %q under a stack", o.LabelKey)
	}
	raw = apply(t, raw, "p2", playCard("C-D"))
	raw = apply(t, raw, "p3", draw)
	s = stateOf(t, raw)
	if len(s.Hands["p3"]) != 7 || s.PendingDraw != 0 || s.Current != "p1" {
		t.Errorf("p3 holds %d, pending %d, %s to move; want 7 (3+4), 0, p1",
			len(s.Hands["p3"]), s.PendingDraw, s.Current)
	}
}

func TestDrawTwoOnlyStacksTwosUnlessAnyDrawStacks(t *testing.T) {
	twos := withState(t, func(s *GameState) {
		s.Stacking = stackTwos
		s.DiscardPile = []string{"T-D"}
		s.PendingDraw = 2
		s.Hands["p1"] = []string{cardWildDrawFour, "A-7"}
	})
	refused(t, twos, "p1", playWild(cardWildDrawFour, "A"), ErrMustAnswerDraw)

	any := withState(t, func(s *GameState) {
		s.Stacking = stackAny
		s.DiscardPile = []string{"T-D"}
		s.PendingDraw = 2
		s.Hands["p1"] = []string{cardWildDrawFour, "T-9"}
	})
	// Holding teal does not make it a bluff: stacked, it is an answer.
	s := stateOf(t, apply(t, any, "p1", playWild(cardWildDrawFour, "A")))
	if s.PendingDraw != 6 || s.DrawFour != nil {
		t.Errorf("pending %d, challenge %+v; want 6 and none", s.PendingDraw, s.DrawFour)
	}

	// Never a Draw Two on a Wild Draw Four.
	onFour := withState(t, func(s *GameState) {
		s.Stacking = stackAny
		s.DiscardPile = []string{"T-5", cardWildDrawFour}
		s.DeclaredColour = "T"
		s.PendingDraw = 4
		s.Hands["p1"] = []string{"T-D", "A-7"}
	})
	refused(t, onFour, "p1", playCard("T-D"), ErrMustAnswerDraw)
}

func TestAStackedDrawFourCanStillBeChallengedWhenItStartsTheStack(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Stacking = stackAny
		s.ChallengeOn = true
		s.Hands["p1"] = []string{cardWildDrawFour, "T-9", "A-7"}
		s.Hands["p2"] = []string{cardWildDrawFour, "V-3"}
	})
	raw = apply(t, raw, "p1", playWild(cardWildDrawFour, "A"))
	s := stateOf(t, raw)
	if s.DrawFour == nil || s.PendingDraw != 4 {
		t.Fatalf("challenge %+v, pending %d", s.DrawFour, s.PendingDraw)
	}
	// p2 may stack instead of answering — and that ends the challenge.
	stacked := stateOf(t, apply(t, raw, "p2", playWild(cardWildDrawFour, "V")))
	if stacked.DrawFour != nil || stacked.PendingDraw != 8 || stacked.Current != "p3" {
		t.Errorf("after the stack: challenge %+v, pending %d, %s to move", stacked.DrawFour, stacked.PendingDraw, stacked.Current)
	}
	// Or challenge it: p1 was bluffing (they held teal), so p1 draws four.
	caught := stateOf(t, apply(t, raw, "p2", challenge))
	if len(caught.Hands["p1"]) != 6 || caught.PendingDraw != 0 || caught.Current != "p2" {
		t.Errorf("after the challenge: p1 %d, pending %d, %s to move", len(caught.Hands["p1"]), caught.PendingDraw, caught.Current)
	}
}

// --- draw until playable -----------------------------------------------------------

func TestDrawUntilPlayableStopsOnTheFirstFit(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DrawUntil = true
		s.Hands["p1"] = []string{"A-7"}
		// Drawn from the end: C-1, then C-2, then T-3, which fits teal.
		s.DrawPile = []string{"C-6", "T-3", "C-2", "C-1"}
	})
	s := stateOf(t, apply(t, raw, "p1", draw))
	if len(s.Hands["p1"]) != 4 || s.DrawnCard != "T-3" || s.Current != "p1" {
		t.Errorf("p1 holds %d, drawn %q, %s to move; want 4, T-3, p1", len(s.Hands["p1"]), s.DrawnCard, s.Current)
	}
}

// --- sevens and zeros ----------------------------------------------------------------

func TestASevenSwapsWithTheShortestHand(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.SevenZero = true
		s.Hands["p1"] = []string{"T-7", "A-7", "A-8", "A-9"}
		s.Hands["p2"] = []string{"C-1", "C-2", "C-3"}
		s.Hands["p3"] = []string{"V-1"}
	})
	s := stateOf(t, apply(t, raw, "p1", playCard("T-7")))
	if len(s.Hands["p1"]) != 1 || s.Hands["p1"][0] != "V-1" || len(s.Hands["p3"]) != 3 {
		t.Errorf("after the 7: p1 %v, p3 %v", s.Hands["p1"], s.Hands["p3"])
	}
}

func TestAZeroPassesEveryHandOn(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.SevenZero = true
		s.Hands["p1"] = []string{"T-0", "A-7"}
		s.Hands["p2"] = []string{"C-1", "C-2"}
		s.Hands["p3"] = []string{"V-1", "V-2", "V-3"}
	})
	s := stateOf(t, apply(t, raw, "p1", playCard("T-0")))
	if len(s.Hands["p2"]) != 1 || len(s.Hands["p3"]) != 2 || len(s.Hands["p1"]) != 3 {
		t.Errorf("after the 0: p1 %v, p2 %v, p3 %v", s.Hands["p1"], s.Hands["p2"], s.Hands["p3"])
	}
}

// --- the bots play every house rule to the end ----------------------------------------

func TestBotsPlayEveryHouseRule(t *testing.T) {
	m := New()
	ps := players("p1", "p2", "p3")
	cfg := module.MatchConfig{Options: module.Options{
		OptStacking: stackAny, OptDrawUntilPlayable: module.OptOn, OptSevenZero: module.OptOn,
		OptTargetScore: 200,
	}}
	skills := map[string]module.Skill{"p1": module.SkillEasy, "p2": module.SkillMedium, "p3": module.SkillHard}
	for seed := int64(1); seed <= 8; seed++ {
		state, err := m.NewMatch(cfg, ps, seed)
		if err != nil {
			t.Fatal(err)
		}
		for step := 0; ; step++ {
			if step > 40000 {
				t.Fatalf("seed %d: no finish", seed)
			}
			if done, _, _ := m.Finished(state); done {
				break
			}
			s, _ := decode(state)
			actor := s.Current
			if s.Intermission.Open {
				actor = s.Intermission.Waiting(s.TurnOrder)[0]
			}
			offers, _ := m.LegalActions(state, actor)
			a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor]}, offers)
			if !ok {
				t.Fatalf("seed %d: %s had no move", seed, actor)
			}
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("seed %d: %s proposed %v: %v", seed, actor, a, err)
			}
			state = next
		}
	}
}

func TestHouseRulesAreStatedOnlyWhereTheyArePlayed(t *testing.T) {
	plain := module.StatedRuleIDs(New(), module.MatchConfig{})
	for _, id := range []string{"lastcard.rules.stacking.any", "lastcard.rules.sevens", "lastcard.rules.turn.drawUntil"} {
		if plain[id] {
			t.Errorf("a default table states %s", id)
		}
	}
	house := module.StatedRuleIDs(New(), module.MatchConfig{Options: module.Options{
		OptStacking: stackAny, OptDrawUntilPlayable: module.OptOn, OptSevenZero: module.OptOn,
	}})
	for _, id := range []string{"lastcard.rules.stacking.any", "lastcard.rules.sevens", "lastcard.rules.zeros", "lastcard.rules.turn.drawUntil"} {
		if !house[id] {
			t.Errorf("a house-rules table does not state %s", id)
		}
	}
}
