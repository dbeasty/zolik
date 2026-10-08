package sedma

import (
	"math/rand"
	"slices"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

func refs(n int) []module.PlayerRef {
	var out []module.PlayerRef
	for i := 1; i <= n; i++ {
		out = append(out, module.PlayerRef{ID: "p" + string(rune('0'+i))})
	}
	return out
}

func newState(t *testing.T, n int) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{}, refs(n), 42)
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

func card(c string) module.Action {
	return module.Action{OfferID: OfferPlay, Verb: VerbPlay, Cards: []string{c}}
}

var stopAction = module.Action{OfferID: OfferStop, Verb: VerbStop}

// fourAt is a four-player hand with these hands and stock, p1 leading.
func fourAt(t *testing.T, hands map[string][]string, stock []string) *GameState {
	t.Helper()
	s := newState(t, 4)
	s.FirstDealer, s.Deal = 3, 0 // p4 deals, p1 leads
	s.Hands, s.Stock = hands, stock
	s.Leader, s.Current, s.Phase = "p1", "p1", phasePlay
	return s
}

func TestTheDeal(t *testing.T) {
	for _, c := range []struct{ n, stock int }{{2, 24}, {3, 18}, {4, 16}} {
		s := newState(t, c.n)
		if len(s.Stock) != c.stock {
			t.Errorf("%d players: stock of %d, want %d", c.n, len(s.Stock), c.stock)
		}
		seen := map[string]bool{}
		for _, p := range s.Players {
			if len(s.Hands[p]) != handSize {
				t.Errorf("%d players: %s holds %d", c.n, p, len(s.Hands[p]))
			}
			for _, x := range s.Hands[p] {
				seen[x] = true
			}
		}
		for _, x := range s.Stock {
			seen[x] = true
		}
		if want := len(buildDeck(c.n)); len(seen) != want {
			t.Errorf("%d players: %d distinct cards, want %d", c.n, len(seen), want)
		}
		if s.Leader != s.next(s.dealer()) || s.Current != s.Leader {
			t.Errorf("%d players: %s leads, dealer %s", c.n, s.Leader, s.dealer())
		}
	}
	if d := buildDeck(3); slices.Contains(d, "8D") || slices.Contains(d, "8S") || len(d) != 30 {
		t.Errorf("three-player pack: %v", d)
	}
}

func TestTheLastCaptureTakesTheTrick(t *testing.T) {
	for _, c := range []struct {
		trick []string
		want  int
	}{
		{[]string{"KH", "9H", "AH", "JD"}, 0},       // nobody matched: the leader
		{[]string{"KH", "KD", "AH", "JD"}, 1},       // the king of bells
		{[]string{"KH", "KD", "7S", "JD"}, 2},       // the seven, after the king
		{[]string{"KH", "7D", "KS", "JD"}, 2},       // a king after the seven
		{[]string{"7H", "KD", "AH", "7C"}, 3},       // a seven led: another seven
		{[]string{"KH", "KD", "7S", "JD", "KC"}, 0}, // continued: the leader's king
	} {
		var plays []tricks.Play
		for i, x := range c.trick {
			plays = append(plays, tricks.Play{Seat: i % 4, Card: x})
		}
		if got := winning(plays); got != c.want {
			t.Errorf("%v: seat %d takes it, want %d", c.trick, got, c.want)
		}
	}
}

func TestContinuingATrick(t *testing.T) {
	s := fourAt(t, map[string][]string{
		"p1": {"KH", "KC", "AH", "8D"}, "p2": {"KD", "9S", "QS", "JS"},
		"p3": {"TH", "9H", "QH", "JH"}, "p4": {"AS", "8S", "8H", "9C"},
	}, []string{"TC", "TD", "TS", "QD", "QC", "JC", "JD", "AD", "AC", "7H", "7D", "7C", "7S", "9D", "8C", "KS"})
	s = must(t, s, "p1", card("KH"))
	refused(t, s, "p1", card("KC"), ErrNotYourTurn)
	s = must(t, s, "p2", card("KD")) // p2 captures
	s = must(t, s, "p3", card("TH"))
	s = must(t, s, "p4", card("8H"))
	if s.Phase != phaseDecide || s.Current != "p1" {
		t.Fatalf("after a round: phase %s, %s to act", s.Phase, s.Current)
	}
	refused(t, s, "p1", card("AH"), ErrMustCapture)
	s = must(t, s, "p1", card("KC")) // takes it back
	s = must(t, s, "p2", card("9S"))
	s = must(t, s, "p3", card("9H"))
	s = must(t, s, "p4", card("AS"))
	// p1 has no king or seven left: the trick ends on its own.
	if len(s.Trick) != 0 || s.Points["0"] != 20 || s.Cards["0"] != 8 {
		t.Fatalf("trick %v, side 0 has %d points in %d cards", s.Trick, s.Points["0"], s.Cards["0"])
	}
	// Two rounds played, so two cards each are drawn, the winner first.
	for _, p := range s.Players {
		if len(s.Hands[p]) != handSize {
			t.Errorf("%s holds %d after the draw", p, len(s.Hands[p]))
		}
	}
	if !hasCard(s.Hands["p1"], "TC") || !hasCard(s.Hands["p2"], "TD") || len(s.Stock) != 8 {
		t.Errorf("draw order: p1 %v p2 %v stock %d", s.Hands["p1"], s.Hands["p2"], len(s.Stock))
	}
	if s.Leader != "p1" || s.Current != "p1" {
		t.Errorf("%s leads, want the winner p1", s.Leader)
	}
}

func TestTheLeaderMayLetATrickGo(t *testing.T) {
	s := fourAt(t, map[string][]string{
		"p1": {"KH", "7C", "AH", "8D"}, "p2": {"KD", "9S", "QS", "JS"},
		"p3": {"TH", "9H", "QH", "JH"}, "p4": {"AS", "8S", "8H", "9C"},
	}, nil)
	s = must(t, s, "p1", card("KH"))
	refused(t, s, "p2", stopAction, ErrNotDeciding)
	s = must(t, s, "p2", card("KD"))
	s = must(t, s, "p3", card("TH"))
	s = must(t, s, "p4", card("AS"))
	s = must(t, s, "p1", stopAction)
	if s.Points["1"] != 20 || s.Leader != "p2" {
		t.Fatalf("side 1 has %d, %s leads; want 20 and p2", s.Points["1"], s.Leader)
	}
}

func TestScoringAHand(t *testing.T) {
	end := func(n int, points, cards map[string]int) HandRecord {
		s := newState(t, n)
		s.Pause = true
		s.Points, s.Cards = points, cards
		s.endHand()
		return s.Rounds[0]
	}
	for _, c := range []struct {
		name   string
		n      int
		points map[string]int
		cards  map[string]int
		kind   string
		stakes int
		won    int
	}{
		{"most points", 4, map[string]int{"0": 60, "1": 30}, map[string]int{"0": 20, "1": 12}, kindWon, 1, 2},
		{"all points", 4, map[string]int{"0": 90}, map[string]int{"0": 28, "1": 4}, kindAllPoints, 2, 2},
		{"every card", 4, map[string]int{"0": 90}, map[string]int{"0": 32}, kindAllCards, 3, 2},
		{"heads-up", 2, map[string]int{"0": 40, "1": 50}, map[string]int{"0": 16, "1": 16}, kindWon, 1, 1},
		{"three, two level", 3, map[string]int{"0": 40, "1": 40, "2": 10}, map[string]int{"0": 10, "1": 10, "2": 10}, kindTie, 1, 2},
		{"three, all level", 3, map[string]int{"0": 30, "1": 30, "2": 30}, map[string]int{"0": 10, "1": 10, "2": 10}, kindNone, 0, 0},
	} {
		h := end(c.n, c.points, c.cards)
		if h.Kind != c.kind || h.Stakes != c.stakes || len(h.Winners) != c.won {
			t.Errorf("%s: %+v", c.name, h)
		}
	}
}

func TestPartnersShareTheirScore(t *testing.T) {
	s := newState(t, 4)
	if got := New().Sides(module.MatchConfig{}, refs(4)); !slices.Equal(got[0], []string{"p1", "p3"}) {
		t.Fatalf("sides %v", got)
	}
	s.Pause = true
	s.Points, s.Cards = map[string]int{"1": 70, "0": 20}, map[string]int{"1": 20, "0": 12}
	s.endHand()
	if s.Scores["p2"] != 1 || s.Scores["p4"] != 1 || s.Scores["p1"] != 0 {
		t.Fatalf("scores %v", s.Scores)
	}
	s.Scores["p2"], s.Scores["p4"] = 4, 4
	s.Points, s.Cards = map[string]int{"1": 70, "0": 20}, map[string]int{"1": 20, "0": 12}
	s.Intermission.Close()
	s.endHand()
	raw, _ := encode(s)
	done, winners, _ := New().Finished(raw)
	if !done || !slices.Equal(winners, []string{"p2", "p4"}) {
		t.Fatalf("finished %v, winners %v", done, winners)
	}
}

func TestTheLastTrickIsWorthTen(t *testing.T) {
	s := fourAt(t, map[string][]string{"p1": {"8D"}, "p2": {"9S"}, "p3": {"QH"}, "p4": {"JS"}}, nil)
	s.Pause = true
	s.Points = map[string]int{"0": 40, "1": 40}
	for _, c := range []struct{ p, c string }{{"p1", "8D"}, {"p2", "9S"}, {"p3", "QH"}, {"p4", "JS"}} {
		s = must(t, s, c.p, card(c.c))
	}
	h := s.Rounds[0]
	if h.Points["p1"] != 50 || !slices.Contains(h.Winners, "p1") {
		t.Fatalf("hand %+v", h)
	}
}

func TestViewHidesWhatIsHidden(t *testing.T) {
	s := newState(t, 4)
	raw, _ := encode(s)
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		switch z.ID {
		case handZoneID("p2"), handZoneID("p3"), handZoneID("p4"), stockZoneID:
			if len(z.Cards) != 0 {
				t.Errorf("%s shows %v", z.ID, z.Cards)
			}
		}
		if z.ID == handZoneID("p3") && z.LabelKey != "sedma.zone.partnerHand" {
			t.Errorf("the partner's hand is labelled %s", z.LabelKey)
		}
	}
	for _, seat := range vm.Seats {
		if want := sideOf(s.seat(seat.PlayerID), 4); seat.Side != want {
			t.Errorf("%s on side %q, want %q", seat.PlayerID, seat.Side, want)
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

// --- bots -------------------------------------------------------------------

func playMatch(t *testing.T, n int, seed int64, skills map[string]module.Skill) map[string]int {
	t.Helper()
	m := New()
	state, err := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, refs(n), seed)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 5000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			return s.Scores
		}
		actor := s.Current
		offers, _ := m.LegalActions(state, actor)
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor], Seed: seed}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move", seed, actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s %v: %v", seed, actor, a.Verb, a.Cards, err)
		}
		state = next
	}
	t.Fatalf("seed %d: the match did not finish", seed)
	return nil
}

func all(n int, skill module.Skill) map[string]module.Skill {
	out := map[string]module.Skill{}
	for _, p := range refs(n) {
		out[p.ID] = skill
	}
	return out
}

func TestBotsPlayWholeMatchesLegally(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			seeds := int64(6)
			if skill == module.SkillHard {
				seeds = 2
			}
			for seed := int64(1); seed <= seeds; seed++ {
				playMatch(t, n, seed, all(n, skill))
			}
		}
	}
}

// TestBotDoesNotPeek: the same seat, the same hand and table, and the cards
// it cannot see dealt differently, must get the same move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 4; seed++ {
			raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, refs(4), seed)
			for step := 0; step < 30; step++ {
				s, _ := decode(raw)
				if s.Status != "active" || len(s.Rounds) > 0 {
					break
				}
				me := s.Current
				other := reshuffled(s, me, seed+int64(step))
				act := func(st *GameState) module.Action {
					enc, _ := encode(st)
					offers, _ := m.LegalActions(enc, me)
					a, _ := m.Bot().Act(enc, module.BotSeat{PlayerID: me, Skill: skill, Seed: seed}, offers)
					return a
				}
				a, b := act(s), act(other)
				if a.OfferID != b.OfferID || !slices.Equal(a.Cards, b.Cards) {
					t.Fatalf("%s seed %d step %d: %v in one world, %v in the other", skill, seed, step, a, b)
				}
				raw, _, _ = m.Apply(raw, me, a)
			}
		}
	}
}

// TestStrengthIsOrdered: medium beats easy and hard beats medium, one strong
// side against a weaker one, seats swapped across seeds.
func TestStrengthIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, c := range []struct{ strong, weak module.Skill }{
		{module.SkillMedium, module.SkillEasy},
		{module.SkillHard, module.SkillMedium},
	} {
		total, n := 0, 0
		for seed := int64(1); seed <= 30; seed++ {
			for _, strongSide := range []int{0, 1} {
				skills := map[string]module.Skill{}
				for i, p := range refs(4) {
					skills[p.ID] = c.weak
					if i%2 == strongSide {
						skills[p.ID] = c.strong
					}
				}
				sc := playMatch(t, 4, seed, skills)
				strongP, weakP := refs(4)[strongSide].ID, refs(4)[1-strongSide].ID
				total += sc[strongP] - sc[weakP]
				n++
			}
		}
		mean := float64(total) / float64(n)
		t.Logf("%s minus %s: %+.2f stakes a match over %d matches", c.strong, c.weak, mean, n)
		if mean <= 0 {
			t.Errorf("%s played %+.2f against %s", c.strong, mean, c.weak)
		}
	}
}

// reshuffled is s, everything public kept, with the cards me cannot see —
// the other hands and the stock — dealt out again.
func reshuffled(s *GameState, me string, seed int64) *GameState {
	raw, _ := encode(s)
	out, _ := decode(raw)
	var pool []string
	for _, p := range out.Players {
		if p != me {
			pool = append(pool, out.Hands[p]...)
		}
	}
	pool = append(pool, out.Stock...)
	rand.New(rand.NewSource(seed)).Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for _, p := range out.Players {
		if p != me {
			n := len(out.Hands[p])
			out.Hands[p], pool = append([]string(nil), pool[:n]...), pool[n:]
			sortHand(out.Hands[p])
		}
	}
	out.Stock = pool
	return out
}
