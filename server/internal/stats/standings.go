package stats

import (
	"sort"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Standings, for every game.
//
// This used to be rummy arithmetic: a penalty total where lower was better,
// deals won, go-outs read out of the action log, and a winner recomputed with
// `rules.DetermineMatchWinner`. All of that described one game, which is why
// statistics existed for Žolíky and for nothing else.
//
// A module already answers "who is ahead" in its own terms — `module.Standing`,
// with a rank, a score and a unit key — so this package no longer computes a
// placing at all. It records one. The direction is fixed at the source too:
// every module reports higher-is-better, so Žolíky negates its penalty and
// nothing downstream has to know which way a given game counts.

// Standing is one player's finish, as recorded.
type Standing struct {
	PlayerID string  `bson:"playerId" json:"playerId"`
	Subject  Subject `bson:"subject" json:"subject"`
	Name     string  `bson:"name" json:"name"`
	// Seat is the index in turn order, and the tiebreaker of last resort so a
	// scoreboard never reorders itself between two reads of the same state.
	Seat int `bson:"seat" json:"seat"`

	// Score is the module's own measure — points, chips, cards left — always
	// oriented so higher is better. ScoreLabelKey says which, so a client can
	// put a unit on it without knowing what game it is showing.
	Score         int    `bson:"score" json:"score"`
	ScoreLabelKey string `bson:"scoreLabelKey,omitempty" json:"scoreLabelKey,omitempty"`

	// Rank is 1-based, with ties sharing a rank and the next rank skipping
	// (1, 1, 3). On a completed match rank 1 is the winner — or every tied
	// player, when the match ended level.
	Rank int  `bson:"rank" json:"rank"`
	Won  bool `bson:"won,omitempty" json:"won,omitempty"`
	Drew bool `bson:"drew,omitempty" json:"drew,omitempty"`
}

// Scoreboard is a match's standings at whatever point it has reached.
type Scoreboard struct {
	MatchID string `bson:"matchId" json:"matchId"`
	// ModuleID and Variation are which game this was. They replace the old
	// `rulesProfile`, which could only ever name a rummy ruleset.
	ModuleID  string `bson:"moduleId" json:"moduleId"`
	Variation string `bson:"variation,omitempty" json:"variation,omitempty"`

	Status   string `bson:"status" json:"status"`
	Complete bool   `bson:"complete" json:"complete"`

	Winners []string `bson:"winners,omitempty" json:"winners,omitempty"`
	// IsDraw is more than one winner — a Canasta partnership is not a draw,
	// but two seats level on chips at the end of a fixed-length poker match is.
	IsDraw bool `bson:"isDraw,omitempty" json:"isDraw,omitempty"`

	Standings   []Standing  `bson:"standings" json:"standings"`
	Composition Composition `bson:"composition" json:"composition"`

	// Rounds is the module's round history, reduced to arithmetic.
	Rounds        []RoundRecord `bson:"rounds,omitempty" json:"rounds,omitempty"`
	RoundLabelKey string        `bson:"roundLabelKey,omitempty" json:"roundLabelKey,omitempty"`
}

// roundRecords reduces a module's round log to what a permanent row keeps: the
// numbers, and who took each round. Every label and every fact is dropped — see
// RoundRecord for why a stored row must not carry a locale bundle's params.
func roundRecords(log *module.RoundLog) []RoundRecord {
	if log == nil || len(log.Rounds) == 0 {
		return nil
	}
	out := make([]RoundRecord, 0, len(log.Rounds))
	for _, r := range log.Rounds {
		rec := RoundRecord{Number: r.Number, Winners: append([]string(nil), r.Winners...)}
		for _, sc := range r.Scores {
			rec.Scores = append(rec.Scores, RoundScore{
				PlayerID: sc.PlayerID, Delta: sc.Delta, Total: sc.Total,
			})
		}
		out = append(out, rec)
	}
	return out
}

// Composition summarises who is at the table. Storing it on the match record
// is what lets a lifetime record answer "how do I do against people, versus
// against bots?" without re-reading every match's roster.
type Composition struct {
	Players int `bson:"players" json:"players"`
	Users   int `bson:"users" json:"users"`
	Guests  int `bson:"guests" json:"guests"`
	AIs     int `bson:"ais" json:"ais"`
	// AIDifficulties lists the distinct bot difficulties present, sorted.
	AIDifficulties []string `bson:"aiDifficulties,omitempty" json:"aiDifficulties,omitempty"`
}

// BuildScoreboard turns a match and its module's standings into a record.
//
// Pure: no I/O, no clock, and it mutates neither argument. It computes no
// placing of its own — the module ranked the players, and a second opinion
// here would be a second implementation of "who is winning".
func BuildScoreboard(m models.Match, out module.Outcome) Scoreboard {
	standings := out.Standings
	sb := Scoreboard{
		MatchID:   m.ID.Hex(),
		ModuleID:  m.ModuleID,
		Variation: m.Variation,
		Status:    m.Status,
		Complete:  m.Status == "completed",
		Winners:   append([]string(nil), m.Winners...),
	}
	sb.IsDraw = sb.Complete && len(sb.Winners) > 1

	seatOf := map[string]int{}
	for i, id := range seatOrder(m) {
		seatOf[id] = i
	}
	byID := map[string]models.Player{}
	for _, p := range m.Players {
		byID[p.ID] = p
	}
	won := map[string]bool{}
	for _, w := range sb.Winners {
		won[w] = true
	}

	for _, s := range standings {
		p := byID[s.PlayerID]
		sb.Standings = append(sb.Standings, Standing{
			PlayerID:      s.PlayerID,
			Subject:       SubjectForPlayer(p),
			Name:          p.Name,
			Seat:          seatOf[s.PlayerID],
			Score:         s.Score,
			ScoreLabelKey: s.LabelKey,
			Rank:          s.Rank,
			// On a finished match the winner is the one the *engine* named,
			// not whoever the ranking put first: a match can end on a rule the
			// scoreboard does not model, and a record has to agree with the
			// match the players actually watched end.
			Won:  boolOr(sb.Complete, won[s.PlayerID], s.Won),
			Drew: sb.Complete && len(sb.Winners) > 1 && won[s.PlayerID],
		})
	}

	// A stable order: best rank first, then seat, so two reads of one state
	// never disagree about the order of tied players.
	sort.SliceStable(sb.Standings, func(i, j int) bool {
		if sb.Standings[i].Rank != sb.Standings[j].Rank {
			return sb.Standings[i].Rank < sb.Standings[j].Rank
		}
		return sb.Standings[i].Seat < sb.Standings[j].Seat
	})

	sb.Composition = compositionOf(m)
	sb.Rounds = roundRecords(out.Rounds)
	if out.Rounds != nil {
		sb.RoundLabelKey = out.Rounds.LabelKey
	}
	return sb
}

// boolOr picks the authoritative answer when there is one.
func boolOr(useFirst, first, fallback bool) bool {
	if useFirst {
		return first
	}
	return fallback
}

// seatOrder is turn order when the match has one, and the lobby roster before
// it starts.
func seatOrder(m models.Match) []string {
	if len(m.TurnOrder) > 0 {
		return m.TurnOrder
	}
	out := make([]string, 0, len(m.Players))
	for _, p := range m.Players {
		out = append(out, p.ID)
	}
	return out
}

func compositionOf(m models.Match) Composition {
	c := Composition{Players: len(m.Players)}
	seen := map[string]bool{}
	for _, p := range m.Players {
		switch {
		case p.IsAI:
			c.AIs++
			d := p.AIDifficulty
			if d == "" {
				d = "default"
			}
			if !seen[d] {
				seen[d] = true
				c.AIDifficulties = append(c.AIDifficulties, d)
			}
		case p.UserID != "":
			c.Users++
		default:
			c.Guests++
		}
	}
	sort.Strings(c.AIDifficulties)
	return c
}
