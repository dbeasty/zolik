package marias

import (
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var players = []module.PlayerRef{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}}

func newState(t *testing.T, opts module.Options) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Variation: variationVoleny, Options: opts}, players, 42)
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
		t.Fatalf("%s %s/%s refused: %v", who, a.Verb, a.OfferID, err)
	}
	return next
}

func refused(t *testing.T, s *GameState, who string, a module.Action, code string) {
	t.Helper()
	if _, err := apply(t, s, who, a); module.CodeOf(err) != code {
		t.Fatalf("%s %s/%s %v: got %v, want %s", who, a.Verb, a.OfferID, a.Cards, err, code)
	}
}

// playState is a deal in play with the hands given, p1 declaring hra with
// hearts as trumps, p1 to lead.
func playState(t *testing.T, hands map[string][]string) *GameState {
	t.Helper()
	s := newState(t, nil)
	s.Hands = hands
	s.Unseen, s.Talon = nil, []string{"7D", "8D"}
	s.Declarer, s.Game, s.TrumpCard = "p1", gameHra, "9H"
	s.Phase, s.Current = phasePlay, "p1"
	return s
}

func TestTheChooserSeesSevenThenTwelve(t *testing.T) {
	s := newState(t, nil)
	c := s.chooser()
	if len(s.Hands[c]) != 7 || len(s.Unseen) != 5 {
		t.Fatalf("chooser holds %d with %d unseen, want 7 and 5", len(s.Hands[c]), len(s.Unseen))
	}
	for _, p := range s.Players {
		if p != c && len(s.Hands[p]) != 10 {
			t.Errorf("%s holds %d, want 10", p, len(s.Hands[p]))
		}
	}
	s = must(t, s, c, module.Action{OfferID: OfferTrump, Verb: VerbChooseTrump, Cards: s.Hands[c][:1]})
	if len(s.Hands[c]) != 12 || s.Phase != phaseAnnounce {
		t.Fatalf("after naming trumps: %d cards in phase %s", len(s.Hands[c]), s.Phase)
	}
}

func TestZLiduNamesTrumpsFromAnUnseenCard(t *testing.T) {
	s := newState(t, nil)
	c, unseen := s.chooser(), s.Unseen[0]
	s = must(t, s, c, module.Action{OfferID: OfferZLidu, Verb: VerbChooseTrump})
	if s.TrumpCard != unseen || !hasCard(s.Hands[c], unseen) {
		t.Fatalf("z lidu named %s, want the unseen %s, now in hand", s.TrumpCard, unseen)
	}

	off := newState(t, module.Options{OptZLidu: module.OptOff})
	refused(t, off, off.chooser(), module.Action{OfferID: OfferZLidu, Verb: VerbChooseTrump}, ErrNoZLidu)
}

// announced walks a fresh deal to the talon phase with the chooser's hand
// replaced, trumps named by the first card.
func announced(t *testing.T, hand []string, offer string) (*GameState, string) {
	t.Helper()
	s := newState(t, nil)
	c := s.chooser()
	s.Hands[c], s.Unseen = hand[:7], hand[7:]
	s = must(t, s, c, module.Action{OfferID: OfferTrump, Verb: VerbChooseTrump, Cards: []string{hand[0]}})
	return must(t, s, c, module.Action{OfferID: offer, Verb: VerbAnnounce}), c
}

var twelve = []string{"9H", "7H", "AH", "KH", "QH", "AC", "TC", "8C", "9S", "8S", "7S", "JD"}

func TestTheTalonTakesNoSharpCardInATrumpGame(t *testing.T) {
	s, c := announced(t, twelve, OfferHra)
	refused(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"AC", "8C"}}, ErrSharpInTalon)
	refused(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"8C"}}, ErrDiscardTwo)
	s = must(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"8C", "JD"}})
	if len(s.Hands[c]) != 10 || s.Phase != phaseAnswer || s.Current != s.next(c) {
		t.Fatalf("after the talon: %d cards, phase %s, %s to answer", len(s.Hands[c]), s.Phase, s.Current)
	}

	// In betl an ace may go.
	b, c := announced(t, twelve, OfferBetl)
	must(t, b, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"AC", "AH"}})
}

func TestASevenIsAnnouncedOnlyIfHeldAndIsKept(t *testing.T) {
	noSeven := append([]string{"9H"}, twelve[2:]...)
	noSeven = append(noSeven, "7C")
	s := newState(t, nil)
	c := s.chooser()
	s.Hands[c], s.Unseen = noSeven[:7], noSeven[7:]
	s = must(t, s, c, module.Action{OfferID: OfferTrump, Verb: VerbChooseTrump, Cards: []string{"9H"}})
	refused(t, s, c, module.Action{OfferID: OfferHraSedma, Verb: VerbAnnounce}, ErrNoSeven)

	s, c = announced(t, twelve, OfferHraSedma)
	refused(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"7H", "8C"}}, ErrSevenInTalon)
}

func TestATakeOverHandsTheTalonToTheTaker(t *testing.T) {
	s, c := announced(t, twelve, OfferHra)
	s = must(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"8C", "JD"}})
	taker := s.Current
	s = must(t, s, taker, module.Action{OfferID: OfferBadBetl, Verb: VerbTakeOver})
	if s.Declarer != taker || s.Game != gameBetl || len(s.Hands[taker]) != 12 || len(s.Talon) != 0 {
		t.Fatalf("after špatná: declarer %s, game %s, %d cards, talon %v", s.Declarer, s.Game, len(s.Hands[taker]), s.Talon)
	}
	s = must(t, s, taker, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: s.Hands[taker][:2]})
	// The others answer again, the original chooser included — and betl
	// cannot be overcalled with betl.
	next := s.Current
	refused(t, s, next, module.Action{OfferID: OfferBadBetl, Verb: VerbTakeOver}, ErrNotHigher)
	s = must(t, s, next, module.Action{OfferID: OfferGood, Verb: VerbGood})
	s = must(t, s, s.Current, module.Action{OfferID: OfferGood, Verb: VerbGood})
	if s.Phase != phaseFlek {
		t.Fatalf("both answered good, phase is %s", s.Phase)
	}
}

func TestNothingGoesOverDurch(t *testing.T) {
	s, c := announced(t, twelve, OfferDurch)
	s = must(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"7S", "8S"}})
	if s.Phase != phaseFlek {
		t.Fatalf("after a durch talon the phase is %s, want the doubling round", s.Phase)
	}
}

// doubling is a deal in the doubling round: p1 declared hra, p2 to speak.
func doubling(t *testing.T, opts module.Options) *GameState {
	s := playState(t, map[string][]string{"p1": {"AH"}, "p2": {"7H"}, "p3": {"8C"}})
	if opts != nil {
		s.FlekLimit = opts[OptFlekLimit]
	}
	s.Phase, s.Current = phaseFlek, "p2"
	return s
}

func TestDoublingAlternatesBetweenTheSides(t *testing.T) {
	s := doubling(t, nil)
	flek := module.Action{OfferID: OfferFlekGame, Verb: VerbFlek}
	s = must(t, s, "p2", flek)
	refused(t, s, "p2", flek, ErrNotYoursToFlek) // the re is the declarer's
	s = must(t, s, "p2", module.Action{OfferID: OfferPass, Verb: VerbPass})
	s = must(t, s, "p3", module.Action{OfferID: OfferPass, Verb: VerbPass})
	s = must(t, s, "p1", flek) // re
	if s.Fleks[partGame] != 2 {
		t.Fatalf("flek and re made %d doublings", s.Fleks[partGame])
	}
	for _, p := range []string{"p1", "p2", "p3"} {
		s = must(t, s, p, module.Action{OfferID: OfferPass, Verb: VerbPass})
	}
	if s.Phase != phasePlay || s.Current != "p1" {
		t.Fatalf("a full circle of passes left phase %s with %s to act", s.Phase, s.Current)
	}

	limited := doubling(t, module.Options{OptFlekLimit: 1})
	limited = must(t, limited, "p2", flek)
	limited.Current = "p1"
	refused(t, limited, "p1", flek, ErrFlekLimit)
}

func TestProtiNeedsItsOwnCard(t *testing.T) {
	s := doubling(t, nil)
	refused(t, s, "p3", module.Action{OfferID: OfferProtiSedma, Verb: VerbProti}, ErrNotYourTurn)
	s = must(t, s, "p2", module.Action{OfferID: OfferProtiSedma, Verb: VerbProti})
	if s.ProtiSedma != "p2" || !s.hasPart(partProtiSedma) {
		t.Fatal("sedma proti was not recorded")
	}
	// The declarer doubles a defenders' part first.
	refused(t, s, "p2", module.Action{OfferID: OfferFlekPSedma, Verb: VerbFlek}, ErrNotYoursToFlek)
}

func TestPlayRefusalsNameTheObligation(t *testing.T) {
	s := playState(t, map[string][]string{
		"p1": {"KC", "AS"},
		"p2": {"AC", "7C", "7S"},
		"p3": {"7H", "9H", "TD"},
	})
	s = must(t, s, "p1", module.Action{OfferID: OfferPlay, Verb: VerbPlay, Cards: []string{"KC"}})
	refused(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"7S"}}, ErrMustFollow)
	refused(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"7C"}}, ErrMustBeat)
	s = must(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"AC"}})
	refused(t, s, "p3", module.Action{Verb: VerbPlay, Cards: []string{"TD"}}, ErrMustTrump)

	over := playState(t, map[string][]string{
		"p1": {"KC"}, "p2": {"9H"}, "p3": {"7H", "AH", "TD"},
	})
	over = must(t, over, "p1", module.Action{Verb: VerbPlay, Cards: []string{"KC"}})
	over = must(t, over, "p2", module.Action{Verb: VerbPlay, Cards: []string{"9H"}})
	refused(t, over, "p3", module.Action{Verb: VerbPlay, Cards: []string{"7H"}}, ErrMustOvertrump)
}

func TestAnAnnouncedSevenWaitsForTheLastTrick(t *testing.T) {
	s := playState(t, map[string][]string{
		"p1": {"7H", "9H"}, "p2": {"AC", "KC"}, "p3": {"8C", "9C"},
	})
	s.Sedma = true
	refused(t, s, "p1", module.Action{Verb: VerbPlay, Cards: []string{"7H"}}, ErrKeepSeven)
	if got := s.legal("p1"); len(got) != 1 || got[0] != "9H" {
		t.Fatalf("legal = %v, want only 9H", got)
	}
}

func TestASvrsekPlayedBeforeItsKralIsAMarriage(t *testing.T) {
	s := playState(t, map[string][]string{
		"p1": {"QC", "KC", "QS"}, "p2": {"7C", "8C", "9C"}, "p3": {"AC", "TC", "JC"},
	})
	s = must(t, s, "p1", module.Action{Verb: VerbPlay, Cards: []string{"QC"}})
	if got := s.Marriages["p1"]; len(got) != 1 || got[0] != "C" {
		t.Fatalf("marriages = %v, want clubs", got)
	}
}

// ended is a finished trump deal: points as given, p1 declaring, hearts or
// clubs as trumps.
func ended(t *testing.T, trump string, points map[string]int, marriages map[string][]string) *GameState {
	s := playState(t, map[string][]string{"p1": nil, "p2": nil, "p3": nil})
	s.TrumpCard = "9" + trump
	s.Points = points
	if marriages != nil {
		s.Marriages = marriages
	}
	s.LastTrick = []tricks.Play{{Seat: 0, Card: "AS"}, {Seat: 1, Card: "8S"}, {Seat: 2, Card: "9S"}}
	return s
}

func part(t *testing.T, s *GameState, name string) PartResult {
	t.Helper()
	for _, p := range s.results() {
		if p.Part == name {
			return p
		}
	}
	t.Fatalf("no %s part in %+v", name, s.results())
	return PartResult{}
}

func TestSettlement(t *testing.T) {
	// Hra, won on cards.
	s := ended(t, "C", map[string]int{"p1": 60, "p2": 20, "p3": 10}, nil)
	if p := part(t, s, partGame); !p.ForDeclarer || p.Units != 1 {
		t.Errorf("hra 60–30: %+v", p)
	}
	// Lost to the defenders' marriage.
	s = ended(t, "C", map[string]int{"p1": 50, "p2": 20, "p3": 20}, map[string][]string{"p2": {"C"}})
	if p := part(t, s, partGame); p.ForDeclarer || p.Units != 1 {
		t.Errorf("hra 50 against 40+40: %+v", p)
	}
	// A quiet hundred and twenty: the doubled game, and the doubled game
	// again for each of the two tens past a hundred (ČSM general V/7).
	s = ended(t, "C", map[string]int{"p1": 80, "p2": 10}, map[string][]string{"p1": {"C"}})
	if p := part(t, s, partGame); !p.ForDeclarer || p.Units != 6 {
		t.Errorf("hra with a quiet 120: %+v", p)
	}
	// Hearts double; a flek doubles again.
	s = ended(t, "H", map[string]int{"p1": 60, "p2": 30}, nil)
	s.Fleks[partGame] = 1
	if p := part(t, s, partGame); p.Units != 4 {
		t.Errorf("hra in hearts, flekked: %d units, want 4", p.Units)
	}
	// An announced sto that falls short pays the sto tariff flat.
	s = ended(t, "C", map[string]int{"p1": 90}, nil)
	s.Game = gameSto
	if p := part(t, s, partGame); p.ForDeclarer || p.Units != 4 {
		t.Errorf("sto at 90: %+v", p)
	}
}

func TestSevens(t *testing.T) {
	// An announced seven that takes the last trick.
	s := ended(t, "C", map[string]int{"p1": 60}, nil)
	s.Sedma = true
	s.LastTrick = []tricks.Play{{Seat: 0, Card: "7C"}, {Seat: 1, Card: "8S"}, {Seat: 2, Card: "9S"}}
	if p := part(t, s, partSedma); !p.ForDeclarer || p.Units != 2 {
		t.Errorf("sedma won: %+v", p)
	}
	// Beaten in the last trick.
	s.LastTrick[1].Card = "AC"
	if p := part(t, s, partSedma); p.ForDeclarer {
		t.Errorf("sedma beaten by the ace: %+v", p)
	}
	// Unannounced and killed: the side that played it pays half a seven.
	s.Sedma = false
	if p := part(t, s, partQuietSeven); p.ForDeclarer || p.Units != 1 {
		t.Errorf("killed quiet seven: %+v", p)
	}
}

func TestBetlEndsAtTheDeclarersFirstTrick(t *testing.T) {
	s := playState(t, map[string][]string{
		"p1": {"AC", "7S"}, "p2": {"7C", "8S"}, "p3": {"8C", "9S"},
	})
	s.Game, s.TrumpCard = gameBetl, "9H"
	s = must(t, s, "p1", module.Action{Verb: VerbPlay, Cards: []string{"AC"}})
	s = must(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"7C"}})
	s = must(t, s, "p3", module.Action{Verb: VerbPlay, Cards: []string{"8C"}})
	if len(s.Rounds) != 1 {
		t.Fatalf("betl not settled at the declarer's first trick (rounds: %d)", len(s.Rounds))
	}
	d := s.Rounds[0]
	if d.Parts[0].ForDeclarer || d.Deltas["p1"] != -30 || d.Deltas["p2"] != 15 {
		t.Fatalf("lost betl settled as %+v", d)
	}
}

func TestEveryDealIsZeroSum(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, players, 7)
	final, _, err := module.PlayWithOffers(m, raw, players, module.DriverOptions{
		MaxActions: 5000, Prefer: []string{"play_card", "discard", "choose_trump", "announce", "good", "pass"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s, _ := decode(final)
	if s.Status != "completed" || len(s.Rounds) != s.Deals {
		t.Fatalf("status %s after %d of %d deals", s.Status, len(s.Rounds), s.Deals)
	}
	for _, d := range s.Rounds {
		sum := 0
		for _, v := range d.Deltas {
			sum += v
		}
		if sum != 0 {
			t.Errorf("deal %d pays %+v, which sums to %d", d.Number, d.Deltas, sum)
		}
	}
}

func TestTheOthersDoNotSeeTrumpsOrTheTalon(t *testing.T) {
	s, c := announced(t, twelve, OfferHra)
	s = must(t, s, c, module.Action{OfferID: OfferTalon, Verb: VerbDiscard, Cards: []string{"8C", "JD"}})
	raw, _ := encode(s)
	for _, p := range s.Players {
		vm, err := New().View(raw, p)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range vm.Header {
			if f.LabelKey == "marias.header.trumps" && p != c {
				t.Errorf("%s sees trumps before the game is agreed", p)
			}
		}
		for _, z := range vm.Zones {
			if z.ID == talonZoneID && len(z.Cards) > 0 && p != c {
				t.Errorf("%s sees the talon", p)
			}
		}
	}
}

// Every refusal this engine can hand a player points at a rule that table
// actually states.
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
