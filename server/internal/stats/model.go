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
	// GameID carries a unique index: recording is idempotent, so a retried or
	// concurrent completion writes one record, not two.
	GameID bson.ObjectID `bson:"gameId" json:"gameId"`

	RulesProfile string `bson:"rulesProfile" json:"rulesProfile"`
	MatchEndMode string `bson:"matchEndMode" json:"matchEndMode"`
	TargetScore  int    `bson:"targetScore,omitempty" json:"targetScore,omitempty"`
	DealCount    int    `bson:"dealCount,omitempty" json:"dealCount,omitempty"`

	StartedAt   time.Time `bson:"startedAt" json:"startedAt"`
	CompletedAt time.Time `bson:"completedAt" json:"completedAt"`
	// DurationSeconds is wall-clock from lobby creation to completion, so it
	// includes lobby idling and any suspension. It is a rough "how long did
	// this take" figure, not a measure of playing time.
	DurationSeconds int `bson:"durationSeconds" json:"durationSeconds"`

	DealsPlayed int         `bson:"dealsPlayed" json:"dealsPlayed"`
	Composition Composition `bson:"composition" json:"composition"`

	// Participants is the final standings, best-ranked first.
	Participants []Standing `bson:"participants" json:"participants"`
	// SubjectKeys duplicates the durable participants' keys as a flat array
	// purely so "every match this subject played" is a single indexed lookup
	// instead of a scan into the participants array.
	SubjectKeys []string `bson:"subjectKeys" json:"subjectKeys"`

	WinnerPlayerID string `bson:"winnerPlayerId,omitempty" json:"winnerPlayerId,omitempty"`
	IsDraw         bool   `bson:"isDraw,omitempty" json:"isDraw,omitempty"`

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

	DealsPlayed int `bson:"dealsPlayed" json:"dealsPlayed"`
	DealsWon    int `bson:"dealsWon" json:"dealsWon"`
	// GoOuts counts deals ended by emptying the hand — the thing a player is
	// actually trying to do, and a better skill signal than match wins alone
	// in a format where one bad deal can sink a good match.
	GoOuts int `bson:"goOuts" json:"goOuts"`

	// PenaltyPoints is the total penalty accrued across all deals. Lower is
	// better; combined with DealsPlayed it gives the average penalty per deal.
	PenaltyPoints int `bson:"penaltyPoints" json:"penaltyPoints"`
	// RankSum over Matches gives the average finishing position, which is the
	// only cross-table-size comparable placing figure (winning a 2-player
	// match is not the same achievement as winning a 6-player one).
	RankSum int `bson:"rankSum" json:"rankSum"`

	// BestMatchTotal is the lowest match total ever posted and WorstMatchTotal
	// the highest. Both are only meaningful when Matches > 0.
	BestMatchTotal  int `bson:"bestMatchTotal" json:"bestMatchTotal"`
	WorstMatchTotal int `bson:"worstMatchTotal" json:"worstMatchTotal"`
}

// TallyView is a Tally plus the figures derived from it. Derived values are
// computed on read rather than stored, so a fix to an average can never
// require rewriting history.
type TallyView struct {
	Tally
	WinRate            float64 `json:"winRate"`
	DealWinRate        float64 `json:"dealWinRate"`
	AvgPenaltyPerDeal  float64 `json:"avgPenaltyPerDeal"`
	AvgPenaltyPerMatch float64 `json:"avgPenaltyPerMatch"`
	AvgRank            float64 `json:"avgRank"`
	BestMatchTotal     *int    `json:"bestMatchTotal"`
	WorstMatchTotal    *int    `json:"worstMatchTotal"`
}

// View expands a Tally with its derived figures. Rates are 0 when there is
// nothing to divide by, and the best/worst totals are null rather than a
// sentinel integer so a client never renders a placeholder as a real score.
func (t Tally) View() TallyView {
	v := TallyView{Tally: t}
	if t.Matches > 0 {
		v.WinRate = float64(t.Wins) / float64(t.Matches)
		v.AvgPenaltyPerMatch = float64(t.PenaltyPoints) / float64(t.Matches)
		v.AvgRank = float64(t.RankSum) / float64(t.Matches)
		best, worst := t.BestMatchTotal, t.WorstMatchTotal
		v.BestMatchTotal, v.WorstMatchTotal = &best, &worst
	}
	if t.DealsPlayed > 0 {
		v.DealWinRate = float64(t.DealsWon) / float64(t.DealsPlayed)
		v.AvgPenaltyPerDeal = float64(t.PenaltyPoints) / float64(t.DealsPlayed)
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
	t.DealsPlayed += len(s.DealScores)
	t.DealsWon += s.DealsWon
	t.GoOuts += s.GoOuts
	t.PenaltyPoints += s.Total
	t.RankSum += s.Rank
	if first || s.Total < t.BestMatchTotal {
		t.BestMatchTotal = s.Total
	}
	if first || s.Total > t.WorstMatchTotal {
		t.WorstMatchTotal = s.Total
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
	// PointsFor and PointsAgainst are the two sides' accumulated penalties
	// across those matches. Lower is better, so PointsFor below
	// PointsAgainst is the good direction.
	PointsFor     int       `bson:"pointsFor" json:"pointsFor"`
	PointsAgainst int       `bson:"pointsAgainst" json:"pointsAgainst"`
	LastPlayedAt  time.Time `bson:"lastPlayedAt" json:"lastPlayedAt"`
}

// MatchRef is a compact pointer to a played match, for a recent-form list.
type MatchRef struct {
	MatchID      bson.ObjectID `bson:"matchId" json:"matchId"`
	GameID       bson.ObjectID `bson:"gameId" json:"gameId"`
	PlayedAt     time.Time     `bson:"playedAt" json:"playedAt"`
	RulesProfile string        `bson:"rulesProfile" json:"rulesProfile"`
	Players      int           `bson:"players" json:"players"`
	Rank         int           `bson:"rank" json:"rank"`
	Total        int           `bson:"total" json:"total"`
	Won          bool          `bson:"won" json:"won"`
	Drew         bool          `bson:"drew" json:"drew"`
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
	// ByProfile is keyed by rules profile, since a Continental total and a
	// Žolíky total are not the same currency and averaging them is meaningless.
	ByProfile map[string]Tally `bson:"byProfile,omitempty" json:"byProfile,omitempty"`
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
