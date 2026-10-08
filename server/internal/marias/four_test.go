package marias

import (
	"math/rand"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// Čtyřhranný mariáš: four at the table, the dealer sitting each deal out.

var fourPlayers = []module.PlayerRef{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}, {ID: "p4"}}

func newFour(t *testing.T, variation string, opts module.Options) *GameState {
	t.Helper()
	raw, err := New().NewMatch(module.MatchConfig{Variation: variation, Options: opts}, fourPlayers, 42)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFourSeatsDealTheDealerOut(t *testing.T) {
	for _, variation := range []string{variationVoleny, variationLicit} {
		s := newFour(t, variation, nil)
		seen := map[string]bool{}
		for deal := 0; deal < 8; deal++ {
			s.Deal = deal
			startDeal(s)
			c := s.chooser()
			if want := s.Players[(s.seat(c)+3)%4]; s.Sitter != want {
				t.Fatalf("%s deal %d: %s sits out, want the dealer %s on the chooser %s's right", variation, deal, s.Sitter, want, c)
			}
			if deal < 4 {
				seen[c] = true
			}
			if _, dealt := s.Hands[s.Sitter]; dealt {
				t.Errorf("%s deal %d: the sitter %s was dealt cards", variation, deal, s.Sitter)
			}
			cards := len(s.Unseen) + len(s.Talon)
			for _, p := range s.active() {
				cards += len(s.Hands[p])
			}
			if cards != 32 {
				t.Errorf("%s deal %d: %d cards dealt, want 32", variation, deal, cards)
			}
			if s.Current == s.Sitter || s.Holder == s.Sitter || s.Bidder == s.Sitter || s.Waiting == s.Sitter {
				t.Errorf("%s deal %d: the sitter %s has a role in the deal", variation, deal, s.Sitter)
			}
			for _, p := range s.active() {
				if s.next(p) == s.Sitter {
					t.Errorf("%s deal %d: the turn passes from %s to the sitter", variation, deal, p)
				}
			}
		}
		if len(seen) != 4 {
			t.Errorf("%s: four deals had %d different choosers, want all four", variation, len(seen))
		}
	}
}

func TestThreeSeatsHaveNoSitter(t *testing.T) {
	s := newState(t, nil)
	if s.Sitter != "" || len(s.active()) != 3 {
		t.Fatalf("a table of three sits out %q, plays %v", s.Sitter, s.active())
	}
}

func TestFourSeatsRoundTheMatchUp(t *testing.T) {
	for _, c := range []struct{ deals, three, four int }{{9, 9, 12}, {12, 12, 12}, {18, 18, 20}, {24, 24, 24}} {
		opts := module.Options{OptDeals: c.deals}
		if got := newState(t, opts).Deals; got != c.three {
			t.Errorf("%d deals at three: %d", c.deals, got)
		}
		if got := newFour(t, variationVoleny, opts).Deals; got != c.four {
			t.Errorf("%d deals at four: %d, want %d", c.deals, got, c.four)
		}
	}
}

// playFour runs a whole four-seat match between bots, checking at every step
// that the sitter is never on turn, never offered a move, and shown sitting
// out; and at the end that nobody's units moved in a deal they sat out.
func playFour(t *testing.T, variation string, seed int64, skill module.Skill) {
	t.Helper()
	m := New()
	cfg := module.MatchConfig{Variation: variation, Options: module.Options{OptDeals: 8, module.OptPauseBetweenRounds: module.OptOff}}
	state, err := m.NewMatch(cfg, fourPlayers, seed)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 6000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			if len(s.Rounds) != s.Deals {
				t.Fatalf("seed %d: %d deals played, want %d", seed, len(s.Rounds), s.Deals)
			}
			sum := 0
			for _, p := range s.Players {
				sum += s.Scores[p]
			}
			if sum != 0 {
				t.Errorf("seed %d: the units add to %d, want 0", seed, sum)
			}
			sat := map[string]int{}
			for _, r := range s.Rounds {
				if r.Sitter == "" {
					t.Fatalf("seed %d deal %d: nobody sat out", seed, r.Number)
				}
				sat[r.Sitter]++
				if d := r.Deltas[r.Sitter]; d != 0 {
					t.Errorf("seed %d deal %d: the sitter %s moved %d units", seed, r.Number, r.Sitter, d)
				}
			}
			for _, p := range s.Players {
				if sat[p] != s.Deals/4 {
					t.Errorf("seed %d: %s sat out %d deals, want %d", seed, p, sat[p], s.Deals/4)
				}
			}
			return
		}
		if s.Sitter == "" || s.Current == s.Sitter {
			t.Fatalf("seed %d step %d: sitter %q, %s on turn in %s", seed, step, s.Sitter, s.Current, s.Phase)
		}
		if offers, _ := m.LegalActions(state, s.Sitter); len(offers) != 0 {
			t.Fatalf("seed %d step %d: the sitter is offered %d moves", seed, step, len(offers))
		}
		vm, err := m.View(state, s.Sitter)
		if err != nil {
			t.Fatal(err)
		}
		for _, z := range vm.Zones {
			if z.Kind == module.ZoneHand && z.OwnerID == s.Sitter && z.LabelKey == "zone.opponentHand" {
				t.Fatalf("seed %d: the sitter's empty hand is shown as an opponent's", seed)
			}
		}
		labelled := false
		for _, seat := range vm.Seats {
			if seat.PlayerID == s.Sitter {
				for _, k := range seat.LabelKeys {
					labelled = labelled || k == "marias.seat.sittingOut"
				}
			}
		}
		if !labelled {
			t.Fatalf("seed %d step %d: the sitter's seat does not say so", seed, step)
		}
		for _, seat := range vm.Seats {
			if seat.PlayerID == s.Sitter && len(seat.Facts) > 0 {
				t.Fatalf("seed %d step %d: the sitter's seat shows %v", seed, step, seat.Facts)
			}
		}

		actor := s.Current
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skill, Seed: seed}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move in phase %s", seed, actor, s.Phase)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s/%s %v in phase %s: %v", seed, actor, a.Verb, a.OfferID, a.Cards, s.Phase, err)
		}
		state = next
	}
	t.Fatalf("seed %d: the match did not finish", seed)
}

func TestFourSeatsPlayWholeMatches(t *testing.T) {
	for _, variation := range []string{variationVoleny, variationLicit} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			seeds := int64(6)
			if skill == module.SkillHard {
				seeds = 1
			}
			for seed := int64(1); seed <= seeds; seed++ {
				playFour(t, variation, seed, skill)
			}
		}
	}
}

// withSitterFirst is s at a table of four: the same deal, with a sitter "p0"
// added at seat zero and every played card's seat moved along by one.
func withSitterFirst(t *testing.T, s *GameState) *GameState {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	out, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	out.Players = append([]string{"p0"}, out.Players...)
	out.Sitter = "p0"
	out.Scores["p0"] = 0
	shift := func(plays []tricks.Play) {
		for i := range plays {
			plays[i].Seat++
		}
	}
	shift(out.Trick)
	shift(out.LastTrick)
	shift(out.PrevTrick)
	for _, h := range out.History {
		shift(h)
	}
	return out
}

func TestSevenWithTheSitterAtSeatZero(t *testing.T) {
	s := ended(t, "C", map[string]int{"p1": 60}, nil)
	s.Sedma = true
	s.LastTrick = []tricks.Play{{Seat: 0, Card: "7C"}, {Seat: 1, Card: "8S"}, {Seat: 2, Card: "9S"}}
	four := withSitterFirst(t, s)
	if p := part(t, four, partSedma); !p.ForDeclarer || p.Units != 2 {
		t.Errorf("sedma won with a sitter at seat zero: %+v", p)
	}
}

// TestSearchIgnoresTheSitter: the hard bot's search must see a four-seat
// deal exactly as the same deal at three — the same card, in the same nodes.
func TestSearchIgnoresTheSitter(t *testing.T) {
	lim := searchLimits{total: 150_000, perSample: 40_000, capped: true}
	positions := searchPositions(t, 1)
	checked := 0
	for i := 0; i < len(positions); i += 7 {
		p := positions[i]
		four := withSitterFirst(t, p.state)
		c3, n3, ok3 := p.state.searchPlay(p.me, p.legal, rand.New(rand.NewSource(p.seed)), lim)
		c4, n4, ok4 := four.searchPlay(p.me, p.legal, rand.New(rand.NewSource(p.seed)), lim)
		if c3 != c4 || n3 != n4 || ok3 != ok4 {
			t.Errorf("%s: three seats play %s in %d nodes, four %s in %d", p.id, c3, n3, c4, n4)
		}
		checked++
	}
	if checked < 10 {
		t.Fatalf("only %d positions checked", checked)
	}
}
