package snaps

import (
	"math/rand"
	"slices"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// playMatch runs a whole match between bots, every action through Apply so
// an illegal one fails here. It returns the final game points.
func playMatch(t *testing.T, variation string, seed int64, seats map[string]module.Skill) map[string]int {
	t.Helper()
	m := New()
	cfg := module.MatchConfig{Variation: variation, Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}
	state, err := m.NewMatch(cfg, players, seed)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 3000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			return s.Scores
		}
		actor := s.Current
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: seats[actor], Seed: seed}, offers)
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

func TestBotsPlayWholeMatchesLegally(t *testing.T) {
	for _, variation := range []string{variationSnaps, variation66} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			seeds := int64(10)
			if skill == module.SkillHard {
				seeds = 3
			}
			for seed := int64(1); seed <= seeds; seed++ {
				scores := playMatch(t, variation, seed, map[string]module.Skill{"p1": skill, "p2": skill})
				if max(scores["p1"], scores["p2"]) < defaultTarget {
					t.Errorf("%s seed %d ended at %v", variation, seed, scores)
				}
			}
		}
	}
}

// brute is the strict-phase value by plain minimax, no pruning, for
// checking the solver against.
func brute(p *position) int {
	p.exhaustive = true
	defer func() { p.exhaustive = false }()
	toPlay := p.leader
	if p.led != "" {
		toPlay = 1 - p.leader
	}
	best := 4
	if toPlay == 0 {
		best = -4
	}
	for _, c := range p.legal(toPlay) {
		v := p.after(toPlay, c, -100, 100) // wide window: no cut-offs
		if toPlay == 0 {
			best = max(best, v)
		} else {
			best = min(best, v)
		}
	}
	return best
}

func TestSolverMatchesBruteForce(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	for _, variation := range []string{variationSnaps, variation66} {
		rs := rulesFor(variation)
		deck := buildDeck(rs.ranks)
		for i := 0; i < 150; i++ {
			r.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
			n := 2 + r.Intn(4)
			p := &position{r: rs, trump: "HDCS"[r.Intn(4)], marry: !rs.strictMarriages}
			p.hands[0] = slices.Clone(deck[:n])
			p.hands[1] = slices.Clone(deck[n : 2*n])
			p.t = tally{points: [2]int{r.Intn(50), r.Intn(50)}, tricks: [2]int{r.Intn(3), r.Intn(3)}}
			p.close = notClosed
			if r.Intn(2) == 0 {
				p.close = closing{by: r.Intn(2), oppTricks: r.Intn(2), oppPoints: r.Intn(40)}
				p.closed = true
			}
			p.leader = r.Intn(2)
			want := brute(p)
			_, got, ok := p.solve(0)
			if !ok || got != want {
				t.Fatalf("%s position %d: solver %d (ok %v), brute force %d", variation, i, got, ok, want)
			}
		}
	}
}

// TestBotDoesNotPeek: the same seat, the same hand and table, and two
// different hidden worlds — the other hand and the talon reshuffled between
// them — must get the same move.
func TestBotDoesNotPeek(t *testing.T) {
	m := New()
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 6; seed++ {
			raw, _ := m.NewMatch(module.MatchConfig{Options: module.Options{module.OptPauseBetweenRounds: module.OptOff}}, players, seed)
			for step := 0; step < 40; step++ {
				s, _ := decode(raw)
				if s.Status != "active" || s.Intermission.Open {
					break
				}
				me := s.Current
				other := reshuffled(s, me, seed)
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
				offers, _ := m.LegalActions(raw, me)
				next, _, err := m.Apply(raw, me, act(s))
				if err != nil {
					t.Fatalf("illegal: %v (%v)", err, offers)
				}
				raw = next
			}
		}
	}
}

// reshuffled is s with the cards me cannot see dealt again between the
// other hand and the face-down talon.
func reshuffled(s *GameState, me string, seed int64) *GameState {
	raw, _ := encode(s)
	out, _ := decode(raw)
	opp := s.other(me)
	known := map[string]bool{}
	for _, c := range out.Known[opp] {
		known[c] = true
	}
	var pool []string
	var free []int
	for i, c := range out.Hands[opp] {
		if !known[c] {
			pool = append(pool, c)
			free = append(free, i)
		}
	}
	down := len(out.Talon)
	if out.open() {
		down-- // the turned-up trump is seen
	}
	pool = append(pool, out.Talon[:down]...)
	rand.New(rand.NewSource(seed+99)).Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for k, i := range free {
		out.Hands[opp][i] = pool[k]
	}
	copy(out.Talon[:down], pool[len(free):])
	sortHand(out.Hands[opp])
	return out
}

// TestHardBeatsMedium is the search's strength check, paired to cut the
// noise: each seed is played with hard in either seat against medium.
func TestHardBeatsMedium(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, variation := range []string{variationSnaps, variation66} {
		total, n := 0, 0
		for seed := int64(1); seed <= 40; seed++ {
			for _, strong := range []string{"p1", "p2"} {
				seats := map[string]module.Skill{"p1": module.SkillMedium, "p2": module.SkillMedium}
				seats[strong] = module.SkillHard
				sc := playMatch(t, variation, seed, seats)
				weak := "p1"
				if strong == "p1" {
					weak = "p2"
				}
				total += sc[strong] - sc[weak]
				n++
			}
		}
		mean := float64(total) / float64(n)
		t.Logf("%s — hard minus medium: %+.2f game points a match over %d matches", variation, mean, n)
		if mean <= 0 {
			t.Errorf("%s: hard played %+.2f a match against medium", variation, mean)
		}
	}
}

var _ = tricks.NoTrump

// TestSolverAgreesWithTheEngine: from real strict-phase positions with both
// hands known, both sides play the solver's line through Apply, and the deal
// must end exactly as the solver said it would.
func TestSolverAgreesWithTheEngine(t *testing.T) {
	m := New()
	checked := 0
	for _, variation := range []string{variationSnaps, variation66} {
		for seed := int64(1); seed <= 30; seed++ {
			cfg := module.MatchConfig{Variation: variation, Options: module.Options{module.OptPauseBetweenRounds: module.OptOn}}
			raw, _ := m.NewMatch(cfg, players, seed)
			// Medium play until the strict phase, with the medium seats
			// closing when they like, or the deal ends first.
			for step := 0; step < 60; step++ {
				s, _ := decode(raw)
				if s.Status != "active" || s.Intermission.Open {
					break
				}
				if s.strict() && len(s.Trick) == 0 {
					want := solvedValue(s)
					final := playSolvedLine(t, m, raw)
					if final != want {
						t.Fatalf("%s seed %d: solver said %d, the engine scored %d", variation, seed, want, final)
					}
					checked++
					break
				}
				offers, _ := m.LegalActions(raw, s.Current)
				a, _ := m.Bot().Act(raw, module.BotSeat{PlayerID: s.Current, Skill: module.SkillMedium, Seed: seed}, offers)
				raw, _, _ = m.Apply(raw, s.Current, a)
			}
		}
	}
	if checked < 20 {
		t.Fatalf("only %d positions reached the strict phase", checked)
	}
}

// open is the position with both real hands.
func openPosition(s *GameState) *position {
	p := s.positionFor(s.Players[0], slices.Clone(s.Hands[s.Players[1]]), false)
	p.limit = 0
	return p
}

func solvedValue(s *GameState) int {
	_, v, _ := openPosition(s).solve(0)
	return v
}

// playSolvedLine plays the deal out with both seats following the solver,
// and returns its result signed for seat 0.
func playSolvedLine(t *testing.T, m *Module, raw module.State) int {
	t.Helper()
	for {
		s, _ := decode(raw)
		if s.Intermission.Open || s.Status != "active" || len(s.Rounds) > 0 && s.Rounds[len(s.Rounds)-1].Number == s.Deal {
			last := s.Rounds[len(s.Rounds)-1]
			if last.Winner == "" {
				return 0
			}
			return signed(s.seat(last.Winner), last.GamePoints)
		}
		card, _, _ := openPosition(s).solve(s.seat(s.Current))
		next, _, err := m.Apply(raw, s.Current, playCard(card))
		if err != nil {
			t.Fatalf("solver's card %s refused: %v", card, err)
		}
		raw = next
	}
}
