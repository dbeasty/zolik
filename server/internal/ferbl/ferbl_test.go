package ferbl

import (
	"math/rand"
	"slices"
	"strconv"
	"testing"

	"zolik/server/internal/module"
)

func refs(n int) []module.PlayerRef {
	var out []module.PlayerRef
	for i := 1; i <= n; i++ {
		out = append(out, module.PlayerRef{ID: "p" + strconv.Itoa(i)})
	}
	return out
}

func newState(t *testing.T, n int, opts module.Options) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Options: opts}, refs(n), 42)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func apply(t *testing.T, s *GameState, who, verb string) (*GameState, error) {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := New().Apply(raw, who, module.Action{OfferID: verb, Verb: verb})
	if err != nil {
		return s, err
	}
	next, err := decode(out)
	if err != nil {
		t.Fatal(err)
	}
	return next, nil
}

func must(t *testing.T, s *GameState, who, verb string) *GameState {
	t.Helper()
	next, err := apply(t, s, who, verb)
	if err != nil {
		t.Fatalf("%s %s refused: %v", who, verb, err)
	}
	return next
}

func refused(t *testing.T, s *GameState, who, verb, code string) {
	t.Helper()
	if _, err := apply(t, s, who, verb); module.CodeOf(err) != code {
		t.Fatalf("%s %s: got %v, want %s", who, verb, err, code)
	}
}

func chips(s *GameState) int {
	n := 0
	for _, st := range s.Seats {
		n += st.Stack + st.Total
	}
	if s.Status != "active" || s.Intermission.Open || s.Current == "" {
		n = 0
		for _, st := range s.Seats {
			n += st.Stack
			if st.In && s.Current != "" {
				n += st.Total
			}
		}
	}
	return n
}

func TestHandRanking(t *testing.T) {
	ordered := [][]string{
		{"AH", "AD", "AC", "AS"}, // four aces
		{"7H", "7D", "7C", "7S"}, // four sevens
		{"AH", "KH", "QH", "JH"}, // banda 41
		{"7H", "8H", "9H", "TH"}, // banda 34
		{"AH", "AD", "AC", "7S"}, // three aces
		{"8H", "8D", "8C", "9S"}, // three eights
		{"AH", "KH", "QH", "7S"}, // three of a suit, 31
		{"7H", "8H", "9H", "AS"}, // three of a suit, 24
		{"AH", "AD", "7C", "8S"}, // two aces
		{"AH", "KH", "7C", "8S"}, // two of a suit, 21
		{"7H", "8H", "9C", "TS"}, // two of a suit, 15
		{"AH", "7D", "8C", "9S"}, // a card of each suit, 11
		{"8H", "8D", "7C", "7S"}, // a card of each suit, 8: the worst hand
	}
	for i := 1; i < len(ordered); i++ {
		if score(ordered[i-1]) <= score(ordered[i]) {
			t.Errorf("%v (%d) should beat %v (%d)", ordered[i-1], score(ordered[i-1]), ordered[i], score(ordered[i]))
		}
	}
	// Pagat: cards outside the combination count for nothing.
	if score([]string{"AH", "AS", "8D", "7C"}) != score([]string{"AH", "AS", "KD", "KC"}) {
		t.Error("A-A-8-7 and A-A-K-K should be equal")
	}
}

// rigged is a three-player hand: p1 deals, so p2 acts first, then p3, then
// p1. Each has paid a vizi of 2 and holds the two cards given.
func rigged(t *testing.T, hands map[string][]string, deck []string) *GameState {
	t.Helper()
	s := newState(t, 3, module.Options{module.OptPauseBetweenRounds: module.OptOn})
	s.FirstDealer, s.Hand = 0, 0
	for i := range s.Seats {
		st := &s.Seats[i]
		*st = Seat{PlayerID: st.PlayerID, Stack: 98, In: true, Total: 2, Hand: hands[st.PlayerID]}
	}
	s.Deck = deck
	s.openRound(phaseFirst)
	return s
}

func TestBettingRounds(t *testing.T) {
	// The second two cards go round from the dealer's left to the hands
	// still in: p3 first (9H TD), then p1 (AC AS).
	s := rigged(t, map[string][]string{"p1": {"AH", "AD"}, "p2": {"7H", "8D"}, "p3": {"KC", "QC"}},
		[]string{"9H", "TD", "AC", "AS", "JC", "TC"})
	if s.Current != "p2" {
		t.Fatalf("%s acts first, want p2 on the dealer's left", s.Current)
	}
	refused(t, s, "p2", VerbCall, ErrNothingToCall)
	refused(t, s, "p2", VerbFold, ErrNoNeedToFold)
	s = must(t, s, "p2", VerbCheck)
	s = must(t, s, "p3", VerbBet) // 2
	refused(t, s, "p1", VerbCheck, ErrCannotCheck)
	s = must(t, s, "p1", VerbRaise) // to 4
	s = must(t, s, "p2", VerbFold)
	s = must(t, s, "p3", VerbRaise) // to 6
	s = must(t, s, "p1", VerbRaise) // to 8: the third raise
	refused(t, s, "p3", VerbRaise, ErrRaiseCap)
	s = must(t, s, "p3", VerbCall)
	// Round two: four cards each, bets of 4.
	if s.Phase != phaseSecond || len(s.seat("p1").Hand) != 4 || len(s.seat("p2").Hand) != 2 {
		t.Fatalf("phase %s, hands %v / %v", s.Phase, s.seat("p1").Hand, s.seat("p2").Hand)
	}
	if s.Current != "p3" {
		t.Fatalf("%s acts first in round two, want p3 (p2 folded)", s.Current)
	}
	s = must(t, s, "p3", VerbBet)
	if s.CurrentBet != 4 {
		t.Fatalf("a bet on four cards is %d, want 4", s.CurrentBet)
	}
	s = must(t, s, "p1", VerbCall)
	// p1's AH AD AC AS is four aces.
	h := s.Rounds[0]
	if !slices.Equal(h.Winners, []string{"p1"}) || h.Pot != 2+2+2+8+8+4+4 {
		t.Fatalf("hand %+v", h)
	}
	if _, shown := h.Shown["p2"]; shown {
		t.Error("a folded hand was shown")
	}
}

func TestALoneSurvivorTakesThePotUnseen(t *testing.T) {
	s := rigged(t, map[string][]string{"p1": {"AH", "AD"}, "p2": {"7H", "8D"}, "p3": {"KC", "QC"}}, []string{"AC", "AS", "9H", "TD", "JC", "TC"})
	s = must(t, s, "p2", VerbBet)
	s = must(t, s, "p3", VerbFold)
	s = must(t, s, "p1", VerbFold)
	h := s.Rounds[0]
	if !slices.Equal(h.Winners, []string{"p2"}) || len(h.Shown) != 0 || s.seat("p2").Stack != 98-2+8 {
		t.Fatalf("hand %+v, p2 has %d", h, s.seat("p2").Stack)
	}
}

func TestTiesGoToTheFirstInTurn(t *testing.T) {
	// p2 and p3 both hold a card of each suit topped by an eso; p2 acts first.
	// Dealt round from p2: p2 AH 7D 9C TS and p3 AD 8S 8C 9H, a card of
	// each suit topped by an eso each; p1 7H 8D 7C 9S tops out at 9.
	s := rigged(t, map[string][]string{"p1": {"7H", "8D"}, "p2": {"AH", "7D"}, "p3": {"AD", "8S"}},
		[]string{"9C", "TS", "8C", "9H", "7C", "9S"})
	for _, who := range []string{"p2", "p3", "p1"} {
		s = must(t, s, who, VerbCheck)
	}
	for _, who := range []string{"p2", "p3", "p1"} {
		s = must(t, s, who, VerbCheck)
	}
	h := s.Rounds[0]
	if h.Scores["p2"] != h.Scores["p3"] || !slices.Equal(h.Winners, []string{"p2"}) {
		t.Fatalf("hand %+v", h)
	}
}

func TestSidePots(t *testing.T) {
	// p2 is all in for 3 more; p3 and p1 play on. p2 holds the best hand
	// and takes only what it matched.
	s := rigged(t, map[string][]string{"p1": {"7H", "8D"}, "p2": {"AH", "AD"}, "p3": {"KC", "QC"}},
		[]string{"AC", "AS", "9H", "TD", "JC", "TC"})
	s.seat("p2").Stack = 3
	before := 0
	for _, st := range s.Seats {
		before += st.Stack + st.Total
	}
	s = must(t, s, "p2", VerbBet)   // 2
	s = must(t, s, "p3", VerbRaise) // 4
	s = must(t, s, "p1", VerbCall)  // 4
	s = must(t, s, "p2", VerbCall)  // 1 more: all in at 5
	if s.Phase != phaseSecond {
		t.Fatalf("phase %s", s.Phase)
	}
	s = must(t, s, "p3", VerbBet)
	s = must(t, s, "p1", VerbCall)
	h := s.Rounds[0]
	after := 0
	for _, st := range s.Seats {
		after += st.Stack
	}
	if after != before {
		t.Fatalf("chips went from %d to %d", before, after)
	}
	// Main pot: 5 from each = 15, to p2's four aces. The rest, 2 from p3
	// and p1 in round one and 4 each in round two, to the better of them.
	if h.Deltas["p2"] != 15-5 {
		t.Fatalf("p2 won %d net, want 10 (deltas %v)", h.Deltas["p2"], h.Deltas)
	}
	if len(h.Winners) != 2 {
		t.Fatalf("winners %v, want p2 and the side pot's", h.Winners)
	}
}

func TestViewHidesHands(t *testing.T) {
	s := newState(t, 3, nil)
	raw, _ := encode(s)
	vm, _ := New().View(raw, "p1")
	for _, z := range vm.Zones {
		if z.OwnerID != "p1" && len(z.Cards) > 0 {
			t.Errorf("p1 sees %s's %v", z.OwnerID, z.Cards)
		}
	}
}

func TestSummariesNeedNoParameters(t *testing.T) {
	secs, _ := New().Rules(module.MatchConfig{})
	for _, v := range New().Descriptor().Variations {
		for _, f := range v.Summary {
			for _, sec := range secs {
				for _, it := range sec.Items {
					if it.LabelKey == f.LabelKey && len(it.Params) > 0 {
						t.Errorf("summary sentence %s takes %v", f.LabelKey, it.Params)
					}
				}
			}
		}
	}
}

func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			"GAME_NOT_ACTIVE": "not a rule — the game is over or has not started",
			"UNKNOWN_ACTION":  "not a rule — a verb this module does not have",
			"TOO_FEW_PLAYERS": "not a rule of play — the table could not be seated",
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
		total := 0
		for _, st := range s.Seats {
			total += st.Stack + st.Total
		}
		if s.Status != "active" {
			out := map[string]int{}
			sum := 0
			for _, st := range s.Seats {
				out[st.PlayerID] = st.Stack
				sum += st.Stack
			}
			if sum != n*s.StartingStack {
				t.Fatalf("seed %d: %d chips at the end, want %d", seed, sum, n*s.StartingStack)
			}
			return out
		}
		if total != n*s.StartingStack {
			t.Fatalf("seed %d step %d: %d chips at the table, want %d", seed, step, total, n*s.StartingStack)
		}
		actor := s.Current
		offers, _ := m.LegalActions(state, actor)
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor], Seed: seed}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move", seed, actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s: %v", seed, actor, a.Verb, err)
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
	for _, n := range []int{2, 4, 6} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			for seed := int64(1); seed <= 3; seed++ {
				playMatch(t, n, seed, all(n, skill))
			}
		}
	}
}

// TestBotDoesNotPeek: the other hands and the deck dealt differently must
// not change a seat's move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 4; seed++ {
			raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, refs(4), seed)
			for step := 0; step < 30; step++ {
				s, _ := decode(raw)
				if s.Status != "active" {
					break
				}
				me := s.Current
				act := func(st *GameState) module.Action {
					enc, _ := encode(st)
					offers, _ := m.LegalActions(enc, me)
					a, _ := m.Bot().Act(enc, module.BotSeat{PlayerID: me, Skill: skill, Seed: seed}, offers)
					return a
				}
				a, b := act(s), act(reshuffled(s, me, seed+int64(step)))
				if a.Verb != b.Verb {
					t.Fatalf("%s seed %d step %d: %s in one world, %s in the other", skill, seed, step, a.Verb, b.Verb)
				}
				raw, _, _ = m.Apply(raw, me, a)
			}
		}
	}
}

func reshuffled(s *GameState, me string, seed int64) *GameState {
	raw, _ := encode(s)
	out, _ := decode(raw)
	var pool []string
	for _, st := range out.Seats {
		if st.PlayerID != me {
			pool = append(pool, st.Hand...)
		}
	}
	pool = append(pool, out.Deck...)
	rand.New(rand.NewSource(seed)).Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for i := range out.Seats {
		if out.Seats[i].PlayerID != me {
			n := len(out.Seats[i].Hand)
			out.Seats[i].Hand, pool = append([]string(nil), pool[:n]...), pool[n:]
		}
	}
	out.Deck = pool
	return out
}

func TestStrengthIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, c := range []struct{ strong, weak module.Skill }{
		{module.SkillMedium, module.SkillEasy},
		{module.SkillHard, module.SkillMedium},
	} {
		total, n := 0, 0
		for seed := int64(1); seed <= 40; seed++ {
			for _, strong := range []string{"p1", "p2", "p3"} {
				skills := all(3, c.weak)
				skills[strong] = c.strong
				sc := playMatch(t, 3, seed, skills)
				total += sc[strong] - 100
				n++
			}
		}
		mean := float64(total) / float64(n)
		t.Logf("%s among %s: %+.2f chips a match over %d matches", c.strong, c.weak, mean, n)
		if mean <= 0 {
			t.Errorf("%s lost %+.2f a match among %s", c.strong, mean, c.weak)
		}
	}
}

var _ = chips

// maniac bets and raises whenever it may: the player a reading bot must
// not simply fold to.
type maniac struct{}

func (maniac) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	for _, verb := range []string{module.VerbContinue, VerbRaise, VerbBet, VerbCall, VerbCheck} {
		for _, o := range offers {
			if o.Verb == verb && o.Enabled {
				return module.Action{OfferID: o.ID, Verb: verb}, true
			}
		}
	}
	return module.ChooseAction(offers, nil)
}

// TestHardIsNotBulliedByAlwaysBetting: hard seats against one seat that
// bets and raises every chance it gets come out ahead.
func TestHardIsNotBulliedByAlwaysBetting(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	m := New()
	total, n := 0, 0
	for seed := int64(1); seed <= 40; seed++ {
		for _, wild := range []string{"p1", "p2", "p3"} {
			state, _ := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, refs(3), seed)
			for step := 0; step < 5000; step++ {
				s, _ := decode(state)
				if s.Status != "active" {
					total += s.seat(wild).Stack - 100
					n++
					break
				}
				offers, _ := m.LegalActions(state, s.Current)
				var a module.Action
				if s.Current == wild {
					a, _ = maniac{}.Act(state, module.BotSeat{PlayerID: wild}, offers)
				} else {
					a, _ = m.Bot().Act(state, module.BotSeat{PlayerID: s.Current, Skill: module.SkillHard, Seed: seed}, offers)
				}
				var err error
				if state, _, err = m.Apply(state, s.Current, a); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	mean := float64(total) / float64(n)
	t.Logf("always betting against hard: %+.2f chips a match over %d matches", mean, n)
	if mean >= 0 {
		t.Errorf("a seat that always bets won %+.2f a match against hard", mean)
	}
}
