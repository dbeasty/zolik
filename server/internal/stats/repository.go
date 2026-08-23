package stats

import (
	"context"
	"errors"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

// ErrAlreadyRecorded is returned when a match has already been written. It is
// an expected outcome, not a failure: two servers can observe the same game
// completing, and the unique index on gameId is what makes the second one a
// no-op instead of a double count.
var ErrAlreadyRecorded = errors.New("match already recorded")

// ErrNotFound is returned when a match or lifetime record does not exist.
var ErrNotFound = errors.New("not found")

type Repository struct {
	matches *mongo.Collection
	players *mongo.Collection
}

func NewRepository(m *db.Mongo) *Repository {
	c := m.Collections()
	return &Repository{matches: c.MatchResults, players: c.PlayerStats}
}

// InsertMatch writes the permanent match record, returning ErrAlreadyRecorded
// if one already exists for this game. The caller must treat that as a signal
// to skip aggregation, since the aggregates for this match are already in.
func (r *Repository) InsertMatch(ctx context.Context, m MatchResult) (MatchResult, error) {
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}
	if _, err := r.matches.InsertOne(ctx, m); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return MatchResult{}, ErrAlreadyRecorded
		}
		return MatchResult{}, err
	}
	return m, nil
}

func (r *Repository) FindMatchByGameID(ctx context.Context, gameID bson.ObjectID) (MatchResult, error) {
	var m MatchResult
	err := r.matches.FindOne(ctx, bson.M{"gameId": gameID}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return MatchResult{}, ErrNotFound
	}
	return m, err
}

// ListMatchesForSubject pages a subject's match history, newest first. before
// is the completedAt of the last row of the previous page; a zero value starts
// at the top. Keyset paging rather than skip/limit, so a match recorded
// mid-scroll cannot make the reader skip or repeat a row.
func (r *Repository) ListMatchesForSubject(ctx context.Context, key string, before time.Time, limit int) ([]MatchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	filter := bson.M{"subjectKeys": key}
	if !before.IsZero() {
		filter["completedAt"] = bson.M{"$lt": before}
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "completedAt", Value: -1}, {Key: "_id", Value: -1}}).
		SetLimit(int64(limit))

	cur, err := r.matches.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := []MatchResult{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindPlayerStats returns a subject's lifetime record, or ErrNotFound when the
// subject has not finished a match yet. Callers that want a renderable zero
// record should use ZeroStats.
func (r *Repository) FindPlayerStats(ctx context.Context, key string) (PlayerStats, error) {
	var ps PlayerStats
	err := r.players.FindOne(ctx, bson.M{"subjectKey": key}).Decode(&ps)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return PlayerStats{}, ErrNotFound
	}
	return ps, err
}

// FindManyPlayerStats fetches several lifetime records at once, keyed by
// subject key. Missing subjects are simply absent from the map — this backs
// the "who am I sitting with" panel, where a first-timer at the table is
// normal and must not fail the whole request.
func (r *Repository) FindManyPlayerStats(ctx context.Context, keys []string) (map[string]PlayerStats, error) {
	out := map[string]PlayerStats{}
	if len(keys) == 0 {
		return out, nil
	}
	cur, err := r.players.Find(ctx, bson.M{"subjectKey": bson.M{"$in": keys}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []PlayerStats
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, ps := range rows {
		out[ps.SubjectKey] = ps
	}
	return out, nil
}

// UpsertPlayerStats writes a subject's lifetime record, creating it on first
// use. Replace rather than $inc because the aggregation is a pure function of
// the loaded record plus the match (see ApplyMatch) — expressing streaks and
// best/worst as update operators would put that logic in two places.
func (r *Repository) UpsertPlayerStats(ctx context.Context, ps PlayerStats) error {
	ps.ID = bson.ObjectID{}
	_, err := r.players.ReplaceOne(ctx,
		bson.M{"subjectKey": ps.SubjectKey},
		ps,
		options.Replace().SetUpsert(true),
	)
	return err
}

// LeaderboardQuery selects and orders a leaderboard page.
type LeaderboardQuery struct {
	// Kind restricts rows to one subject kind. Empty means users only:
	// mixing bots into the human leaderboard by default would let a bot that
	// has played thousands of matches sit permanently at the top.
	Kind SubjectKind
	// Scope picks which tally ranks: "" or "overall", "vs_humans", "vs_ai".
	Scope string
	// MinMatches filters out small samples, whose win rates are noise.
	MinMatches int
	Limit      int
}

// LeaderboardRow is one ranked entry.
type LeaderboardRow struct {
	Rank    int       `json:"rank"`
	Subject Subject   `json:"subject"`
	Tally   TallyView `json:"tally"`
	// Streaks come from the overall record regardless of scope, since a
	// streak is a property of the player's run, not of a filtered subset.
	CurrentStreak    int       `json:"currentStreak"`
	LongestWinStreak int       `json:"longestWinStreak"`
	LastMatchAt      time.Time `json:"lastMatchAt,omitempty"`
}

// Leaderboard ranks subjects by wins, then win rate, then fewest average
// penalty points. It reads and sorts in the application rather than in an
// aggregation pipeline: the scope choice selects a different embedded
// document, and the tiebreakers are derived values that are not stored.
//
// This is fine at the scale a card game reaches; if the player table ever
// outgrows it, the fix is a stored, denormalised sort key per scope rather
// than a more elaborate query.
func (r *Repository) Leaderboard(ctx context.Context, q LeaderboardQuery) ([]LeaderboardRow, error) {
	kind := q.Kind
	if kind == "" {
		kind = SubjectUser
	}
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	cur, err := r.players.Find(ctx, bson.M{"subject.kind": string(kind)})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []PlayerStats
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}

	type entry struct {
		ps PlayerStats
		t  Tally
	}
	entries := make([]entry, 0, len(rows))
	for _, ps := range rows {
		t := ScopedTally(ps, q.Scope)
		if t.Matches == 0 || t.Matches < q.MinMatches {
			continue
		}
		entries = append(entries, entry{ps: ps, t: t})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.t.Wins != b.t.Wins {
			return a.t.Wins > b.t.Wins
		}
		aw := float64(a.t.Wins) / float64(a.t.Matches)
		bw := float64(b.t.Wins) / float64(b.t.Matches)
		if aw != bw {
			return aw > bw
		}
		ap := float64(a.t.PenaltyPoints) / float64(a.t.Matches)
		bp := float64(b.t.PenaltyPoints) / float64(b.t.Matches)
		if ap != bp {
			return ap < bp
		}
		return a.ps.SubjectKey < b.ps.SubjectKey
	})

	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]LeaderboardRow, 0, len(entries))
	for i, e := range entries {
		out = append(out, LeaderboardRow{
			Rank:             i + 1,
			Subject:          e.ps.Subject,
			Tally:            e.t.View(),
			CurrentStreak:    e.ps.CurrentStreak,
			LongestWinStreak: e.ps.LongestWinStreak,
			LastMatchAt:      e.ps.LastMatchAt,
		})
	}
	return out, nil
}

// ScopedTally picks the tally a scope name refers to. An unknown scope falls
// back to the overall record rather than erroring, so a stale client asking
// for a scope this server does not know still renders something truthful.
func ScopedTally(ps PlayerStats, scope string) Tally {
	switch scope {
	case "vs_humans":
		return ps.VsHumans
	case "vs_ai":
		return ps.VsAI
	default:
		return ps.Overall
	}
}

// ZeroStats is the renderable empty record for a subject with no finished
// matches — so a brand-new player's profile screen shows zeroes rather than a
// 404 the client has to special-case.
func ZeroStats(s Subject) PlayerStats {
	return PlayerStats{SubjectKey: s.Key(), Subject: s}
}
