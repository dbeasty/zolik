package marias

import (
	"reflect"
	"testing"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

func botAct(t *testing.T, s *GameState, who string, skill module.Skill) module.Action {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	offers, err := New().LegalActions(raw, who)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := New().Bot().Act(raw, module.BotSeat{PlayerID: who, Skill: skill, Seed: 3}, offers)
	if !ok {
		t.Fatal("the bot had no move")
	}
	return a
}

// playMatch runs a whole match between bots, every action through Apply so
// an illegal one fails here rather than being swapped for a legal one by the
// runtime's recovery path. It returns the final units.
func playMatch(t *testing.T, seed int64, seats map[string]module.Skill) map[string]int {
	t.Helper()
	m := New()
	cfg := module.MatchConfig{Options: module.Options{OptDeals: 9, module.OptPauseBetweenRounds: module.OptOff}}
	state, err := m.NewMatch(cfg, players, seed)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 4000; step++ {
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
			t.Fatalf("seed %d: %s had no move in phase %s", seed, actor, s.Phase)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %s/%s %v in phase %s: %v", seed, actor, a.Verb, a.OfferID, a.Cards, s.Phase, err)
		}
		state = next
	}
	t.Fatalf("seed %d: the match did not finish", seed)
	return nil
}

func TestBotPlaysWholeMatchesLegally(t *testing.T) {
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		for seed := int64(1); seed <= 15; seed++ {
			playMatch(t, seed, map[string]module.Skill{"p1": skill, "p2": skill, "p3": skill})
		}
	}
}

// TestBotDoesNotPeek: the same seat, the same hand and table, and two
// different hidden worlds — the other hands and the talon — must get the
// same move.
func TestBotDoesNotPeek(t *testing.T) {
	build := func(p2, p3, talon []string) *GameState {
		s := playState(t, map[string][]string{
			"p1": {"AH", "TH", "KH", "QC", "KC", "7S"},
			"p2": p2, "p3": p3,
		})
		s.Talon = talon
		return s
	}
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		honest := botAct(t, build([]string{"7H", "8C", "9C", "TC", "AC", "8S"}, []string{"8H", "9H", "JC", "AS", "TS", "KS"}, []string{"7D", "8D"}), "p1", skill)
		rigged := botAct(t, build([]string{"QH", "JH", "AS", "TS", "7D", "8D"}, []string{"9H", "8H", "7C", "8C", "KS", "QS"}, []string{"AD", "TD"}), "p1", skill)
		if !reflect.DeepEqual(honest, rigged) {
			t.Errorf("%s led %v against one deal and %v against another", skill, honest.Cards, rigged.Cards)
		}
	}
}

func TestBotAnnouncesTheSevenItHolds(t *testing.T) {
	hand := []string{"7H", "8H", "9H", "AH", "TH", "KC", "QS", "8C", "9S", "8S", "JD", "7D"}
	if got := announcement(hand, 'H', module.SkillMedium); got != OfferHraSedma {
		t.Fatalf("five trumps and the seven: announced %s", got)
	}
	if got := announcement(hand, 'H', module.SkillEasy); got != OfferHra {
		t.Fatalf("an easy seat announced %s", got)
	}
}

func TestBotSmearsPointsOnItsPartnersTrick(t *testing.T) {
	// p2, a defender, has led the ace of clubs and the declarer followed low.
	// p3 — the other defender, with no clubs and no trumps — plays last to a
	// trick its side has won, and should put its ten on it.
	s := playState(t, map[string][]string{
		"p1": {"9D"}, "p2": {"JD"}, "p3": {"7S", "TS", "8S"},
	})
	s.Trick = []tricks.Play{{Seat: 1, Card: "AC"}, {Seat: 0, Card: "7C"}}
	s.Current = "p3"
	if a := botAct(t, s, "p3", module.SkillMedium); a.Cards[0] != "TS" {
		t.Fatalf("with the trick its side's, the bot played %v rather than its ten", a.Cards)
	}
	// Not when the declarer is winning it.
	s.Trick = []tricks.Play{{Seat: 1, Card: "7C"}, {Seat: 0, Card: "AC"}}
	if a := botAct(t, s, "p3", module.SkillMedium); a.Cards[0] == "TS" {
		t.Fatal("the bot gave its ten to the declarer's trick")
	}
}

// TestMediumBeatsEasy is a strength check, loose on purpose: one medium seat
// against two easy ones, the medium seat moved round the table, and it must
// come out ahead on average. A backwards result is a broken bot; a narrow
// one is Mariáš.
func TestMediumBeatsEasy(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	total, matches := 0, 0
	for seed := int64(1); seed <= 40; seed++ {
		for _, strong := range []string{"p1", "p2", "p3"} {
			seats := map[string]module.Skill{"p1": module.SkillEasy, "p2": module.SkillEasy, "p3": module.SkillEasy}
			seats[strong] = module.SkillMedium
			total += playMatch(t, seed, seats)[strong]
			matches++
		}
	}
	mean := float64(total) / float64(matches)
	t.Logf("medium against two easy: %+.2f units a match over %d matches", mean, matches)
	if mean <= 0 {
		t.Errorf("medium averaged %+.2f units against easy seats", mean)
	}
}
