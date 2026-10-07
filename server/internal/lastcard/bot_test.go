package lastcard

import (
	"testing"

	"zolik/server/internal/module"
)

// botAct asks the module's own bot what it would do, through the path the
// runtime uses: the offer list and nothing handed in beside it.
func botAct(t *testing.T, raw module.State, playerID string, skill module.Skill) module.Action {
	t.Helper()
	m := New()
	offers, err := m.LegalActions(raw, playerID)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: playerID, Skill: skill}, offers)
	if !ok {
		t.Fatalf("bot had no move for %s", playerID)
	}
	return a
}

func TestBotKeepsTheWildWhileAColouredCardFits(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{cardWild, "T-9", "A-7"}
	})
	for _, sk := range []module.Skill{module.SkillMedium, module.SkillHard} {
		if got := botAct(t, raw, "p1", sk); got.Cards[0] != "T-9" {
			t.Errorf("%s played %v with the teal 9 to follow", sk, got.Cards)
		}
	}
}

func TestBotNamesTheColourItHoldsMost(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{cardWild, "A-7", "A-8", "V-1"}
	})
	got := botAct(t, raw, "p1", module.SkillEasy)
	if got.Cards[0] != cardWild || got.Params["colour"] != "A" {
		t.Errorf("played %v naming %q, want the wild naming amber", got.Cards, got.Params["colour"])
	}
}

func TestHardBotPlaysTheActionCardFirst(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{"T-9", "T-D", "A-7"}
	})
	if got := botAct(t, raw, "p1", module.SkillHard); got.Cards[0] != "T-D" {
		t.Errorf("hard played %v, want the Draw Two", got.Cards)
	}
	if got := botAct(t, raw, "p1", module.SkillMedium); got.Cards[0] != "T-9" {
		t.Errorf("medium played %v, want the first plain card", got.Cards)
	}
}

func TestEasyBotSpendsItsWildsFirst(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.Hands["p1"] = []string{cardWild, "T-9", "A-7"}
	})
	if got := botAct(t, raw, "p1", module.SkillEasy); got.Cards[0] != cardWild {
		t.Errorf("easy played %v, want the wild", got.Cards)
	}
}

// The bot must decide on what a player at the table could know. Changing
// what an opponent holds — keeping its size — must not change the move.
func TestBotDoesNotPeek(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		raw := newGame(t, seed, "p1", "p2", "p3")
		s := stateOf(t, raw)
		actor := s.Current
		base := botAct(t, raw, actor, module.SkillHard)

		for _, other := range s.TurnOrder {
			if other == actor {
				continue
			}
			s2 := stateOf(t, raw)
			n := len(s2.Hands[other])
			swapped := make([]string, n)
			for i := range swapped {
				swapped[i] = cardWildDrawFour
			}
			s2.Hands[other] = swapped
			raw2, _ := encode(s2)
			got := botAct(t, raw2, actor, module.SkillHard)
			if got.Verb != base.Verb || len(got.Cards) != len(base.Cards) ||
				(len(got.Cards) > 0 && got.Cards[0] != base.Cards[0]) {
				t.Fatalf("seed %d: %s changed its move from %v to %v when %s's hidden cards changed",
					seed, actor, base, got, other)
			}
		}
	}
}

// TestBotLadderIsOrdered: the settings are strengths, not merely habits.
// Adjacent pairs, both seats, over a fixed seed sweep.
func TestBotLadderIsOrdered(t *testing.T) {
	if testing.Short() {
		t.Skip("a strength sweep is not a fast test")
	}
	for _, pair := range [][2]module.Skill{
		{module.SkillHard, module.SkillMedium},
		{module.SkillMedium, module.SkillEasy},
		{module.SkillHard, module.SkillEasy},
	} {
		strong, weak := pair[0], pair[1]
		wins, games := 0, 0
		for seed := int64(1); seed <= 600; seed++ {
			for _, swap := range []bool{false, true} {
				seats := map[string]module.Skill{"p1": strong, "p2": weak}
				if swap {
					seats = map[string]module.Skill{"p1": weak, "p2": strong}
				}
				winner := playOut(t, seed, seats)
				if winner == "" {
					continue
				}
				games++
				if seats[winner] == strong {
					wins++
				}
			}
		}
		if games == 0 {
			t.Fatalf("%s vs %s: no game finished", strong, weak)
		}
		rate := float64(wins) / float64(games)
		t.Logf("%s vs %s: %d of %d (%.1f%%)", strong, weak, wins, games, 100*rate)
		// Each rung measured 5-7 points over the one below (bot.go), against
		// a standard error of about 1.4 on 1 200 games; this bar fails only
		// on a rung that has gone level or backwards.
		if rate < 0.52 {
			t.Errorf("%s won %.1f%% of %d against %s — the ladder is upside down here",
				strong, 100*rate, games, weak)
		}
	}
}

// playOut runs one game to its winner, or "" if it did not finish.
func playOut(t *testing.T, seed int64, seats map[string]module.Skill) string {
	t.Helper()
	m := New()
	ps := []module.PlayerRef{{ID: "p1"}, {ID: "p2"}}
	state, err := m.NewMatch(module.MatchConfig{}, ps, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	for step := 0; step < 4000; step++ {
		s, err := decode(state)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.Status != "active" {
			return s.WinnerID
		}
		actor := s.Current
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions(%s): %v", actor, err)
		}
		a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: seats[actor]}, offers)
		if !ok {
			t.Fatalf("seed %d: %s had no move", seed, actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("seed %d: %s proposed an illegal %v: %v", seed, actor, a, err)
		}
		state = next
	}
	return ""
}

func TestUnknownSkillPlaysMedium(t *testing.T) {
	for _, sk := range []module.Skill{"", "nonsense"} {
		if got := profileFor(sk); got.skill != module.SkillMedium {
			t.Errorf("profileFor(%q) = %q, want medium", sk, got.skill)
		}
	}
}
