package match

import (
	"context"
	"log"
	"strings"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Handing a seat to somebody else.
//
// A stand-in keeps a game going for somebody who has gone; sometimes they are
// not coming back, and a person in the room would rather play the seat than
// watch a bot do it. The host hands the seat over: a seat link (seatlink.go)
// made for whoever is going to take it. They open it, give their name, and
// play the seat from then on under that name — the cards, the score and the
// turn order are the seat's and stay where they are.
//
// The seat stops being its first player's. Their own sign-in no longer sits
// there (handleWS refuses it), so the two cannot fight over one seat; and the
// match counts on nobody's record, theirs or the newcomer's, because neither
// played all of it (stats: Standing.StandIn).

// HandOverSeat makes the link that gives seatID to somebody else. Only the
// host may, and only for a person's seat whose player is away.
func (m *Manager) HandOverSeat(ctx context.Context, matchID, callerID, seatID string) (string, error) {
	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return "", module.Error{Code: "MATCH_NOT_FOUND", Message: matchID}
	}
	defer e.mu.Unlock()
	match := e.match
	if match.HostID != callerID {
		return "", module.Error{Code: "NOT_THE_HOST", Message: callerID}
	}
	if match.Status != "active" && match.Status != "suspended" {
		return "", module.Error{Code: "MATCH_NOT_ACTIVE"}
	}
	target := playerByID(match.Players, seatID)
	if target == nil || target.IsAI || seatID == callerID {
		return "", module.Error{Code: "STAND_IN_NOT_A_PERSON", Message: seatID}
	}
	if m.seatHere(match.ID.Hex(), seatID) {
		return "", module.Error{Code: "STAND_IN_PLAYER_HERE", Message: seatID}
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
			next.Players[i].PendingHandover = true
		}
	}
	if err := m.saveLocked(ctx, e, next); err != nil {
		return "", err
	}
	return secret, nil
}

// TakeOverSeat is the newcomer taking a seat handed to them, under name.
func (m *Manager) TakeOverSeat(ctx context.Context, matchID, seatID, name string) (models.Match, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.Match{}, module.Error{Code: "NAME_REQUIRED"}
	}
	if len([]rune(name)) > 24 {
		name = string([]rune(name)[:24])
	}
	e, err := m.lockMatch(ctx, matchID)
	if err != nil {
		return models.Match{}, module.Error{Code: "MATCH_NOT_FOUND", Message: matchID}
	}
	match := e.match
	target := playerByID(match.Players, seatID)
	if target == nil || !target.PendingHandover {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "SEAT_LINK_EXPIRED"}
	}
	next := match
	next.Players = append([]models.Player(nil), match.Players...)
	for i := range next.Players {
		p := &next.Players[i]
		if p.ID != seatID {
			continue
		}
		if p.FormerName == "" {
			p.FormerName = p.Name
		}
		p.Name = name
		p.PendingHandover, p.HandedOver = false, true
		p.StandIn = nil
	}
	if !contains(next.HandedOver, seatID) {
		next.HandedOver = append(append([]string(nil), next.HandedOver...), seatID)
	}
	if next.Status == "suspended" && next.SuspendedPlayer == seatID {
		next = unsuspended(next)
	}
	if err := m.saveSeats(ctx, e, next, map[string]bool{seatID: false}); err != nil {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "MATCH_MOVED_ON", Message: err.Error()}
	}
	saved := e.match
	e.mu.Unlock()
	m.turns.forgetSeat(matchID, seatID)
	m.awayMu.Lock()
	delete(m.standInHeld, matchID+"|"+seatID)
	m.awayMu.Unlock()
	log.Printf("match=%s seat=%s handed over to %q", matchID, seatID, name)
	m.Broadcast(saved)
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), matchID)
	return saved, nil
}

// SeatHandedOver reports whether a seat now belongs to whoever took it over.
func (m *Manager) SeatHandedOver(ctx context.Context, matchID, seatID string) bool {
	match, err := m.current(ctx, matchID)
	if err != nil {
		return false
	}
	p := playerByID(match.Players, seatID)
	return p != nil && p.HandedOver
}

// errSeatHandedOver is what a seat's first player's socket is told once the
// seat has been handed to somebody else.
var errSeatHandedOver = module.Error{Code: "SEAT_HANDED_OVER"}
