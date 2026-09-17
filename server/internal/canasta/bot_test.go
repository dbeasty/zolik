package canasta

import (
	"testing"

	"zolik/server/internal/module"
)

// botAct asks the module's own bot what it would do, through exactly the path
// the runtime uses: the offer list from LegalActions and nothing handed in
// beside it.
func botAct(t *testing.T, raw module.State, playerID string, skill module.Skill) module.Action {
	t.Helper()
	m := New()
	offers, err := m.LegalActions(raw, playerID)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: playerID, Skill: skill}, offers)
	if !ok {
		t.Fatalf("bot had no move for %s", playerID)
	}
	return a
}

// --- the wild cards ----------------------------------------------------------

// TestBotDoesNotDiscardAWildCard is the report this file was written for, at
// the exact spot it happened.
//
// The hand holds a deuce and nine ordinary cards. Card notation sorts digits
// ahead of letters, discardableCards returns its answer sorted, and
// module.SubmissionFor takes the front of that list — so the old
// offer-preference bot discarded the deuce, every turn it held one, ahead of
// every card it had no use for. A wild is twenty points thrown away, one of
// the eight cards that can finish a canasta gone, and a frozen pile handed to
// the table as a bonus.
func TestBotDoesNotDiscardAWildCard(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Hands["p1"] = []string{"2C", "JOKER1", "5H", "9D", "QS", "4C", "KH", "7D", "TS", "6C"}
		s.DiscardPile = []string{"8H"}
	})
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		got := botAct(t, raw, "p1", skill)
		if got.Verb != VerbDiscard {
			t.Fatalf("%s: %s, want a discard", skill, got.Verb)
		}
		if len(got.Cards) != 1 {
			t.Fatalf("%s: discarded %v, want exactly one card", skill, got.Cards)
		}
		if isWild(got.Cards[0]) {
			t.Errorf("%s discarded %s — a wild card, with nine ordinary cards in hand", skill, got.Cards[0])
		}
	}
}

// TestBotDiscardsAWildOnlyWhenItIsAllThereIs.
//
// The rule above must be a preference and not a refusal: a hand of nothing but
// wilds still has to end its turn, and a bot that will not name a card is a
// bot that wedges the table.
func TestBotDiscardsAWildOnlyWhenItIsAllThereIs(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Hands["p1"] = []string{"2C", "2D", "JOKER1"}
		s.DiscardPile = []string{"8H"}
	})
	got := botAct(t, raw, "p1", module.SkillHard)
	if got.Verb != VerbDiscard || len(got.Cards) != 1 {
		t.Fatalf("a hand of three wilds played %v, want a discard of one of them", got)
	}
}

// TestHardKeepsAWildOutOfASmallMeld.
//
// Three of a rank plus a wild is a legal meld and a bad one: it takes the card
// that could have been the seventh of something out of play to make a
// four-card meld worth forty points. The side already has a meld down, so
// there is nothing else the wild is buying.
func TestHardKeepsAWildOutOfASmallMeld(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KC", "KD"},
		}}
		s.Hands["p1"] = []string{"9H", "9C", "9D", "JOKER1", "5H", "4C", "7D"}
		s.DiscardPile = []string{"8H"}
	})
	got := botAct(t, raw, "p1", module.SkillHard)
	if got.Verb == VerbLayMeld {
		for _, c := range got.Cards {
			if isWild(c) {
				t.Errorf("hard laid %v, spending a wild on a meld four cards short of a canasta", got.Cards)
			}
		}
	}
}

// TestHardSpendsAWildToOpenTheAccount is the other half of the same rule: a
// side that has not melded at all is buying the right to lay off, to unfreeze
// the pile and to score anything whatever, and a wild is a fair price for it.
func TestHardSpendsAWildToOpenTheAccount(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].Score = 0 // a 15-point minimum
		s.Hands["p1"] = []string{"KH", "KC", "JOKER1", "5H", "4C", "7D"}
		s.DiscardPile = []string{"8H"}
	})
	got := botAct(t, raw, "p1", module.SkillHard)
	if got.Verb != VerbLayMeld {
		t.Errorf("a side with nothing down and a meldable pair played %v, want the initial meld", got.Verb)
	}
}

// --- the pile ----------------------------------------------------------------

// TestBotTakesAPileWorthTaking. Capturing the pile is the biggest single swing
// in this game, and the old bot took it only because take_pile happened to
// come before draw in a list of verbs.
func TestBotTakesAPileWorthTaking(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseDraw
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KC", "KD"},
		}}
		s.Hands["p1"] = []string{"9H", "9C", "5H", "4C", "7D"}
		s.DiscardPile = []string{"QH", "QC", "JD", "TS", "4H", "9D"}
	})
	if got := botAct(t, raw, "p1", module.SkillHard); got.Verb != VerbTakePile {
		t.Errorf("a pile of six with the top card pairable in hand: %s, want the pile", got.Verb)
	}
}

// TestBotDrawsRatherThanBuyASingleCard.
//
// The capture spends two naturals out of the hand to claim one card off the
// pile. A pile of one is not worth that, and a bot that reads only the offer
// list cannot tell the difference — take_pile is take_pile.
func TestBotDrawsRatherThanBuyASingleCard(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseDraw
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "K"), TeamID: 0, Rank: "K",
			Cards: []string{"KH", "KC", "KD"},
		}}
		s.Hands["p1"] = []string{"9H", "9C", "5H", "4C", "7D"}
		s.DiscardPile = []string{"9D"}
	})
	if got := botAct(t, raw, "p1", module.SkillHard); got.Verb != VerbDraw {
		t.Errorf("a pile of one: %s, want a draw", got.Verb)
	}
}

// --- reading the other side --------------------------------------------------

// TestMediumDoesNotFeedTheOpposingMeld.
//
// Both cards are equally useless to the hand holding them and the nine is the
// cheaper one, so the plain "throw the cheapest card" rule names it — into a
// side that has nines on the table and takes the whole pile with it.
func TestMediumDoesNotFeedTheOpposingMeld(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[1].HasMelded = true
		s.Teams[1].Melds = []Meld{{
			ID: meldID(1, "9"), TeamID: 1, Rank: "9",
			Cards: []string{"9H", "9C", "9S"},
		}}
		s.Hands["p1"] = []string{"9D", "4C", "KH", "QS", "TD"}
		s.DiscardPile = []string{"8H"}
	})
	got := botAct(t, raw, "p1", module.SkillMedium)
	if got.Verb != VerbDiscard {
		t.Fatalf("%s, want a discard", got.Verb)
	}
	if rankOf(got.Cards[0]) == "9" {
		t.Errorf("discarded %s into a side with nines on the table", got.Cards[0])
	}
}

// --- points against closing --------------------------------------------------

// behindOnPoints is a side that could go out this turn and should not: it holds
// a canasta, so the engine would allow it, and the other side has more on the
// table, so ending the deal now banks second place.
func behindOnPoints(mutate func(s *GameState)) module.State {
	return twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{
			{ID: meldID(0, "K"), TeamID: 0, Rank: "K",
				Cards: []string{"KH", "KC", "KD", "KS", "KH", "KC", "KD"}},
			{ID: meldID(0, "9"), TeamID: 0, Rank: "9",
				Cards: []string{"9H", "9C", "9S"}},
		}
		s.Teams[1].HasMelded = true
		s.Teams[1].Melds = []Meld{
			{ID: meldID(1, "A"), TeamID: 1, Rank: "A",
				Cards: []string{"AH", "AC", "AD", "AS", "AH", "AC", "AD"}},
			{ID: meldID(1, "Q"), TeamID: 1, Rank: "Q",
				Cards: []string{"QH", "QC", "QD", "QS", "QH", "QC", "QD"}},
		}
		s.Hands["p1"] = []string{"9D", "5C"}
		// Long enough that nobody is about to end the deal by themselves —
		// otherwise the right answer is to close and the test would be asking
		// a different question.
		s.Hands["p2"] = []string{"8H", "8C", "8D", "8S", "7H", "7C", "7D", "7S"}
		s.DiscardPile = []string{"4H"}
		if mutate != nil {
			mutate(s)
		}
	})
}

// TestHardDoesNotCloseWhileItIsBehind is the judgement an offer list cannot
// make.
//
// The nine goes onto the side's own meld and empties the hand down to the
// discard that ends the deal. Every rule says it is legal and the greedy
// reading says take it — and it hands the deal to a side with two canastas
// against this one's one, for a hundred points. The cards are still coming;
// the right move is to keep playing.
func TestHardDoesNotCloseWhileItIsBehind(t *testing.T) {
	got := botAct(t, behindOnPoints(nil), "p1", module.SkillHard)
	if got.Verb == VerbLayOff {
		t.Errorf("laid off %v to go out while two canastas behind", got.Cards)
	}
	if got.Verb != VerbDiscard {
		t.Errorf("%s, want the turn ended without going out", got.Verb)
	}
}

// TestHardClosesWhenItIsAhead is the same position with the tables turned, so
// the rule above is a judgement and not a refusal to ever finish a deal.
func TestHardClosesWhenItIsAhead(t *testing.T) {
	raw := behindOnPoints(func(s *GameState) {
		s.Teams[1].Melds = []Meld{{
			ID: meldID(1, "Q"), TeamID: 1, Rank: "Q",
			Cards: []string{"QH", "QC", "QD"},
		}}
	})
	if got := botAct(t, raw, "p1", module.SkillHard); got.Verb != VerbLayOff {
		t.Errorf("%s while a canasta ahead, want the lay-off that ends the deal", got.Verb)
	}
}

// TestHardShedsTheExpensiveCardWhenTheDealIsEnding.
//
// Ordinarily the cheap card goes and the ace is kept to be melded. Once an
// opponent is two cards from out, the ace is not material any more — it is
// twenty points about to be subtracted from this side's deal, and it has one
// turn left to leave.
func TestHardShedsTheExpensiveCardWhenTheDealIsEnding(t *testing.T) {
	hand := []string{"AS", "4C", "5H", "6D"}
	quiet := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "K"), TeamID: 0, Rank: "K", Cards: []string{"KH", "KC", "KD"},
		}}
		s.Hands["p1"] = hand
		s.Hands["p2"] = []string{"8H", "8C", "8D", "8S", "7H", "7C"}
		s.DiscardPile = []string{"3H"}
	})
	pressed := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].HasMelded = true
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "K"), TeamID: 0, Rank: "K", Cards: []string{"KH", "KC", "KD"},
		}}
		s.Hands["p1"] = hand
		s.Hands["p2"] = []string{"8H", "8C"}
		s.DiscardPile = []string{"3H"}
	})

	calm := botAct(t, quiet, "p1", module.SkillHard)
	if calm.Verb != VerbDiscard || cardValue(calm.Cards[0]) != 5 {
		t.Errorf("with the deal still young: discarded %v, want a five-point card", calm.Cards)
	}
	urgent := botAct(t, pressed, "p1", module.SkillHard)
	if urgent.Verb != VerbDiscard || urgent.Cards[0] != "AS" {
		t.Errorf("with an opponent two cards from out: discarded %v, want the ace out of hand", urgent.Cards)
	}
}

// TestBotDoesNotPeek.
//
// The bot is handed the whole state — every seat's hand — because that is what
// the runtime has, and it reads opponents' hand *sizes* to know when a deal is
// ending. Nothing but this test stops it reading the cards in them, and a bot
// that quietly did would be undetectable from the outside and would ruin the
// game.
func TestBotDoesNotPeek(t *testing.T) {
	build := func(theirs []string) module.State {
		return twoHanded(func(s *GameState) {
			s.Phase = phaseMeld
			s.Teams[0].HasMelded = true
			s.Teams[0].Melds = []Meld{{
				ID: meldID(0, "K"), TeamID: 0, Rank: "K", Cards: []string{"KH", "KC", "KD"},
			}}
			s.Hands["p1"] = []string{"AS", "4C", "5H", "6D", "2C"}
			s.Hands["p2"] = theirs
			s.DiscardPile = []string{"3H"}
		})
	}
	honest := botAct(t, build([]string{"8H", "8C", "8D", "8S", "7H"}), "p1", module.SkillHard)
	// Same number of cards, entirely different cards — including the ones that
	// would change every decision if the bot could see them.
	rigged := botAct(t, build([]string{"AH", "AC", "AD", "JOKER1", "JOKER2"}), "p1", module.SkillHard)
	if honest.Verb != rigged.Verb || len(honest.Cards) != len(rigged.Cards) {
		t.Fatalf("played %v against one hand and %v against another of the same size", honest, rigged)
	}
	for i := range honest.Cards {
		if honest.Cards[i] != rigged.Cards[i] {
			t.Fatalf("played %v against one hand and %v against another of the same size", honest, rigged)
		}
	}
}

// --- whole deals -------------------------------------------------------------

// TestBotPlaysWholeDealsLegally.
//
// Every action goes through Apply, so an illegal one fails here rather than
// being quietly swapped for a legal one by the runtime's recovery path — which
// is the only way to find out whether the bot's own moves are legal or whether
// it has been living off that safety net. Both table sizes and every strength,
// because the skill dial changes which offers get taken and a profile that
// composes an illegal one would otherwise only show up in production.
func TestBotPlaysWholeDealsLegally(t *testing.T) {
	m := New()
	wedged := 0
	for _, seats := range []int{2, 4} {
		ids := make([]string, 0, seats)
		for i := 1; i <= seats; i++ {
			ids = append(ids, "p"+string(rune('0'+i)))
		}
		players := make([]module.PlayerRef, 0, len(ids))
		for _, id := range ids {
			players = append(players, module.PlayerRef{ID: id})
		}
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			for seed := int64(1); seed <= 3; seed++ {
				// A short target, so a match is a handful of deals rather
				// than the twenty-odd a five-thousand-point game takes: this
				// is asserting legality per action, and more deals buys more
				// runtime rather than more coverage.
				state, err := m.NewMatch(module.MatchConfig{
					Options: module.Options{
						OptTargetScore:               500,
						module.OptPauseBetweenRounds: module.OptOff,
					},
				}, players, seed)
				if err != nil {
					t.Fatalf("NewMatch: %v", err)
				}
				wilds, stuck := playOut(t, m, state, players, skill, 6000)
				if wilds > 0 {
					t.Errorf("%d seats, %s, seed %d: %d wild cards discarded with ordinary cards still in hand",
						seats, skill, seed, wilds)
				}
				if stuck {
					wedged++
				}
			}
		}
	}
	if wedged > 0 {
		t.Logf("%d of 18 deals reached a turn with no legal move — see TestATurnCanStillWedge", wedged)
	}
}

// TestATurnCanStillWedge records a dead end in the engine, so that it is
// written down in the package it belongs to rather than only in a ticket.
//
// applyDiscard refuses to end a turn that laid cards toward the initial meld
// without reaching the floor (ErrInitialMeldNotMet), on the grounds that the
// player can go on melding, and checkInitialMeld is supposed to make that safe
// by refusing any lay that puts the floor out of reach. Its bound is meld.go's
// reachableValue, and reachableValue over-estimates: it counts melds the
// engine will not actually accept, so a side can lay a hundred and five points
// of the hundred and twenty it needs and find the rest was never there. No
// further meld, no lay-off (it has not opened) and no discard (it laid): the
// turn has no legal move in it and the deal wedges for the whole table.
//
// It is not this bot's doing and not new. module.OfferBot — the bot this
// module shipped with — walks into it in eight of forty two-handed matches on
// main today. What this bot adds is opensTheAccount, which refuses to *start*
// an opening it cannot finish, and that takes it from twenty-three of forty to
// three: the ones that are left come through applyTakePile, which gates a
// capture by an unmelded side on the same bound, and which no amount of care
// on this side of the seam can predict.
//
// Tightening reachableValue was tried here and reverted. It is a change to
// which melds the engine accepts, two of its own tests pin the current answer
// (TestReachableValueIsAchievable, TestOpeningTheTable), and getting it right
// means deciding what those tests should say — which is the engine's business
// and wants its own change.

// playOut drives a match to its end. It returns how many wild cards were
// discarded out of a hand that still held something else — the number this
// change is about, counted over real deals rather than over a position
// somebody set up — and whether the deal reached a turn with no legal move.
func playOut(t *testing.T, m *Module, state module.State, players []module.PlayerRef, skill module.Skill, maxActions int) (wilds int, stuck bool) {
	t.Helper()
	for step := 0; step < maxActions; step++ {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.Status != "active" {
			return wilds, false
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return wilds, false
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions(%s): %v", actor, err)
		}
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skill}, offers)
		if !ok {
			// No enabled offer describes a submission. Not the bot declining
			// to move — there is nothing to move. See TestATurnCanStillWedge.
			return wilds, true
		}
		if a.Verb == VerbDiscard && len(a.Cards) == 1 && isWild(a.Cards[0]) {
			// Legal, and only ever right when the hand holds nothing else the
			// engine would take. Anything above that is the bug.
			if hasNonWild(s.Hands[actor]) {
				wilds++
			}
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("%s proposed an illegal %v: %v", actor, a, err)
		}
		state = next
	}
	t.Fatalf("match did not finish in %d actions", maxActions)
	return wilds, false
}

func hasNonWild(hand []string) bool {
	for _, c := range hand {
		if !isWild(c) && !isRedThree(c) {
			return true
		}
	}
	return false
}

// TestUnknownSkillPlaysMedium is module.BotSeat's compatibility rule: every
// seat taken before skills existed carries an empty one, and defaulting down
// would silently weaken every table already in the database.
func TestUnknownSkillPlaysMedium(t *testing.T) {
	for _, sk := range []module.Skill{"", "nonsense"} {
		if got := profileFor(sk); got.skill != module.SkillMedium {
			t.Errorf("profileFor(%q) = %q, want medium", sk, got.skill)
		}
	}
}

// TestBotNeverTakesAMoveBack is the loop guard, stated as a test.
//
// An undo restores the position exactly and the bot is a function of the
// position, so a bot that took one would be handed back the state it just left
// and make the same move again. Handed a list whose only enabled offer is an
// undo, it must decline to move rather than oscillate.
func TestBotNeverTakesAMoveBack(t *testing.T) {
	raw := twoHanded(func(s *GameState) {
		s.Phase = phaseMeld
		s.Teams[0].Melds = []Meld{{
			ID: meldID(0, "A"), TeamID: 0, Kind: meldSet, Rank: "A",
			Cards: []string{"AH", "AD", "AS"},
		}}
		s.LaidThisTurn = 60
		s.Hands["p1"] = []string{"8C"}
		s.MeldsLaid = []MeldLaid{{
			MeldID: meldID(0, "A"), Cards: []string{"AH", "AD", "AS"},
			PriorHand: []string{"AH", "AD", "AS", "8C"},
		}}
	})
	offers, err := New().LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	// Whatever else is on offer, the undo is — and it is the one the bot must
	// not reach for.
	if o := module.FindOffer(offers, OfferUndoLayMeld); o == nil || !o.Enabled {
		t.Fatalf("expected an enabled undo to tempt the bot, got %+v", o)
	}
	a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillMedium}, offers)
	if ok && (a.Verb == VerbUndoLayMeld || a.Verb == VerbUndoLayOff || a.Verb == VerbUndoTakePile) {
		t.Errorf("the bot took a move back: %+v", a)
	}
}
