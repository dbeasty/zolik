package match

import (
	"context"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// Rematch plays a finished table again: the same game, variation and
// options, the same seat order, and the same opponents — a bot keeps its
// persona and skill, so "again" means against the players who were actually
// there rather than whoever a fresh draw seats.
//
// The first seated human to ask opens it and hosts it. Everybody else from the
// last table has a seat held for them, which they take by asking too: the
// finished table remembers where its rematch is, so every later press lands at
// the same one instead of each opening a table of their own. A table with
// nobody else to wait for is dealt straight away.
//
// The table has to be over — played out, or set aside by the sweeper, where
// starting afresh is the alternative to picking it up.
func (m *Manager) Rematch(ctx context.Context, idOrCode, callerID string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	old := e.match
	if old.Status != "completed" && old.Status != string(rules.StatusAbandoned) {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "MATCH_NOT_OVER"}
	}
	caller := playerByID(old.Players, callerID)
	if caller == nil || caller.IsAI {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "NOT_AT_THIS_TABLE", Message: callerID}
	}
	host := *caller
	host.ConnectionID = ""
	// A seat link belongs to the table it was made for; carried over, the old
	// table's link would open this seat at the new one too.
	host.SeatKeyHash = ""

	// Somebody got there first: sit down at theirs.
	if old.Rematch != nil {
		next := old.Rematch.MatchID
		e.mu.Unlock()
		return m.Join(ctx, next, host)
	}

	next, err := m.openRematch(ctx, old, host)
	if err != nil {
		e.mu.Unlock()
		return models.Match{}, err
	}
	finished := old
	finished.Rematch = &models.RematchRef{MatchID: next.ID.Hex(), HostID: host.ID}
	err = m.saveLocked(ctx, e, finished)
	finished = e.match
	e.mu.Unlock()
	if err != nil {
		return models.Match{}, err
	}
	// Everybody still looking at the result hears about it now, rather than on
	// whatever next moves this table — which, being over, is nothing.
	m.Broadcast(finished)

	if len(next.Reserved) == 0 {
		return m.Start(ctx, next.ID.Hex())
	}
	return next, nil
}

// openRematch creates the new lobby for Rematch, seating the host and every
// bot and holding a seat for every other human.
func (m *Manager) openRematch(ctx context.Context, old models.Match, host models.Player) (models.Match, error) {
	lobby, err := m.Create(ctx, old.ModuleID, module.MatchConfig{Variation: old.Variation, Options: old.Options}, host)
	if err != nil {
		return models.Match{}, err
	}
	e, err := m.lockMatch(ctx, lobby.ID.Hex())
	if err != nil {
		return models.Match{}, err
	}
	defer e.mu.Unlock()

	next := e.match
	next.RematchOf = old.ID.Hex()
	next.SeatOrder = append([]string(nil), old.TurnOrder...)
	next.Players = []models.Player{host}
	for _, id := range old.TurnOrder {
		p := playerByID(old.Players, id)
		switch {
		case p == nil || p.ID == host.ID:
		case p.IsAI:
			// The same bot, down to its seat id: nothing keys a bot's record
			// on that, and keeping it is what lets SeatOrder place it.
			next.Players = append(next.Players, models.Player{
				ID:           p.ID,
				Name:         p.Name,
				IsAI:         true,
				AIDifficulty: p.AIDifficulty,
				AIPersona:    p.AIPersona,
				Avatar:       p.Avatar,
			})
		default:
			next.Reserved = append(next.Reserved, models.Reservation{PlayerID: p.ID, Name: p.Name, Avatar: p.Avatar})
		}
	}
	next.Players, next.TurnOrder = inSeatOrder(next.Players, next.SeatOrder)
	if err := m.saveLocked(ctx, e, next); err != nil {
		return models.Match{}, err
	}
	return e.match, nil
}

// DeclineRematch gives up the seat a rematch was holding for the caller, so
// the host is not left waiting for somebody who has said no. Saying it twice,
// or about a table that is holding nothing for you, is not an error.
func (m *Manager) DeclineRematch(ctx context.Context, idOrCode, playerID string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	i := reservationIndex(e.match.Reserved, playerID)
	if e.match.Status != "lobby" || i < 0 {
		match := e.match
		e.mu.Unlock()
		return match, nil
	}
	next := e.match
	next.Reserved = withoutReservation(next.Reserved, i)
	err = m.saveLocked(ctx, e, next)
	match := e.match
	e.mu.Unlock()
	if err != nil {
		return models.Match{}, err
	}
	m.Broadcast(match)
	return match, nil
}

func reservationIndex(held []models.Reservation, playerID string) int {
	for i, r := range held {
		if r.PlayerID == playerID {
			return i
		}
	}
	return -1
}

func withoutReservation(held []models.Reservation, i int) []models.Reservation {
	out := append(append([]models.Reservation(nil), held[:i]...), held[i+1:]...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// inSeatOrder sorts players by where they sat at the last table, with anybody
// who was not there after them in the order they arrived, and returns the
// matching turn order.
func inSeatOrder(players []models.Player, order []string) ([]models.Player, []string) {
	rank := make(map[string]int, len(order))
	for i, id := range order {
		rank[id] = i
	}
	sorted := make([]models.Player, 0, len(players))
	for _, id := range order {
		if p := playerByID(players, id); p != nil {
			sorted = append(sorted, *p)
		}
	}
	for _, p := range players {
		if _, known := rank[p.ID]; !known {
			sorted = append(sorted, p)
		}
	}
	turns := make([]string, len(sorted))
	for i, p := range sorted {
		turns[i] = p.ID
	}
	return sorted, turns
}
