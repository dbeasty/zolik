package holdem

import (
	"encoding/json"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// The showdown: the stop after every hand, what it turns face up, and what a
// player may turn face up themselves.
//
// Poker used to deal the next hand inside the same Apply that ended the last
// one. The board vanished, the hole cards were overwritten, and the showdown
// this module had carefully computed reached a player as a sentence under the
// table describing cards nobody had seen. Everything below is about the
// window that fixed it.

// heads deals a two-handed match and returns it with its state decoded.
func heads(t *testing.T, opts module.Options) (module.State, *GameState) {
	t.Helper()
	if opts == nil {
		opts = module.Options{}
	}
	m := New()
	raw, err := m.NewMatch(
		module.MatchConfig{Variation: "timed", Options: opts},
		[]module.PlayerRef{{ID: "p1"}, {ID: "p2"}}, 7)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	return raw, mustDecode(t, raw)
}

// playToTheStop folds the hand out as fast as the offers allow and returns the
// state sitting at the showdown that follows.
func playToTheStop(t *testing.T, raw module.State) module.State {
	t.Helper()
	for step := 0; step < 200; step++ {
		s := mustDecode(t, raw)
		if s.Break.Open {
			return raw
		}
		if s.Status != "active" || s.Current < 0 {
			t.Fatalf("step %d: the hand ended without stopping", step)
		}
		actor := s.Seats[s.Current].PlayerID
		next, code := apply(t, raw, actor, module.Action{Verb: VerbFold})
		if code != "" {
			t.Fatalf("step %d: fold refused: %s", step, code)
		}
		raw = next
	}
	t.Fatal("never reached a showdown")
	return nil
}

// --- the stop ---------------------------------------------------------------

// TestEveryHandStopsAtItsShowdown is the whole premise. Hold'em declares no
// pause option, so there is no setting under which this does not happen — and
// that is exactly why it is worth a test rather than left to the cross-module
// pause checks, which can only ask games that advertise a choice.
func TestEveryHandStopsAtItsShowdown(t *testing.T) {
	raw, _ := heads(t, module.Options{OptHandLimit: 3})
	if New().Descriptor().Option(module.OptPauseBetweenRounds) != nil {
		t.Error("hold'em should not offer a pause setting it does not honour")
	}

	raw = playToTheStop(t, raw)
	s := mustDecode(t, raw)
	if s.HandNumber != 1 {
		t.Errorf("hand %d was dealt before the first showdown was looked at", s.HandNumber)
	}
	if s.LastHand == nil {
		t.Fatal("stopped with no hand to show")
	}
	if len(s.Board) == 0 && !s.LastHand.Uncontested {
		t.Error("a contested hand stopped with no board on the table")
	}

	// And the table says so, which is the only way a client knows to put the
	// results screen up.
	log, err := New().Rounds(raw)
	if err != nil {
		t.Fatalf("Rounds: %v", err)
	}
	if !log.Paused {
		t.Error("the table is stopped and the round log does not say so")
	}
	if len(log.WaitingFor) != 2 {
		t.Errorf("waiting for %v, want both seats", log.WaitingFor)
	}
}

// TestTheLastShowdownIsShownToo. The hand that decides a match is the one
// people talk about afterwards, and it used to be the one hand nobody saw:
// endHand settled the match and skipped the stop entirely.
func TestTheLastShowdownIsShownToo(t *testing.T) {
	m := New()
	raw, _ := heads(t, module.Options{OptHandLimit: 1})
	raw = playToTheStop(t, raw)

	s := mustDecode(t, raw)
	if !s.Closing {
		t.Error("the showdown after the last hand should know it is the last")
	}
	if done, _, _ := m.Finished(raw); done {
		t.Error("the match finished before its final showdown was looked at")
	}

	// The control says what it does, rather than promising a hand that is not
	// coming.
	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	var onward *module.ActionOffer
	for i := range offers {
		if offers[i].Verb == module.VerbContinue {
			onward = &offers[i]
		}
	}
	if onward == nil {
		t.Fatal("no way on from the last showdown")
	}
	if onward.LabelKey != "holdem.offer.finish" {
		t.Errorf("the last showdown's control is labelled %q", onward.LabelKey)
	}

	for _, id := range []string{"p1", "p2"} {
		next, code := apply(t, raw, id, module.Action{Verb: module.VerbContinue})
		if code != "" {
			t.Fatalf("%s could not go on: %s", id, code)
		}
		raw = next
	}
	if done, winners, _ := m.Finished(raw); !done || len(winners) == 0 {
		t.Fatalf("agreeing to leave the last showdown should end the match; done=%v", done)
	}

	// The cards stay where they are: a completed match is still looking at the
	// hand that ended it.
	end := mustDecode(t, raw)
	if end.LastHand == nil || len(end.Seats[0].Hole) == 0 {
		t.Error("the deciding hand was cleared off the table")
	}
	if !showdownOpen(end) {
		t.Error("a completed match should still be showing its last hand")
	}
}

// --- what goes face up ------------------------------------------------------

// showdownState builds a finished, contested hand with two known holdings and
// a board, stopped at its showdown.
func showdownState(t *testing.T, reveal int) (module.State, *GameState) {
	t.Helper()
	s := &GameState{
		Status: "active", Street: streetRiver, HandNumber: 1, HandLimit: 5,
		BigBlind: 20, SmallBlind: 10, MinRaise: 20, StartingStack: 1000,
		CurrentBet: 20, Pot: 60,
		Reveal: reveal, Button: 0, Current: 0, Deck: buildDeck(),
		Board: []string{"AS", "KH", "QD", "2C", "7H"},
		Seats: []Seat{
			// p1 makes a pair of aces; p2 makes nothing at all.
			{PlayerID: "p1", Stack: 980, Bet: 20, Committed: 20, Acted: true, Hole: []string{"AC", "3D"}},
			{PlayerID: "p2", Stack: 980, Bet: 20, Committed: 20, Acted: true, Hole: []string{"4S", "5D"}},
		},
	}
	raw, err := encode(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// The river bet is matched, so a check closes the hand.
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbCheck})
	if code != "" {
		t.Fatalf("check refused: %s", code)
	}
	return next, mustDecode(t, next)
}

func shownIDs(res *HandResult) []string {
	out := make([]string, 0, len(res.Shown))
	for _, sh := range res.Shown {
		out = append(out, sh.PlayerID)
	}
	return out
}

// TestRevealEveryoneShowsEveryCalledHand — the default, and what this module
// has always done.
func TestRevealEveryoneShowsEveryCalledHand(t *testing.T) {
	_, s := showdownState(t, RevealEveryone)
	if got := shownIDs(s.LastHand); len(got) != 2 {
		t.Errorf("shown %v, want both seats", got)
	}
	if len(s.LastHand.Mucked) != 0 {
		t.Errorf("nobody mucked, got %v", s.LastHand.Mucked)
	}
	for _, sh := range s.LastHand.Shown {
		if sh.LabelKey == "" || len(sh.Best) != 5 {
			t.Errorf("%s was shown without a hand: %+v", sh.PlayerID, sh)
		}
		if sh.Voluntary {
			t.Errorf("%s was turned over by the showdown, not by choice", sh.PlayerID)
		}
	}
}

// TestRevealWinnersKeepsTheLoserSCards. The stricter setting, and the one this
// test exists for: the cards it hides must be absent from the *state*, not
// merely undrawn. HandResult.Shown is what View builds its sentences out of,
// and those sentences carry the hole cards in their params — a setting that
// filtered at the point of drawing would hide cards from the screen and ship
// them on the wire regardless.
func TestRevealWinnersKeepsTheLosersCards(t *testing.T) {
	raw, s := showdownState(t, RevealWinners)

	if got := shownIDs(s.LastHand); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("shown %v, want only the winner", got)
	}
	if len(s.LastHand.Mucked) != 1 || s.LastHand.Mucked[0] != "p2" {
		t.Errorf("mucked %v, want p2 named", s.LastHand.Mucked)
	}

	// Nothing anywhere in what p1 is sent names a card p2 kept.
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	blob, err := json.Marshal(vm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, card := range s.Seats[1].Hole {
		if strings.Contains(string(blob), `"`+card+`"`) {
			t.Errorf("p1's view names %s, which p2 never showed", card)
		}
	}
}

// TestAShownHandIsOnTheTable — the cards reach the board as cards, in the seat
// they belong to, rather than only as words underneath it.
func TestAShownHandIsOnTheTable(t *testing.T) {
	raw, s := showdownState(t, RevealEveryone)
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	var theirs *module.Zone
	for i := range vm.Zones {
		if vm.Zones[i].ID == "hole:p2" {
			theirs = &vm.Zones[i]
		}
	}
	if theirs == nil {
		t.Fatal("p2 has no hand zone")
	}
	if len(theirs.Cards) != 2 {
		t.Fatalf("p2's shown hand holds %d cards: %+v", len(theirs.Cards), theirs.Cards)
	}
	if theirs.LabelKey != "zone.shownHand" {
		t.Errorf("a shown hand is labelled %q", theirs.LabelKey)
	}
	// And the board it was made against is still there to read it against.
	for _, z := range vm.Zones {
		if z.ID == "board" && len(z.Cards) != len(s.Board) {
			t.Errorf("the board holds %d cards, the hand was made against %d", len(z.Cards), len(s.Board))
		}
	}
}

// TestAHiddenHandStaysHidden — the other half, and the one that matters. A
// seat that has not shown is a count and nothing else, to everyone but its
// owner, for as long as the showdown is up.
func TestAHiddenHandStaysHidden(t *testing.T) {
	raw, s := showdownState(t, RevealWinners)
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	for _, z := range vm.Zones {
		if z.ID != "hole:p2" {
			continue
		}
		if len(z.Cards) != 0 {
			t.Errorf("p2's hand is face up to p1: %+v", z.Cards)
		}
		if z.Count != len(s.Seats[1].Hole) {
			t.Errorf("p2's hand counts %d, holds %d", z.Count, len(s.Seats[1].Hole))
		}
	}
}

// TestTheShowdownStopsBeingSpokenAbout. The status sentences used to run from
// the moment a hand ended until the moment the next one did, so "p2 showed a
// flush" sat under a board it had nothing to do with for a whole hand.
func TestTheShowdownStopsBeingSpokenAbout(t *testing.T) {
	raw, _ := showdownState(t, RevealEveryone)
	saidAt := func(raw module.State) int {
		vm, err := New().View(raw, "p1")
		if err != nil {
			t.Fatalf("View: %v", err)
		}
		n := 0
		for _, f := range vm.Status {
			if f.LabelKey == "holdem.status.shown" || f.LabelKey == "holdem.status.pot" {
				n++
			}
		}
		return n
	}
	if saidAt(raw) == 0 {
		t.Fatal("a showdown says nothing about itself")
	}
	for _, id := range []string{"p1", "p2"} {
		next, code := apply(t, raw, id, module.Action{Verb: module.VerbContinue})
		if code != "" {
			t.Fatalf("%s could not go on: %s", id, code)
		}
		raw = next
	}
	if n := saidAt(raw); n != 0 {
		t.Errorf("the next hand still carries %d sentences about the last one", n)
	}
}

// --- showing by choice ------------------------------------------------------

// TestABluffCanBeShown — the case the feature exists for. A pot everyone
// folded out of is never turned over by the game, and its winner is exactly
// the player most likely to want it seen.
func TestABluffCanBeShown(t *testing.T) {
	raw, _ := heads(t, module.Options{OptHandLimit: 5})
	raw = playToTheStop(t, raw)
	s := mustDecode(t, raw)
	if !s.LastHand.Uncontested {
		t.Fatal("this fixture is meant to fold out")
	}
	if len(s.LastHand.Shown) != 0 {
		t.Fatal("a hand nobody called must not be shown by the game")
	}
	winner := s.LastHand.Pots[0].Winners[0]

	next, code := apply(t, raw, winner, module.Action{Verb: VerbShow})
	if code != "" {
		t.Fatalf("show refused: %s", code)
	}
	after := mustDecode(t, next)
	if !after.LastHand.shows(winner) {
		t.Fatal("the hand did not go face up")
	}
	sh := after.LastHand.Shown[0]
	if len(sh.Hole) != 2 || !sh.Voluntary {
		t.Errorf("shown hand is %+v", sh)
	}
	// Named by nobody, because there was no showdown to name it at — and the
	// board may not even be complete. `Best` answers "high card" for fewer
	// than five cards, which would print a claim this player never made.
	if sh.LabelKey != "" || len(sh.Best) != 0 {
		t.Errorf("a bluff shown after a fold claims %q / %v", sh.LabelKey, sh.Best)
	}

	// And it is on the table, for the other player.
	vm, err := New().View(next, other(after, winner))
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	for _, z := range vm.Zones {
		if z.ID == "hole:"+winner && len(z.Cards) != 2 {
			t.Errorf("the shown bluff holds %d cards for the opponent", len(z.Cards))
		}
	}
}

func other(s *GameState, id string) string {
	for i := range s.Seats {
		if s.Seats[i].PlayerID != id {
			return s.Seats[i].PlayerID
		}
	}
	return ""
}

// TestShowingIsOnceAndOnlyAtAShowdown gathers the refusals: twice, too late,
// and while a hand is being played.
func TestShowingIsOnceAndOnlyAtAShowdown(t *testing.T) {
	raw, _ := heads(t, module.Options{OptHandLimit: 5})

	// Mid-hand there is no finished hand to show.
	if _, code := apply(t, raw, "p1", module.Action{Verb: VerbShow}); code != module.ErrNotPaused {
		t.Errorf("showing mid-hand was refused with %q", code)
	}

	raw = playToTheStop(t, raw)
	shown, code := apply(t, raw, "p1", module.Action{Verb: VerbShow})
	if code != "" {
		t.Fatalf("show refused: %s", code)
	}
	if _, code := apply(t, shown, "p1", module.Action{Verb: VerbShow}); code != ErrAlreadyShown {
		t.Errorf("showing twice was refused with %q, want %s", code, ErrAlreadyShown)
	}

	// Saying "go on" is letting the hand go: the table may deal the moment the
	// last seat agrees, so there is no hand left to turn over.
	readied, code := apply(t, raw, "p2", module.Action{Verb: module.VerbContinue})
	if code != "" {
		t.Fatalf("continue refused: %s", code)
	}
	if _, code := apply(t, readied, "p2", module.Action{Verb: VerbShow}); code != module.ErrAlreadyReady {
		t.Errorf("showing after going on was refused with %q, want %s", code, module.ErrAlreadyReady)
	}
	if _, code := apply(t, raw, "ghost", module.Action{Verb: VerbShow}); code != module.ErrNotSeated {
		t.Errorf("a stranger showing was refused with %q", code)
	}
}

// TestTheShowOfferAgreesWithTheEngine. This offer is the one in the module
// built without probing Apply — see showOffer — so the agreement the probe
// would have guaranteed is asserted here instead.
func TestTheShowOfferAgreesWithTheEngine(t *testing.T) {
	m := New()
	raw, _ := heads(t, module.Options{OptHandLimit: 5})
	raw = playToTheStop(t, raw)

	check := func(raw module.State, who string) {
		t.Helper()
		offers, err := m.LegalActions(raw, who)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		var show *module.ActionOffer
		for i := range offers {
			if offers[i].ID == OfferShow {
				show = &offers[i]
			}
		}
		if show == nil {
			t.Fatalf("%s was offered no way to show", who)
		}
		_, _, err = m.Apply(raw, who, module.Action{Verb: VerbShow})
		if got := module.CodeOf(err); (err == nil) != show.Enabled || got != show.WhyNot {
			t.Errorf("%s: offer says enabled=%v/%q, engine says %q",
				who, show.Enabled, show.WhyNot, got)
		}
	}

	check(raw, "p1")
	shown, _ := apply(t, raw, "p1", module.Action{Verb: VerbShow})
	check(shown, "p1")
	readied, _ := apply(t, raw, "p2", module.Action{Verb: module.VerbContinue})
	check(readied, "p2")

	// And the way on always comes first, because a bot with no preference
	// takes the first enabled offer it is handed.
	offers, err := m.LegalActions(raw, "p1")
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	if offers[0].Verb != module.VerbContinue {
		t.Errorf("the first offer at a showdown is %q, not the way on", offers[0].Verb)
	}
}

// TestBotsDoNotShow. The bot reaches the same offer list a player does, and
// `show` is legal for it — so what keeps a bot's bluffs face down is a stated
// preference, and a stated preference is a thing that can be deleted.
func TestBotsDoNotShow(t *testing.T) {
	m := New()
	raw, _ := heads(t, module.Options{OptHandLimit: 5})
	raw = playToTheStop(t, raw)

	for _, id := range []string{"p1", "p2"} {
		offers, err := m.LegalActions(raw, id)
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: id}, offers)
		if !ok {
			t.Fatalf("%s had no move at the showdown", id)
		}
		if a.Verb != module.VerbContinue {
			t.Errorf("a bot at a showdown played %q — bots must not show", a.Verb)
		}
	}
}

// TestAShownHandNeverReachesTheRoundLog. The round log is public and permanent
// and takes no viewer, so a card in it would leak twice over. Showing is the
// first thing in this module that puts a card anywhere on purpose, which makes
// it the first thing that could put one here by accident.
func TestAShownHandNeverReachesTheRoundLog(t *testing.T) {
	raw, _ := heads(t, module.Options{OptHandLimit: 5})
	raw = playToTheStop(t, raw)
	s := mustDecode(t, raw)
	winner := s.LastHand.Pots[0].Winners[0]
	raw, code := apply(t, raw, winner, module.Action{Verb: VerbShow})
	if code != "" {
		t.Fatalf("show refused: %s", code)
	}

	log, err := New().Rounds(raw)
	if err != nil {
		t.Fatalf("Rounds: %v", err)
	}
	blob, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	after := mustDecode(t, raw)
	for _, card := range after.LastHand.Shown[0].Hole {
		if strings.Contains(string(blob), `"`+card+`"`) {
			t.Errorf("the round log names %s; it is public and permanent", card)
		}
	}
}

// potFact finds the sentence that says who took the pot.
func potFact(t *testing.T, raw module.State) module.Fact {
	t.Helper()
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	for _, f := range vm.Status {
		if f.LabelKey == "holdem.status.pot" || f.LabelKey == "holdem.status.potSplit" {
			return f
		}
	}
	t.Fatalf("no pot sentence in %+v", vm.Status)
	return module.Fact{}
}

// TestThePotNamesTheWinningFive — the line at the end of a hand says which
// five cards took it, in the order a player would read them out.
func TestThePotNamesTheWinningFive(t *testing.T) {
	for _, reveal := range []int{RevealEveryone, RevealWinners} {
		raw, s := showdownState(t, reveal)
		if got := s.LastHand.Pots[0].Cards; strings.Join(got, " ") != "AC AS KH QD 7H" {
			t.Errorf("reveal %d: pot cards %v, want p1's pair of aces", reveal, got)
		}
		f := potFact(t, raw)
		if f.LabelKey != "holdem.status.pot" {
			t.Fatalf("reveal %d: pot sentence is %s", reveal, f.LabelKey)
		}
		if got, _ := f.Params["cards"].([]string); len(got) != 5 {
			t.Errorf("reveal %d: pot sentence carries cards %v", reveal, f.Params["cards"])
		}
	}
}

// TestASplitPotNamesNoCards. Two winners tie on rank, not on suits, so no one
// set of five is theirs; the sentence must not carry one.
func TestASplitPotNamesNoCards(t *testing.T) {
	s := &GameState{
		Status: "active", Street: streetRiver, HandNumber: 1, HandLimit: 5,
		BigBlind: 20, SmallBlind: 10, MinRaise: 20, StartingStack: 1000,
		CurrentBet: 20, Pot: 60,
		Reveal: RevealEveryone, Button: 0, Current: 0, Deck: buildDeck(),
		// Broadway on the board: both seats play it.
		Board: []string{"AS", "KH", "QD", "JC", "TS"},
		Seats: []Seat{
			{PlayerID: "p1", Stack: 980, Bet: 20, Committed: 20, Acted: true, Hole: []string{"2C", "3D"}},
			{PlayerID: "p2", Stack: 980, Bet: 20, Committed: 20, Acted: true, Hole: []string{"4S", "5D"}},
		},
	}
	raw, err := encode(s)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	next, code := apply(t, raw, "p1", module.Action{Verb: VerbCheck})
	if code != "" {
		t.Fatalf("check refused: %s", code)
	}
	after := mustDecode(t, next)
	if p := after.LastHand.Pots[0]; len(p.Winners) != 2 || len(p.Cards) != 0 {
		t.Fatalf("split pot %+v, want two winners and no cards", p)
	}
	f := potFact(t, next)
	if f.LabelKey != "holdem.status.potSplit" {
		t.Errorf("split pot sentence is %s", f.LabelKey)
	}
	if _, ok := f.Params["cards"]; ok {
		t.Errorf("split pot sentence carries cards: %v", f.Params["cards"])
	}
}
