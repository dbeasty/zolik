package learn

import "zolik/server/internal/module"

// turnGuard is match.botLoop's one-way unwinding, for the drivers in this
// package: the environment and the bench.
//
// Both play a hand-written seat by its bot's pick, falling back to the offer
// list in order when the bot declines. An undo is the one fallback that can
// succeed and leave the seat facing the decision it just made, and a bot is a
// pure function of the position, so it makes the same move again. Canasta's
// heuristic does this: it lays a meld that leaves nothing enabled but taking
// it back, declines to move, the fallback's first offer is the undo, and the
// seat lays and lifts the same three cards until the match's action budget
// runs out — fifty thousand actions of a trainer's time and a stalled match,
// dozens of times in a twelve-minute run, and only ever against a learner,
// whose play reaches positions the bots never reach against each other.
//
// The fix is the runtime's own (see the botTurn comment in match/bots.go):
// once a move has been taken back, what this turn has already played is off
// the list, and the seat works down to something it has not done. It arms only
// after an undo, so ordinary play is untouched.
type turnGuard struct {
	actor     string
	played    []module.Action
	unwinding bool
}

// begin starts tracking actor's turn, forgetting the last one if the turn has
// moved on.
func (g *turnGuard) begin(actor string) {
	if actor != g.actor {
		*g = turnGuard{actor: actor}
	}
}

// botCandidate is one thing a seat could send: the bot's own pick, or a
// submission from the offer list.
type botCandidate struct {
	action module.Action
	undo   bool
	bot    bool
}

// candidates is what to try, in order: the bot's pick, then every offer's
// submission, less the moves this turn has made and taken back.
func (g *turnGuard) candidates(s module.State, bot module.Bot, seat module.BotSeat, offers []module.ActionOffer) []botCandidate {
	out := make([]botCandidate, 0, len(offers)+1)
	if bot != nil {
		if a, ok := bot.Act(s, seat, offers); ok {
			out = append(out, botCandidate{action: a, undo: isUndoIn(offers, a), bot: true})
		}
	}
	for i := range offers {
		if a, ok := module.SubmissionFor(offers[i]); ok {
			out = append(out, botCandidate{action: a, undo: offers[i].Undo})
		}
	}
	kept := out[:0]
	for _, c := range out {
		if !g.skips(c) {
			kept = append(kept, c)
		}
	}
	return kept
}

func (g *turnGuard) skips(c botCandidate) bool {
	if !g.unwinding || c.undo {
		return false
	}
	for _, p := range g.played {
		if sameAction(p, c.action) {
			return true
		}
	}
	return false
}

// note records a candidate that was just played.
func (g *turnGuard) note(c botCandidate) {
	if c.undo {
		g.unwinding = true
		return
	}
	g.played = append(g.played, c.action)
}

func isUndoIn(offers []module.ActionOffer, a module.Action) bool {
	for i := range offers {
		if !offers[i].Undo {
			continue
		}
		if b, ok := module.SubmissionFor(offers[i]); ok && sameAction(a, b) {
			return true
		}
	}
	return false
}

// sameAction compares what a submission does, not which offer it came from.
func sameAction(a, b module.Action) bool {
	if a.Verb != b.Verb || a.Target != b.Target || len(a.Cards) != len(b.Cards) {
		return false
	}
	for i := range a.Cards {
		if a.Cards[i] != b.Cards[i] {
			return false
		}
	}
	return true
}

// Stepper is the same one-way unwinding for a driver outside this package —
// cmd/game-mcp, whose tables mix bot seats with seats a person or a model
// plays. One Stepper per table, kept for the table's life.
type Stepper struct{ guard turnGuard }

// Step makes one move for seat with bot: the bot's own pick first, then each
// offer's submission in order, less anything this turn has already made and
// taken back. apply is the caller's Apply, called for each attempt until one
// is accepted. It returns the action that was accepted, whether it was the
// bot's own pick, and how many of the bot's own picks the engine refused
// (a bug at any strength). ok is false when nothing could be played.
func (st *Stepper) Step(s module.State, bot module.Bot, seat module.BotSeat, offers []module.ActionOffer,
	apply func(module.Action) error) (played module.Action, own bool, refused int, ok bool) {
	st.guard.begin(seat.PlayerID)
	for _, c := range st.guard.candidates(s, bot, seat, offers) {
		if err := apply(c.action); err == nil {
			st.guard.note(c)
			return c.action, c.bot, refused, true
		}
		if c.bot {
			refused++
		}
	}
	return module.Action{}, false, refused, false
}

// Played tells the Stepper that actor made a move it did not choose, so a
// turn that moves on to another seat is forgotten as it would be after Step.
func (st *Stepper) Played(actor string) { st.guard.begin(actor) }
