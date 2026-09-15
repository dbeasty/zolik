package prsi

import (
	"testing"

	"zolik/server/internal/module"
)

// botAct asks the module's own bot what it would do, through exactly the path
// the runtime uses: the offer list from LegalActions and nothing handed in
// beside it.
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

// --- the wild ----------------------------------------------------------------

// TestBotKeepsTheQueenWhileAnythingElseFits.
//
// The queen plays on anything, which makes it the hand's answer to a suit it
// cannot otherwise follow — and therefore the last card to spend, not the
// first. Here the nine of spades follows the pile perfectly well.
func TestBotKeepsTheQueenWhileAnythingElseFits(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"9S"}
		s.Hands["p1"] = []string{"QH", "9D", "KS", "8C"}
	})
	for _, skill := range []module.Skill{module.SkillMedium, module.SkillHard} {
		got := botAct(t, raw, "p1", skill)
		if got.Verb != VerbPlay {
			t.Fatalf("%s: %s, want a card played", skill, got.Verb)
		}
		if rankOf(got.Cards[0]) == rankWild {
			t.Errorf("%s played its queen with three ordinary cards that fit", skill)
		}
	}
}

// TestBotPlaysTheQueenWhenNothingElseFits, and names the suit it is longest
// in — not the first suit in the list, which is what module.defaultParam
// picked and which was hearts whatever the hand held.
func TestBotPlaysTheQueenWhenNothingElseFits(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"9S"}
		// Nothing follows a nine of spades except the queen: no spades, no
		// nines. Three clubs behind it, which is the suit to switch play to.
		s.Hands["p1"] = []string{"QH", "8C", "TC", "KC"}
	})
	got := botAct(t, raw, "p1", module.SkillHard)
	if got.Verb != VerbPlay || rankOf(got.Cards[0]) != rankWild {
		t.Fatalf("played %v, want the queen — nothing else fits", got)
	}
	if got.Params["suit"] != "C" {
		t.Errorf("named %q holding three clubs and no hearts", got.Params["suit"])
	}
}

// --- the attack cards --------------------------------------------------------

// TestHardSavesTheSevenUntilItBites.
//
// A seven makes the next player draw two. Played while they are holding six
// cards it is worth almost nothing; held until they are holding two it is most
// of the game. The old bot played it first every time, because "7" sorts ahead
// of every other rank in this deck.
func TestHardSavesTheSevenUntilItBites(t *testing.T) {
	hand := []string{"7S", "9S", "KS"}
	early := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"8S"}
		s.Hands["p1"] = hand
		s.Hands["p2"] = []string{"7C", "AD", "8H", "9H", "TH", "JH"}
	})
	late := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"8S"}
		s.Hands["p1"] = hand
		s.Hands["p2"] = []string{"7C", "AD"}
	})

	if got := botAct(t, early, "p1", module.SkillHard); rankOf(got.Cards[0]) == rankDrawTwo {
		t.Errorf("spent the seven on an opponent holding six cards")
	}
	if got := botAct(t, late, "p1", module.SkillHard); rankOf(got.Cards[0]) != rankDrawTwo {
		t.Errorf("played %v against an opponent holding two cards, want the seven", got.Cards)
	}
}

// TestBotAnswersAPendingDrawWithASeven at every strength: passing the
// obligation on costs one card and taking it costs two, so there is no skill
// setting at which picking up is the better move.
func TestBotAnswersAPendingDrawWithASeven(t *testing.T) {
	raw := withState(t, func(s *GameState) {
		s.DiscardPile = []string{"7H"}
		s.PendingDraw = 2
		s.Hands["p1"] = []string{"7S", "9D", "KC"}
	})
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		got := botAct(t, raw, "p1", skill)
		if got.Verb != VerbPlay || rankOf(got.Cards[0]) != rankDrawTwo {
			t.Errorf("%s answered two cards owed with %v, want the seven", skill, got)
		}
	}
}

// --- the disciplines ---------------------------------------------------------

// TestBotDoesNotPeek.
//
// The bot is handed the whole state — every seat's hand — because that is what
// the runtime has, and it reads the next player's hand *size* to time an
// attack card. Nothing but this test stops it reading the cards in it.
func TestBotDoesNotPeek(t *testing.T) {
	build := func(theirs []string) module.State {
		return withState(t, func(s *GameState) {
			s.DiscardPile = []string{"9S"}
			s.Hands["p1"] = []string{"7S", "9D", "9C", "TC", "KC"}
			s.Hands["p2"] = theirs
		})
	}
	honest := botAct(t, build([]string{"8H", "8C", "TD"}), "p1", module.SkillHard)
	rigged := botAct(t, build([]string{"QH", "QS", "7D"}), "p1", module.SkillHard)
	if honest.Verb != rigged.Verb || honest.Cards[0] != rigged.Cards[0] ||
		honest.Params["suit"] != rigged.Params["suit"] {
		t.Fatalf("played %v against one hand and %v against another of the same size", honest, rigged)
	}
}

// TestBotPlaysWholeGamesLegally.
//
// Every action goes through Apply, so an illegal one fails here rather than
// being quietly swapped for a legal one by the runtime's recovery path. The
// suit parameter is the one thing this bot can get wrong that the offer list
// would not have caught, so it is worth playing whole games for.
func TestBotPlaysWholeGamesLegally(t *testing.T) {
	m := New()
	for _, seats := range []int{2, 3, 4} {
		ids := make([]string, 0, seats)
		for i := 1; i <= seats; i++ {
			ids = append(ids, "p"+string(rune('0'+i)))
		}
		players := make([]module.PlayerRef, 0, len(ids))
		for _, id := range ids {
			players = append(players, module.PlayerRef{ID: id})
		}
		for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
			for seed := int64(1); seed <= 5; seed++ {
				state, err := m.NewMatch(module.MatchConfig{}, players, seed)
				if err != nil {
					t.Fatalf("NewMatch: %v", err)
				}
				finished := false
				for step := 0; step < 4000 && !finished; step++ {
					s, err := decode(state)
					if err != nil {
						t.Fatalf("decode: %v", err)
					}
					if s.Status != "active" {
						finished = true
						break
					}
					actor := module.ActiveSeat(m, state, players[0].ID, players)
					if actor == "" {
						finished = true
						break
					}
					offers, err := m.LegalActions(state, actor)
					if err != nil {
						t.Fatalf("LegalActions(%s): %v", actor, err)
					}
					a, ok := m.Bot().Act(state, module.BotSeat{PlayerID: actor, Skill: skill}, offers)
					if !ok {
						t.Fatalf("%d seats %s seed %d: %s had no move", seats, skill, seed, actor)
					}
					next, _, err := m.Apply(state, actor, a)
					if err != nil {
						t.Fatalf("%d seats %s seed %d: %s proposed an illegal %v: %v",
							seats, skill, seed, actor, a, err)
					}
					state = next
				}
				if !finished {
					t.Errorf("%d seats %s seed %d: game did not finish in 4000 actions", seats, skill, seed)
				}
			}
		}
	}
}

// TestBotLadderIsOrdered is the test the skill dial rests on.
//
// Every other test here says a profile *does* something. This one says the
// order is the order — that the settings are strengths and not merely
// different habits — by playing them against each other over a fixed seed
// sweep and counting who runs out of cards first.
//
// Adjacent pairs only: a ladder breaks by neighbours swapping, and hard
// against easy would be an easier test and a weaker one. Both seats play the
// same sweep from both sides of the table, because whoever leads a deal of
// Prší has a real advantage and a one-sided sweep would measure that instead.
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
		for seed := int64(1); seed <= 120; seed++ {
			for _, swap := range []bool{false, true} {
				seats := map[string]module.Skill{"p1": strong, "p2": weak}
				if swap {
					seats = map[string]module.Skill{"p1": weak, "p2": strong}
				}
				winner := playPrsi(t, seed, seats)
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
			t.Fatalf("%s vs %s: no game finished — the sweep measured nothing", strong, weak)
		}
		rate := float64(wins) / float64(games)
		t.Logf("%s vs %s: %d of %d (%.1f%%)", strong, weak, wins, games, 100*rate)
		// Deliberately loose, and the looseness is measured rather than
		// guessed. Prší is a high-variance game: TestSweepKnobs priced the
		// whole difference between the strongest profile and a bot with no
		// judgement at all at two or three points, against a standard error of
		// about one on a sweep this size. So a level sweep is not evidence of
		// a broken ladder and a required margin would be a coin flip in a test
		// suite. What is evidence is a sweep that comes out backwards by more
		// than the noise, and that is what this bar is.
		if rate < 0.46 {
			t.Errorf("%s won %.1f%% of %d against %s — the ladder is upside down here",
				strong, 100*rate, games, weak)
		}
	}
}

// playPrsi runs one game to its winner and returns them, or "" if it did not
// finish inside the action budget.
func playPrsi(t *testing.T, seed int64, seats map[string]module.Skill) string {
	t.Helper()
	m := New()
	players := []module.PlayerRef{{ID: "p1"}, {ID: "p2"}}
	state, err := m.NewMatch(module.MatchConfig{}, players, seed)
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
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return s.WinnerID
		}
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

// TestUnknownSkillPlaysMedium is module.BotSeat's compatibility rule.
func TestUnknownSkillPlaysMedium(t *testing.T) {
	for _, sk := range []module.Skill{"", "nonsense"} {
		if got := profileFor(sk); got.skill != module.SkillMedium {
			t.Errorf("profileFor(%q) = %q, want medium", sk, got.skill)
		}
	}
}
