package snaps

import (
	"slices"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var players = []module.PlayerRef{{ID: "p1"}, {ID: "p2"}}

func newState(t *testing.T, variation string) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Variation: variation}, players, 42)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func apply(t *testing.T, s *GameState, who string, a module.Action) (*GameState, error) {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := New().Apply(raw, who, a)
	if err != nil {
		return s, err
	}
	next, err := decode(out)
	if err != nil {
		t.Fatal(err)
	}
	return next, nil
}

func must(t *testing.T, s *GameState, who string, a module.Action) *GameState {
	t.Helper()
	next, err := apply(t, s, who, a)
	if err != nil {
		t.Fatalf("%s %s %v refused: %v", who, a.Verb, a.Cards, err)
	}
	return next
}

func refused(t *testing.T, s *GameState, who string, a module.Action, code string) {
	t.Helper()
	if _, err := apply(t, s, who, a); module.CodeOf(err) != code {
		t.Fatalf("%s %s %v: got %v, want %s", who, a.Verb, a.Cards, err, code)
	}
}

func playCard(c string) module.Action {
	return module.Action{OfferID: OfferPlay, Verb: VerbPlay, Cards: []string{c}}
}

var (
	exchangeAction = module.Action{OfferID: OfferExchange, Verb: VerbExchange}
	closeAction    = module.Action{OfferID: OfferClose, Verb: VerbClose}
)

// position is a hand-built Šnaps deal: p1 to lead, hearts trumps, the hands
// given and the talon given (its last card the turned-up trump).
func deal(t *testing.T, variation string, hands map[string][]string, talon []string) *GameState {
	t.Helper()
	s := newState(t, variation)
	s.FirstDealer, s.Deal = 1, 0 // p2 deals, p1 leads
	s.Hands, s.Talon, s.Trump = hands, talon, "H"
	s.Current = "p1"
	return s
}

func TestTheDeal(t *testing.T) {
	for _, c := range []struct {
		variation string
		hand      int
		talon     int
	}{{variationSnaps, 5, 10}, {variation66, 6, 12}} {
		s := newState(t, c.variation)
		dealer := s.dealer()
		leader := s.other(dealer)
		if s.Current != leader {
			t.Errorf("%s: %s leads, want the non-dealer %s", c.variation, s.Current, leader)
		}
		for _, p := range s.Players {
			if len(s.Hands[p]) != c.hand {
				t.Errorf("%s: %s holds %d, want %d", c.variation, p, len(s.Hands[p]), c.hand)
			}
		}
		if len(s.Talon) != c.talon || s.Trump != string(tricks.Suit(s.turnedUp())) {
			t.Errorf("%s: talon of %d turned up %s, trumps %s", c.variation, len(s.Talon), s.turnedUp(), s.Trump)
		}
		seen := map[string]bool{}
		for _, c := range append(append(append([]string(nil), s.Hands["p1"]...), s.Hands["p2"]...), s.Talon...) {
			if seen[c] {
				t.Fatalf("%s dealt twice", c)
			}
			seen[c] = true
		}
		s.Deal++
		if s.dealer() == dealer {
			t.Errorf("%s: the deal did not pass", c.variation)
		}
	}
}

func TestOpenPlayHasNoObligations(t *testing.T) {
	s := deal(t, variationSnaps, map[string][]string{
		"p1": {"AS", "KC", "QC", "JD", "TD"}, "p2": {"TS", "JS", "AH", "KD", "QD"},
	}, []string{"AC", "TC", "JC", "KS", "QS", "AD", "KH", "QH", "TH", "JH"})
	s = must(t, s, "p1", playCard("AS"))
	// p2 holds spades and trumps but may throw a diamond.
	s = must(t, s, "p2", playCard("KD"))
	if s.TricksWon["p1"] != 1 || s.Points["p1"] != 15 {
		t.Fatalf("p1 took %d tricks for %d, want 1 for 15", s.TricksWon["p1"], s.Points["p1"])
	}
	// The winner draws first: p1 the top card, p2 the next.
	if !hasCard(s.Hands["p1"], "AC") || !hasCard(s.Hands["p2"], "TC") || len(s.Talon) != 8 {
		t.Fatalf("draws: p1 %v p2 %v talon %v", s.Hands["p1"], s.Hands["p2"], s.Talon)
	}
	if s.Current != "p1" {
		t.Fatalf("%s leads, want the winner p1", s.Current)
	}
}

func TestStrictPlayAfterClosing(t *testing.T) {
	s := deal(t, variationSnaps, map[string][]string{
		"p1": {"AS", "KC", "QC", "JD", "TD"}, "p2": {"TS", "JS", "AH", "AD", "QD"},
	}, []string{"AC", "TC", "JC", "KS", "QS", "AD", "KH", "QH", "TH", "JH"})
	s = must(t, s, "p1", closeAction)
	if s.ClosedBy != "p1" || !s.strict() || s.turnedUp() != "" {
		t.Fatalf("closed by %q, strict %v", s.ClosedBy, s.strict())
	}
	refused(t, s, "p1", exchangeAction, ErrTalonClosed)
	refused(t, s, "p1", closeAction, ErrTalonClosed)

	s = must(t, s, "p1", playCard("TD"))
	refused(t, s, "p2", playCard("TS"), ErrMustFollow)
	refused(t, s, "p2", playCard("QD"), ErrMustBeat) // the ace beats the ten
	s = must(t, s, "p2", playCard("AD"))
	// p2 leads a spade: p1 must head it with the ace.
	s = must(t, s, "p2", playCard("JS"))
	s = must(t, s, "p1", playCard("AS"))
	// A club led to a player with none: trump it.
	s = must(t, s, "p1", playCard("KC"))
	refused(t, s, "p2", playCard("TS"), ErrMustTrump)
	must(t, s, "p2", playCard("AH"))
}

func TestExchangingTheTrumpCard(t *testing.T) {
	hands := func() map[string][]string {
		return map[string][]string{"p1": {"JH", "KC", "QC", "JD", "TD"}, "p2": {"TS", "JS", "AH", "KD", "QD"}}
	}
	talon := func() []string {
		return []string{"AC", "TC", "JC", "KS", "QS", "AD", "KH", "QH", "AS", "TH"}
	}
	s := deal(t, variationSnaps, hands(), talon())
	refused(t, s, "p2", exchangeAction, ErrNotYourTurn)
	s = must(t, s, "p1", exchangeAction)
	if !hasCard(s.Hands["p1"], "TH") || s.turnedUp() != "JH" || !slices.Contains(s.Known["p1"], "TH") {
		t.Fatalf("after the exchange: hand %v, turned up %s, known %v", s.Hands["p1"], s.turnedUp(), s.Known["p1"])
	}
	refused(t, s, "p1", exchangeAction, ErrNoExchangeCard)
	s = must(t, s, "p1", playCard("TD"))
	refused(t, s, "p2", exchangeAction, ErrNotOnLead)

	// Šedesát šest: the nine, and only after a trick.
	h := hands()
	h["p1"] = append(h["p1"], "9H")
	h["p2"] = append(h["p2"], "9S")
	s = deal(t, variation66, h, talon())
	refused(t, s, "p1", exchangeAction, ErrExchangeNoTrick)
	s.TricksWon["p1"] = 1
	s = must(t, s, "p1", exchangeAction)
	if !hasCard(s.Hands["p1"], "TH") || s.turnedUp() != "9H" {
		t.Fatalf("66 exchange: hand %v, turned up %s", s.Hands["p1"], s.turnedUp())
	}
}

func TestMarriages(t *testing.T) {
	s := deal(t, variationSnaps, map[string][]string{
		"p1": {"KC", "QC", "KH", "QH", "TD"}, "p2": {"TS", "JS", "AH", "KD", "QD"},
	}, []string{"AC", "TC", "JC", "KS", "QS", "AD", "AS", "JD", "TH", "JH"})
	s = must(t, s, "p1", playCard("QC"))
	if !slices.Equal(s.Marriages["p1"], []string{"C"}) || !slices.Contains(s.Known["p1"], "KC") {
		t.Fatalf("marriages %v, known %v", s.Marriages["p1"], s.Known["p1"])
	}
	if s.total("p1") != 0 {
		t.Fatalf("a marriage counted before a trick: %d", s.total("p1"))
	}
	s = must(t, s, "p2", playCard("TS")) // p1 wins: the leader's suit, p2 threw off
	if s.total("p1") != 3+10+20 {
		t.Fatalf("after the trick: %d, want 33", s.total("p1"))
	}
	// The trump marriage takes p1 from 33 to 73 on the lead: out at once.
	s = must(t, s, "p1", playCard("KH"))
	if len(s.Rounds) != 1 || s.Rounds[0].Winner != "p1" || s.Rounds[0].Reason != reasonOut {
		t.Fatalf("rounds %+v", s.Rounds)
	}
	// p2 has no trick: schwarz, three game points.
	if s.Rounds[0].GamePoints != 3 {
		t.Fatalf("out against no tricks scored %d, want 3", s.Rounds[0].GamePoints)
	}
}

func TestNoMarriageOnceStrictIn66(t *testing.T) {
	s := deal(t, variation66, map[string][]string{
		"p1": {"KC", "QC", "KH", "9H", "TD", "9D"}, "p2": {"TS", "JS", "AH", "KD", "QD", "9S"},
	}, []string{"AC", "TC", "JC", "KS", "QS", "AD", "AS", "JD", "TH", "JH", "9C", "QH"})
	s = must(t, s, "p1", closeAction)
	s = must(t, s, "p1", playCard("QC"))
	if len(s.Marriages["p1"]) != 0 {
		t.Fatalf("a marriage announced after closing in Šedesát šest: %v", s.Marriages["p1"])
	}
}

func TestScoring(t *testing.T) {
	snaps, six := rulesFor(variationSnaps), rulesFor(variation66)
	tl := func(p0, p1, t0, t1 int) tally {
		return tally{points: [2]int{p0, p1}, tricks: [2]int{t0, t1}}
	}
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"out, loser at 33", snaps.outPoints(tl(66, 33, 3, 2), notClosed, 0), 1},
		{"out, loser at 32", snaps.outPoints(tl(66, 32, 3, 2), notClosed, 0), 2},
		{"out, loser without a trick", snaps.outPoints(tl(66, 0, 5, 0), notClosed, 0), 3},
		{"Šnaps closer paid as at closing", snaps.outPoints(tl(70, 40, 4, 2), closing{by: 0, oppTricks: 0, oppPoints: 0}, 0), 3},
		{"66 closer paid as at the end", six.outPoints(tl(70, 40, 4, 2), closing{by: 0, oppTricks: 0, oppPoints: 0}, 0), 1},
		{"Šnaps: the closer's opponent goes out, no trick at closing", snaps.outPoints(tl(30, 66, 1, 3), closing{by: 0, oppTricks: 0}, 1), 3},
		{"66: the closer's opponent goes out", six.outPoints(tl(30, 66, 1, 3), closing{by: 0, oppTricks: 0}, 1), 2},
		{"closer fails, opponent had a trick", closerFailed(closing{by: 0, oppTricks: 1}), 2},
		{"closer fails, opponent had none", closerFailed(closing{by: 0, oppTricks: 0}), 3},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
	if w, p := snaps.runOut(tl(60, 60, 3, 2), 1); w != 1 || p != 1 {
		t.Errorf("Šnaps run out: seat %d for %d, want the last trick's seat 1 for 1", w, p)
	}
	if w, p := six.runOut(tl(70, 50, 3, 3), 1); w != 0 || p != 1 {
		t.Errorf("66 run out: seat %d for %d, want the higher seat 0 for 1", w, p)
	}
	if w, p := six.runOut(tl(65, 65, 3, 3), 1); w != -1 || p != 0 {
		t.Errorf("66 at 65 each: seat %d for %d, want a draw", w, p)
	}
	// Marriages count only once a trick is taken.
	if got := (tally{points: [2]int{0, 0}, marriages: [2]int{40, 0}}).total(0); got != 0 {
		t.Errorf("a trickless marriage counted %d", got)
	}
}

func TestCloserWhoFailsLosesTheDeal(t *testing.T) {
	// p1 closes holding little; p2 takes everything that is left.
	s := deal(t, variationSnaps, map[string][]string{
		"p1": {"JC", "QC", "JD"}, "p2": {"AC", "TC", "AD"},
	}, []string{"KS", "QS", "TH", "JH"})
	s.TricksWon = map[string]int{"p1": 1, "p2": 1}
	s.Points = map[string]int{"p1": 20, "p2": 10}
	s = must(t, s, "p1", closeAction)
	s = must(t, s, "p1", playCard("JC"))
	s = must(t, s, "p2", playCard("AC"))
	s = must(t, s, "p2", playCard("TC"))
	s = must(t, s, "p1", playCard("QC"))
	s = must(t, s, "p2", playCard("AD"))
	s = must(t, s, "p1", playCard("JD"))
	r := s.Rounds[0]
	if r.Winner != "p2" || r.Reason != reasonCloserFailed || r.GamePoints != 2 {
		t.Fatalf("round %+v", r)
	}
}

func TestLastTrickInEachGame(t *testing.T) {
	// The talon used up, nobody near 66: Šnaps gives the deal to the last
	// trick, Šedesát šest adds ten and compares.
	for _, c := range []struct {
		variation string
		reason    string
		points    int
	}{{variationSnaps, reasonLastTrick, 1}, {variation66, reasonHigher, 1}} {
		s := deal(t, c.variation, map[string][]string{"p1": {"JC"}, "p2": {"QC"}}, nil)
		s.TricksWon = map[string]int{"p1": 4, "p2": 4}
		s.Points = map[string]int{"p1": 40, "p2": 50}
		s.StrictFrom = 0
		s = must(t, s, "p1", playCard("JC"))
		s = must(t, s, "p2", playCard("QC"))
		r := s.Rounds[0]
		if r.Winner != "p2" || r.Reason != c.reason || r.GamePoints != c.points {
			t.Errorf("%s: round %+v", c.variation, r)
		}
		if c.variation == variation66 && r.Points["p2"] != 50+5+10 {
			t.Errorf("66: the last trick counted %d, want 65", r.Points["p2"])
		}
	}
}

func TestTheMatchEndsAtTheTarget(t *testing.T) {
	raw, err := New().NewMatch(module.MatchConfig{Options: module.Options{OptTarget: 3, module.OptPauseBetweenRounds: module.OptOff}}, players, 7)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := decode(raw)
	s.Scores["p1"] = 2
	s.endDeal("p1", 1, reasonOut)
	if s.Status != "completed" {
		t.Fatalf("status %s at %v", s.Status, s.Scores)
	}
	raw, _ = encode(s)
	done, winners, _ := New().Finished(raw)
	if !done || !slices.Equal(winners, []string{"p1"}) {
		t.Fatalf("finished %v, winners %v", done, winners)
	}
}

func TestViewHidesWhatIsHidden(t *testing.T) {
	s := newState(t, variationSnaps)
	raw, _ := encode(s)
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		switch z.ID {
		case handZoneID("p2"), talonZoneID:
			if len(z.Cards) != 0 {
				t.Errorf("%s shows %v", z.ID, z.Cards)
			}
		case trumpZoneID:
			if len(z.Cards) != 1 || z.Cards[0].Card != s.turnedUp() {
				t.Errorf("the turned-up trump shows %v", z.Cards)
			}
		case handZoneID("p1"):
			if len(z.Cards) != 5 {
				t.Errorf("own hand shows %d cards", len(z.Cards))
			}
		}
	}
}

func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			"CARD_NOT_IN_HAND": "not a rule — the client and server disagree about the hand",
			"GAME_NOT_ACTIVE":  "not a rule — the game is over or has not started",
			"UNKNOWN_ACTION":   "not a rule — a verb this module does not have",
			"TOO_FEW_PLAYERS":  "not a rule of play — the table could not be seated",
		},
	}.Run()
	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestTheTrickZoneIsNamedForWhatItHolds(t *testing.T) {
	label := func(s *GameState) string {
		raw, _ := encode(s)
		vm, _ := New().View(raw, "p1")
		for _, z := range vm.Zones {
			if z.ID == trickZoneID {
				return z.LabelKey
			}
		}
		return ""
	}
	s := newState(t, variationSnaps)
	if got := label(s); got != "snaps.zone.trick" {
		t.Errorf("a fresh deal's trick is labelled %s", got)
	}
	s.LastTrick = []tricks.Play{{Seat: 0, Card: "AH"}, {Seat: 1, Card: "KH"}}
	if got := label(s); got != "snaps.zone.lastTrick" {
		t.Errorf("between tricks: %s", got)
	}
}
