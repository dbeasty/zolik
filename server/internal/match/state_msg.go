package match

import (
	"crypto/rand"
	"math/big"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// MatchStateMsg is what a client renders a hosted match from.
//
// Compare GameStateMsg: 24 rummy-named fields (drawPile, melds, meldMeta,
// roundReqMet, discardDrawnCardPendingMeld…) plus a growing set of ad-hoc
// booleans. This has none, because it has no idea what game it is describing —
// a board is zones, and what a player may do is offers.
type MatchStateMsg struct {
	Type      string `json:"type"`
	MatchID   string `json:"matchId"`
	ModuleID  string `json:"moduleId"`
	Variation string `json:"variation,omitempty"`
	Status    string `json:"status"`
	JoinCode  string `json:"joinCode,omitempty"`
	HostID    string `json:"hostId,omitempty"`
	// WinnerID is the first winner; Winners is all of them. Both ship, because
	// a partnership and a split pot each have more than one and a client
	// written before that was true should not break.
	WinnerID string   `json:"winnerId,omitempty"`
	Winners  []string `json:"winners,omitempty"`

	Players []PlayerMsg `json:"players"`
	// View is the board as this viewer may see it — the only place hidden
	// information is filtered, decided by the module.
	View module.ViewModel `json:"view"`
	// LegalActions is what this viewer may do right now.
	LegalActions []module.ActionOffer `json:"legalActions"`
	// Standings is the scoreboard, in a shape no game owns — so one screen can
	// show who is ahead at rummy, canasta and poker without knowing what any of
	// those measure.
	Standings []module.Standing `json:"standings,omitempty"`
	// SuspendedPlayer names the seat a paused match is waiting for.
	SuspendedPlayer string `json:"suspendedPlayer,omitempty"`
}

type PlayerMsg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IsAI bool   `json:"isAI"`
}

// BuildStateMsg renders one viewer's state.
//
// A lobby has no state yet, so the module is not asked for a view — which is
// itself a small demonstration that the runtime can describe a match before
// the game owning it has done anything.
func (m *Manager) BuildStateMsg(match models.Match, viewerID string) MatchStateMsg {
	msg := MatchStateMsg{
		Type:            "match_state",
		MatchID:         match.ID.Hex(),
		ModuleID:        match.ModuleID,
		Variation:       match.Variation,
		Status:          match.Status,
		JoinCode:        match.JoinCode,
		HostID:          match.HostID,
		WinnerID:        match.WinnerID,
		Winners:         match.Winners,
		SuspendedPlayer: match.SuspendedPlayer,
		// Never nil: these round-trip to JSON, and a nil slice serialises to
		// `null`, which every client then has to guard before indexing.
		LegalActions: []module.ActionOffer{},
	}
	for _, p := range match.Players {
		msg.Players = append(msg.Players, PlayerMsg{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}

	mod := m.registry.Get(match.ModuleID)
	if mod == nil || len(match.State) == 0 {
		return msg
	}
	if vm, err := mod.View(match.State, viewerID); err == nil {
		msg.View = vm
	}
	if offers, err := mod.LegalActions(match.State, viewerID); err == nil && offers != nil {
		msg.LegalActions = offers
	}
	msg.Standings = module.StandingsFor(mod, match.State)
	return msg
}

func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}
