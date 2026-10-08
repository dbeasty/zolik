package marias

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

func TestBetlDanger(t *testing.T) {
	cases := []struct {
		hand, gone []string
		want       map[string]bool // the cards in danger
	}{
		// From the bottom with nothing between: nothing to force.
		{[]string{"7H", "8H", "9H"}, nil, map[string]bool{}},
		// The eight between: the nine can be forced.
		{[]string{"7H", "9H"}, nil, map[string]bool{"9H": true}},
		// …unless the declarer laid it away.
		{[]string{"7H", "9H"}, []string{"8H"}, map[string]bool{}},
		// A singleton eight, forced by the seven.
		{[]string{"8S"}, nil, map[string]bool{"8S": true}},
	}
	for _, c := range cases {
		got := betlDanger(c.hand, c.gone)
		for card := range c.want {
			if got[card] == 0 {
				t.Errorf("%v (talon %v): %s not in danger, got %v", c.hand, c.gone, card, got)
			}
		}
		for card := range got {
			if !c.want[card] {
				t.Errorf("%v (talon %v): %s in danger, got %v", c.hand, c.gone, card, got)
			}
		}
	}
	// The ace over a seven is forced by any lead and covered by nothing.
	if d := betlDanger([]string{"7S", "AS"}, nil)["AS"]; d < 1 {
		t.Errorf("an ace beside a seven: danger %.2f", d)
	}
}

func TestDurchLosers(t *testing.T) {
	cases := []struct {
		hand []string
		want int
	}{
		{[]string{"AH", "KH", "QH"}, 0},
		{[]string{"AH", "QH"}, 1}, // the král is out
		{[]string{"AH", "KH", "7H"}, 1},
		// Seven of the eight: the one left out falls to the first rounds.
		{[]string{"AH", "KH", "QH", "JH", "TH", "9H", "7H"}, 0},
		{[]string{"AD"}, 0},
	}
	for _, c := range cases {
		if got := durchLosers(c.hand, nil); len(got) != c.want {
			t.Errorf("%v: losers %v, want %d", c.hand, got, c.want)
		}
	}
}

// announceState is the chooser, p1, with twelve cards and a game to name.
func announceState(t *testing.T, hand []string) *GameState {
	t.Helper()
	s := newState(t, nil)
	s.Hands["p1"] = hand
	s.Unseen = nil
	s.Declarer, s.TrumpCard = "p1", "9H"
	s.Phase, s.Current = phaseAnnounce, "p1"
	return s
}

func TestBotAnnouncesDurchAndBetl(t *testing.T) {
	durch := []string{"AH", "KH", "QH", "JH", "TH", "AC", "KC", "QC", "AS", "KS", "7D", "8D"}
	betl := []string{"7H", "8H", "9H", "7C", "8C", "9C", "7S", "8S", "7D", "8D", "AS", "KD"}
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		if a := botAct(t, announceState(t, durch), "p1", skill); a.OfferID != OfferDurch {
			t.Errorf("%s: a durch hand announced %s", skill, a.OfferID)
		}
		if a := botAct(t, announceState(t, betl), "p1", skill); a.OfferID != OfferBetl {
			t.Errorf("%s: a betl hand announced %s", skill, a.OfferID)
		}
		// And lays away the two cards that would break it.
		s := announceState(t, betl)
		s.Game, s.Phase = gameBetl, phaseTalon
		a := botAct(t, s, "p1", skill)
		sort.Strings(a.Cards)
		if !reflect.DeepEqual(a.Cards, []string{"AS", "KD"}) {
			t.Errorf("%s: a betl declarer laid away %v", skill, a.Cards)
		}
	}
	if a := botAct(t, announceState(t, durch), "p1", module.SkillEasy); a.OfferID == OfferDurch {
		t.Error("an easy seat announced durch")
	}
}

func TestBotSaysSpatnaWithABetl(t *testing.T) {
	s := newState(t, nil)
	s.Hands["p2"] = []string{"7H", "8H", "9H", "7C", "8C", "7S", "8S", "9S", "7D", "8D"}
	s.Unseen, s.Talon = nil, []string{"QC", "KC"}
	s.Declarer, s.Game, s.TrumpCard = "p1", gameHra, "AH"
	s.Phase, s.Current = phaseAnswer, "p2"
	if a := botAct(t, s, "p2", module.SkillMedium); a.OfferID != OfferBadBetl {
		t.Fatalf("ten low cards answered %s", a.OfferID)
	}
	s.Hands["p2"] = []string{"7H", "8H", "9H", "7C", "8C", "7S", "8S", "9S", "7D", "AD"}
	if a := botAct(t, s, "p2", module.SkillMedium); a.OfferID != OfferGood {
		t.Fatalf("an ace beside a seven answered %s", a.OfferID)
	}
}

// contractTally is what a sweep of bot matches declared: each game by how it
// came to be played (announced by the chooser, taken over with špatná, or
// won in the licitovaný auction), and how the betls and durchs went.
type contractTally struct {
	games map[string]int // game/how
	made  map[string]int // betl and durch made, by game/how
	units map[string]int // the declarer's units from them, by game/how
	deals int
}

func newContractTally() *contractTally {
	return &contractTally{games: map[string]int{}, made: map[string]int{}, units: map[string]int{}}
}

func (c *contractTally) String() string {
	var keys []string
	for k := range c.games {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "%d deals:", c.deals)
	for _, k := range keys {
		fmt.Fprintf(&b, " %s %d", k, c.games[k])
		if g := strings.SplitN(k, "/", 2)[0]; g == gameBetl || g == gameDurch {
			fmt.Fprintf(&b, " (made %d, %+d units)", c.made[k], c.units[k])
		}
		b.WriteString(";")
	}
	return b.String()
}

// tallyMatch plays a whole match between bot seats, the way playMatchVariation
// does, and counts every deal's contract.
func tallyMatch(t *testing.T, bot module.Bot, variation string, seed int64, seats map[string]module.Skill, c *contractTally) {
	t.Helper()
	m := New()
	cfg := module.MatchConfig{Variation: variation, Options: module.Options{OptDeals: 9, module.OptPauseBetweenRounds: module.OptOff}}
	state, err := m.NewMatch(cfg, players, seed)
	if err != nil {
		t.Fatal(err)
	}
	how := ""
	for step := 0; step < 4000; step++ {
		s, _ := decode(state)
		if s.Status != "active" {
			return
		}
		actor := s.Current
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		a, ok := bot.Act(state, module.BotSeat{PlayerID: actor, Skill: seats[actor], Seed: seed}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move in phase %s", seed, actor, s.Phase)
		}
		switch {
		case a.Verb == VerbTakeOver:
			how = "takeover"
		case s.Phase == phaseAnnounce && s.licit():
			how = "licit"
		case s.Phase == phaseAnnounce:
			how = "announce"
		}
		rounds := len(s.Rounds)
		if state, _, err = m.Apply(state, actor, a); err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s/%s: %v", seed, actor, a.Verb, a.OfferID, err)
		}
		after, _ := decode(state)
		if len(after.Rounds) > rounds {
			rec := after.Rounds[len(after.Rounds)-1]
			k := rec.Game + "/" + how
			c.games[k]++
			c.deals++
			if rec.Game == gameBetl || rec.Game == gameDurch {
				if rec.Parts[0].ForDeclarer {
					c.made[k]++
				}
				c.units[k] += rec.Deltas[rec.Declarer]
			}
			how = ""
		}
	}
	t.Fatalf("seed %d: the match did not finish", seed)
}

// TestContractRates measures how often each skill declares each game, every
// seat at the table the same skill. A measurement, not a check:
//
//	MARIAS_CONTRACT_RATES=30 go test ./internal/marias -run ContractRates -v
//
// The number is the seeds per variation; hard plays a fifth of them.
func TestContractRates(t *testing.T) {
	seeds, _ := strconv.Atoi(os.Getenv("MARIAS_CONTRACT_RATES"))
	if seeds <= 0 {
		t.Skip("a measurement: set MARIAS_CONTRACT_RATES to the seeds per variation")
	}
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		n := int64(seeds)
		if skill == module.SkillHard {
			n = max(1, n/5)
		}
		for _, variation := range []string{variationVoleny, variationLicit} {
			c := newContractTally()
			for seed := int64(1); seed <= n; seed++ {
				tallyMatch(t, bot{}, variation, seed, map[string]module.Skill{"p1": skill, "p2": skill, "p3": skill}, c)
			}
			t.Logf("%-6s %-10s %s", skill, variation, c)
		}
	}
}

// TestContractCalibration deals random hands and sets the rules of thumb
// beside the solver: of the hands judged a betl or durch, how often does an
// open-handed defence break it. A measurement:
//
//	MARIAS_CONTRACT_CALIBRATION=20000 go test ./internal/marias -run ContractCalibration -v
func TestContractCalibration(t *testing.T) {
	deals, _ := strconv.Atoi(os.Getenv("MARIAS_CONTRACT_CALIBRATION"))
	if deals <= 0 {
		t.Skip("a measurement: set MARIAS_CONTRACT_CALIBRATION to the deals")
	}
	r := rand.New(rand.NewSource(1))
	type bucket struct {
		n    int
		odds float64
	}
	buckets := map[string]*bucket{}
	add := func(k string, odds float64) {
		b := buckets[k]
		if b == nil {
			b = &bucket{}
			buckets[k] = b
		}
		b.n++
		b.odds += odds
	}
	for d := 0; d < deals; d++ {
		deck := buildDeck()
		r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
		for _, size := range []int{12, 10} {
			hand := deck[:size]
			discard := size - 10
			for _, game := range []string{gameBetl, gameDurch} {
				var j contractHand
				if game == gameBetl {
					j = judgeBetl(hand, discard)
				} else {
					j = judgeDurch(hand, discard)
				}
				if j.odds < 0.2 {
					continue
				}
				keep := removeCards(append([]string(nil), hand...), j.discard...)
				odds := contractOdds(game, keep, j.discard, r, 30, 200_000)
				add(fmt.Sprintf("%s/%d/est%.1f", game, size, math.Floor(j.odds*10)/10), odds)
			}
		}
	}
	var keys []string
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := buckets[k]
		t.Logf("%-28s %6d hands (%.2f%% of deals)  made %.0f%%", k, b.n, 100*float64(b.n)/float64(deals), 100*b.odds/float64(b.n))
	}
}

// TestContractBars sets each betl and durch the chooser could have
// announced beside the game the bot would have played instead: the deal is
// played out both ways from the announcement, every seat the same skill,
// and the declarer's units compared, bucketed by what the rules of thumb
// made of the hand. It is what the bars in contracts.go were set from. A
// measurement:
//
//	MARIAS_CONTRACT_BARS=300 go test ./internal/marias -run ContractBars -v
//
// MARIAS_CONTRACT_BARS_DUMP names a file to write every hand to, with both
// results, for judging other rules of thumb against the same deals.
func TestContractBars(t *testing.T) {
	seeds, _ := strconv.Atoi(os.Getenv("MARIAS_CONTRACT_BARS"))
	if seeds <= 0 {
		t.Skip("a measurement: set MARIAS_CONTRACT_BARS to the matches")
	}
	var dump *os.File
	if path := os.Getenv("MARIAS_CONTRACT_BARS_DUMP"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		dump = f
	}
	m := New()
	// playOut plays the deal in state out to its settlement, the first
	// action given, and returns the declarer's units from it.
	playOut := func(state module.State, first module.Action, skill module.Skill, seed int64) (int, bool) {
		s, _ := decode(state)
		actor := s.Current
		rounds := len(s.Rounds)
		a := first
		for step := 0; step < 400; step++ {
			next, _, err := m.Apply(state, actor, a)
			if err != nil {
				t.Fatalf("%s/%s: %v", a.Verb, a.OfferID, err)
			}
			state = next
			s, _ = decode(state)
			if len(s.Rounds) > rounds {
				rec := s.Rounds[len(s.Rounds)-1]
				return rec.Deltas[rec.Declarer], rec.Parts[0].ForDeclarer
			}
			actor = s.Current
			offers, err := m.LegalActions(state, actor)
			if err != nil {
				t.Fatal(err)
			}
			var ok bool
			if s.Phase == phaseAnswer {
				// The bars are the chooser's; nobody takes the game over.
				a = module.Action{OfferID: OfferGood, Verb: VerbGood}
				continue
			}
			if a, ok = (bot{}).Act(state, module.BotSeat{PlayerID: actor, Skill: skill, Seed: seed}, offers); !ok {
				t.Fatal("no move")
			}
		}
		t.Fatal("the deal did not settle")
		return 0, false
	}
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		type bucket struct{ n, made, units, alt int }
		buckets := map[string]*bucket{}
		for seed := int64(1); seed <= int64(seeds); seed++ {
			cfg := module.MatchConfig{Variation: variationVoleny, Options: module.Options{OptDeals: 9, module.OptPauseBetweenRounds: module.OptOff}}
			state, err := m.NewMatch(cfg, players, seed)
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 4000; step++ {
				s, _ := decode(state)
				if s.Status != "active" {
					break
				}
				actor := s.Current
				offers, err := m.LegalActions(state, actor)
				if err != nil {
					t.Fatal(err)
				}
				// Medium seats deal the match along; the deals compared are
				// played at the skill measured.
				a, ok := (bot{classicContracts: true}).Act(state, module.BotSeat{PlayerID: actor, Skill: module.SkillMedium, Seed: seed}, offers)
				if !ok {
					t.Fatal("no move")
				}
				if s.Phase == phaseAnnounce {
					hand := s.Hands[actor]
					game, key, j := "", "", contractHand{}
					if d := judgeDurch(hand, 2); d.odds >= durchOneLoser {
						game, j, a.OfferID = gameDurch, d, OfferDurch
						key = fmt.Sprintf("durch est%.1f", d.odds)
					} else if b := judgeBetl(hand, 2); b.odds >= 0.25 {
						game, j = gameBetl, b
						key = fmt.Sprintf("betl est%.1f", math.Floor(b.odds*10)/10)
					}
					if game != "" {
						id := OfferBetl
						if game == gameDurch {
							id = OfferDurch
						}
						alt, _ := (bot{classicContracts: true}).Act(state, module.BotSeat{PlayerID: actor, Skill: skill, Seed: seed}, offers)
						units, made := playOut(state, module.Action{OfferID: id, Verb: VerbAnnounce}, skill, seed)
						altUnits, _ := playOut(state, alt, skill, seed)
						b := buckets[key]
						if b == nil {
							b = &bucket{}
							buckets[key] = b
						}
						b.n++
						b.units += units
						b.alt += altUnits
						if made {
							b.made++
						}
						if dump != nil {
							keep := removeCards(append([]string(nil), hand...), j.discard...)
							fmt.Fprintf(dump, "%s %s %v %v %v %d %d\n", skill, game, keep, j.discard, made, units, altUnits)
						}
					}
				}
				if state, _, err = m.Apply(state, actor, a); err != nil {
					t.Fatalf("seed %d: %s/%s: %v", seed, a.Verb, a.OfferID, err)
				}
			}
		}
		var keys []string
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b := buckets[k]
			t.Logf("%-6s %-16s %4d played, made %3.0f%%, %+6.2f units a deal against %+6.2f for the game it replaced",
				skill, k, b.n, 100*float64(b.made)/float64(b.n), float64(b.units)/float64(b.n), float64(b.alt)/float64(b.n))
		}
	}
}
