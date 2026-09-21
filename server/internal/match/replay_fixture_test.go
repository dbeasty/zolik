package match

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// fixtureLogs serves the moves and snapshots of matches the replay tests
// played in memory and never stored: the two repository methods a replay
// reads, and nothing else. Any other method panics through the nil embedded
// interface, which is the point — a replay that starts writing would fail
// here rather than pass.
type fixtureLogs struct {
	Repository
	mu     sync.Mutex
	logs   map[bson.ObjectID][]models.MatchAction
	boards map[bson.ObjectID]map[int]models.JSONDoc
}

var fixtures = &fixtureLogs{
	logs:   map[bson.ObjectID][]models.MatchAction{},
	boards: map[bson.ObjectID]map[int]models.JSONDoc{},
}

func (f *fixtureLogs) record(id bson.ObjectID, log []models.MatchAction, boards map[int]models.JSONDoc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs[id] = append([]models.MatchAction(nil), log...)
	f.boards[id] = boards
}

func (f *fixtureLogs) appendAction(id bson.ObjectID, a models.MatchAction) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs[id] = append(f.logs[id], a)
}

func (f *fixtureLogs) logOf(id bson.ObjectID) []models.MatchAction {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logs[id]
}

func (f *fixtureLogs) history(m models.Match) replayLog {
	return replayLog{
		actions:   f.logOf(m.ID),
		snapshots: m.Snapshots,
		snapshot:  func(seq int) (models.JSONDoc, error) { return f.Snapshot(context.Background(), m.ID, seq) },
	}
}

func (f *fixtureLogs) Moves(_ context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error) {
	log := f.logOf(id)
	var out []models.MatchAction
	for seq := from + 1; seq <= len(log) && (to < 0 || seq <= to); seq++ {
		if log[seq-1].Seq != seq {
			break
		}
		out = append(out, log[seq-1])
	}
	return out, nil
}

func (f *fixtureLogs) Snapshot(_ context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.boards[id][seq]; ok {
		return b, nil
	}
	return nil, db.ErrNotFound
}

// roundsClosed is how many rounds a log's moves closed.
func roundsClosed(moves []models.MatchAction) int {
	n := 0
	for _, mv := range moves {
		if mv.Rounds > 0 {
			n = mv.Rounds
		}
	}
	return n
}
