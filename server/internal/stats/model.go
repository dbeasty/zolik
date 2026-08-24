package stats

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// MatchResult is the permanent record of one finished match. It is written
// once, when a game reaches the completed status, and never updated: the
// lifetime aggregates are derived from it, so if an aggregate is ever found to
// be wrong it can be rebuilt from these records rather than being lost.
//
// It deliberately duplicates the standings rather than pointing at the game
// document. Games are heavy (hands, draw pile, full action log) and are
// candidates for expiry; the record has to outlive them.
type MatchResult struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`
	// MatchID carries a unique index: recording is idempotent, so a retried or
	// concurrent completion writes one record, not two.
	MatchID bson.ObjectID `bson:"matchId" json:"matchId"`

	// ModuleID and Variation are which game this was. They replace
	// `rulesProfile`, which could only ever name a rummy ruleset.
	ModuleID  string `bson:"moduleId" json:"moduleId"`
	Variation string `bson:"variation,omitempty" json:"variation,omitempty"`

	StartedAt   time.Time `bson:"startedAt" json:"startedAt"`
	CompletedAt time.Time `bson:"completedAt" json:"completedAt"`
	// DurationSeconds is wall-clock from lobby creation to completion, so it
	// includes lobby idling and any suspension. It is a rough "how long did
	// this take" figure, not a measure of playing time.
	DurationSeconds int `bson:"durationSeconds" json:"durationSeconds"`

	Composition Composition `bson:"composition" json:"composition"`

	// Participants is the final standings, best-ranked first.
	Participants []Standing `bson:"participants" json:"participants"`
	// SubjectKeys duplicates the durable participants' keys as a flat array
	// purely so "every match this subject played" is a single indexed lookup
	// instead of a scan into the participants array.
	SubjectKeys []string `bson:"subjectKeys" json:"subjectKeys"`

	// Winners is everyone who won: one player usually, a whole partnership in
	// Canasta, several seats when a fixed-length match ends level.
	Winners []string `bson:"winners,omitempty" json:"winners,omitempty"`
	IsDraw  bool     `bson:"isDraw,omitempty" json:"isDraw,omitempty"`

	RecordedAt time.Time `bson:"recordedAt" json:"recordedAt"`
}

// Tally is one bucket of lifetime counters. The same shape is reused for the
// overall record and for every split (vs humans, vs a difficulty, per profile)
// so a client can render any of them with one component.
type Tally struct {
	// Matches counts match participations. For a user that is the same as
	// matches, since a user can hold only one seat. An AI subject can hold
	// several seats in one match — two "medium" bots at the same table — and
	// each seat counts, because each played a distinct hand.
	Matches int `bson:"matches" json:"matches"`
	Wins    int `bson:"wins" json:"wins"`
	Losses  int `bson:"losses" json:"losses"`
	Draws   int `bson:"draws" json:"draws"`

	// ScoreSum totals the module's own measure across every match, always
	// oriented so higher is better.
	//
	// Deal-level counters used to live here — deals played, deals won,
	// go-outs, penalty points — and every one of them described rummy. A game
	// with no deals had nothing to put in them, which is why statistics
	// existed for Žolíky and for nothing else.
	ScoreSum int `bson:"scoreSum" json:"scoreSum"`
	// RankSum over Matches gives the average finishing position, which is the
	// only cross-table-size comparable placing figure (winning a 2-player
	// match is not the same achievement as winning a 6-player one).
	RankSum int `bson:"rankSum" json:"rankSum"`

	// BestScore is the highest score ever posted and WorstScore the lowest.
	// Both are only meaningful when Matches > 0. They used to be the other way
	// round because rummy is scored downwards; every module now reports
	// higher-is-better, so these read the way the words do.
	BestScore  int `bson:"bestScore" json:"bestScore"`
	WorstScore int `bson:"worstScore" json:"worstScore"`
}

// TallyView is a Tally plus the figures derived from it. Derived values are
// computed on read rather than stored, so a fix to an average can never
// require rewriting history.
type TallyView struct {
	Tally
	WinRate    float64 `json:"winRate"`
	AvgScore   float64 `json:"avgScore"`
	AvgRank    float64 `json:"avgRank"`
	BestScore  *int    `json:"bestScore"`
	WorstScore *int    `json:"worstScore"`
}

// View expands a Tally with its derived figures. Rates are 0 when there is
// nothing to divide by, and the best/worst totals are null rather than a
// sentinel integer so a client never renders a placeholder as a real score.
func (t Tally) View() TallyView {
	v := TallyView{Tally: t}
	if t.Matches > 0 {
		v.WinRate = float64(t.Wins) / float64(t.Matches)
		v.AvgScore = float64(t.ScoreSum) / float64(t.Matches)
		v.AvgRank = float64(t.RankSum) / float64(t.Matches)
		best, worst := t.BestScore, t.WorstScore
		v.BestScore, v.WorstScore = &best, &worst
	}
	return v
}

// Add folds one match participation into the tally.
func (t *Tally) Add(s Standing, isDraw bool) {
	first := t.Matches == 0
	t.Matches++
	switch {
	case isDraw && s.Drew:
		t.Draws++
	case s.Won:
		t.Wins++
	default:
		t.Losses++
	}
	t.ScoreSum += s.Score
	t.RankSum += s.Rank
	if first || s.Score > t.BestScore {
		t.BestScore = s.Score
	}
	if first || s.Score < t.WorstScore {
		t.WorstScore = s.Score
	}
}

// HeadToHead is one subject's record against one other durable subject.
type HeadToHead struct {
	Subject Subject `bson:"subject" json:"subject"`
	// Matches counts matches the two shared a table in — including matches
	// with other players present, where "ahead" and "behind" are decided
	// pairwise on final total rather than by who won the match overall.
	Matches int `bson:"matches" json:"matches"`
	Ahead   int `bson:"ahead" json:"ahead"`
	Behind  int `bson:"behind" json:"behind"`
	Level   int `bson:"level" json:"level"`
	// ScoreFor and ScoreAgainst are the two sides' accumulated scores across
	// those matches, in whatever the module measures. Higher is better, so
	// ScoreFor above ScoreAgainst is the good direction — the opposite of what
	// these meant when they were rummy penalties.
	ScoreFor     int       `bson:"scoreFor" json:"scoreFor"`
	ScoreAgainst int       `bson:"scoreAgainst" json:"scoreAgainst"`
	LastPlayedAt time.Time `bson:"lastPlayedAt" json:"lastPlayedAt"`
}

// MatchRef is a compact pointer to a played match, for a recent-form list.
type MatchRef struct {
	// RecordID is the match_results row; MatchID is the match it recorded.
	RecordID  bson.ObjectID `bson:"recordId" json:"recordId"`
	MatchID   bson.ObjectID `bson:"matchId" json:"matchId"`
	PlayedAt  time.Time     `bson:"playedAt" json:"playedAt"`
	ModuleID  string        `bson:"moduleId" json:"moduleId"`
	Variation string        `bson:"variation,omitempty" json:"variation,omitempty"`
	Players   int           `bson:"players" json:"players"`
	Rank      int           `bson:"rank" json:"rank"`
	Score     int           `bson:"score" json:"score"`
	Won       bool          `bson:"won" json:"won"`
	Drew      bool          `bson:"drew" json:"drew"`
	// Outcome is "win" | "loss" | "draw", denormalised so a recent-form strip
	// needs no logic to render.
	Outcome string `bson:"outcome" json:"outcome"`
	// AgainstAI and AgainstHumans describe the table this result came from —
	// a 3-win streak means something different against bots.
	AgainstAI     bool `bson:"againstAI" json:"againstAI"`
	AgainstHumans bool `bson:"againstHumans" json:"againstHumans"`
}

// recentMatchesKept caps the inline recent-form list on a lifetime record.
// The full history is not truncated — it lives in the match_results
// collection and is paged from there; this is only the strip a profile screen
// shows without a second query.
const recentMatchesKept = 20

// PlayerStats is the lifetime record for one durable subject: a registered
// user, or an AI difficulty. Guests have none, by design — see SubjectGuest.
//
// Bots keeping a record on the same footing as people is deliberate. It makes
// "you are 4-11 against hard" answerable from the same data as any
// player-versus-player record, and it turns the AI's aggregate win rate into a
// tuning signal that costs nothing extra to collect.
type PlayerStats struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"-"`
	SubjectKey string        `bson:"subjectKey" json:"subjectKey"`
	Subject    Subject       `bson:"subject" json:"subject"`

	Overall Tally `bson:"overall" json:"overall"`
	// VsHumans covers matches with at least one human opponent (registered or
	// guest); VsAI, matches with at least one bot opponent. A mixed table
	// counts in both — they are overlapping views of the same matches, not a
	// partition, because the interesting question is "was a person involved",
	// not "was the table pure".
	VsHumans Tally `bson:"vsHumans" json:"vsHumans"`
	VsAI     Tally `bson:"vsAI" json:"vsAI"`

	// ByAIDifficulty is keyed by the difficulty of an *opponent* bot, so a
	// table with an easy and a hard bot counts once under each.
	ByAIDifficulty map[string]Tally `bson:"byAIDifficulty,omitempty" json:"byAIDifficulty,omitempty"`
	// ByModule is keyed by game. A Canasta total and a poker stack are not
	// comparable numbers, so a lifetime average across both would be noise —
	// this split is what keeps each game's figures meaningful. It replaces a
	// per-rummy-profile split, which could only ever describe one game.
	ByModule map[string]Tally `bson:"byModule,omitempty" json:"byModule,omitempty"`
	// ByPlayerCount is keyed by the table size as a decimal string ("2".."8"),
	// because BSON map keys must be strings.
	ByPlayerCount map[string]Tally `bson:"byPlayerCount,omitempty" json:"byPlayerCount,omitempty"`

	// HeadToHead is keyed by the opponent's Subject.Key().
	HeadToHead map[string]HeadToHead `bson:"headToHead,omitempty" json:"headToHead,omitempty"`

	// CurrentStreak is signed: positive for consecutive wins, negative for
	// consecutive losses, zero after a draw or before the first match.
	CurrentStreak     int `bson:"currentStreak" json:"currentStreak"`
	LongestWinStreak  int `bson:"longestWinStreak" json:"longestWinStreak"`
	LongestLossStreak int `bson:"longestLossStreak" json:"longestLossStreak"`

	FirstMatchAt time.Time `bson:"firstMatchAt,omitempty" json:"firstMatchAt,omitempty"`
	LastMatchAt  time.Time `bson:"lastMatchAt,omitempty" json:"lastMatchAt,omitempty"`

	// RecentMatches is newest-first and capped at recentMatchesKept.
	RecentMatches []MatchRef `bson:"recentMatches,omitempty" json:"recentMatches,omitempty"`

	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}
