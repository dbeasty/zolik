package match

import (
	"strings"
	"testing"

	"zolik/server/internal/module"
)

// The oscillation botTurn exists to break, written as the loop meets it.
//
// A seat takes the discard pile, finds the turn goes nowhere, and puts it back.
// The position is then bit for bit the one it started from, so the module's bot
// — a pure function of the position — asks to take the pile again. Before the
// guard the loop obliged, thirty times, and then gave up on a table nothing
// would ever move again.
func TestABotDoesNotRetakeAMoveItJustUndid(t *testing.T) {
	takePile := botMove{action: module.Action{Verb: "take_pile", Cards: []string{"5S", "5D"}}}
	undo := botMove{action: module.Action{Verb: "undo_take_pile"}, undo: true}
	draw := botMove{action: module.Action{Verb: "draw"}}

	var turn botTurn

	if turn.skips(takePile) {
		t.Fatal("the first move of a turn was skipped")
	}
	turn.note(takePile)

	// Nothing is off the list until something has actually been taken back:
	// a turn that merely repeats itself is not what this guards.
	if turn.skips(takePile) {
		t.Error("a move was skipped before anything had been undone")
	}

	turn.note(undo)

	if !turn.skips(takePile) {
		t.Error("the seat was allowed to retake the pile it had just put back")
	}
	if turn.skips(draw) {
		t.Error("a move the turn has not made was skipped")
	}
	if turn.skips(undo) {
		t.Error("an undo was skipped — unwinding needs every one of them")
	}
}

// A turn's memory is a turn's. Carrying it into the next seat's turn would
// forbid perfectly ordinary play — every deal is full of seats drawing.
func TestABotTurnsMemoryDoesNotOutliveTheTurn(t *testing.T) {
	draw := botMove{action: module.Action{Verb: "draw"}}

	var turn botTurn
	turn.note(draw)
	turn.note(botMove{action: module.Action{Verb: "undo_lay_meld"}, undo: true})
	if !turn.skips(draw) {
		t.Fatal("precondition: the guard should be armed")
	}

	turn = botTurn{} // what botLoop does when the awaited seat changes
	if turn.skips(draw) {
		t.Error("a new turn started with the last one's moves already spent")
	}
}

// botCandidates carries the one property of an offer the loop cannot work out
// from the submission alone — see module.ActionOffer.Undo.
func TestBotCandidatesCarryTheUndoFlag(t *testing.T) {
	offers := []module.ActionOffer{
		{ID: "draw:deck", Verb: "draw", Enabled: true},
		{ID: "undo_take_pile", Verb: "undo_take_pile", Enabled: true, Undo: true},
		{ID: "discard", Verb: "discard", Enabled: false, WhyNot: "INITIAL_MELD_NOT_MET"},
	}
	got := botCandidates(offers)
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want the 2 enabled offers: %+v", len(got), got)
	}
	if got[0].undo {
		t.Error("draw was marked as an undo")
	}
	if !got[1].undo {
		t.Error("undo_take_pile was not marked as an undo")
	}
}

// Every offer that takes a move back says so, at every table this server
// hosts, so the guard above actually arms in a real deal.
//
// The spelling is not the contract — the flag is — but every undo this server
// ships is spelled this way, and an undo added tomorrow that forgets the flag
// would put the oscillation back with nothing to notice it. Žolíky offers its
// four from the first turn of a deal, enabled or not, which is what keeps this
// from passing merely because it found nothing to look at; canasta's appear
// only once there is something to take back and are checked in that package,
// where the position is easy to build.
func TestTheModulesWithAnUndoDeclareIt(t *testing.T) {
	seen := 0
	for _, tc := range botCases() {
		t.Run(tc.name, func(t *testing.T) {
			state, err := tc.mod.NewMatch(tc.cfg, tc.players, 4)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			offers, err := tc.mod.LegalActions(state, tc.players[0].ID)
			if err != nil {
				t.Fatalf("LegalActions: %v", err)
			}
			for _, o := range offers {
				if looksLikeAnUndo(o) {
					seen++
					if !o.Undo {
						t.Errorf("offer %q takes a move back but does not say so", o.ID)
					}
				} else if o.Undo {
					t.Errorf("offer %q says it takes a move back but is not one", o.ID)
				}
			}
		})
	}
	if seen == 0 {
		t.Error("no undo offer was examined — this test proved nothing")
	}
}

func looksLikeAnUndo(o module.ActionOffer) bool {
	return strings.HasPrefix(o.ID, "undo")
}
