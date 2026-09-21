package match

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// fixtureLogs serves the log of matches the replay tests played in memory and
// never stored: the two repository methods a replay reads, and nothing else.
// Any other method panics through the nil embedded interface, which is the
// point — a replay that starts writing would fail here rather than pass.
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

func (f *fixtureLogs) history(id bson.ObjectID) replayLog {
	return replayLog{
		actions: f.logOf(id),
		board:   func(seq int) (models.JSONDoc, error) { return f.CheckpointBoard(context.Background(), id, seq) },
	}
}

func (f *fixtureLogs) ActionLog(_ context.Context, id bson.ObjectID, from, to int) ([]models.MatchAction, error) {
	return sliceLog(f.logOf(id), from, to), nil
}

func (f *fixtureLogs) CheckpointBoard(_ context.Context, id bson.ObjectID, seq int) (models.JSONDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.boards[id][seq]; ok {
		return b, nil
	}
	return nil, db.ErrNotFound
}
