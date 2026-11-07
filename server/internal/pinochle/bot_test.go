package pinochle

import (
	"math/rand"
	"slices"
	"sort"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

func playMatch(t *testing.T, v string, seed int64, skills map[string]module.Skill) map[string]int {
	t.Helper()
	m := New()
	state, err := m.NewMatch(cfgOf(v, nil), refs(seatsOf(v)), seed)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 40000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			return s.Scores
		}
		actor := s.Current
		offers, _ := m.LegalActions(state, actor)
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skills[actor], Seed: seed}, offers)
		if !ok {
			t.Fatalf("%s seed %d: %s had no move", v, seed, actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("%s seed %d: %s proposed an illegal %s %v %v: %v", v, seed, actor, a.Verb, a.Cards, a.Params, err)
		}
		state = next
	}
	t.Fatalf("%s seed %d: the match did not finish", v, seed)
	return nil
}

func all(v string, skill module.Skill) map[string]module.Skill {
	out := map[string]module.Skill{}
	for _, p := range refs(seatsOf(v)) {
		out[p.ID] = skill
	}
	return out
}

func TestBotsPlayWholeMatchesLegally(t *testing.T) {
	for _, v := range allVariations {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			seeds := int64(3)
			if skill == module.SkillHard {
				seeds = 1
			}
			for seed := int64(1); seed <= seeds; seed++ {
				playMatch(t, v, seed, all(v, skill))
			}
		}
	}
}

// TestBotDoesNotPeek: the same seat, the same public table, and the cards
// it cannot see dealt differently, must get the same move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	for _, v := range allVariations {
		for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
			for seed := int64(1); seed <= 2; seed++ {
				raw, _ := m.NewMatch(cfgOf(v, nil), refs(seatsOf(v)), seed)
				for step := 0; step < 80; step++ {
					s, _ := decode(raw)
					if s.Status != "active" || len(s.Rounds) > 0 || s.Intermission.Open {
						break
					}
					me := s.Current
					other := reshuffled(s, me, seed*100+int64(step))
					act := func(st *GameState) module.Action {
						enc, _ := encode(st)
						offers, _ := m.LegalActions(enc, me)
						a, _ := m.Bot().Act(enc, module.BotSeat{PlayerID: me, Skill: skill, Seed: seed}, offers)
						return a
					}
					a, b := act(s), act(other)
					if a.OfferID != b.OfferID || !slices.Equal(a.Cards, b.Cards) || a.Params[ParamAmount] != b.Params[ParamAmount] || a.Params[ParamSuit] != b.Params[ParamSuit] {
						t.Fatalf("%s %s seed %d step %d (%s): %v in one world, %v in the other", v, skill, seed, step, s.Phase, a, b)
					}
					raw, _, _ = m.Apply(raw, me, a)
				}
			}
		}
	}
}

// reshuffled is s with everything me cannot see dealt out again: the other
// hands, the stock, the widow before it is turned and another's bury. What is
// public stays where it is — the meld on the table, the turned trump, the
// widow once shown — and so does anything me passed to its partner.
func reshuffled(s *GameState, me string, seed int64) *GameState {
	raw, _ := encode(s)
	out, _ := decode(raw)
	keep := map[string][]string{}
	var pool []string
	for _, p := range out.Players {
		if p == me {
			continue
		}
		fixed := counts{}
		if m, ok := out.Melds[p]; ok {
			for _, c := range m.MeldCards() {
				fixed[c]++
			}
		}
		if u := out.Used[p]; u != nil {
			for c, n := range countOf(u.shown()) {
				fixed[c] = max(fixed[c], n)
			}
		}
		if out.partner(me) == p {
			for c, n := range countOf(out.Given[me]) {
				fixed[c] = max(fixed[c], n)
			}
		}
		if p == out.Bidder && out.Phase != phaseAuction {
			for c, n := range countOf(out.Widow) {
				fixed[c] = max(fixed[c], n)
			}
		}
		for _, c := range out.Hands[p] {
			if fixed[c] > 0 {
				fixed[c]--
				keep[p] = append(keep[p], c)
			} else {
				pool = append(pool, c)
			}
		}
	}
	pool = append(pool, out.Stock...)
	if out.Phase == phaseAuction {
		pool = append(pool, out.Widow...)
	}
	if me != out.Bidder {
		pool = append(pool, out.Buried...)
	}
	sort.Strings(pool)
	rand.New(rand.NewSource(seed)).Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for _, p := range out.Players {
		if p == me {
			continue
		}
		n := len(out.Hands[p]) - len(keep[p])
		out.Hands[p] = append(keep[p], pool[:n]...)
		pool = pool[n:]
		sortHand(out.Hands[p], out.trump(), out.table().order())
	}
	n := len(out.Stock)
	out.Stock, pool = append([]string(nil), pool[:n]...), pool[n:]
	if out.Phase == phaseAuction {
		n = len(out.Widow)
		out.Widow, pool = append([]string(nil), pool[:n]...), pool[n:]
	}
	if me != out.Bidder {
		out.Buried = append([]string(nil), pool...)
	}
	return out
}

// TestHardSolvesATwoHandedEnding: after the stock the other hand is known,
// and the solver must find the line that wins the last trick.
func TestHardSolvesATwoHandedEnding(t *testing.T) {
	s := newState(t, VarTwoHanded, nil)
	s.Trump, s.TurnUp, s.Stock, s.Late = "H", "", nil, true
	s.Hands = map[string][]string{"p1": {"AH", "9C"}, "p2": {"TH", "AC"}}
	// Every other card has been played, so the solver's world is exactly
	// these four.
	rest := countOf(s.table().buildDeck())
	for _, h := range s.Hands {
		for _, c := range h {
			rest[c]--
		}
	}
	var hist [][]tricks.Play
	var pending []string
	for _, c := range sortedKeys(rest) {
		for i := 0; i < rest[c]; i++ {
			pending = append(pending, c)
		}
	}
	for i := 0; i+1 < len(pending); i += 2 {
		// The other player leads every one, so the play says nothing about
		// what it holds.
		hist = append(hist, []tricks.Play{{Seat: 1, Card: pending[i]}, {Seat: 0, Card: pending[i+1]}})
	}
	s.History = hist
	s.Leader, s.Current, s.Phase = "p1", "p1", phasePlay
	got := s.hardCard("p1", s.legal("p1"), rand.New(rand.NewSource(1)))
	// Leading the ace of trumps takes the ten but gives up the last trick;
	// the nine of clubs gives the opponent the ace of clubs and wins the ten
	// of trumps under the ace, with the last trick's ten.
	if got != "9C" {
		t.Fatalf("hard led %s, want 9C", got)
	}
}

func TestRenameApartKeepsEveryCardDistinct(t *testing.T) {
	deck := tableOf(VarPartnership).buildDeck()
	hands := [][]string{deck[:12], deck[12:24], deck[24:36], deck[36:]}
	out, _ := renameApart(hands, nil)
	seen := map[string]bool{}
	back := counts{}
	for _, h := range out {
		for _, c := range h {
			if seen[c] {
				t.Fatalf("%s appears twice", c)
			}
			seen[c] = true
			if tricks.Rank(c) != c[0] || tricks.Suit(c) != c[1] {
				t.Fatalf("%s lost its rank or suit", c)
			}
			back[c[:2]]++
		}
	}
	if len(seen) != 48 {
		t.Fatalf("%d distinct cards, want 48", len(seen))
	}
	for c, n := range countOf(deck) {
		if back[c] != n {
			t.Fatalf("%s: %d named back, want %d", c, back[c], n)
		}
	}
}

// TestStrengthIsOrdered: medium beats easy and hard beats medium, a strong
// side against a weak one, seats swapped across seeds.
func TestStrengthIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, c := range []struct {
		strong, weak module.Skill
		seeds        int64
	}{
		{module.SkillMedium, module.SkillEasy, 16},
		{module.SkillHard, module.SkillMedium, 6},
	} {
		total, n := 0, 0
		for seed := int64(1); seed <= c.seeds; seed++ {
			for _, strongSide := range []int{0, 1} {
				skills := map[string]module.Skill{}
				for i, p := range refs(4) {
					skills[p.ID] = c.weak
					if i%2 == strongSide {
						skills[p.ID] = c.strong
					}
				}
				sc := playMatch(t, VarPartnership, seed, skills)
				ss := sideOf(strongSide, true)
				total += sc[ss] - sc[sideOf(1-strongSide, true)]
				n++
			}
		}
		mean := float64(total) / float64(n)
		t.Logf("%s minus %s: %+.1f points a match over %d matches", c.strong, c.weak, mean, n)
		if mean <= 0 {
			t.Errorf("%s played %+.1f against %s", c.strong, mean, c.weak)
		}
	}
}
