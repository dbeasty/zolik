package lastcard

import (
	"testing"

	"zolik/server/internal/module"
)

var (
	call      = module.Action{Verb: VerbCall}
	catch     = module.Action{Verb: VerbCatch}
	challenge = module.Action{Verb: VerbChallenge}
	accept    = module.Action{Verb: VerbAccept}
	draw      = module.Action{Verb: VerbDraw}
)

func offerIDs(t *testing.T, raw module.State, pid string) map[string]module.ActionOffer {
	t.Helper()
	offers, err := New().LegalActions(raw, pid)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]module.ActionOffer{}
	for _, o := range offers {
		out[o.ID] = o
	}
	return out
}

// --- "Last card!" --------------------------------------------------------------

func TestCallingBeforeTheLastButOnePlayIsSafe(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.CallOn = true
		s.Hands["p1"] = []string{"T-9", "A-7"}
	})
	if o, ok := offerIDs(t, raw, "p1")[OfferCall]; !ok || !o.Enabled {
		t.Fatalf("no enabled call offer with two cards in hand: %+v", o)
	}
	raw = apply(t, raw, "p1", call)
	refused(t, raw, "p1", call, ErrAlreadyCalled)
	raw = apply(t, raw, "p1", playCard("T-9"))
	s := stateOf(t, raw)
	if s.Unannounced != "" {
		t.Errorf("p1 called and is still catchable: %q", s.Unannounced)
	}
	if _, ok := offerIDs(t, raw, "p2")[OfferCatch]; ok {
		t.Error("p2 is offered a catch on a player who called")
	}
	vm, _ := New().View(raw, "p2")
	if !hasBadge(vm, "p1", "lastcard.seat.lastCard") {
		t.Error("p1 called and wears no last-card badge")
	}
}

func TestASilentLastCardCanBeCaughtForTwo(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.CallOn = true
		s.Hands["p1"] = []string{"T-9", "A-7"}
	})
	raw = apply(t, raw, "p1", playCard("T-9"))
	if s := stateOf(t, raw); s.Unannounced != "p1" {
		t.Fatalf("unannounced = %q, want p1", s.Unannounced)
	}
	vm, _ := New().View(raw, "p2")
	if hasBadge(vm, "p1", "lastcard.seat.lastCard") {
		t.Error("a silent last card wears the badge")
	}
	if o, ok := offerIDs(t, raw, "p2")[OfferCatch]; !ok || !o.Enabled {
		t.Fatalf("p2 has no enabled catch: %+v", o)
	}
	caught := stateOf(t, apply(t, raw, "p2", catch))
	if len(caught.Hands["p1"]) != 3 || caught.Unannounced != "" || caught.Current != "p2" {
		t.Errorf("after the catch p1 holds %d, unannounced %q, %s to move; want 3, none, p2",
			len(caught.Hands["p1"]), caught.Unannounced, caught.Current)
	}
}

func TestTheCatchWindowClosesOnTheNextMove(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.CallOn = true
		s.Hands["p1"] = []string{"T-9", "A-7"}
	})
	raw = apply(t, raw, "p1", playCard("T-9"))
	raw = apply(t, raw, "p2", draw)
	if s := stateOf(t, raw); s.Unannounced != "" {
		t.Errorf("the window stayed open after p2 drew: %q", s.Unannounced)
	}
}

func TestNoCallWhereTheTableDoesNotPlayIt(t *testing.T) {
	raw := withState(t, func(s *GameState) { s.Hands["p1"] = []string{"T-9", "A-7"} })
	if _, ok := offerIDs(t, raw, "p1")[OfferCall]; ok {
		t.Error("a call offered at a table without the call")
	}
	refused(t, raw, "p1", call, ErrCallNotNow)
	raw = apply(t, raw, "p1", playCard("T-9"))
	if s := stateOf(t, raw); s.Unannounced != "" {
		t.Error("a silent last card is catchable at a table without the call")
	}
}

// --- the Wild Draw Four challenge ------------------------------------------------

func challengeTable(t *testing.T, p1 []string) module.State {
	return withState(t, func(s *GameState) {
		s.ChallengeOn = true
		s.Hands["p1"] = p1
	})
}

func TestABluffIsAllowedAndWaitsOnTheVictim(t *testing.T) {
	raw := challengeTable(t, []string{cardWildDrawFour, "T-9", "A-7"})
	raw = apply(t, raw, "p1", playWild(cardWildDrawFour, "A"))
	s := stateOf(t, raw)
	if s.DrawFour == nil || s.Current != "p2" || len(s.Hands["p2"]) != 3 {
		t.Fatalf("pending %+v, %s to move, p2 holds %d", s.DrawFour, s.Current, len(s.Hands["p2"]))
	}
	refused(t, raw, "p2", draw, ErrAnswerDrawFour)
	refused(t, raw, "p2", playCard("A-1"), ErrAnswerDrawFour)
	o := offerIDs(t, raw, "p2")
	if !o[OfferChallenge].Enabled || !o[OfferAccept].Enabled {
		t.Errorf("challenge/accept not both enabled: %+v", o)
	}
}

func TestACaughtBluffCostsTheBluffer(t *testing.T) {
	raw := challengeTable(t, []string{cardWildDrawFour, "T-9", "A-7"})
	raw = apply(t, raw, "p1", playWild(cardWildDrawFour, "A"))
	raw = apply(t, raw, "p2", challenge)
	s := stateOf(t, raw)
	if len(s.Hands["p1"]) != 6 || len(s.Hands["p2"]) != 3 || s.Current != "p2" {
		t.Errorf("p1 %d, p2 %d, %s to move; want 6, 3 and p2 playing on",
			len(s.Hands["p1"]), len(s.Hands["p2"]), s.Current)
	}
	// The challenger sees the hand; nobody else does.
	for _, viewer := range []string{"p2", "p3", "p1"} {
		vm, _ := New().View(raw, viewer)
		shown := hasPrompt(vm, "lastcard.prompt.bluffCaught")
		if shown != (viewer == "p2") {
			t.Errorf("%s sees the challenged hand: %v", viewer, shown)
		}
	}
}

func TestAnHonestDrawFourCostsTheChallengerSix(t *testing.T) {
	raw := challengeTable(t, []string{cardWildDrawFour, "V-9", "A-7"})
	raw = apply(t, raw, "p1", playWild(cardWildDrawFour, "A"))
	raw = apply(t, raw, "p2", challenge)
	s := stateOf(t, raw)
	if len(s.Hands["p2"]) != 9 || s.Current != "p3" {
		t.Errorf("p2 holds %d, %s to move; want 9 and p3", len(s.Hands["p2"]), s.Current)
	}
	vm, _ := New().View(raw, "p2")
	if !hasPrompt(vm, "lastcard.prompt.noBluff") {
		t.Error("the challenger was not shown the honest hand")
	}
}

func TestAcceptingADrawFour(t *testing.T) {
	raw := challengeTable(t, []string{cardWildDrawFour, "V-9", "A-7"})
	raw = apply(t, raw, "p1", playWild(cardWildDrawFour, "A"))
	s := stateOf(t, apply(t, raw, "p2", accept))
	if len(s.Hands["p2"]) != 7 || s.Current != "p3" || s.DrawFour != nil {
		t.Errorf("p2 holds %d, %s to move, pending %+v", len(s.Hands["p2"]), s.Current, s.DrawFour)
	}
	refused(t, apply(t, raw, "p2", accept), "p3", challenge, ErrNoDrawFour)
}

// --- scoring across deals ----------------------------------------------------------

func TestADealScoresTheCardsLeftInOtherHands(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.TargetScore = 500
		s.Pause = true
		s.Scores = map[string]int{"p1": 0, "p2": 0, "p3": 0}
		s.Hands["p1"] = []string{"T-9"}
		s.Hands["p2"] = []string{"C-8", "V-S", cardWild}         // 8 + 20 + 50
		s.Hands["p3"] = []string{"T-1", cardWildDrawFour, "A-D"} // 1 + 50 + 20
	})
	raw = apply(t, raw, "p1", playCard("T-9"))
	s := stateOf(t, raw)
	if s.Status != "active" || !s.Intermission.Open {
		t.Fatalf("status %s, paused %v; want an active match paused between deals", s.Status, s.Intermission.Open)
	}
	if s.Scores["p1"] != 149 {
		t.Errorf("p1 scored %d, want 149", s.Scores["p1"])
	}
	log, err := New().Rounds(raw)
	if err != nil || len(log.Rounds) != 1 {
		t.Fatalf("rounds %+v, %v", log, err)
	}
	for _, rs := range log.Rounds[0].Scores {
		if err := module.CheckLines(rs); err != nil {
			t.Error(err)
		}
	}
	// Everyone says go on, and the next deal is dealt.
	for _, p := range []string{"p1", "p2", "p3"} {
		raw = apply(t, raw, p, module.Action{Verb: module.VerbContinue})
	}
	s = stateOf(t, raw)
	if s.Intermission.Open || s.DealNumber != 1 || len(s.Hands["p1"]) < defaultHandSize {
		t.Errorf("after continuing: paused %v, deal %d, p1 holds %d", s.Intermission.Open, s.DealNumber, len(s.Hands["p1"]))
	}
}

func TestReachingTheTargetEndsTheMatch(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.TargetScore = 200
		s.Scores = map[string]int{"p1": 190, "p2": 40, "p3": 0}
		s.Hands["p1"] = []string{"T-9"}
	})
	raw = apply(t, raw, "p1", playCard("T-9"))
	done, winners, _ := New().Finished(raw)
	if !done || len(winners) != 1 || winners[0] != "p1" {
		t.Errorf("finished %v winners %v", done, winners)
	}
	st, _ := New().Standings(raw)
	if st[0].PlayerID != "p1" || st[0].Score < 200 {
		t.Errorf("standings lead %+v", st[0])
	}
}

func TestAFullMatchReachesItsTarget(t *testing.T) {
	m := New()
	ps := players("p1", "p2", "p3")
	cfg := module.MatchConfig{Options: module.Options{OptTargetScore: 200}}
	state, err := m.NewMatch(cfg, ps, 11)
	if err != nil {
		t.Fatal(err)
	}
	_, res, err := module.PlayWithOffers(m, state, ps, module.DriverOptions{
		MaxActions: 20000,
		Prefer:     []string{module.VerbContinue, VerbAccept, VerbCatch, VerbCall, VerbPlay, VerbPass, VerbDraw},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Finished {
		t.Fatalf("no finish in %d actions", res.Actions)
	}
}

// --- the written rules follow the table ---------------------------------------------

func TestRulesStateOnlyWhatTheTablePlays(t *testing.T) {
	on := module.StatedRuleIDs(New(), module.MatchConfig{})
	for _, id := range []string{"lastcard.rules.call", "lastcard.rules.challenge", "lastcard.rules.scoring"} {
		if !on[id] {
			t.Errorf("default table does not state %s", id)
		}
	}
	off := module.StatedRuleIDs(New(), module.MatchConfig{Options: module.Options{
		OptLastCardCall: module.OptOff, OptDrawFourChallenge: module.OptOff, OptTargetScore: 0,
	}})
	for _, id := range []string{"lastcard.rules.call", "lastcard.rules.challenge", "lastcard.rules.scoring"} {
		if off[id] {
			t.Errorf("a table without it states %s", id)
		}
	}
	if !off["lastcard.rules.wildDrawFour"] {
		t.Error("without the challenge the Wild Draw Four's restriction is not stated")
	}
}

func hasBadge(vm module.ViewModel, pid, key string) bool {
	for _, s := range vm.Seats {
		if s.PlayerID != pid {
			continue
		}
		for _, k := range s.LabelKeys {
			if k == key {
				return true
			}
		}
	}
	return false
}

func hasPrompt(vm module.ViewModel, key string) bool {
	for _, p := range vm.Prompts {
		if p.LabelKey == key {
			return true
		}
	}
	return false
}
