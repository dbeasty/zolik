package lora

import (
	"math/rand"
	"slices"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

func refs() []module.PlayerRef {
	return []module.PlayerRef{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}, {ID: "p4"}}
}

var noPause = module.Options{module.OptPauseBetweenRounds: module.OptOff}

func newState(t *testing.T, opts module.Options) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Options: opts}, refs(), 42)
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
		t.Fatalf("%s %s %v %v refused: %v", who, a.Verb, a.Cards, a.Params, err)
	}
	return next
}

func refused(t *testing.T, s *GameState, who string, a module.Action, code string) {
	t.Helper()
	if _, err := apply(t, s, who, a); module.CodeOf(err) != code {
		t.Fatalf("%s %s %v: got %v, want %s", who, a.Verb, a.Cards, err, code)
	}
}

func card(c string) module.Action { return playCard(c) }

var (
	done = module.Action{OfferID: OfferDone, Verb: VerbDone}
	tuk  = module.Action{OfferID: OfferTuk, Verb: VerbTuk}
)

func choose(g string) module.Action {
	return module.Action{OfferID: OfferChoose, Verb: VerbChoose, Params: map[string]string{ParamGame: g}}
}

// at is game g of the first tálie with these hands, p1 leading.
func at(t *testing.T, g string, hands map[string][]string) *GameState {
	t.Helper()
	s := newState(t, noPause)
	s.FirstLeader, s.Talie = 0, 0
	for i, x := range sequence {
		if x == g {
			s.Game = i
		}
	}
	s.startDeal()
	s.Hands = hands
	return s
}

// The usual crafted deal: each player one suit.
func bySuit() map[string][]string {
	row := func(suit byte) []string {
		var out []string
		for i := range ascending {
			out = append(out, cardAt(i, suit))
		}
		return out
	}
	return map[string][]string{"p1": row('D'), "p2": row('H'), "p3": row('C'), "p4": row('S')}
}

func TestTheDeal(t *testing.T) {
	s := newState(t, nil)
	seen := map[string]bool{}
	for _, p := range s.Players {
		if len(s.Hands[p]) != handSize {
			t.Errorf("%s holds %d", p, len(s.Hands[p]))
		}
		for _, c := range s.Hands[p] {
			seen[c] = true
		}
	}
	if len(seen) != 32 {
		t.Errorf("%d distinct cards dealt", len(seen))
	}
	if s.Contract != gameCervene || s.Phase != phasePlay || s.Current != s.talieLeader() {
		t.Errorf("first deal: %s/%s, %s to play, leader %s", s.Contract, s.Phase, s.Current, s.talieLeader())
	}
	if s.seat(s.dealer()) != (s.seat(s.talieLeader())+3)%4 {
		t.Errorf("dealer %s does not sit before leader %s", s.dealer(), s.talieLeader())
	}
	if _, err := New().NewMatch(module.MatchConfig{}, refs()[:3], 1); module.CodeOf(err) != "TOO_FEW_PLAYERS" {
		t.Errorf("three players: %v", err)
	}
}

func TestTrickPenalties(t *testing.T) {
	trick := func(cards ...string) []tricks.Play {
		var out []tricks.Play
		for i, c := range cards {
			out = append(out, tricks.Play{Seat: i, Card: c})
		}
		return out
	}
	for _, c := range []struct {
		game   string
		trick  []tricks.Play
		number int
		want   int
	}{
		{gameCervene, trick("7H", "KH", "AD", "9H"), 2, 3},
		{gameFilky, trick("QH", "QD", "AD", "9H"), 2, 4},
		{gamePrPo, trick("7H", "KH", "AD", "9H"), 0, 4},
		{gamePrPo, trick("7H", "KH", "AD", "9H"), 3, 0},
		{gamePrPo, trick("7H", "KH", "AD", "9H"), 7, 4},
		{gameVsechny, trick("7H", "8H", "9H", "TH"), 5, 1},
		{gameKral, trick("AH", "KH", "AD", "9H"), 1, 8},
		{gameKral, trick("AH", "QH", "AD", "9H"), 1, 0},
	} {
		if got := trickPenalty(c.game, c.trick, c.number); got != c.want {
			t.Errorf("%s trick %d %v: %d, want %d", c.game, c.number, c.trick, got, c.want)
		}
	}
}

func TestFollowingAndTheRedLead(t *testing.T) {
	s := at(t, gameCervene, map[string][]string{
		"p1": {"7H", "8H", "AD", "9C", "KS", "TD", "JD", "QD"},
		"p2": {"9H", "TH", "7D", "8C", "AS", "7C", "8D", "9D"},
		"p3": {"JH", "QH", "KD", "TC", "QS", "JC", "QC", "KC"},
		"p4": {"KH", "AH", "AC", "7S", "8S", "9S", "TS", "JS"},
	})
	refused(t, s, "p1", card("7H"), ErrNoRedLead)
	s = must(t, s, "p1", card("AD"))
	refused(t, s, "p2", card("9H"), ErrMustFollow)
	s = must(t, s, "p2", card("7D"))
	s = must(t, s, "p3", card("KD"))
	s = must(t, s, "p4", card("KH")) // void in bells: a red card thrown away
	if s.Points["p1"] != 1 || s.Won["p1"] != 1 || s.Current != "p1" {
		t.Fatalf("p1's ace took KH: points %v, won %v, current %s", s.Points, s.Won, s.Current)
	}
	if s.Voids["p4"] != "D" {
		t.Errorf("p4 showed void in D: %q", s.Voids["p4"])
	}
}

// A red king game ends the moment the king is taken.
func TestTheRedKingEndsTheDeal(t *testing.T) {
	s := at(t, gameKral, bySuit())
	s = must(t, s, "p1", card("8D"))
	s = must(t, s, "p2", card("KH")) // void in bells: the king thrown on p1's trick
	s = must(t, s, "p3", card("7C"))
	s = must(t, s, "p4", card("7S"))
	if len(s.Rounds) != 1 || s.Rounds[0].Contract != gameKral {
		t.Fatalf("the deal did not end: %d rounds", len(s.Rounds))
	}
	if d := s.Rounds[0].Deltas; d["p1"] != 8 || d["p2"]+d["p3"]+d["p4"] != 0 {
		t.Errorf("deltas %v, want 8 to p1", d)
	}
	if s.Contract != gameKvarty || s.Phase != phaseQuart {
		t.Errorf("next game %s/%s, want kvarty", s.Contract, s.Phase)
	}
}

func TestPrvniPosledni(t *testing.T) {
	s := at(t, gamePrPo, bySuit())
	for trick := 0; trick < numTrick; trick++ {
		for _, p := range []string{"p1", "p2", "p3", "p4"} {
			s = must(t, s, p, card(s.Hands[p][0]))
		}
	}
	// p1 led diamonds every time and won every trick.
	if d := s.Rounds[0].Deltas; d["p1"] != 8 {
		t.Errorf("deltas %v, want p1 8 for the first and last trick", d)
	}
}

func TestKvartyLaysTheWholeQuart(t *testing.T) {
	s := at(t, gameKvarty, map[string][]string{
		"p1": {"7H", "9H", "AD", "9C", "KS", "TD", "JD", "QD"},
		"p2": {"8H", "TH", "7D", "8C", "AS", "7C", "8D", "9D"},
		"p3": {"JH", "QH", "KD", "TC", "QS", "JC", "QC", "KC"},
		"p4": {"KH", "AH", "AC", "7S", "8S", "9S", "TS", "JS"},
	})
	s = must(t, s, "p1", card("7H"))
	if got := cardsOf(s.Quart); !slices.Equal(got, []string{"7H", "8H", "9H", "TH"}) {
		t.Fatalf("quart %v", got)
	}
	if s.Current != "p2" || len(s.Hands["p1"]) != 6 || len(s.Hands["p2"]) != 6 {
		t.Fatalf("p2 laid the top card and leads: current %s, hands %d/%d", s.Current, len(s.Hands["p1"]), len(s.Hands["p2"]))
	}
	// From the king the quart is only two long; from the 9 of hearts it
	// stops at once at the 10 already laid.
	s = must(t, s, "p2", card("8C"))
	if got := cardsOf(s.Quart); !slices.Equal(got, []string{"8C", "9C", "TC", "JC"}) {
		t.Fatalf("quart %v", got)
	}
	s = must(t, s, "p3", card("KD"))
	if got := cardsOf(s.Quart); !slices.Equal(got, []string{"KD", "AD"}) || s.Current != "p1" {
		t.Fatalf("quart %v, current %s", got, s.Current)
	}
}

func cardsOf(plays []tricks.Play) []string {
	var out []string
	for _, p := range plays {
		out = append(out, p.Card)
	}
	return out
}

func TestDesitky(t *testing.T) {
	s := at(t, gameDesitky, map[string][]string{
		"p1": {"TH", "9H", "AD", "9C", "KS", "TD", "JD", "QD"},
		"p2": {"8H", "JH", "7D", "8C", "AS", "7C", "8D", "9D"},
		"p3": {"7H", "QH", "KD", "TC", "QS", "JC", "QC", "KC"},
		"p4": {"KH", "AH", "AC", "7S", "8S", "9S", "TS", "JS"},
	})
	refused(t, s, "p1", card("9H"), ErrCannotLay)
	refused(t, s, "p1", tuk, ErrMustLay)
	refused(t, s, "p1", done, ErrNothingLaid)
	s = must(t, s, "p1", card("TH"))
	s = must(t, s, "p1", card("9H"))
	s = must(t, s, "p1", done)
	if s.Current != "p2" {
		t.Fatalf("turn passed to %s", s.Current)
	}
	s = must(t, s, "p2", card("8H"))
	s = must(t, s, "p2", card("JH"))
	// p2 has nothing more to lay: the turn passes on its own.
	if s.Current != "p3" {
		t.Fatalf("after p2 laid all it could, %s is on turn", s.Current)
	}
	s = must(t, s, "p3", card("7H"))
	s = must(t, s, "p3", card("QH"))
	s = must(t, s, "p3", card("TC"))
	s = must(t, s, "p3", done)
	s = must(t, s, "p4", card("KH"))
	s = must(t, s, "p4", card("AH"))
	s = must(t, s, "p4", card("TS"))
	s = must(t, s, "p4", done)
	// The rows: hearts complete, clubs and leaves at their tens.
	if lo, hi, ok := s.row('H'); !ok || lo != 0 || hi != 7 {
		t.Fatalf("hearts row %d..%d", lo, hi)
	}
	s = must(t, s, "p1", card("9C"))
	s = must(t, s, "p1", done)
	s = must(t, s, "p2", card("8C"))
	s = must(t, s, "p2", card("7C")) // nothing more: the turn passes
	s = must(t, s, "p3", card("JC"))
	s = must(t, s, "p3", card("QC"))
	s = must(t, s, "p3", card("KC"))
	s = must(t, s, "p4", card("AC"))
	s = must(t, s, "p4", card("9S"))
	s = must(t, s, "p4", card("8S"))
	s = must(t, s, "p4", card("7S"))
	s = must(t, s, "p4", card("JS"))
	// p4 is out: the deal ends.
	if len(s.Rounds) != 1 {
		t.Fatalf("deal still running, p4 holds %v", s.Hands["p4"])
	}
	d := s.Rounds[0].Deltas
	if d["p4"] != 0 || d["p1"] != 5 || d["p2"] != 4 || d["p3"] != 2 {
		t.Errorf("deltas %v", d)
	}
}

func TestATukCostsAPoint(t *testing.T) {
	s := at(t, gameDesitky, bySuit())
	// p1 holds every diamond: lays its ten and stops. p2 holds only hearts
	// and no hearts row is open: ťuk.
	s = must(t, s, "p1", card("TD"))
	s = must(t, s, "p1", done)
	refused(t, s, "p2", card("9H"), ErrCannotLay)
	s = must(t, s, "p2", card("TH"))
	s = must(t, s, "p2", done)
	s = must(t, s, "p3", card("TC"))
	s = must(t, s, "p3", done)
	s = must(t, s, "p4", card("TS"))
	s = must(t, s, "p4", done)
	s.Hands["p1"] = []string{"7C"} // nothing of it can be laid
	s.Laid = append(s.Laid, "9D", "8D", "7D", "JD", "QD", "KD", "AD")
	s = must(t, s, "p1", tuk)
	if s.Tuks["p1"] != 1 || s.Points["p1"] != 1 || s.Current != "p2" {
		t.Errorf("tuk: tuks %v points %v current %s", s.Tuks, s.Points, s.Current)
	}
}

func TestMaturita(t *testing.T) {
	s := newState(t, module.Options{OptTalie: 1, module.OptPauseBetweenRounds: module.OptOff})
	s.FirstLeader, s.Game = 0, len(sequence)
	s.startDeal()
	if s.Phase != phaseChoose || s.Current != "p1" {
		t.Fatalf("maturita opens with %s, %s to act", s.Phase, s.Current)
	}
	refused(t, s, "p1", choose(gamePrPo), ErrPrPoThirdOnly)
	refused(t, s, "p1", choose("poker"), ErrNoSuchGame)
	refused(t, s, "p2", choose(gameFilky), ErrNotYourTurn)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		s = must(t, s, "p1", choose(gameVsechny))
		s.Hands = bySuit() // p1 leads its diamonds and takes the first trick
		for _, p := range []string{"p1", "p2", "p3", "p4"} {
			s = must(t, s, p, card(s.Hands[p][0]))
		}
		d := s.Rounds[len(s.Rounds)-1]
		if d.Maturant != "p1" || d.Passed || d.Deltas["p1"] != 8 || d.Deltas["p2"] != 0 {
			t.Fatalf("attempt %d: %+v", attempt+1, d)
		}
	}
	if s.Status != "completed" || s.Failed != "p1" || s.Scores["p1"] != 24 {
		t.Fatalf("after three failures: status %s, failed %q, scores %v", s.Status, s.Failed, s.Scores)
	}
	raw, _ := encode(s)
	st, _ := New().Standings(raw)
	if last := st[len(st)-1]; last.PlayerID != "p1" || *last.Shown != 24 {
		t.Errorf("the failed maturant finishes last: %+v", st)
	}
	_, winners, _ := New().Finished(raw)
	if slices.Contains(winners, "p1") || len(winners) != 3 {
		t.Errorf("winners %v", winners)
	}
}

func TestPrPoOnTheThirdAttempt(t *testing.T) {
	s := newState(t, noPause)
	s.Game, s.Attempt = len(sequence), maxAttempts-1
	s.startDeal()
	s = must(t, s, s.Current, choose(gamePrPo))
	if s.Contract != gamePrPo || s.Phase != phasePlay {
		t.Errorf("chose pr-po: %s/%s", s.Contract, s.Phase)
	}
}

func TestATalieRunsInOrder(t *testing.T) {
	for _, mat := range []int{module.OptOff, module.OptOn} {
		scores, s := playMatch(t, 7, all(module.SkillMedium), module.Options{OptTalie: 1, OptMaturita: mat})
		_ = scores
		var got []string
		for _, d := range s.Rounds {
			if d.Maturant == "" {
				got = append(got, d.Contract)
			}
		}
		if !slices.Equal(got, sequence) {
			t.Errorf("maturita %d: games %v", mat, got)
		}
		maturitas := len(s.Rounds) - len(sequence)
		if (mat == module.OptOn) != (maturitas > 0) {
			t.Errorf("maturita %d: %d maturita deals", mat, maturitas)
		}
	}
}

func TestViewHidesTheOtherHands(t *testing.T) {
	s := newState(t, nil)
	raw, _ := encode(s)
	vm, err := New().View(raw, "p1")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		if z.OwnerID != "" && z.OwnerID != "p1" && len(z.Cards) > 0 {
			t.Errorf("p1 sees %s's cards", z.OwnerID)
		}
		if z.OwnerID == "p1" && len(z.Cards) != handSize {
			t.Errorf("p1 sees %d of its own cards", len(z.Cards))
		}
	}
	spect, _ := New().View(raw, "")
	for _, z := range spect.Zones {
		if len(z.Cards) > 0 && z.OwnerID != "" {
			t.Errorf("a spectator sees %s's cards", z.OwnerID)
		}
	}
}

func TestSummariesNeedNoParameters(t *testing.T) {
	for _, v := range New().Descriptor().Variations {
		for _, f := range v.Summary {
			if len(f.Params) > 0 {
				t.Errorf("%s: %s takes params", v.ID, f.LabelKey)
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

// --- bots -------------------------------------------------------------------

func playMatch(t *testing.T, seed int64, skills map[string]module.Skill, opts module.Options) (map[string]int, *GameState) {
	t.Helper()
	m := New()
	o := module.Options{module.OptPauseBetweenRounds: module.OptOff}
	for k, v := range opts {
		o[k] = v
	}
	state, err := m.NewMatch(module.MatchConfig{Options: o}, refs(), seed)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 20000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			return s.Scores, s
		}
		actor := s.Current
		offers, _ := m.LegalActions(state, actor)
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor], Seed: seed}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move", seed, actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s %v %v: %v", seed, actor, a.Verb, a.Cards, a.Params, err)
		}
		state = next
	}
	t.Fatalf("seed %d: the match did not finish", seed)
	return nil, nil
}

func all(skill module.Skill) map[string]module.Skill {
	out := map[string]module.Skill{}
	for _, p := range refs() {
		out[p.ID] = skill
	}
	return out
}

func TestBotsPlayWholeMatchesLegally(t *testing.T) {
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		seeds := int64(4)
		if skill == module.SkillHard {
			seeds = 1
		}
		for seed := int64(1); seed <= seeds; seed++ {
			playMatch(t, seed, all(skill), module.Options{OptTalie: 1})
		}
	}
	playMatch(t, 9, all(module.SkillMedium), nil) // a full match of four tálie
}

// TestBotDoesNotPeek: the same seat, the same hand and table, and the cards
// it cannot see dealt differently, must get the same move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 3; seed++ {
			raw, _ := m.NewMatch(module.MatchConfig{Options: noPause}, refs(), seed)
			moves := 0
			for step := 0; step < 400 && moves < 40; step++ {
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
				a := act(s)
				// Only the interesting positions, and fewer for hard.
				if skill == module.SkillMedium || step%5 == 0 {
					b := act(reshuffled(s, me, seed+int64(step)))
					if a.OfferID != b.OfferID || !slices.Equal(a.Cards, b.Cards) || a.Params[ParamGame] != b.Params[ParamGame] {
						t.Fatalf("%s seed %d step %d (%s): %v in one world, %v in the other", skill, seed, step, s.Contract, a, b)
					}
					moves++
				}
				raw, _, _ = m.Apply(raw, me, a)
			}
		}
	}
}

// reshuffled is s, everything public kept, with the cards me cannot see
// dealt out again to the same hand sizes.
func reshuffled(s *GameState, me string, seed int64) *GameState {
	raw, _ := encode(s)
	out, _ := decode(raw)
	var pool []string
	for _, p := range out.Players {
		if p != me {
			pool = append(pool, out.Hands[p]...)
		}
	}
	rand.New(rand.NewSource(seed)).Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for _, p := range out.Players {
		if p != me {
			n := len(out.Hands[p])
			out.Hands[p], pool = append([]string(nil), pool[:n]...), pool[n:]
			sortHand(out.Hands[p])
		}
	}
	return out
}

// TestStrengthIsOrdered: medium beats easy and hard beats medium, two of a
// skill against two of the other, seats swapped across seeds.
func TestStrengthIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, c := range []struct {
		strong, weak module.Skill
		seeds        int64
	}{
		{module.SkillMedium, module.SkillEasy, 30},
		{module.SkillHard, module.SkillMedium, 10},
	} {
		total, n := 0, 0
		for seed := int64(1); seed <= c.seeds; seed++ {
			for _, strongSide := range []int{0, 1} {
				skills := map[string]module.Skill{}
				for i, p := range refs() {
					skills[p.ID] = c.weak
					if i%2 == strongSide {
						skills[p.ID] = c.strong
					}
				}
				sc, _ := playMatch(t, seed, skills, module.Options{OptTalie: 1})
				for i, p := range refs() {
					if i%2 == strongSide {
						total -= sc[p.ID]
					} else {
						total += sc[p.ID]
					}
				}
				n++
			}
		}
		mean := float64(total) / float64(2*n)
		t.Logf("%s against %s: %+.2f points a seat a tálie over %d matches", c.strong, c.weak, mean, n)
		if mean <= 0 {
			t.Errorf("%s played %+.2f against %s", c.strong, mean, c.weak)
		}
	}
}
