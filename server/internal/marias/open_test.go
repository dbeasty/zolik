package marias

import (
	"math/rand"
	"slices"
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// Open betl and durch: the declarer's hand laid face up once the first
// trick has been taken, at a table that plays them so.

// openPosition is p1 declaring game with the hands given, the option as
// open says, and no trick played yet.
func openPosition(t *testing.T, game string, open bool, hands map[string][]string) *GameState {
	t.Helper()
	s := playState(t, hands)
	s.Game, s.OpenBetlDurch = game, open
	return s
}

func betlHands() map[string][]string {
	return map[string][]string{
		"p1": {"7S", "AC", "9H"}, "p2": {"8S", "7C", "KH"}, "p3": {"9S", "8C", "AH"},
	}
}

// shownTo is the cards of owner's hand that viewer is shown, and the zone's
// label.
func shownTo(t *testing.T, s *GameState, viewer, owner string) ([]string, string) {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	vm, err := New().View(raw, viewer)
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range vm.Zones {
		if z.ID == handZoneID(owner) {
			var cards []string
			for _, c := range z.Cards {
				cards = append(cards, c.Card)
			}
			return cards, z.LabelKey
		}
	}
	t.Fatalf("no hand zone for %s in %s's view", owner, viewer)
	return nil, ""
}

// firstTrick plays 7S, 8S, 9S: p3 takes it, and the betl goes on.
func firstTrick(t *testing.T, s *GameState) *GameState {
	t.Helper()
	s = must(t, s, "p1", module.Action{Verb: VerbPlay, Cards: []string{"7S"}})
	s = must(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"8S"}})
	return must(t, s, "p3", module.Action{Verb: VerbPlay, Cards: []string{"9S"}})
}

func TestOpenBetlShowsTheDeclarersHandAfterTheFirstTrick(t *testing.T) {
	s := openPosition(t, gameBetl, true, betlHands())
	s = must(t, s, "p1", module.Action{Verb: VerbPlay, Cards: []string{"7S"}})
	if cards, _ := shownTo(t, s, "p2", "p1"); len(cards) != 0 {
		t.Fatalf("shown %v before the first trick was taken", cards)
	}
	s = must(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"8S"}})
	s = must(t, s, "p3", module.Action{Verb: VerbPlay, Cards: []string{"9S"}})
	if s.Phase != phasePlay {
		t.Fatalf("the betl ended after the first trick: phase %s", s.Phase)
	}

	for _, viewer := range []string{"p2", "p3", ""} {
		cards, label := shownTo(t, s, viewer, "p1")
		if !slices.Equal(cards, s.Hands["p1"]) || label != "zone.shownHand" {
			t.Errorf("viewer %q is shown %v under %s, want %v under zone.shownHand", viewer, cards, label, s.Hands["p1"])
		}
	}
	for _, other := range []string{"p2", "p3"} {
		if cards, _ := shownTo(t, s, "p1", other); len(cards) != 0 {
			t.Errorf("the declarer is shown %s's %v", other, cards)
		}
		if cards, _ := shownTo(t, s, "p2", "p3"); len(cards) != 0 {
			t.Errorf("a defender is shown their partner's %v", cards)
		}
	}
}

func TestClosedOrTrumpGamesShowNothing(t *testing.T) {
	cases := []struct {
		name string
		game string
		open bool
	}{
		{"betl, option off", gameBetl, false},
		{"hra, option on", gameHra, true},
	}
	for _, c := range cases {
		s := firstTrick(t, openPosition(t, c.game, c.open, betlHands()))
		if cards, _ := shownTo(t, s, "p2", "p1"); len(cards) != 0 {
			t.Errorf("%s: shown %v", c.name, cards)
		}
	}
}

func durchHands() map[string][]string {
	return map[string][]string{
		"p1": {"AS", "KS", "AC", "KC"}, "p2": {"7S", "8C", "9H", "TH"}, "p3": {"8S", "9C", "JH", "QH"},
	}
}

// declarerSamples is what twenty samples drawn for p2 deal the declarer.
func declarerSamples(t *testing.T, s *GameState) [][]string {
	t.Helper()
	k := s.infer()
	r := rand.New(rand.NewSource(1))
	var out [][]string
	for i := 0; i < 20; i++ {
		deal, ok := s.sample("p2", k, r)
		if !ok {
			t.Fatal("no consistent sample")
		}
		hand := slices.Clone(deal["p1"])
		slices.Sort(hand)
		out = append(out, hand)
	}
	return out
}

func TestTheSearchReadsAnOpenHand(t *testing.T) {
	durchTrick := func(s *GameState) *GameState {
		s = must(t, s, "p1", module.Action{Verb: VerbPlay, Cards: []string{"AS"}})
		s = must(t, s, "p2", module.Action{Verb: VerbPlay, Cards: []string{"7S"}})
		return must(t, s, "p3", module.Action{Verb: VerbPlay, Cards: []string{"8S"}})
	}

	open := durchTrick(openPosition(t, gameDurch, true, durchHands()))
	want := slices.Clone(open.Hands["p1"])
	slices.Sort(want)
	for _, got := range declarerSamples(t, open) {
		if !slices.Equal(got, want) {
			t.Fatalf("a sample dealt the open declarer %v, want %v", got, want)
		}
	}

	varies := func(name string, s *GameState) {
		samples := declarerSamples(t, s)
		for _, h := range samples[1:] {
			if !slices.Equal(h, samples[0]) {
				return
			}
		}
		t.Errorf("%s: every sample dealt the declarer %v, as though it were shown", name, samples[0])
	}
	varies("before the first trick", openPosition(t, gameDurch, true, durchHands()))
	varies("option off", durchTrick(openPosition(t, gameDurch, false, durchHands())))
}

// TestBotsPlayOpenBetlAndDurch plays betl and durch deals out between bots,
// every card through Apply. Bots seldom declare either game on their own
// (about one deal in five hundred between medium seats), so each deal is a
// real shuffle with the contract forced on the chooser.
func TestBotsPlayOpenBetlAndDurch(t *testing.T) {
	m := New()
	opened := 0
	for _, game := range []string{gameBetl, gameDurch} {
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			seeds := int64(12)
			if skill == module.SkillHard {
				seeds = 3
			}
			for seed := int64(1); seed <= seeds; seed++ {
				raw, err := m.NewMatch(module.MatchConfig{Variation: variationVoleny, Options: module.Options{
					OptOpenBetlDurch: module.OptOn, module.OptPauseBetweenRounds: module.OptOff,
				}}, players, seed)
				if err != nil {
					t.Fatal(err)
				}
				s, _ := decode(raw)
				d := s.chooser()
				hand := append(s.Hands[d], s.Unseen...)
				sortHand(hand)
				s.Hands[d], s.Unseen = hand[:10], nil
				s.Talon = hand[10:]
				s.TrumpCard = hand[0]
				s.Declarer, s.Game = d, game
				s.Phase, s.Current = phasePlay, s.firstLeader()

				state, _ := encode(s)
				for deal := s.Deal; ; {
					s, _ = decode(state)
					if s.Deal != deal || s.Status != "active" {
						break
					}
					if s.declarerShown() && len(s.Trick) == 0 && len(s.History) == 1 {
						opened++
						if cards, _ := shownTo(t, s, s.next(d), d); !slices.Equal(cards, s.Hands[d]) {
							t.Fatalf("%s seed %d: a defender is shown %v of %v", game, seed, cards, s.Hands[d])
						}
					}
					offers, err := m.LegalActions(state, s.Current)
					if err != nil {
						t.Fatal(err)
					}
					a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: s.Current, Skill: skill, Seed: seed}, offers)
					if !ok {
						t.Fatalf("%s seed %d: %s had no move", game, seed, s.Current)
					}
					if state, _, err = m.Apply(state, s.Current, a); err != nil {
						t.Fatalf("%s seed %d: %s proposed an illegal %v: %v", game, seed, s.Current, a.Cards, err)
					}
				}
			}
		}
	}
	t.Logf("%d deals laid open after the first trick", opened)
	if opened < 20 {
		t.Fatalf("only %d deals reached an open hand", opened)
	}
}

func TestTheRuleIsWrittenOnlyWhenOn(t *testing.T) {
	for _, on := range []bool{false, true} {
		secs, err := New().Rules(module.MatchConfig{Options: module.Options{OptOpenBetlDurch: module.BoolOpt(on)}})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, sec := range secs {
			for _, it := range sec.Items {
				found = found || strings.HasSuffix(it.LabelKey, "play.openHand")
			}
		}
		if found != on {
			t.Errorf("option %v: openHand written %v", on, found)
		}
	}
}
