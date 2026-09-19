package match

import (
	"fmt"
	"testing"

	"zolik/server/internal/canasta"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// A table of bots always finishes the deal it is in the middle of.
//
// The loop itself needs Mongo, but the part of it that decides what to send —
// botCandidates, the module's own bot, botTurn's replay guard, and the stall
// counter that gives up — is all here, and it is where a table gets stuck. So
// this drives exactly that, against the real engine, and asserts the two
// outcomes that leave a match frozen for everybody at it:
//
//   - a dead end: no candidate the offer list describes is accepted, which is
//     where logOfferState says "all N candidate moves refused; stopping";
//   - a stall: botMaxStall actions by one seat without the turn moving on,
//     which is where it says "made no progress".
//
// Canasta, because that is the game that had both. Samba especially: its pile
// is frozen all deal, so a capture always costs two cards out of a hand that
// has not opened yet, which is the turn that used to have nowhere to go. Game
// 6aaa157d0079d0b3a6624b3a stalled here 49 times in 240 before the fix.
func TestABotTableNeverFreezes(t *testing.T) {
	if testing.Short() {
		t.Skip("a seed sweep is not a fast test")
	}
	mod := canasta.New()
	for _, variation := range []string{"samba", "classic"} {
		for _, seats := range []int{2, 4} {
			for _, skill := range []module.Skill{module.SkillEasy, module.SkillMedium, module.SkillHard} {
				for seed := int64(1); seed <= 5; seed++ {
					if why := playLikeTheBotLoop(t, mod, variation, seats, skill, seed, 5000); why != "" {
						t.Errorf("%s, %d seats, %s, seed %d: %s", variation, seats, skill, seed, why)
					}
				}
			}
		}
	}

	// And a few the long way, because the floor is banded and the sweep above
	// only ever reaches the third band. Samba adds a fourth at 7000 — a hundred
	// and fifty to open — which is the band game 6aaa157d0079d0b3a6624b3a was
	// in, and the one where an opening is hardest to reach and so likeliest to
	// strand a turn.
	//
	// Coverage rather than a reproduction: these four did not wedge on the
	// unfixed engine, where five of the sixty above did. They are here so the
	// band is exercised at all, since a ten-thousand-point match is twenty-odd
	// deals and sampling it properly would cost more than this whole file.
	for _, seats := range []int{2, 4} {
		for seed := int64(1); seed <= 2; seed++ {
			if why := playLikeTheBotLoop(t, mod, "samba", seats, module.SkillMedium, seed, 10000); why != "" {
				t.Errorf("samba to 10000, %d seats, seed %d: %s", seats, seed, why)
			}
		}
	}
}

// playLikeTheBotLoop is botLoop with the database and the thinking pause taken
// out, and the give-up paths turned into a description instead of a log line.
func playLikeTheBotLoop(t *testing.T, mod module.GameModule, variation string, seats int, skill module.Skill, seed int64, target int) string {
	t.Helper()
	players := make([]module.PlayerRef, 0, seats)
	for i := 1; i <= seats; i++ {
		players = append(players, module.PlayerRef{ID: fmt.Sprintf("p%d", i), IsAI: true})
	}
	state, err := mod.NewMatch(module.MatchConfig{
		Variation: variation,
		Options: module.Options{
			"targetScore":                target,
			module.OptPauseBetweenRounds: module.OptOff,
		},
	}, players, seed)
	if err != nil {
		t.Fatalf("NewMatch(%s): %v", variation, err)
	}

	lastActor, stall := "", 0
	var turn botTurn
	for step := 0; step < 20000; step++ {
		if done, _, err := mod.Finished(state); err != nil {
			t.Fatalf("Finished: %v", err)
		} else if done {
			return ""
		}
		actor := firstBot(module.AwaitedSeats(mod, state, players[0].ID, players), aiPlayers(players))
		if actor == "" {
			return ""
		}
		if actor == lastActor {
			stall++
			if stall > botMaxStall {
				return fmt.Sprintf("seat %s made no progress in %d actions", actor, stall)
			}
		} else {
			lastActor, stall = actor, 0
			turn = botTurn{}
		}

		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions(%s): %v", actor, err)
		}
		candidates := botCandidates(offers)
		if a, ok := module.BotFor(mod).Act(state, module.BotSeat{PlayerID: actor, Skill: skill}, offers); ok {
			candidates = append([]botMove{{action: a, undo: isUndoIn(offers, a)}}, candidates...)
		}

		played := false
		for i, c := range candidates {
			if i == 1 && sameAction(c.action, candidates[0].action) {
				continue
			}
			if turn.skips(c) {
				continue
			}
			next, _, err := mod.Apply(state, actor, c.action)
			if err != nil {
				continue
			}
			turn.note(c)
			state, played = next, true
			break
		}
		if !played {
			return fmt.Sprintf("seat %s had %d candidate moves and the engine refused every one",
				actor, len(candidates))
		}
	}
	return "the match did not finish in 20000 actions"
}

// aiPlayers is the seat list firstBot reads — every seat here is a bot's.
func aiPlayers(refs []module.PlayerRef) []models.Player {
	out := make([]models.Player, 0, len(refs))
	for _, r := range refs {
		out = append(out, models.Player{ID: r.ID, Name: r.ID, IsAI: true})
	}
	return out
}
