package match

import (
	"context"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// DealResult is one finished attempt at a deal, as the same-deal comparison
// shows it: who, whether they won, and the module's own numbers for the seat.
//
// It names no match. Another player's finished board is not this player's to
// open, and a list of ids would hand them every one.
type DealResult struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
	// You marks the viewer's own attempts.
	You bool `json:"you,omitempty"`
	Won bool `json:"won"`
	// Score and ScoreLabelKey are the module's standing for the seat.
	Score         int    `json:"score"`
	ScoreLabelKey string `json:"scoreLabelKey,omitempty"`
	// Facts are the module's own numbers for the seat, as the board shows
	// them — a score and a move count, for solitaire.
	Facts []module.Fact `json:"facts,omitempty"`
	// Repeat marks an attempt by somebody who had seen the deal before.
	Repeat     bool      `json:"repeat,omitempty"`
	FinishedAt time.Time `json:"finishedAt"`
}

// sameDealLimit bounds the comparison. A deal sent round a group of friends is
// a handful of attempts; one passed round much further than that is a
// leaderboard, which this is not.
const sameDealLimit = 20

// SameDeal is how everybody who played this deal got on: the original game,
// every time it was dealt again, and every time it was sent on.
//
// Only for a player who has finished it themselves, because until then the
// others' results are a hint about the cards in front of them.
func (m *Manager) SameDeal(ctx context.Context, idOrCode, viewerID string) ([]DealResult, error) {
	mine, err := m.Current(ctx, idOrCode)
	if err != nil {
		return nil, err
	}
	if p := playerByID(mine.Players, viewerID); p == nil || p.IsAI {
		return nil, module.Error{Code: "NOT_AT_THIS_TABLE", Message: viewerID}
	}
	if mine.Status != "completed" {
		return nil, module.Error{Code: "MATCH_NOT_OVER"}
	}
	if !m.savedGame(mine) {
		return nil, module.Error{Code: "NOT_A_SOLO_GAME", Message: mine.ModuleID}
	}
	origin := mine.ID
	if mine.DealFrom != "" {
		if id, err := bson.ObjectIDFromHex(mine.DealFrom); err == nil {
			origin = id
		}
	}
	found, err := m.repo.FindDealtFrom(ctx, origin, sameDealLimit)
	if err != nil {
		return nil, err
	}
	mod := m.registry.Get(mine.ModuleID)
	you := playerByID(mine.Players, viewerID)

	out := make([]DealResult, 0, len(found))
	for _, row := range found {
		// The envelope is enough to find a table; its board is what the
		// numbers are read from, and on some backends that lives apart.
		full, err := m.Current(ctx, row.ID.Hex())
		if err != nil || len(full.Players) != 1 || full.ModuleID != mine.ModuleID {
			continue
		}
		p := full.Players[0]
		r := DealResult{
			Name: p.Name, Avatar: p.Avatar,
			You:        p.ID == you.ID || (you.UserID != "" && p.UserID == you.UserID),
			Repeat:     full.DealRepeat,
			FinishedAt: finishedAt(full),
		}
		state := module.State(full.State)
		if st := module.StandingsFor(mod, state); len(st) > 0 {
			r.Won, r.Score, r.ScoreLabelKey = st[0].Won, st[0].Score, st[0].LabelKey
		}
		if vm, err := mod.View(state, p.ID); err == nil {
			if seat := vm.SeatOf(p.ID); seat != nil {
				r.Facts = seat.Facts
			}
		}
		out = append(out, r)
	}
	// Wins first, then the module's own standing, then whoever got there first.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Won != b.Won {
			return a.Won
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.FinishedAt.Before(b.FinishedAt)
	})
	return out, nil
}

func finishedAt(m models.Match) time.Time {
	if m.EndedAt != nil {
		return *m.EndedAt
	}
	return lastActivity(m)
}
