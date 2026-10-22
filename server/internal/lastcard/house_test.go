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

func TestASevenSwapsWithThePlayerItNames(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.SevenZero = true
		s.Hands["p1"] = []string{"T-7", "A-7", "A-8", "A-9"}
		s.Hands["p2"] = []string{"C-1", "C-2", "C-3"}
		s.Hands["p3"] = []string{"V-1"}
	})
	// Three at the table: whom to swap with is the player's to say.
	refused(t, raw, "p1", playCard("T-7"), ErrSwapWithWhom)
	refused(t, raw, "p1", withWhom(playCard("T-7"), "p1"), ErrSwapWithWhom)

	// The offer asks, starting on the shortest hand.
	play := offerIDs(t, raw, "p1")[OfferPlay]
	with := paramNamed(play, "with")
	if with == nil || with.Kind != module.ParamKindPlayer || with.DefaultChoice != "p3" || len(with.Choices) != 2 {
		t.Fatalf("with param %+v; want a player choice of two, starting on p3", with)
	}

	// Not the shortest: the player may choose anyone.
	s := stateOf(t, apply(t, raw, "p1", withWhom(playCard("T-7"), "p2")))
	if len(s.Hands["p1"]) != 3 || s.Hands["p1"][0] != "C-1" || len(s.Hands["p2"]) != 3 || s.Hands["p2"][0] != "A-7" {
		t.Errorf("after the 7: p1 %v, p2 %v", s.Hands["p1"], s.Hands["p2"])
	}
}

func TestBetweenTwoASwapNeedsNoName(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.SevenZero = true
		s.Players, s.TurnOrder = []string{"p1", "p2"}, []string{"p1", "p2"}
		delete(s.Hands, "p3")
		s.Hands["p1"] = []string{"T-7", "A-7", "A-8", "A-9"}
		s.Hands["p2"] = []string{"C-1"}
	})
	if with := paramNamed(offerIDs(t, raw, "p1")[OfferPlay], "with"); with != nil {
		t.Errorf("two-handed, the offer still asks whom: %+v", with)
	}
	s := stateOf(t, apply(t, raw, "p1", playCard("T-7")))
	if len(s.Hands["p1"]) != 1 || len(s.Hands["p2"]) != 3 {
		t.Errorf("after the 7: p1 %v, p2 %v", s.Hands["p1"], s.Hands["p2"])
	}
}

// --- the Wild Swap and the Wild Mayhem -------------------------------------------------

func TestTheHouseWildsJoinThePackOnlyWhereTheyArePlayed(t *testing.T) {
	count := func(deck []string, card string) int {
		n := 0
		for _, c := range deck {
			if c == card {
				n++
			}
		}
		return n
	}
	if d := buildDeck(false, false); count(d, cardWildSwap)+count(d, cardWildMayhem) != 0 {
		t.Errorf("a plain pack holds house wilds")
	}
	d := buildDeck(true, true)
	if len(d) != 112 || count(d, cardWildSwap) != 2 || count(d, cardWildMayhem) != 2 {
		t.Errorf("pack of %d with %d swaps and %d mayhems; want 112, 2, 2", len(d), count(d, cardWildSwap), count(d, cardWildMayhem))
	}
	raw, err := New().NewMatch(module.MatchConfig{Options: module.Options{OptSwapCards: module.OptOn}}, players("a", "b"), 3)
	if err != nil {
		t.Fatal(err)
	}
	s := stateOf(t, raw)
	n := len(s.DrawPile) + len(s.DiscardPile) + len(s.Hands["a"]) + len(s.Hands["b"])
	if n != 110 || !s.SwapCards || s.MayhemCards {
		t.Errorf("a swap table deals %d cards (swap %v, mayhem %v); want 110", n, s.SwapCards, s.MayhemCards)
	}
}

func TestAWildSwapNamesAColourAndTradesHands(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.SwapCards = true
		s.Hands["p1"] = []string{cardWildSwap, "A-7", "A-8", "A-9"}
		s.Hands["p2"] = []string{"C-1", "C-2", "C-3"}
		s.Hands["p3"] = []string{"V-1"}
	})
	refused(t, raw, "p1", withWhom(playCard(cardWildSwap), ""), ErrSwapWithWhom)
	refused(t, raw, "p1", module.Action{Verb: VerbPlay, Cards: []string{cardWildSwap}, Params: map[string]string{"with": "p3"}}, ErrColourRequired)

	play := offerIDs(t, raw, "p1")[OfferPlay]
	if colour := paramNamed(play, "colour"); colour == nil || !hasCard(colour.Cards, cardWildSwap) {
		t.Errorf("the swap is not asked a colour: %+v", colour)
	}
	if with := paramNamed(play, "with"); with == nil || len(with.Cards) != 1 || with.Cards[0] != cardWildSwap {
		t.Errorf("the swap is not asked whom: %+v", with)
	}

	raw, events := applyEvents(t, raw, "p1", withWhom(playWild(cardWildSwap, "V"), "p3"))
	s := stateOf(t, raw)
	if len(s.Hands["p1"]) != 1 || s.Hands["p1"][0] != "V-1" || len(s.Hands["p3"]) != 3 {
		t.Errorf("after the swap: p1 %v, p3 %v", s.Hands["p1"], s.Hands["p3"])
	}
	if s.DeclaredColour != "V" || s.Current != "p2" {
		t.Errorf("colour %q, %s to move; want V, p2", s.DeclaredColour, s.Current)
	}
	// A hand that arrives with one card could not have been called for.
	if s.Unannounced != "" {
		t.Errorf("p1 is catchable for a card they were handed")
	}
	if !hasEvent(events, "hands_swapped") {
		t.Errorf("no hands_swapped event: %v", events)
	}
}

func TestAWildMayhemDealsEveryHandOutAgainEvenly(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.MayhemCards = true
		s.Hands["p1"] = []string{cardWildMayhem, "A-1", "A-2", "A-3", "A-4", "A-5", "A-6", "A-7", "A-8"}
		s.Hands["p2"] = []string{"C-1", "C-2"}
		s.Hands["p3"] = []string{"V-1", "V-2", "V-3"}
	})
	raw, events := applyEvents(t, raw, "p1", playWild(cardWildMayhem, "T"))
	s := stateOf(t, raw)
	// Thirteen cards dealt from p2: p2 and p3 take five and four, p1 four.
	got := []int{len(s.Hands["p1"]), len(s.Hands["p2"]), len(s.Hands["p3"])}
	if got[0] != 4 || got[1] != 5 || got[2] != 4 {
		t.Errorf("hands after the mayhem %v; want [4 5 4]", got)
	}
	seen := map[string]bool{}
	for _, p := range s.TurnOrder {
		for _, c := range s.Hands[p] {
			seen[c] = true
		}
	}
	if len(seen) != 13 || seen[cardWildMayhem] {
		t.Errorf("the redeal holds %d distinct cards (the mayhem among them: %v); want the 13 held", len(seen), seen[cardWildMayhem])
	}
	if s.DeclaredColour != "T" || s.Current != "p2" || s.Mayhems != 1 || !hasEvent(events, "hands_shuffled") {
		t.Errorf("colour %q, %s to move, %d mayhems, events %v", s.DeclaredColour, s.Current, s.Mayhems, events)
	}
}

func TestAHouseWildPlayedLastGoesOut(t *testing.T) {
	for _, card := range []string{cardWildSwap, cardWildMayhem} {
		raw := withState(t, func(s *GameState) {
			s.SwapCards, s.MayhemCards = true, true
			s.Hands["p1"] = []string{card}
		})
		s := stateOf(t, apply(t, raw, "p1", withWhom(playWild(card, "C"), "p2")))
		if len(s.Deals) != 1 || s.Deals[0].Winner != "p1" {
			t.Errorf("%s as the last card: deals %+v", card, s.Deals)
		}
	}
}

func TestBotsSwapDownAndKeepAMayhemThatWouldNotHelp(t *testing.T) {
	// Long hand, short opponent: the swap goes first, aimed at p3.
	raw := withState(t, func(s *GameState) {
		s.SwapCards = true
		s.Hands["p1"] = []string{cardWildSwap, "T-1", "T-2", "T-3", "T-4", "T-6"}
		s.Hands["p3"] = []string{"V-1"}
	})
	offers, _ := New().LegalActions(raw, "p1")
	a, _ := bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillHard}, offers)
	if len(a.Cards) != 1 || a.Cards[0] != cardWildSwap || a.Params["with"] != "p3" {
		t.Errorf("hard bot played %v with %q; want the swap with p3", a.Cards, a.Params["with"])
	}

	// Short hand: a mayhem would only hand it cards, so it draws instead.
	raw = withState(t, func(s *GameState) {
		s.MayhemCards = true
		s.Hands["p1"] = []string{cardWildMayhem, "A-1"}
		s.Hands["p2"] = []string{"C-1", "C-2", "C-3", "C-4", "C-6"}
		s.Hands["p3"] = []string{"V-1", "V-2", "V-3", "V-4", "V-6"}
	})
	offers, _ = New().LegalActions(raw, "p1")
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		a, _ = bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: skill}, offers)
		if a.Verb != VerbDraw {
			t.Errorf("%s bot played %v %v; want a draw", skill, a.Verb, a.Cards)
		}
	}
}

func withWhom(a module.Action, with string) module.Action {
	if a.Params == nil {
		a.Params = map[string]string{}
	}
	a.Params["with"] = with
	return a
}

func paramNamed(o module.ActionOffer, name string) *module.ParamSpec {
	for i := range o.Params {
		if o.Params[i].Name == name {
			return &o.Params[i]
		}
	}
	return nil
}

func applyEvents(t *testing.T, raw module.State, pid string, a module.Action) (module.State, []module.Event) {
	t.Helper()
	next, events, err := New().Apply(raw, pid, a)
	if err != nil {
		t.Fatalf("%s %v: %v", pid, a, err)
	}
	return next, events
}

func hasEvent(events []module.Event, typ string) bool {
	for _, e := range events {
		if e.Type == typ {
			return true
		}
	}
	return false
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
		OptSwapCards: module.OptOn, OptMayhemCards: module.OptOn, OptTargetScore: 200,
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
	for _, id := range []string{"lastcard.rules.stacking.any", "lastcard.rules.sevens", "lastcard.rules.turn.drawUntil",
		"lastcard.rules.wildSwap", "lastcard.rules.wildMayhem"} {
		if plain[id] {
			t.Errorf("a default table states %s", id)
		}
	}
	house := module.StatedRuleIDs(New(), module.MatchConfig{Options: module.Options{
		OptStacking: stackAny, OptDrawUntilPlayable: module.OptOn, OptSevenZero: module.OptOn,
		OptSwapCards: module.OptOn, OptMayhemCards: module.OptOn,
	}})
	for _, id := range []string{"lastcard.rules.stacking.any", "lastcard.rules.sevens", "lastcard.rules.zeros", "lastcard.rules.turn.drawUntil",
		"lastcard.rules.wildSwap", "lastcard.rules.wildMayhem"} {
		if !house[id] {
			t.Errorf("a house-rules table does not state %s", id)
		}
	}
}
