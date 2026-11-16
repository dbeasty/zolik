package match

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// lostAck stores a move and then reports failure, as a Mongo write does when
// it lands but its answer is lost to a timeout — or as CommitSnapshot does when
// the move and snapshot are written and the envelope is not.
type lostAck struct {
	Repository
	fail bool
}

var errLostAck = errors.New("i/o timeout")

func (r *lostAck) AppendMove(ctx context.Context, id bson.ObjectID, mv models.MatchAction) error {
	if err := r.Repository.AppendMove(ctx, id, mv); err != nil || !r.fail {
		return err
	}
	return errLostAck
}

func (r *lostAck) CommitSnapshot(ctx context.Context, id bson.ObjectID, expected int64, next models.Match,
	moves []models.MatchAction, seq int, state models.JSONDoc) error {
	if !r.fail {
		return r.Repository.CommitSnapshot(ctx, id, expected, next, moves, seq, state)
	}
	for _, mv := range moves {
		if err := r.Repository.AppendMove(ctx, id, mv); err != nil {
			return err
		}
	}
	return errLostAck
}

// A persist error that is not a version conflict used to keep the entry: the
// board in memory stayed a move behind the log the store actually held, and
// the player's next move was refused against it. Any failure now drops the
// entry, so the next lock reads what was stored.
func TestAFailedPersistDropsTheCachedMatch(t *testing.T) {
	g := withRounds(t, "prsi")
	h := newLiveHarness(t)
	repo := &lostAck{Repository: h.repo}
	h.m.repo = repo
	m := h.startGame(t, g, 1)
	id := m.ID.Hex()
	h.play(t, g, id, 2, func(int, module.State) {})

	repo.fail = true
	cur, err := h.m.current(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	actor := module.AwaitedSeats(g.mod, module.State(cur.State), "", refsOf(cur))[0]
	offers, err := g.mod.LegalActions(module.State(cur.State), actor)
	if err != nil {
		t.Fatal(err)
	}
	failed := false
	for _, c := range botCandidates(orderByPreference(offers, g.prefer)) {
		if err := h.m.HandleAction(context.Background(), id, actor, c.action); errors.Is(err, errLostAck) {
			failed = true
			break
		}
	}
	if !failed {
		t.Fatal("no move reached the store to fail")
	}
	repo.fail = false

	stored, err := h.repo.Moves(context.Background(), m.ID, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	after, err := h.m.current(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if after.LogSeq != len(stored) {
		t.Fatalf("the runtime holds the board after %d moves; the store holds %d", after.LogSeq, len(stored))
	}
	// And play goes on from there.
	if n := h.play(t, g, id, 1, func(int, module.State) {}); n != 1 {
		t.Fatal("no move could be made after the failed persist")
	}
}
