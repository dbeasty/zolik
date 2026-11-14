package blackpeter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"zolik/server/internal/lessontest"
	"zolik/server/internal/module"
)

func seats(ids ...string) []module.PlayerRef {
	out := make([]module.PlayerRef, len(ids))
	for i, id := range ids {
		out[i] = module.PlayerRef{ID: id, Name: id}
	}
	return out
}

func mustState(t *testing.T, raw module.State) *GameState {
	t.Helper()
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func noPause() module.MatchConfig {
	return module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}
}

// cardsInPlay is every card in a hand or put down: the whole 51, always.
func cardsInPlay(s *GameState) []string {
	var out []string
	for _, st := range s.Seats {
		out = append(out, st.Hand...)
	}
	for _, p := range s.Pairs {
		out = append(out, p...)
	}
	return out
}

func TestDeckIsFiftyOneWithPeterUnpaired(t *testing.T) {
	d := buildDeck()
	if len(d) != 51 {
		t.Fatalf("deck has %d cards", len(d))
	}
	for _, byColour := range []bool{false, true} {
		rest, _ := takePairs(d, byColour)
		if len(rest) != 1 || rest[0] != peter {
			t.Errorf("byColour=%v: pairing the whole pack leaves %v, want only %s", byColour, rest, peter)
		}
	}
}

func TestPairs(t *testing.T) {
	cases := []struct {
		a, b     string
		byColour bool
		want     bool
	}{
		{"5H", "5S", false, true},
		{"5H", "5S", true, false},
		{"5H", "5D", true, true},
		{"JH", "JS", false, false}, // Black Peter pairs with nothing
		{"JH", "JD", true, true},
		{"5H", "6H", false, false},
	}
	for _, c := range cases {
		if got := pairs(c.a, c.b, c.byColour); got != c.want {
			t.Errorf("pairs(%s,%s,colour=%v) = %v", c.a, c.b, c.byColour, got)
		}
	}
}

// Every deal, every draw: the 51 cards are all somewhere, no hand holds a
// pair, and the round ends with exactly one seat holding Black Peter alone.
func TestInvariantsThroughWholeMatches(t *testing.T) {
	m := New()
	for _, n := range []int{2, 3, 4, 6} {
		for _, byColour := range []int{pairsByRank, pairsByColour} {
			for seed := int64(1); seed <= 25; seed++ {
				ids := make([]string, n)
				for i := range ids {
					ids[i] = fmt.Sprintf("p%d", i)
				}
				cfg := module.MatchConfig{Options: module.Options{OptPairs: byColour, OptRounds: 3, module.OptPauseBetweenRounds: module.OptOff}}
				raw, err := m.NewMatch(cfg, seats(ids...), seed)
				if err != nil {
					t.Fatal(err)
				}
				for step := 0; ; step++ {
					s := mustState(t, raw)
					if s.Status != "active" {
						if len(s.Rounds) != 3 {
							t.Fatalf("n=%d seed=%d: finished after %d rounds", n, seed, len(s.Rounds))
						}
						break
					}
					if step > 2000 {
						t.Fatalf("n=%d seed=%d: no end in sight", n, seed)
					}
					if got := len(cardsInPlay(s)); got != 51 {
						t.Fatalf("n=%d seed=%d step %d: %d cards in play", n, seed, step, got)
					}
					for _, st := range s.Seats {
						if rest, found := takePairs(st.Hand, s.ByColour); len(found) > 0 {
							t.Fatalf("seed %d: %s holds a pair %v (rest %v)", seed, st.PlayerID, found, rest)
						}
					}
					var lastRounds = len(s.Rounds)
					raw, _, err = m.Apply(raw, s.Current, module.Action{Verb: VerbDraw})
					if err != nil {
						t.Fatalf("n=%d seed=%d step %d: %v", n, seed, step, err)
					}
					after := mustState(t, raw)
					if len(after.Rounds) > lastRounds {
						rec := after.Rounds[len(after.Rounds)-1]
						if rec.Loser == "" {
							t.Fatalf("seed %d: round ended with no loser", seed)
						}
						// The loser held Black Peter, and only it.
						if len(rec.OutOrder) != n-1 {
							t.Errorf("seed %d: %d went out, want %d", seed, len(rec.OutOrder), n-1)
						}
					}
				}
			}
		}
	}
}

func TestLoserHoldsOnlyPeterAndDealsNext(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{OptRounds: 2}}, seats("a", "b", "c"), 7)
	for {
		s := mustState(t, raw)
		var err error
		raw, _, err = m.Apply(raw, s.Current, module.Action{Verb: VerbDraw})
		if err != nil {
			t.Fatal(err)
		}
		after := mustState(t, raw)
		if after.Intermission.Open {
			l := after.index(after.Rounds[0].Loser)
			if got := after.Seats[l].Hand; len(got) != 1 || got[0] != peter {
				t.Fatalf("loser holds %v, want only %s", got, peter)
			}
			if after.Dealer != l {
				t.Errorf("dealer = %d, want the loser %d", after.Dealer, l)
			}
			if after.Seats[l].Marks != 1 {
				t.Errorf("loser has %d marks", after.Seats[l].Marks)
			}
			return
		}
	}
}

func TestPickTakesThatCard(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(noPause(), seats("a", "b", "c", "d"), 3)
	s := mustState(t, raw)
	from := s.source(s.Current)
	want := s.Seats[from].Hand[1]
	out, events, err := m.Apply(raw, s.Current, module.Action{Verb: VerbDraw, Params: map[string]string{ParamPick: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	after := mustState(t, out)
	me := after.seat(s.Current)
	held := strings.Contains(strings.Join(me.Hand, ","), want)
	laid := false
	for _, ev := range events {
		if ev.Type == "pair_laid" {
			laid = strings.Contains(fmt.Sprint(ev.Data["cards"]), want)
		}
	}
	if !held && !laid {
		t.Errorf("took position 1 (%s) but it is neither in hand nor laid down", want)
	}
	for _, bad := range []string{"-1", "99", "x"} {
		if _, _, err := m.Apply(raw, s.Current, module.Action{Verb: VerbDraw, Params: map[string]string{ParamPick: bad}}); module.CodeOf(err) != ErrNoCard {
			t.Errorf("pick %q: got %v, want %s", bad, err, ErrNoCard)
		}
	}
}

func TestOnlyTheCurrentSeatMayDraw(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(noPause(), seats("a", "b", "c"), 11)
	s := mustState(t, raw)
	for _, st := range s.Seats {
		offers, _ := m.LegalActions(raw, st.PlayerID)
		if len(offers) != 1 {
			t.Fatalf("%s: %d offers", st.PlayerID, len(offers))
		}
		o := offers[0]
		if (st.PlayerID == s.Current) != o.Enabled {
			t.Errorf("%s: enabled=%v, current=%s", st.PlayerID, o.Enabled, s.Current)
		}
		if !o.Enabled && (o.WhyNot != ErrNotYourTurn || len(o.RuleIDs) == 0) {
			t.Errorf("%s: whyNot=%q rules=%v", st.PlayerID, o.WhyNot, o.RuleIDs)
		}
		if o.Source == nil || o.Source.OwnerID == st.PlayerID || o.Source.ZoneID != handZoneID(o.Source.OwnerID) {
			t.Errorf("%s: source %+v should be another seat's hand", st.PlayerID, o.Source)
		}
	}
}

// The view shows a seat its own cards and nobody else's — counts only.
func TestViewHidesOtherHands(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(noPause(), seats("a", "b", "c"), 5)
	s := mustState(t, raw)
	vm, err := m.View(raw, "a")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		if z.Kind != module.ZoneHand {
			continue
		}
		owner := s.seat(z.OwnerID)
		if z.Count != len(owner.Hand) {
			t.Errorf("%s: count %d, holds %d", z.ID, z.Count, len(owner.Hand))
		}
		if z.OwnerID == "a" && len(z.Cards) != len(owner.Hand) {
			t.Errorf("own hand shows %d of %d", len(z.Cards), len(owner.Hand))
		}
		if z.OwnerID != "a" && len(z.Cards) > 0 {
			t.Errorf("%s leaks %d cards", z.ID, len(z.Cards))
		}
	}
	b, _ := json.Marshal(vm)
	for _, c := range s.seat("b").Hand {
		if strings.Contains(string(b), `"card":"`+c+`"`) && !inPairs(s, c) {
			t.Errorf("b's %s appears in a's view", c)
		}
	}
	open, _ := m.OpenView(raw)
	shown := 0
	for _, z := range open.Zones {
		if z.Kind == module.ZoneHand {
			shown += len(z.Cards)
		}
	}
	if total := len(cardsInPlay(s)) - 2*len(s.Pairs); shown != total {
		t.Errorf("open view shows %d hand cards, want %d", shown, total)
	}
}

func inPairs(s *GameState, c string) bool {
	for _, p := range s.Pairs {
		for _, x := range p {
			if x == c {
				return true
			}
		}
	}
	return false
}

// The bot reads nothing but the size of the hand it takes from: two states
// that differ only in what the hidden cards are get the same move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(noPause(), seats("a", "b", "c"), 9)
	s := mustState(t, raw)
	offers, _ := m.LegalActions(raw, s.Current)
	seat := module.BotSeat{PlayerID: s.Current, Seed: 42}
	a1, ok := bot{}.Act(raw, seat, offers)
	if !ok {
		t.Fatal("bot passed")
	}
	from := s.source(s.Current)
	h := s.Seats[from].Hand
	for i, j := 0, len(h)-1; i < j; i, j = i+1, j-1 {
		h[i], h[j] = h[j], h[i]
	}
	raw2, _ := encode(s)
	a2, _ := bot{}.Act(raw2, seat, offers)
	if a1.Params[ParamPick] != a2.Params[ParamPick] {
		t.Errorf("bot's pick changed with the hidden order: %v vs %v", a1.Params, a2.Params)
	}
	if _, _, err := m.Apply(raw, s.Current, a1); err != nil {
		t.Errorf("bot's move refused: %v", err)
	}
}

func TestConformance_PlaysWholeMatchesToAWinner(t *testing.T) {
	m := New()
	for _, n := range []int{2, 4, 6} {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("p%d", i)
		}
		for seed := int64(1); seed <= 10; seed++ {
			state, err := m.NewMatch(module.MatchConfig{Options: module.Options{OptRounds: 3}}, seats(ids...), seed)
			if err != nil {
				t.Fatal(err)
			}
			final, res, err := module.PlayWithOffers(m, state, seats(ids...), module.DriverOptions{MaxActions: 5000})
			if err != nil {
				t.Fatalf("n=%d seed %d: %v", n, seed, err)
			}
			if !res.Finished || len(res.Winners) == 0 {
				t.Fatalf("n=%d seed %d: finished=%v winners=%v", n, seed, res.Finished, res.Winners)
			}
			done, winners, _ := m.Finished(final)
			if !done || len(winners) == 0 {
				t.Errorf("seed %d: Finished disagrees: %v %v", seed, done, winners)
			}
		}
	}
}

func TestBotPlaysWholeMatches(t *testing.T) {
	m := New()
	ids := []string{"a", "b", "c", "d"}
	for seed := int64(1); seed <= 10; seed++ {
		raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{OptRounds: 2}}, seats(ids...), seed)
		for step := 0; ; step++ {
			if done, _, _ := m.Finished(raw); done {
				break
			}
			if step > 3000 {
				t.Fatalf("seed %d: bots never finished", seed)
			}
			moved := false
			for _, id := range ids {
				offers, _ := m.LegalActions(raw, id)
				live := false
				for _, o := range offers {
					live = live || o.Enabled
				}
				if !live {
					continue
				}
				a, ok := bot{}.Act(raw, module.BotSeat{PlayerID: id, Seed: seed}, offers)
				if !ok {
					continue
				}
				out, _, err := m.Apply(raw, id, a)
				if err != nil {
					t.Fatalf("seed %d: %s's %v refused: %v", seed, id, a, err)
				}
				raw, moved = out, true
				break
			}
			if !moved {
				t.Fatalf("seed %d: nobody could move", seed)
			}
		}
	}
}

func TestStandingsAndRoundsCountMarks(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{OptRounds: 3, module.OptPauseBetweenRounds: module.OptOff}}, seats("a", "b", "c"), 2)
	for {
		s := mustState(t, raw)
		if s.Status != "active" {
			break
		}
		raw, _, _ = m.Apply(raw, s.Current, module.Action{Verb: VerbDraw})
	}
	s := mustState(t, raw)
	st, _ := m.Standings(raw)
	if st[0].Shown == nil || *st[0].Shown != s.seat(st[0].PlayerID).Marks {
		t.Errorf("standing shows %v", st[0].Shown)
	}
	_, winners, _ := m.Finished(raw)
	for _, w := range winners {
		if s.seat(w).Marks != *st[0].Shown {
			t.Errorf("winner %s has %d marks, leader shows %d", w, s.seat(w).Marks, *st[0].Shown)
		}
	}
	log, _ := m.Rounds(raw)
	if len(log.Rounds) != 3 {
		t.Fatalf("%d rounds logged", len(log.Rounds))
	}
	sum := 0
	for _, r := range log.Rounds {
		for _, sc := range r.Scores {
			sum += *sc.Shown
		}
	}
	if sum != 3 {
		t.Errorf("marks handed out = %d, want one a round", sum)
	}
}

func TestEventsAreNarrated(t *testing.T) {
	m := New()
	raw, _ := m.NewMatch(noPause(), seats("a", "b"), 4)
	seen := map[string]bool{}
	for i := 0; i < 400; i++ {
		s := mustState(t, raw)
		if s.Status != "active" {
			break
		}
		out, events, err := m.Apply(raw, s.Current, module.Action{Verb: VerbDraw})
		if err != nil {
			t.Fatal(err)
		}
		for _, ev := range events {
			if mv, ok := m.NarrateEvent(out, ev); ok {
				seen[mv.Fact.LabelKey] = true
			}
		}
		raw = out
	}
	for _, k := range []string{"blackpeter.move.took", "blackpeter.move.pair", "blackpeter.move.out", "blackpeter.move.lost"} {
		if !seen[k] {
			t.Errorf("never narrated %s", k)
		}
	}
}

func TestRuleIndex(t *testing.T) {
	problems, notes := module.RuleIndexCheck{
		Module: New(),
		Dirs:   []string{"."},
		Exempt: map[string]string{
			"GAME_NOT_ACTIVE":         "not a rule — the round is over or has not started",
			"UNKNOWN_ACTION":          "not a rule — a verb this module does not have",
			"WRONG_PLAYER_COUNT":      "not a rule of play — the table could not be seated",
			"BLACKPETER_NO_SUCH_CARD": "not a rule — the client named a position the hand does not have",
		},
	}.Run()
	for _, n := range notes {
		t.Log(n)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestLessonsWalk(t *testing.T) { lessontest.Check(t, New()) }

func TestLessonFollowsOptions(t *testing.T) {
	text := func(opts module.Options) string {
		ls, _ := New().Lessons(module.MatchConfig{Options: opts})
		for _, st := range ls[0].Steps {
			if st.ID == "pairs" {
				return st.TextKey
			}
		}
		return ""
	}
	if text(module.Options{OptPairs: pairsByRank}) == text(module.Options{OptPairs: pairsByColour}) {
		t.Error("pairs step should follow the pairing option")
	}
}
