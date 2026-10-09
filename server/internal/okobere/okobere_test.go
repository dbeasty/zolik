package okobere

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
		t.Fatalf("%s %s %v refused: %v", who, a.Verb, a.Params, err)
	}
	return next
}

func refused(t *testing.T, s *GameState, who string, a module.Action, code string) {
	t.Helper()
	if _, err := apply(t, s, who, a); module.CodeOf(err) != code {
		t.Fatalf("%s %s %v: got %v, want %s", who, a.Verb, a.Params, err, code)
	}
}

func betOf(n int) module.Action {
	return module.Action{OfferID: OfferBet, Verb: VerbBet, Params: map[string]string{ParamAmount: strconv.Itoa(n)}}
}

var (
	cardAction  = module.Action{OfferID: OfferCard, Verb: VerbCard}
	standAction = module.Action{OfferID: OfferStand, Verb: VerbStand}
)

// rigged is a three-player round with p1 banking: the hands and the deck
// given, p2 to play.
func rigged(t *testing.T, hands map[string][]string, deck []string) *GameState {
	t.Helper()
	s := newState(t, 3, module.Options{module.OptPauseBetweenRounds: module.OptOn})
	// Put the bank with p1 whatever the seed chose.
	s.banker().Stack += s.Bank
	s.FirstBanker, s.Turn = 0, 0
	s.Bank = 50
	s.Seats[0].Stack = 50
	for i := range s.Seats {
		s.Seats[i].Hand, s.Seats[i].Bet, s.Seats[i].Done, s.Seats[i].Result = hands[s.Seats[i].PlayerID], 0, false, ""
	}
	s.Deck = deck
	s.Phase, s.Current = phasePunters, "p2"
	return s
}

// chips is every chip at the table: stacks, stakes down, and the bank.
func chips(s *GameState) int {
	n := s.Bank
	for _, st := range s.Seats {
		n += st.Stack
		if st.Result == "" {
			n += st.Bet
		}
	}
	return n
}

func TestCardValues(t *testing.T) {
	for _, c := range []struct {
		hand []string
		want int
	}{
		{[]string{"7H", "8D", "9C", "TS"}, 34},
		{[]string{"JH", "QH", "KH"}, 4},
		{[]string{"AH", "TH"}, 21},
	} {
		if got := total(c.hand); got != c.want {
			t.Errorf("%v counts %d, want %d", c.hand, got, c.want)
		}
	}
	if !oko([]string{"AH", "AS"}) || oko([]string{"AH", "AS", "7D"}) || oko([]string{"AH", "TS"}) {
		t.Error("oko is two aces, and only as the first two cards")
	}
}

func TestTheDealAndTheBank(t *testing.T) {
	s := newState(t, 4, nil)
	b := s.bankerIndex()
	if s.Bank != 50 || s.Seats[b].Stack != 50 {
		t.Fatalf("bank %d, banker keeps %d; want 50 and 50", s.Bank, s.Seats[b].Stack)
	}
	for _, st := range s.Seats {
		if len(st.Hand) != 1 {
			t.Errorf("%s holds %d cards", st.PlayerID, len(st.Hand))
		}
	}
	if want := s.Seats[(b+1)%4].PlayerID; s.Current != want || s.Phase != phasePunters {
		t.Fatalf("%s to act in %s, want %s", s.Current, s.Phase, want)
	}
	raw, _ := encode(s)
	vm, _ := New().View(raw, s.Current)
	for _, z := range vm.Zones {
		if z.OwnerID != s.Current && len(z.Cards) > 0 {
			t.Errorf("%s sees %s's %v", s.Current, z.OwnerID, z.Cards)
		}
	}
}

func TestStakesAreBoundedByStackAndBank(t *testing.T) {
	s := rigged(t, map[string][]string{"p1": {"8H"}, "p2": {"TH"}, "p3": {"9D"}}, []string{"7S", "8S", "9S", "TS", "JS"})
	refused(t, s, "p2", cardAction, ErrBetFirst)
	refused(t, s, "p2", betOf(1), ErrBetTooSmall)
	refused(t, s, "p2", betOf(60), ErrBankTooSmall)
	s = must(t, s, "p2", betOf(40))
	refused(t, s, "p2", betOf(2), ErrAlreadyBet)
	s = must(t, s, "p2", standAction)
	// The bank has 10 left to cover.
	refused(t, s, "p3", betOf(12), ErrBankTooSmall)
	must(t, s, "p3", betOf(10))
}

func TestBustAndOkoSettleAtOnce(t *testing.T) {
	s := rigged(t, map[string][]string{"p1": {"8H"}, "p2": {"TH"}, "p3": {"AD"}}, []string{"TS", "8S", "AC", "7S"})
	before := chips(s)
	s = must(t, s, "p2", betOf(10))
	s = must(t, s, "p2", cardAction) // 20
	s = must(t, s, "p2", cardAction) // 28: bust
	if st := s.seat("p2"); st.Result != resultBust || s.Bank != 60 || s.Current != "p3" {
		t.Fatalf("after the bust: %+v, bank %d, %s to act", st, s.Bank, s.Current)
	}
	s = must(t, s, "p3", betOf(10))
	s = must(t, s, "p3", cardAction) // AD AC: oko, paid three times
	if st := s.seat("p3"); st.Result != resultOko || st.Stack != 130 || s.Bank != 30 {
		t.Fatalf("after the oko: %+v, bank %d", st, s.Bank)
	}
	if chips(s) != before {
		t.Fatalf("chips went from %d to %d", before, chips(s))
	}
}

func TestTheBankPlaysLastAndTakesTies(t *testing.T) {
	s := rigged(t, map[string][]string{"p1": {"TH"}, "p2": {"TD"}, "p3": {"9C"}}, []string{"8S", "TS", "8D", "7C"})
	before := chips(s)
	s = must(t, s, "p2", betOf(10))
	s = must(t, s, "p2", cardAction) // 18
	s = must(t, s, "p2", standAction)
	s = must(t, s, "p3", betOf(10))
	s = must(t, s, "p3", cardAction) // 19
	s = must(t, s, "p3", standAction)
	if s.Phase != phaseBank || s.Current != "p1" {
		t.Fatalf("phase %s, %s to act", s.Phase, s.Current)
	}
	s = must(t, s, "p1", cardAction) // 10 + 8 = 18
	s = must(t, s, "p1", standAction)
	r := s.Rounds[0]
	if r.Results["p2"] != resultLost || r.Results["p3"] != resultWon {
		t.Fatalf("18 against the bank's 18 and 19 against it: %v", r.Results)
	}
	if r.Deltas["p1"] != 0 || r.Deltas["p2"] != -10 || r.Deltas["p3"] != 10 {
		t.Fatalf("deltas %v", r.Deltas)
	}
	if chips(s) != before {
		t.Fatalf("chips went from %d to %d", before, chips(s))
	}
}

func TestABustBankPaysEveryoneStanding(t *testing.T) {
	s := rigged(t, map[string][]string{"p1": {"TH"}, "p2": {"7D"}, "p3": {"9C"}}, []string{"TS", "TD"})
	s = must(t, s, "p2", betOf(5))
	s = must(t, s, "p2", standAction)
	s = must(t, s, "p3", betOf(5))
	s = must(t, s, "p3", standAction)
	s = must(t, s, "p1", cardAction) // 20
	s = must(t, s, "p1", cardAction) // 30: bust
	if r := s.Rounds[0]; r.Results["p2"] != resultWon || r.Results["p3"] != resultWon {
		t.Fatalf("a bust bank: %v", r.Results)
	}
}

func TestTheBankPassesAndTheMatchEnds(t *testing.T) {
	s := newState(t, 3, module.Options{OptRoundsPerBank: 3, module.OptPauseBetweenRounds: module.OptOff})
	first := s.bankerIndex()
	raw, _ := encode(s)
	m := New()
	for step := 0; step < 2000 && s.Status == "active"; step++ {
		offers, _ := m.LegalActions(raw, s.Current)
		a, _ := m.Bot().Act(raw, module.BotSeat{PlayerID: s.Current, Skill: module.SkillMedium, Seed: 1}, offers)
		var err error
		if raw, _, err = m.Apply(raw, s.Current, a); err != nil {
			t.Fatal(err)
		}
		s, _ = decode(raw)
	}
	if s.Status != "completed" {
		t.Fatal("the match did not finish")
	}
	bankers := map[string]bool{}
	for _, r := range s.Rounds {
		bankers[r.Banker] = true
	}
	if len(bankers) < 2 || !bankers[s.Seats[first].PlayerID] {
		t.Errorf("the bank went to %v", bankers)
	}
	sum := 0
	for _, st := range s.Seats {
		sum += st.Stack
	}
	if s.Bank != 0 || sum != 3*s.StartingStack {
		t.Errorf("at the end: bank %d, stacks %d", s.Bank, sum)
	}
}

func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			"GAME_NOT_ACTIVE": "not a rule — the game is over or has not started",
			"UNKNOWN_ACTION":  "not a rule — a verb this module does not have",
			"OKO_DECK_EMPTY":  "not a rule — the pack ran out, which a table of six can only just do",
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
		if total := chips(s); total != n*s.StartingStack {
			t.Fatalf("seed %d step %d: %d chips at the table, want %d", seed, step, total, n*s.StartingStack)
		}
		if s.Status != "active" {
			out := map[string]int{}
			for _, st := range s.Seats {
				out[st.PlayerID] = st.Stack
			}
			return out
		}
		actor := s.Current
		offers, _ := m.LegalActions(state, actor)
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor], Seed: seed}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move", seed, actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s %v: %v", seed, actor, a.Verb, a.Params, err)
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
	for _, n := range []int{2, 3, 6} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			for seed := int64(1); seed <= 5; seed++ {
				playMatch(t, n, seed, all(n, skill))
			}
		}
	}
}

// TestBotDoesNotPeek: the hidden cards dealt differently must not change a
// seat's move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 6; seed++ {
			raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, refs(4), seed)
			for step := 0; step < 40; step++ {
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
				if a.OfferID != b.OfferID || a.Params[ParamAmount] != b.Params[ParamAmount] {
					t.Fatalf("%s seed %d step %d: %v in one world, %v in the other", skill, seed, step, a, b)
				}
				raw, _, _ = m.Apply(raw, me, a)
			}
		}
	}
}

// reshuffled is s with every card me cannot see dealt again: the face-down
// hands of the others (the bank's too, until it plays) and the deck.
func reshuffled(s *GameState, me string, seed int64) *GameState {
	raw, _ := encode(s)
	out, _ := decode(raw)
	seen := out.seen(out.seat(me))
	type slot struct{ seat, card int }
	var slots []slot
	var pool []string
	for i, st := range out.Seats {
		for j, c := range st.Hand {
			if !seen[c] {
				slots = append(slots, slot{i, j})
				pool = append(pool, c)
			}
		}
	}
	pool = append(pool, out.Deck...)
	rand.New(rand.NewSource(seed)).Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for k, sl := range slots {
		out.Seats[sl.seat].Hand[sl.card] = pool[k]
	}
	out.Deck = pool[len(slots):]
	return out
}

// TestStrengthIsOrdered: hard ahead of medium, medium ahead of easy, one
// strong seat against weaker ones, moved round the table.
func TestStrengthIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, c := range []struct{ strong, weak module.Skill }{
		{module.SkillMedium, module.SkillEasy},
		{module.SkillHard, module.SkillMedium},
	} {
		total, n := 0, 0
		for seed := int64(1); seed <= 60; seed++ {
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

var _ = slices.Contains[[]string]

// TestSummariesNeedNoParameters: a variation's summary is drawn with no
// parameters, so none of its sentences may need one.
func TestSummariesNeedNoParameters(t *testing.T) {
	for _, v := range New().Descriptor().Variations {
		for _, f := range v.Summary {
			for _, sec := range mustRules(t) {
				for _, it := range sec.Items {
					if it.LabelKey == f.LabelKey && len(it.Params) > 0 {
						t.Errorf("summary sentence %s takes %v", f.LabelKey, it.Params)
					}
				}
			}
		}
	}
}

func mustRules(t *testing.T) []module.RuleSection {
	t.Helper()
	secs, err := New().Rules(module.MatchConfig{})
	if err != nil {
		t.Fatal(err)
	}
	return secs
}
