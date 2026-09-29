package match

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/url"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// SeatPath is the client route a seat link points at: /seat/<match>/<secret>.
const SeatPath = "/seat/"

// A seat link brings one particular person back to one particular seat.
//
// The join link cannot: it seats whoever opens it as themselves, and at a
// table that has started, "themselves" is only somebody already sitting there
// if they still have the device and the storage they sat down with. A player
// on a new phone, or with a cleared browser, is a stranger to their own
// table — and the others can never pick a swept-up game back up, because it
// waits for everybody.
//
// So a seat link names the seat rather than the table. Anybody seated at the
// table may mint one for any person at it, which is how "send Anna her link"
// works. Opening it shows whose seat it is before anything happens, and taking
// it hands out an access token scoped to this match alone (auth.MatchScope):
// it plays that seat and reaches nothing else of the person it names.
//
// A link lives while the table is started and not finished. Minting again
// replaces it, which is how one sent to the wrong person is killed, and a seat
// with somebody connected to it cannot be taken at all.
func seatLinkOpen(status string) bool {
	switch status {
	case "active", "suspended", string(rules.StatusAbandoned):
		return true
	}
	return false
}

func seatKeyHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// MintSeatLink makes a new link for seatID at the table, replacing any
// earlier one, and returns its secret. callerID must be a person seated there.
func (m *Manager) MintSeatLink(ctx context.Context, matchID, callerID, seatID string) (string, error) {
	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return "", module.Error{Code: "MATCH_NOT_FOUND", Message: matchID}
	}
	defer e.mu.Unlock()
	match := e.match
	if !seatLinkOpen(match.Status) {
		return "", module.Error{Code: "SEAT_LINK_EXPIRED", Message: "status is " + match.Status}
	}
	if c := playerByID(match.Players, callerID); c == nil || c.IsAI {
		return "", module.Error{Code: "NOT_AT_THIS_TABLE", Message: callerID}
	}
	target := playerByID(match.Players, seatID)
	if target == nil || target.IsAI {
		return "", module.Error{Code: "NOT_AT_THIS_TABLE", Message: seatID}
	}
	secret, err := auth.NewRandomToken(16)
	if err != nil {
		return "", err
	}
	next := match
	next.Players = append([]models.Player(nil), match.Players...)
	for i := range next.Players {
		if next.Players[i].ID == seatID {
			next.Players[i].SeatKeyHash = seatKeyHash(secret)
		}
	}
	if err := m.saveLocked(ctx, e, next); err != nil {
		return "", err
	}
	return secret, nil
}

// SeatByLink finds the table and seat a link opens, if it still opens one.
func (m *Manager) SeatByLink(ctx context.Context, matchID, secret string) (models.Match, models.Player, error) {
	expired := module.Error{Code: "SEAT_LINK_EXPIRED"}
	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return models.Match{}, models.Player{}, expired
	}
	match := e.match
	e.mu.Unlock()
	if !seatLinkOpen(match.Status) || secret == "" {
		return models.Match{}, models.Player{}, expired
	}
	want := []byte(seatKeyHash(secret))
	for _, p := range match.Players {
		if p.SeatKeyHash != "" && subtle.ConstantTimeCompare([]byte(p.SeatKeyHash), want) == 1 {
			return match, p, nil
		}
	}
	return models.Match{}, models.Player{}, expired
}

// SeatPresent reports whether somebody holds a socket in seatID right now —
// the one case a link refuses, because it would take the seat from a person
// who is playing it.
func (m *Manager) SeatPresent(matchID, seatID string) bool {
	return m.hub != nil && m.hub.Registry().Has(matchID, seatID)
}

// SeatURL is the link for a seat, or "" with no configured base — the client
// then builds it from its own origin, which is right for a web client.
func (m *Manager) SeatURL(matchID, secret string) string {
	if m == nil || m.inviteBaseURL == "" {
		return ""
	}
	return m.inviteBaseURL + SeatPath + url.PathEscape(matchID) + "/" + url.PathEscape(secret)
}
