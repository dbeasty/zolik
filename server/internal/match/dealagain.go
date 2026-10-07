package match

import (
	"context"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Dealing a finished one-seat game again, card for card (docs/solitaire-plan.md
// §4b): to the player who played it ("Play this deal again"), or to anybody
// they send it to ("Send this deal").
//
// The seed is copied here, on the server, and never leaves it. That is the
// point of doing it this way rather than with a deal number: a deal a player
// could read is a deal a player could choose, and in a game played against
// the deck alone choosing the deal is winning it.
//
// Only a finished game: one still in play could otherwise be dealt again
// beside itself and played out with the knowledge of the other. Only a
// one-seat game, because "the same deal" of a table for several people needs
// the same people, which is what a rematch is for.

// DealAgain deals a finished one-seat game again for the player who sat at it.
func (m *Manager) DealAgain(ctx context.Context, idOrCode, callerID string) (models.Match, error) {
	old, err := m.dealSource(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	caller := playerByID(old.Players, callerID)
	if caller == nil || caller.IsAI {
		return models.Match{}, module.Error{Code: "NOT_AT_THIS_TABLE", Message: callerID}
	}
	host := *caller
	host.ConnectionID = ""
	host.SeatKeyHash = ""
	return m.dealFrom(ctx, old, host, true)
}

// DealLink is a link to send this finished game's deal to somebody else.
// Only somebody who played it may send it: the link is theirs to give.
func (m *Manager) DealLink(ctx context.Context, idOrCode, callerID string) (string, error) {
	old, err := m.dealSource(ctx, idOrCode)
	if err != nil {
		return "", err
	}
	if p := playerByID(old.Players, callerID); p == nil || p.IsAI {
		return "", module.Error{Code: "NOT_AT_THIS_TABLE", Message: callerID}
	}
	token := auth.SealDealLink(old.ID.Hex())
	if token == "" {
		return "", module.Error{Code: "DEAL_LINK_INVALID"}
	}
	return token, nil
}

// SentDeal is what a deal link offers: the game and its settings, and nothing
// that identifies the match it came from.
func (m *Manager) SentDeal(ctx context.Context, token string) (module.MatchConfig, string, error) {
	old, err := m.openDealLink(ctx, token)
	if err != nil {
		return module.MatchConfig{}, "", err
	}
	return module.MatchConfig{Variation: old.Variation, Options: old.Options}, old.ModuleID, nil
}

// PlaySentDeal deals a sent game to the player who opened the link. Anybody
// with the link may; whether they had seen the deal is recorded, so the one
// person it changes nothing for — the sender — is not credited a first try.
func (m *Manager) PlaySentDeal(ctx context.Context, token string, host models.Player) (models.Match, error) {
	old, err := m.openDealLink(ctx, token)
	if err != nil {
		return models.Match{}, err
	}
	seen := false
	for _, p := range old.Players {
		if p.ID == host.ID || (host.UserID != "" && p.UserID == host.UserID) {
			seen = true
		}
	}
	return m.dealFrom(ctx, old, host, seen)
}

func (m *Manager) openDealLink(ctx context.Context, token string) (models.Match, error) {
	id := auth.OpenDealLink(token)
	if id == "" {
		return models.Match{}, module.Error{Code: "DEAL_LINK_INVALID"}
	}
	return m.dealSource(ctx, id)
}

// dealSource is a match whose deal may be dealt again.
func (m *Manager) dealSource(ctx context.Context, idOrCode string) (models.Match, error) {
	old, err := m.Current(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	if old.Status != "completed" {
		return models.Match{}, module.Error{Code: "MATCH_NOT_OVER"}
	}
	if !m.savedGame(old) {
		return models.Match{}, module.Error{Code: "NOT_A_SOLO_GAME", Message: old.ModuleID}
	}
	return old, nil
}

// dealFrom opens a table for host dealt with old's cards, and deals it.
func (m *Manager) dealFrom(ctx context.Context, old models.Match, host models.Player, repeat bool) (models.Match, error) {
	lobby, err := m.Create(ctx, old.ModuleID, module.MatchConfig{Variation: old.Variation, Options: old.Options}, host)
	if err != nil {
		return models.Match{}, err
	}
	e, err := m.lockMatch(ctx, lobby.ID.Hex())
	if err != nil {
		return models.Match{}, err
	}
	next := e.match
	next.Seed = old.Seed
	next.DealFrom = old.DealFrom
	if next.DealFrom == "" {
		next.DealFrom = old.ID.Hex()
	}
	next.DealRepeat = repeat
	err = m.saveLocked(ctx, e, next)
	e.mu.Unlock()
	if err != nil {
		return models.Match{}, err
	}
	return m.Start(ctx, lobby.ID.Hex())
}
