package stats

import (
	"context"
	"errors"
	"fmt"

	"zolik/server/internal/models"
)

// ForgetUser takes a deleted account out of the statistics.
//
// Its own lifetime record goes. The matches it played stay, because every
// other player at those tables has them in their history too, and deleting
// one person's account must not take a game out of somebody else's record.
// What changes is who the seat says played it: it becomes an anonymous guest
// named for its place at the table ("Player 2"), the same seat a player who
// declined to be named in a local match gets (see models.AnonymousGuestID).
// Nothing in the record leads back to the account afterwards.
//
// The opponents' own records keep their totals — they did play those games —
// but lose their head-to-head line against the account, which is the one
// place a lifetime record names somebody else.
//
// It is safe to run again: a second pass finds no match under the account
// and no head-to-head line naming it.
func (c *Claimer) ForgetUser(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("user id is required")
	}
	userKey := Subject{Kind: SubjectUser, ID: userID}.Key()

	opponents := map[string]bool{}
	err := c.repo.EachMatchForSubject(ctx, userKey, func(m MatchResult) error {
		if !anonymiseSubject(&m, userKey) {
			return nil
		}
		for _, seat := range m.Participants {
			if seat.Subject.Durable() {
				opponents[seat.Subject.Key()] = true
			}
		}
		if err := c.repo.ReplaceMatchAttribution(ctx, m); err != nil {
			return fmt.Errorf("anonymise match %s: %w", m.ID.Hex(), err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for key := range opponents {
		ps, err := c.repo.FindPlayerStats(ctx, key)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if _, ok := ps.HeadToHead[userKey]; !ok {
			continue
		}
		delete(ps.HeadToHead, userKey)
		if err := c.repo.UpsertPlayerStats(ctx, ps); err != nil {
			return fmt.Errorf("drop head-to-head from %s: %w", key, err)
		}
	}

	return c.repo.DeletePlayerStats(ctx, userKey)
}

// anonymiseSubject turns every seat held by key into an anonymous guest and
// refreshes the match's flat key list. It reports whether anything changed.
//
// Unlike a claim (rewriteSubject), the seat does not keep the name it was
// played under: the name is what would still say who it was.
func anonymiseSubject(m *MatchResult, key string) bool {
	changed := false
	for i, seat := range m.Participants {
		if seat.Subject.Key() != key {
			continue
		}
		name := models.AnonymousSeatName(seat.Seat)
		m.Participants[i].Subject = Subject{
			Kind: SubjectGuest,
			ID:   models.AnonymousGuestID(m.MatchID.Hex(), seat.PlayerID),
			Name: name,
		}
		m.Participants[i].Name = name
		changed = true
	}
	if !changed {
		return false
	}
	m.SubjectKeys = subjectKeysOf(m.Participants)
	return true
}
