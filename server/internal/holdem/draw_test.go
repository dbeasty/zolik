package holdem

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

var drawCfg = module.MatchConfig{Variation: VarDraw, Options: module.Options{OptHandLimit: 10}}

func seatIDs(n int) []module.PlayerRef {
	ids := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		ids = append(ids, "p"+strconv.Itoa(i))
	}
	return refs(ids...)
}

// drawTable is a Draw hand at the draw itself, built directly.
func drawTable(seats int, mutate func(s *GameState)) module.State {
	return table(seats, func(s *GameState) {
		s.Variation = VarDraw
		s.Street = streetDraw
		s.CurrentBet, s.MinRaise = 0, s.BigBlind
		s.Current = 1
		hands := [][]string{
			{"AS", "AH", "7C", "4D", "2S"},
			{"KD", "QD", "JD", "9D", "3C"},
			{"8S", "8H", "8C", "5D", "5S"},
			{"TC", "6H", "4S", "3D", "2H"},
		}
		for i := range s.Seats {
			s.Seats[i].Hole = append([]string(nil), hands[i%len(hands)]...)
		}
		deck := buildDeck()
		var rest []string
		for _, c := range deck {
			held := false
			for i := range s.Seats {
				if indexOf(s.Seats[i].Hole, c) >= 0 {
					held = true
				}
			}
			if !held {
				rest = append(rest, c)
			}
		}
		s.Deck = rest
		if mutate != nil {
			mutate(s)
		}
	})
}

func TestDrawDealsFiveAndNoBoard(t *testing.T) {
	raw, err := New().NewMatch(drawCfg, seatIDs(4), 3)
	if err != nil {
		t.Fatal(err)
	}
	s := mustDecode(t, raw)
	if s.Street != streetPredraw {
		t.Fatalf("a draw hand opens on %q, want %q", s.Street, streetPredraw)
	}
	for _, st := range s.Seats {
		if len(st.Hole) != 5 {
			t.Errorf("%s was dealt %d cards, want 5", st.PlayerID, len(st.Hole))
		}
	}
	if len(s.Board) != 0 {
		t.Errorf("a draw hand has a board: %v", s.Board)
	}
	if len(s.Deck) != 52-20 {
		t.Errorf("%d cards left in the deck, want 32", len(s.Deck))
	}
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		if z.ID == "board" {
			t.Error("a draw table shows a board zone")
		}
	}
}

func TestDrawSeatsAtMostSix(t *testing.T) {
	if _, err := New().NewMatch(drawCfg, seatIDs(7), 1); err == nil {
		t.Error("seven seats at a draw table should be refused: the deck cannot cover their draws")
	}
	if _, err := New().NewMatch(drawCfg, seatIDs(6), 1); err != nil {
		t.Errorf("six seats should deal: %v", err)
	}
	d := New().Descriptor()
	if lo, hi := d.SeatRange(VarDraw); lo != 2 || hi != 6 {
		t.Errorf("the lobby seats a draw table %d..%d, want 2..6", lo, hi)
	}
	if _, hi := d.SeatRange(VarHoldem); hi != 9 {
		t.Errorf("Hold'em should still seat nine, got %d", hi)
	}
}

// TestDrawOpensAfterTheFirstRoundAndGoesClockwise plays the first round by
// calling and checking, then checks the draw starts left of the button and
// visits every seat still in once.
func TestDrawOpensAfterTheFirstRoundAndGoesClockwise(t *testing.T) {
	m := New()
	players := seatIDs(4)
	raw, err := m.NewMatch(drawCfg, players, 5)
	if err != nil {
		t.Fatal(err)
	}
	for mustDecode(t, raw).Street == streetPredraw {
		s := mustDecode(t, raw)
		actor := s.Seats[s.Current].PlayerID
		verb := VerbCall
		if s.toCall(&s.Seats[s.Current]) == 0 {
			verb = VerbCheck
		}
		if raw, _, err = m.Apply(raw, actor, module.Action{Verb: verb}); err != nil {
			t.Fatalf("%s %s: %v", actor, verb, err)
		}
	}
	s := mustDecode(t, raw)
	if s.Street != streetDraw {
		t.Fatalf("after the first round the street is %q, want the draw", s.Street)
	}
	want := s.nextSeat(s.Button, func(st *Seat) bool { return st.inHand() })
	var order []int
	for mustDecode(t, raw).Street == streetDraw {
		s := mustDecode(t, raw)
		order = append(order, s.Current)
		if raw, _, err = m.Apply(raw, s.Seats[s.Current].PlayerID, module.Action{Verb: VerbStand}); err != nil {
			t.Fatal(err)
		}
	}
	if len(order) != 4 || order[0] != want {
		t.Fatalf("the draw went %v, want four seats starting at %d", order, want)
	}
	for i := 1; i < len(order); i++ {
		if order[i] != (order[i-1]+1)%4 {
			t.Fatalf("the draw went %v, not clockwise", order)
		}
	}
	s = mustDecode(t, raw)
	if s.Street != streetPostdraw || s.CurrentBet != 0 {
		t.Fatalf("after the draw: street %q, bet %d; want a fresh betting round", s.Street, s.CurrentBet)
	}
}

func TestDiscardReplacesTheCardsAndTellsOnlyTheCount(t *testing.T) {
	m := New()
	raw := drawTable(3, nil)
	before := mustDecode(t, raw)
	threw := []string{"3C", "9D"}
	after, events, err := m.Apply(raw, "p2", module.Action{Verb: VerbDiscard, Cards: threw})
	if err != nil {
		t.Fatal(err)
	}
	s := mustDecode(t, after)
	st := s.seat("p2")
	if len(st.Hole) != 5 || !st.Drawn || st.Drew != 2 {
		t.Fatalf("after drawing two: hole %v drawn %v drew %d", st.Hole, st.Drawn, st.Drew)
	}
	for _, c := range threw {
		if indexOf(st.Hole, c) >= 0 {
			t.Errorf("%s was thrown and is still held", c)
		}
	}
	for _, c := range []string{"KD", "QD", "JD"} {
		if indexOf(st.Hole, c) < 0 {
			t.Errorf("%s was kept and is gone", c)
		}
	}
	if len(s.Deck) != len(before.Deck)-2 {
		t.Errorf("the deck shrank by %d, want 2", len(before.Deck)-len(s.Deck))
	}

	// The event and every other seat's view carry the count and nothing else.
	blob, _ := json.Marshal(events)
	for _, c := range append(append([]string(nil), threw...), st.Hole...) {
		if strings.Contains(string(blob), `"`+c+`"`) {
			t.Errorf("the draw's events name %s: %s", c, blob)
		}
	}
	vm, err := m.View(after, "p1")
	if err != nil {
		t.Fatal(err)
	}
	vmBlob, _ := json.Marshal(vm)
	for _, c := range append(append([]string(nil), threw...), st.Hole...) {
		if strings.Contains(string(vmBlob), `"`+c+`"`) {
			t.Errorf("p1's view shows p2's %s", c)
		}
	}
	drewShown := false
	for _, seat := range vm.Seats {
		for _, f := range seat.Facts {
			if seat.PlayerID == "p2" && f.LabelKey == "holdem.seat.drew" && f.Value == "2" {
				drewShown = true
			}
		}
	}
	if !drewShown {
		t.Error("the table is not told that p2 drew two")
	}
	// The next seat is on.
	if s.Current != 2 {
		t.Errorf("after p2 draws, seat %d is on; want p3", s.Current)
	}
}

func TestDrawRefusals(t *testing.T) {
	m := New()
	cases := []struct {
		name string
		raw  module.State
		who  string
		a    module.Action
		want string
	}{
		{"four cards", drawTable(3, nil), "p2", module.Action{Verb: VerbDiscard, Cards: []string{"KD", "QD", "JD", "9D"}}, ErrDrawTooMany},
		{"no cards", drawTable(3, nil), "p2", module.Action{Verb: VerbDiscard}, ErrDrawEmpty},
		{"a card not held", drawTable(3, nil), "p2", module.Action{Verb: VerbDiscard, Cards: []string{"AS"}}, ErrCardNotInHand},
		{"the same card twice", drawTable(3, nil), "p2", module.Action{Verb: VerbDiscard, Cards: []string{"3C", "3C"}}, ErrCardNotInHand},
		{"a bet during the draw", drawTable(3, nil), "p2", raiseTo(40), ErrDrawing},
		{"a check during the draw", drawTable(3, nil), "p2", module.Action{Verb: VerbCheck}, ErrDrawing},
		{"out of turn", drawTable(3, nil), "p3", module.Action{Verb: VerbStand}, ErrNotYourTurn},
		{"a discard while betting", drawTable(3, func(s *GameState) { s.Street = streetPostdraw }), "p2",
			module.Action{Verb: VerbDiscard, Cards: []string{"3C"}}, ErrNotDrawing},
		{"a stand while betting", drawTable(3, func(s *GameState) { s.Street = streetPredraw }), "p2",
			module.Action{Verb: VerbStand}, ErrNotDrawing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := m.Apply(tc.raw, tc.who, tc.a)
			if got := module.CodeOf(err); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAllInSeatsStillDraw: chips decide who may bet, not who holds a hand.
func TestAllInSeatsStillDraw(t *testing.T) {
	m := New()
	raw := drawTable(2, func(s *GameState) {
		s.Current = 1
		for i := range s.Seats {
			s.Seats[i].Stack, s.Seats[i].AllIn, s.Seats[i].Committed = 0, true, 1000
		}
		s.Pot = 2000
	})
	var err error
	for i := 0; i < 2; i++ {
		s := mustDecode(t, raw)
		if s.Street != streetDraw {
			t.Fatalf("draw %d: street is %q, want the draw", i, s.Street)
		}
		if raw, _, err = m.Apply(raw, s.Seats[s.Current].PlayerID, module.Action{Verb: VerbStand}); err != nil {
			t.Fatalf("an all-in seat could not draw: %v", err)
		}
	}
	s := mustDecode(t, raw)
	if s.LastHand == nil || !s.Break.Open {
		t.Fatalf("with both seats all in, the hand should go straight to the showdown after the draw")
	}
	if len(s.LastHand.Shown) != 2 || s.LastHand.Shown[0].LabelKey == "" {
		t.Errorf("the showdown should name both five-card hands: %+v", s.LastHand.Shown)
	}
}

func TestDrawOffers(t *testing.T) {
	m := New()
	raw := drawTable(3, nil)
	offers, err := m.LegalActions(raw, "p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 || offers[0].Verb != VerbStand || offers[1].Verb != VerbDiscard {
		t.Fatalf("the drawing seat should be offered stand then discard, got %+v", offers)
	}
	d := offers[1]
	if !d.Enabled || !d.Composite || d.Source == nil || d.Source.MinCards != 1 || d.Source.MaxCards != 3 || len(d.Source.Cards) != 5 {
		t.Errorf("discard should be a composite pick of one to three of five: %+v", d)
	}
	others, err := m.LegalActions(raw, "p3")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range others {
		if o.Enabled || o.WhyNot != ErrNotYourTurn {
			t.Errorf("a seat waiting to draw should have %s off with NOT_YOUR_TURN, got %+v", o.ID, o)
		}
		if o.Source != nil {
			t.Errorf("a waiting seat's %s lists cards", o.ID)
		}
	}
}

func TestDrawChoiceIsTheTextbook(t *testing.T) {
	cases := []struct {
		hand     string
		beginner bool
		throws   string
	}{
		{"AS KS QS JS TS", false, ""},         // made: stand pat
		{"9C 9D 9H 2S 2C", false, ""},         // full house
		{"5C 6D 7H 8S 9C", false, ""},         // straight
		{"7C 7D 7H KS 2C", false, "2C KS"},    // trips: draw two
		{"7C 7D KH KS 2C", false, "2C"},       // two pair: draw one
		{"JC JD 9H 5S 2C", false, "2C 5S 9H"}, // a pair: draw three
		{"JC JD 9H 5S 2C", true, "2C 5S"},     // a beginner keeps the kicker
		{"AH 9H 6H 3H KC", false, "KC"},       // four to a flush: draw one
		{"5C 6D 7H 8S KC", false, "KC"},       // open-ended: draw one
		{"2C 3D 4H 5S KC", false, "KC"},       // 2-3-4-5 is open (ace or six)
		{"JC QD KH AS 3C", false, "3C JC QD"}, // J-Q-K-A is one-ended: keep A K
		{"AC KD 8H 5S 2C", false, "2C 5S 8H"}, // nothing: keep the top two
	}
	for _, tc := range cases {
		got := drawChoice(strings.Fields(tc.hand), tc.beginner)
		sort.Strings(got)
		want := strings.Fields(tc.throws)
		sort.Strings(want)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s (beginner %v): throws %v, want %v", tc.hand, tc.beginner, got, want)
		}
	}
}

// TestDrawBotPlaysOnlyLegalMovesToTheEnd: whole matches at every table size
// Draw seats, every skill on the ladder, every move asserted legal on the
// spot, and the chips counted at the end.
func TestDrawBotPlaysOnlyLegalMovesToTheEnd(t *testing.T) {
	m := New()
	for seats := 2; seats <= 6; seats++ {
		players := seatIDs(seats)
		for seed := int64(1); seed <= 3; seed++ {
			state, err := m.NewMatch(module.MatchConfig{
				Variation: VarDraw,
				Options:   module.Options{OptStartingStack: 400, OptBigBlind: 20, OptHandLimit: 8},
			}, players, seed)
			if err != nil {
				t.Fatal(err)
			}
			skills := []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard}
			discards := 0
			final := playSkilled(t, state, players, func(i int) module.Skill { return skills[(i+int(seed))%3] }, 6000,
				func(a module.Action) {
					if a.Verb == VerbDiscard {
						discards++
					}
				})
			total := 0
			for i := range final.Seats {
				total += final.Seats[i].Stack
			}
			if want := 400 * seats; total != want {
				t.Errorf("%d seats seed %d: %d chips in play, want %d", seats, seed, total, want)
			}
			if discards == 0 {
				t.Errorf("%d seats seed %d: nobody ever drew a card", seats, seed)
			}
		}
	}
}

// playSkilled is playBots with a skill per seat, and a look at every move.
func playSkilled(t *testing.T, state module.State, players []module.PlayerRef, skill func(int) module.Skill, maxActions int, saw func(module.Action)) *GameState {
	t.Helper()
	m := New()
	b := m.Bot()
	for step := 0; step < maxActions; step++ {
		s := mustDecode(t, state)
		if s.Status != "active" {
			return s
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return s
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		a, ok := b.Act(state, module.BotSeat{PlayerID: actor, Skill: skill(s.seatIndex(actor))}, offers)
		if !ok {
			t.Fatalf("%s had no move on %s", actor, s.Street)
		}
		saw(a)
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("%s proposed an illegal %+v on %s: %v", actor, a, s.Street, err)
		}
		state = next
	}
	t.Fatalf("match did not finish in %d actions", maxActions)
	return nil
}

// TestDrawBotDoesNotPeek: the same position with the other hands, the deck
// and the other seat's discards changed gets the same decision, before the
// draw, at it, and after it.
func TestDrawBotDoesNotPeek(t *testing.T) {
	for _, street := range []string{streetPredraw, streetDraw, streetPostdraw} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			build := func(theirs []string, reversed bool) module.State {
				return drawTable(2, func(s *GameState) {
					s.Street = street
					s.Current = 0
					s.Seats[0].Hole = []string{"JC", "JD", "9H", "5S", "2C"}
					s.Seats[1].Hole = theirs
					if street == streetPostdraw {
						s.Seats[1].Drawn, s.Seats[1].Drew = true, 1
						s.Seats[1].Discarded = []string{theirs[0]}
						s.Seats[0].Drawn, s.Seats[0].Drew = true, 3
						s.Seats[0].Discarded = []string{"4C", "6D", "8H"}
						s.Seats[1].Bet, s.Seats[1].Acted, s.CurrentBet = 60, true, 60
						s.Pot = 80
					}
					if reversed {
						for i, j := 0, len(s.Deck)-1; i < j; i, j = i+1, j-1 {
							s.Deck[i], s.Deck[j] = s.Deck[j], s.Deck[i]
						}
					}
				})
			}
			act := func(raw module.State) module.Action {
				offers, err := New().LegalActions(raw, "p1")
				if err != nil {
					t.Fatal(err)
				}
				a, ok := New().Bot().Act(raw, module.BotSeat{PlayerID: "p1", Skill: skill}, offers)
				if !ok {
					t.Fatalf("%s %s: no move", street, skill)
				}
				return a
			}
			a := act(build([]string{"3H", "4H", "6S", "7D", "KS"}, false))
			for _, rigged := range []module.State{
				build([]string{"AS", "AH", "AD", "KC", "KH"}, false),
				build([]string{"3H", "4H", "6S", "7D", "KS"}, true),
			} {
				b := act(rigged)
				if a.Verb != b.Verb || a.Params[ParamAmount] != b.Params[ParamAmount] || strings.Join(a.Cards, ",") != strings.Join(b.Cards, ",") {
					t.Errorf("%s %s: played %+v, and %+v when hidden cards changed", street, skill, a, b)
				}
			}
		}
	}
}

// TestDrawRulesAreWrittenForDraw: the rules a Draw table is shown describe
// the draw, and a Hold'em table's do not.
func TestDrawRulesAreWrittenForDraw(t *testing.T) {
	has := func(cfg module.MatchConfig, key string) bool {
		secs, err := New().Rules(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, sec := range secs {
			for _, it := range sec.Items {
				if it.LabelKey == key {
					return true
				}
			}
		}
		return false
	}
	for _, key := range []string{"holdem.rules.draw.deal", "holdem.rules.draw.streets", "holdem.rules.draw.draw", "holdem.rules.draw.public"} {
		if !has(drawCfg, key) {
			t.Errorf("Draw's rules leave out %s", key)
		}
		if has(module.MatchConfig{Variation: VarHoldem}, key) {
			t.Errorf("Hold'em's rules state %s", key)
		}
	}
	if has(drawCfg, "holdem.rules.streets") {
		t.Error("Draw's rules describe the flop, turn and river")
	}
}

// TestDrawBotLadderIsOrdered: every skill beats a calling station that stands
// pat on everything, Medium beats Easy, and Hard beats Medium — on fixed
// seeds with both seatings, so it is a measurement that does not move.
//
// The bars are generous for the same reason TestBotLadderIsOrdered's are: a
// few dozen poker matches carry a standard error of several big blinds each.
// The sweep behind the numbers in drawbot.go is 300 matches.
func TestDrawBotLadderIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	m := New()
	station := module.OfferBot(VerbCall, VerbCheck, VerbStand, VerbRaise, VerbFold)
	type side struct {
		bot   module.Bot
		skill module.Skill
	}
	hard, medium, easy := side{m.Bot(), module.SkillHard}, side{m.Bot(), module.SkillMedium}, side{m.Bot(), module.SkillEasy}
	const seeds = 30
	net := func(a, b side) int {
		total := 0
		for seed := int64(1); seed <= seeds; seed++ {
			for _, ids := range [][]string{{"A", "B"}, {"B", "A"}} {
				players := refs(ids...)
				state, err := m.NewMatch(module.MatchConfig{Variation: VarDraw, Options: module.Options{
					OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 15,
				}}, players, seed)
				if err != nil {
					t.Fatal(err)
				}
				final := playSkilledBy(t, state, players, map[string]side{"A": a, "B": b}, 8000,
					func(s side) (module.Bot, module.Skill) { return s.bot, s.skill })
				total += final.seat("A").Stack - 1000
			}
		}
		return total
	}
	for _, c := range []struct {
		name string
		a, b side
		min  int
	}{
		{"hard vs station", hard, side{station, ""}, 1},
		{"medium vs station", medium, side{station, ""}, 1},
		{"easy vs station", easy, side{station, ""}, 1},
		{"medium vs easy", medium, easy, 1},
		{"hard vs medium", hard, medium, 1},
	} {
		got := net(c.a, c.b)
		t.Logf("%s: %+.1f big blinds a match", c.name, float64(got)/float64(2*seeds)/20)
		if got < c.min {
			t.Errorf("%s: %+d chips over %d matches, want a profit", c.name, got, 2*seeds)
		}
	}
}

// playSkilledBy plays a match with a bot and skill per player.
func playSkilledBy[S any](t *testing.T, state module.State, players []module.PlayerRef, by map[string]S, maxActions int, pick func(S) (module.Bot, module.Skill)) *GameState {
	t.Helper()
	m := New()
	for step := 0; step < maxActions; step++ {
		s := mustDecode(t, state)
		if s.Status != "active" {
			return s
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return s
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		b, skill := pick(by[actor])
		a, ok := b.Act(state, module.BotSeat{PlayerID: actor, Skill: skill}, offers)
		if !ok {
			t.Fatalf("%s had no move", actor)
		}
		if state, _, err = m.Apply(state, actor, a); err != nil {
			t.Fatalf("%s proposed an illegal %+v: %v", actor, a, err)
		}
	}
	t.Fatalf("match did not finish in %d actions", maxActions)
	return nil
}
