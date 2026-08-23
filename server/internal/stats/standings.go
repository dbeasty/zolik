package stats

import (
	"sort"

	"zolik/server/internal/models"
	"zolik/server/internal/rules"
)

// Standing is one player's line on a scoreboard. It is produced for
// in-progress matches (the live table) and for finished ones (the record that
// gets persisted) by the same function, so the standings a player watches
// during a match are the standings that get written down at the end of it.
type Standing struct {
	PlayerID string  `bson:"playerId" json:"playerId"`
	Subject  Subject `bson:"subject" json:"subject"`
	Name     string  `bson:"name" json:"name"`
	// Seat is the index in turn order, and the tiebreaker of last resort so
	// a scoreboard never reorders itself between two reads of the same state.
	Seat int `bson:"seat" json:"seat"`

	// DealScores is the penalty taken in each completed deal, oldest first.
	DealScores []int `bson:"dealScores" json:"dealScores"`
	// Total is the running penalty — lower is better.
	Total int `bson:"total" json:"total"`
	// DealsWon counts deals in which this player's penalty was uniquely
	// lowest (rules.DealsWonByPlayer). Note it is the match tiebreaker in the
	// *losing* direction: on equal totals, fewer deals won is better.
	DealsWon int `bson:"dealsWon" json:"dealsWon"`
	// GoOuts counts deals this player ended by emptying their hand. It is
	// read from the action log rather than inferred from a zero score,
	// because zero is exactly what the engine assigns the player who went
	// out and nothing about the number distinguishes the two.
	GoOuts int `bson:"goOuts" json:"goOuts"`

	// Rank is 1-based, with ties sharing a rank and the next rank skipping
	// (1, 1, 3). On a completed match rank 1 is the winner — or every tied
	// player, when the match ended in a draw.
	Rank int `bson:"rank" json:"rank"`
	// Won and Drew are only meaningful once the match is complete.
	Won  bool `bson:"won" json:"won"`
	Drew bool `bson:"drew" json:"drew"`
}

// Scoreboard is the standings view of one match, live or finished.
type Scoreboard struct {
	GameID       string `bson:"gameId" json:"gameId"`
	Status       string `bson:"status" json:"status"`
	RulesProfile string `bson:"rulesProfile" json:"rulesProfile"`
	MatchEndMode string `bson:"matchEndMode" json:"matchEndMode"`
	// TargetScore is the losing threshold under the at_score end mode and 0
	// otherwise; DealCount is the fixed number of deals under after_deals and
	// 0 otherwise. Exactly one of the two is meaningful for a given profile,
	// which is what lets a client render "deal 3 of 7" or "first to 500"
	// without knowing the profile names.
	TargetScore int `bson:"targetScore" json:"targetScore"`
	DealCount   int `bson:"dealCount" json:"dealCount"`

	CurrentDeal int  `bson:"currentDeal" json:"currentDeal"`
	DealsPlayed int  `bson:"dealsPlayed" json:"dealsPlayed"`
	Complete    bool `bson:"complete" json:"complete"`

	WinnerID  string     `bson:"winnerId,omitempty" json:"winnerId,omitempty"`
	IsDraw    bool       `bson:"isDraw,omitempty" json:"isDraw,omitempty"`
	Standings []Standing `bson:"standings" json:"standings"`

	Composition Composition `bson:"composition" json:"composition"`
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

// BuildScoreboard computes standings for a game document at whatever point it
// has reached. It is pure: no I/O, no clock, and it does not mutate g.
//
// cfg is the game's resolved ruleset — passed in rather than derived here so
// that game.GameRules stays the single implementation of "what rules is this
// document running under", including its migration of pre-Rules documents.
//
// For a completed match the winner and draw flag are taken from the document:
// the engine already decided them, and a record has to agree with the match
// the players actually watched end. For an in-progress match they are computed
// with rules.DetermineMatchWinner, so a live table previews the standings
// using the very rule that will settle them.
func BuildScoreboard(g models.Game, cfg rules.RulesConfig) Scoreboard {
	complete := g.Status == string(rules.StatusCompleted)

	dealsWon := rules.DealsWonByPlayer(g.TurnOrder, g.GameScores)
	goOuts := goOutsFromActionLog(g)

	winnerID, isDraw := g.WinnerID, g.IsDraw
	if !complete {
		winnerID, isDraw = rules.DetermineMatchWinner(g.TurnOrder, g.TotalScores, g.GameScores)
	}

	standings := make([]Standing, 0, len(g.Players))
	comp := Composition{}
	diffs := map[string]bool{}
	dealsPlayed := 0

	for seat, p := range seatOrder(g) {
		subject := SubjectForPlayer(p)

		scores := append([]int(nil), g.GameScores[p.ID]...)
		if len(scores) > dealsPlayed {
			dealsPlayed = len(scores)
		}
		// TotalScores is the engine's own running sum; prefer it so a
		// scoreboard can never disagree with the state the players are
		// looking at, and fall back to summing the deals for a document
		// whose totals map has not been written yet.
		total, ok := g.TotalScores[p.ID]
		if !ok {
			for _, v := range scores {
				total += v
			}
		}

		comp.Players++
		switch subject.Kind {
		case SubjectUser:
			comp.Users++
		case SubjectAI:
			comp.AIs++
			diffs[subject.ID] = true
		default:
			comp.Guests++
		}

		standings = append(standings, Standing{
			PlayerID:   p.ID,
			Subject:    subject,
			Name:       p.Name,
			Seat:       seat,
			DealScores: scores,
			Total:      total,
			DealsWon:   dealsWon[p.ID],
			GoOuts:     goOuts[p.ID],
			Won:        complete && !isDraw && p.ID == winnerID,
			// Drew is stamped after ranking — see below.
		})
	}

	for d := range diffs {
		comp.AIDifficulties = append(comp.AIDifficulties, d)
	}
	sort.Strings(comp.AIDifficulties)

	assignRanks(standings)

	// A drawn match is drawn only for the players who actually tied for the
	// lead. In a four-player match where the top two are level, third and
	// fourth still lost it — marking everyone at the table as having drawn
	// would credit them all with a draw in their lifetime record.
	//
	// Rank 1 is exactly the tied group: assignRanks shares a rank on equal
	// total and deals won, which is the same pair rules.DetermineMatchWinner
	// declares a draw on.
	if complete && isDraw {
		for i := range standings {
			standings[i].Drew = standings[i].Rank == 1
		}
	}

	return Scoreboard{
		GameID:       g.ID.Hex(),
		Status:       g.Status,
		RulesProfile: g.RulesProfile,
		MatchEndMode: string(cfg.MatchEndMode),
		TargetScore: func() int {
			if cfg.MatchEndMode == rules.MatchEndAtScore {
				return cfg.TargetScore
			}
			return 0
		}(),
		DealCount: func() int {
			if cfg.MatchEndMode == rules.MatchEndAfterDeals {
				return cfg.FixedDealCount
			}
			return 0
		}(),
		CurrentDeal: g.GameNumber,
		DealsPlayed: dealsPlayed,
		Complete:    complete,
		WinnerID:    winnerID,
		IsDraw:      isDraw,
		Standings:   standings,
		Composition: comp,
	}
}

// seatOrder lists the players in turn order, which is the order the engine
// scores in. Anyone on the document but absent from turn order — a lobby that
// never started, or a malformed document — is appended after it, so a
// scoreboard never silently drops a player it was asked to show.
func seatOrder(g models.Game) []models.Player {
	byID := make(map[string]models.Player, len(g.Players))
	for _, p := range g.Players {
		byID[p.ID] = p
	}
	out := make([]models.Player, 0, len(g.Players))
	seen := map[string]bool{}
	for _, pid := range g.TurnOrder {
		if p, ok := byID[pid]; ok && !seen[pid] {
			out = append(out, p)
			seen[pid] = true
		}
	}
	for _, p := range g.Players {
		if !seen[p.ID] {
			out = append(out, p)
			seen[p.ID] = true
		}
	}
	return out
}

// assignRanks sorts standings best-first and stamps competition ranks onto
// them. The order is total ascending, then deals won ascending — the engine's
// own tiebreak, see rules.DetermineMatchWinner — then seat, purely so the
// result is deterministic. Players level on both total and deals won share a
// rank, which is how a drawn match reports two winners.
func assignRanks(s []Standing) {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].Total != s[j].Total {
			return s[i].Total < s[j].Total
		}
		if s[i].DealsWon != s[j].DealsWon {
			return s[i].DealsWon < s[j].DealsWon
		}
		return s[i].Seat < s[j].Seat
	})
	for i := range s {
		switch {
		case i == 0:
			s[i].Rank = 1
		case s[i].Total == s[i-1].Total && s[i].DealsWon == s[i-1].DealsWon:
			s[i].Rank = s[i-1].Rank
		default:
			s[i].Rank = i + 1
		}
	}
}

// goOutsFromActionLog counts, per player, the deals they ended by going out.
// The engine records each as a "deal_ended" event carrying the winner's ID.
// The surrounding Action.PlayerID is the actor, which is the same player
// today, but the event payload is the field that actually means "went out".
func goOutsFromActionLog(g models.Game) map[string]int {
	out := map[string]int{}
	for _, a := range g.ActionLog {
		if a.Type != "deal_ended" {
			continue
		}
		if id, ok := a.Data["winnerId"].(string); ok && id != "" {
			out[id]++
		}
	}
	return out
}
