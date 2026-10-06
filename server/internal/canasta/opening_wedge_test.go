package canasta

import (
	"fmt"
	"testing"

	"zolik/server/internal/module"
)

// The opening that wedged a Hard-vs-Hard bench, and the whole-match replays
// that pin it. See opening.go for the fault and the fix.

// The turn itself, from seed 1000007 at step 1100.
//
// A side on 3035 needs 120 and has nothing down. The pile is a single ten and
// the hand holds four more tens, three jacks, three kings and a deuce. The
// capture looks like an opening: 30 for the tens it melds, and the 100 left in
// hand after it — two tens, the jacks, the kings, the deuce — makes 130. But a
// side that cannot go out has to finish the turn holding two cards, one to
// discard and one to keep (checkLeavesPlayable), so only seven of those nine
// cards can go down. The best seven are worth 80, and 30 + 80 is 110: ten
// short, and nothing in the engine's bound (reachableValue) knows about the
// two cards.
//
// The old bot captured, laid off both spare tens, melded the jacks and laid the
// deuce off: 100, holding three kings it could not lay without emptying the
// hand. Every enabled offer after that was an undo.
//
// So the turn must end in a discard, having either opened or laid nothing at
// all. Here that means drawing rather than taking the pile.
func TestBotDoesNotCaptureIntoAnOpeningItCannotFinish(t *testing.T) {
	hand := []string{"KS", "KC", "JH", "TC", "JC", "TH", "TD", "TC", "KD", "2D", "JS"}
	played, s := playOpeningTurn(t, hand)
	if played[0].Verb == VerbTakePile {
		t.Errorf("took a pile no opening could follow: %v", played)
	}
	if len(s.Teams[0].Melds) > 0 {
		t.Errorf("the turn put melds down without opening: %v", played)
	}
}

// And an opening that is all there: the capture is 30, and kings, jacks and
// queens are 90 more, which leaves two cards to discard one of. A side that has
// not opened may not lay off, so the melds alone have to carry it. The guard is
// not "never capture before opening"; it is "never capture into an opening that
// is not all there", and this one is.
func TestBotCapturesIntoAnOpeningItCanFinish(t *testing.T) {
	hand := []string{"TC", "TH", "KS", "KC", "KD", "JH", "JC", "JS", "QS", "QC", "QD", "9C", "5S"}
	played, s := playOpeningTurn(t, hand)
	if played[0].Verb != VerbTakePile {
		t.Errorf("passed up a pile that opens the account: %v", played)
	}
	if !s.Teams[0].HasMelded {
		t.Errorf("the turn ended without opening: %v", played)
	}
}

// playOpeningTurn plays p1's whole turn with the Hard bot from that position —
// unopened on 3035, so needing 120, with a ten on the pile — and fails if the
// bot is ever left without a move or the turn ends any way but a discard.
func playOpeningTurn(t *testing.T, hand []string) ([]module.Action, *GameState) {
	t.Helper()
	m := New()
	raw := twoHanded(func(s *GameState) {
		s.Teams[0].Score, s.Teams[1].Score = 3035, 3955
		s.Hands["p1"] = append([]string(nil), hand...)
		s.DiscardPile = []string{"TD"}
	})

	var played []module.Action
	for step := 0; step < 20; step++ {
		s, err := decode(raw)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if s.Current != "p1" {
			break
		}
		offers, err := m.LegalActions(raw, "p1")
		if err != nil {
			t.Fatalf("LegalActions: %v", err)
		}
		a, ok := m.Bot().Act(raw, module.BotSeat{PlayerID: "p1", Skill: module.SkillHard}, offers)
		if !ok {
			t.Fatalf("after %v the bot has no move: hand %v, laid %d, offers %v",
				played, s.Hands["p1"], s.LaidThisTurn, enabledVerbs(offers))
		}
		next, _, err := m.Apply(raw, "p1", a)
		if err != nil {
			t.Fatalf("after %v the bot proposed an illegal %+v: %v", played, a, err)
		}
		played = append(played, a)
		raw = next
	}

	s, err := decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Current == "p1" {
		t.Fatalf("the turn never ended: %v", played)
	}
	if last := played[len(played)-1]; last.Verb != VerbDiscard {
		t.Errorf("the turn ended on %s, want a discard: %v", last.Verb, played)
	}
	return played, s
}

// The three matches the bench stalled on, played to the end. selfPlay has no
// fallback, so a turn the bot declines — the only thing left in the wedge was
// undos, which it will not touch — is a failure here rather than an
// oscillation that runs down a budget.
func TestSelfPlayOpensWithoutWedging(t *testing.T) {
	m := New()
	for _, seed := range []int64{1000007, 1000011, 1000024} {
		if n, stall := selfPlay(m, "classic", 2, seed, module.SkillHard, 20000); stall != "" {
			t.Errorf("classic, 2 seats, hard, seed %d: step %d: %s", seed, n, stall)
		}
	}
}

// The same over a spread of seeds, every table shape, every strength. Slow —
// a few thousand whole matches at lobby rules — so it stays out of -short.
func TestSelfPlaySweepNeverStalls(t *testing.T) {
	if testing.Short() {
		t.Skip("plays thousands of whole matches")
	}
	m := New()
	for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
		for _, seats := range []int{2, 4} {
			for _, variation := range []string{"classic", "samba"} {
				skill, seats, variation := skill, seats, variation
				t.Run(fmt.Sprintf("%s/%d/%s", skill, seats, variation), func(t *testing.T) {
					t.Parallel()
					for seed := int64(1); seed <= 200; seed++ {
						if n, stall := selfPlay(m, variation, seats, seed, skill, 20000); stall != "" {
							t.Errorf("seed %d: step %d: %s", seed, n, stall)
						}
					}
				})
			}
		}
	}
}

// benchPlayers are the seats a bench seats: p0, p1, ...
func benchPlayers(n int) []module.PlayerRef {
	out := make([]module.PlayerRef, n)
	for i := range out {
		id := fmt.Sprintf("p%d", i)
		out[i] = module.PlayerRef{ID: id, Name: id, IsAI: true}
	}
	return out
}

// selfPlay plays a lobby-rules match to its end with this bot in every seat,
// seeded the way the runtime seeds a bot seat. It reports how the match
// stopped if it did not finish: the step, and why — the bot found no move,
// took a move back, proposed one the engine refused, or ran out of budget.
//
// No fallback. A driver that answers a declined turn from the offer list is
// how the wedge became an oscillation, because the only enabled offers left in
// it are undos; so a declined turn is the failure being looked for.
func selfPlay(m *Module, variation string, seats int, seed int64, skill module.Skill, budget int) (actions int, stall string) {
	players := benchPlayers(seats)
	state, err := m.NewMatch(module.MatchConfig{Variation: variation}, players, seed)
	if err != nil {
		return 0, "NewMatch: " + err.Error()
	}
	for step := 0; step < budget; step++ {
		if done, _, err := m.Finished(state); err != nil {
			return step, "Finished: " + err.Error()
		} else if done {
			return step, ""
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return step, "nobody on turn"
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			return step, "LegalActions: " + err.Error()
		}
		seat := module.BotSeat{PlayerID: actor, Skill: skill, Seed: module.SeatSeed(seed, actor, "bot")}
		a, ok := m.Bot().Act(state, seat, offers)
		if !ok {
			s, _ := decode(state)
			return step, fmt.Sprintf("no move for %s: hand %v, laidThisTurn %d, offers %v",
				actor, s.Hands[actor], s.LaidThisTurn, enabledVerbs(offers))
		}
		if a.Verb == VerbUndoTakePile || a.Verb == VerbUndoLayOff || a.Verb == VerbUndoLayMeld {
			return step, fmt.Sprintf("%s took a move back: %+v", actor, a)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			return step, fmt.Sprintf("%s proposed an illegal %+v: %v", actor, a, err)
		}
		state = next
	}
	return budget, "action budget"
}

func enabledVerbs(offers []module.ActionOffer) []string {
	var out []string
	for _, o := range offers {
		if o.Enabled {
			out = append(out, o.Verb)
		}
	}
	return out
}
