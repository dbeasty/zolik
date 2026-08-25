package stats

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// Claimer moves a guest's play history onto a real account.
//
// This is the payoff of the whole "sign in to keep your statistics" promise:
// somebody plays a dozen matches as a guest, decides to make an account, and
// finds their record already there instead of starting from zero.
//
// It works because the guest was never anonymous to the *server* — every match
// they played was recorded against their device's durable guest id (see
// Subject and models.Session.GuestID). Claiming is therefore a re-attribution
// of records that already exist, not a reconstruction of lost history.
//
// It lives in this package rather than in auth because the match record is
// this package's data and its shape is this package's business. auth reaches
// it through a narrow interface, the same way the game manager reaches the
// Recorder.
type Claimer struct {
	repo Repository
}

func NewClaimer(repo Repository) *Claimer { return &Claimer{repo: repo} }

// ClaimGuestHistory re-attributes every match the given guest id played to the
// given user, then rebuilds that user's lifetime record from scratch.
//
// The rebuild is a full recomputation rather than an incremental fold, and
// deliberately so: the aggregates track streaks, firsts and bests, which are
// order-dependent, and the claimed matches are interleaved in time with any
// the account already played. There is no correct way to add them one at a
// time to an existing record. Recomputing from the match records — which the
// package already treats as the source of truth the aggregates are merely a
// cache of — is both simpler and provably consistent.
//
// It returns the number of matches claimed. Claiming nothing is a success:
// a guest who never finished a match has nothing to move, and the caller
// should not treat that as a failure to sign in.
func (c *Claimer) ClaimGuestHistory(ctx context.Context, guestID, userID, username string) (int, error) {
	if guestID == "" || userID == "" {
		return 0, errors.New("guest id and user id are both required")
	}
	guest := Subject{Kind: SubjectGuest, ID: guestID}
	user := Subject{Kind: SubjectUser, ID: userID, Name: username}
	guestKey, userKey := guest.Key(), user.Key()

	claimed := 0
	err := c.repo.EachMatchForSubject(ctx, guestKey, func(m MatchResult) error {
		// A table where this account *and* this guest id both sat is left
		// alone. It is a rare shape — the same person signed in on one device
		// and playing as a guest on another, at the same table — but merging
		// the two seats would put one subject twice in one match, which would
		// double-count it in every tally and record a head-to-head against
		// itself. Leaving the guest seat as a guest keeps the record honest;
		// the cost is one unclaimed match, and the person still sees it in
		// their history under the seat they were signed in on.
		for _, k := range m.SubjectKeys {
			if k == userKey {
				return nil
			}
		}
		if !rewriteSubject(&m, guestKey, user) {
			return nil
		}
		if err := c.repo.ReplaceMatchAttribution(ctx, m); err != nil {
			return fmt.Errorf("re-attribute match %s: %w", m.ID.Hex(), err)
		}
		claimed++
		return nil
	})
	if err != nil {
		return claimed, err
	}

	if err := c.RebuildPlayerStats(ctx, user); err != nil {
		return claimed, err
	}
	return claimed, nil
}

// rewriteSubject swaps every seat held by oldKey over to the new subject and
// refreshes the match's flat key list. It reports whether anything changed, so
// a match that somehow lacks the seat is skipped rather than rewritten to an
// identical value.
func rewriteSubject(m *MatchResult, oldKey string, to Subject) bool {
	changed := false
	for i := range m.Participants {
		if m.Participants[i].Subject.Key() != oldKey {
			continue
		}
		// The seat keeps the name it was played under: the standings are a
		// snapshot of that evening's table, and rewriting the display name
		// would quietly rewrite history that other players saw.
		named := to
		named.Name = m.Participants[i].Subject.Name
		if named.Name == "" {
			named.Name = to.Name
		}
		m.Participants[i].Subject = named
		changed = true
	}
	if !changed {
		return false
	}
	m.SubjectKeys = subjectKeysOf(m.Participants)
	return true
}

// subjectKeysOf rebuilds the flat, indexed key list from the participants —
// the same derivation BuildMatchResult performs, kept in one place so the two
// cannot drift.
func subjectKeysOf(participants []Standing) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range participants {
		k := s.Subject.Key()
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RebuildPlayerStats recomputes a subject's lifetime record from every match
// record that names it, replacing whatever was stored.
//
// This is the repair tool the Recorder's own comments assume exists: it treats
// the match records as the source of truth and the aggregate as a derived
// cache, so a record left stale by a failed aggregate write — or by a claim —
// can always be made correct again. It is also the reason a partial failure
// during recording is an acceptable trade rather than data loss.
func (c *Claimer) RebuildPlayerStats(ctx context.Context, subject Subject) error {
	if !subject.Durable() {
		return fmt.Errorf("subject %q keeps no lifetime record", subject.Key())
	}
	key := subject.Key()
	ps := ZeroStats(subject)
	now := time.Now().UTC()

	// Oldest first: streaks, first-match and best/worst are order-dependent,
	// so replaying in completion order is what makes the rebuilt record
	// identical to one that had been folded in live.
	err := c.repo.EachMatchForSubject(ctx, key, func(m MatchResult) error {
		for _, seat := range m.Participants {
			// Every matching seat is folded in separately — one subject can
			// hold several seats at one table (two bots of a difficulty), and
			// each played its own hand.
			if seat.Subject.Key() == key {
				ps = ApplyMatch(ps, m, seat, now)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.repo.UpsertPlayerStats(ctx, ps)
}

// GuestMatchCount reports how many finished matches are recorded against a
// guest id. It backs the "you have N games on this device — sign in to keep
// them" prompt, which needs a number before the person has committed to
// anything.
func (c *Claimer) GuestMatchCount(ctx context.Context, guestID string) (int, error) {
	if guestID == "" {
		return 0, nil
	}
	return c.repo.CountMatchesForSubject(ctx, Subject{Kind: SubjectGuest, ID: guestID}.Key())
}
