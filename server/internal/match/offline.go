package match

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	zsync "zolik/server/internal/sync"
)

// A match played away from the cloud has to reach it eventually, and the cloud
// has to be able to believe what it is told about it.
//
// The node that hosted the match hands up what happened: the shuffle, every
// move in order, and who sat in which seat. The cloud replays it with the same
// rules the game was played under and checks that the moves it was handed
// really do produce the outcome the bundle claims. That is the whole of the
// trust story, and it is why a phone can be allowed to report a win at all: it
// is not being believed, it is being checked.

// Outbox is what a node that is not the cloud hands finished matches to.
type Outbox interface {
	Hand(b zsync.Bundle) error
}

// HandUp records a finished match by handing it to this node's outbox instead
// of writing a result nobody else would ever see.
//
// It replaces the statistics recorder on a spoke. A phone has no leaderboard
// and no lifetime figures of its own: those are the cloud's, computed from
// records the cloud wrote, and a phone that kept its own would be keeping a
// second set of numbers that quietly disagreed with the ones the player sees
// everywhere else.
type HandUp struct {
	repo Repository
	out  Outbox
}

// NewHandUp returns the recorder a spoke uses.
func NewHandUp(repo Repository, out Outbox) *HandUp { return &HandUp{repo: repo, out: out} }

var _ Recorder = (*HandUp)(nil)

func (h *HandUp) RecordMatchAsync(m models.Match, out module.Outcome) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.Hand(ctx, m); err != nil {
			// The match is not lost: it is still in its own namespace, and the
			// next attempt - a restart, or the next match that finishes - will
			// find it unhanded and try again.
			log.Printf("match=%s: handing the finished match up: %v", m.ID.Hex(), err)
		}
	}()
}

// Hand builds the bundle for a finished match and puts it in the outbox.
func (h *HandUp) Hand(ctx context.Context, m models.Match) error {
	b, err := BundleOf(ctx, h.repo, m)
	if err != nil {
		return err
	}
	return h.out.Hand(b)
}

// BundleOf gathers everything the cloud needs to check a match for itself.
func BundleOf(ctx context.Context, repo Repository, m models.Match) (zsync.Bundle, error) {
	moves, err := repo.Moves(ctx, m.ID, 0, -1)
	if err != nil {
		return zsync.Bundle{}, fmt.Errorf("match %s: reading its moves: %w", m.ID.Hex(), err)
	}
	encoded := make([]json.RawMessage, 0, len(moves))
	for _, mv := range moves {
		raw, err := json.Marshal(mv)
		if err != nil {
			return zsync.Bundle{}, err
		}
		encoded = append(encoded, raw)
	}
	envelope, err := json.Marshal(m)
	if err != nil {
		return zsync.Bundle{}, err
	}
	finished := time.Now().UTC()
	if m.EndedAt != nil {
		finished = *m.EndedAt
	}
	return zsync.Bundle{
		Match:      m.ID.Hex(),
		Module:     m.ModuleID,
		Variation:  m.Variation,
		Seed:       m.Seed,
		Envelope:   envelope,
		Moves:      encoded,
		Seats:      seatsOf(m),
		FinishedAt: finished,
	}, nil
}

// seatsOf maps each seat to the subject that held it, in the spelling the rest
// of the server uses for a subject key.
func seatsOf(m models.Match) map[string]string {
	seats := map[string]string{}
	for _, p := range m.Players {
		switch {
		case p.UserID != "":
			// A host that seated somebody from an offline pass has already
			// recorded them as a subject rather than as a bare account id:
			// the prefix is how that host says "I decided you were this
			// account". Prefixing it a second time yields "user:user:<hex>",
			// which is nobody's subject key and which names a namespace the
			// database refuses - so the match arrives at the cloud and takes
			// the recorder down with it.
			seats[p.ID] = subjectKeyForUser(p.UserID)
		case p.GuestID != "":
			seats[p.ID] = "guest:" + p.GuestID
		case p.IsAI:
			seats[p.ID] = "ai:" + aiKey(p)
		}
	}
	return seats
}

// Importer is the cloud's half: it replays a handed-up match, and records it
// only if the replay agrees with what was handed up.
type Importer struct {
	repo     Repository
	registry *module.Registry
	recorder Recorder
	// claims maps a guest id to the account that has claimed it, so a match
	// played as a guest before signing in is credited to the person who
	// played it.
	claims GuestClaims
}

// GuestClaims answers which account, if any, a guest has become.
type GuestClaims interface {
	AccountForGuest(ctx context.Context, guestID string) (string, bool, error)
}

// NewImporter returns the cloud's importer of handed-up matches.
func NewImporter(repo Repository, registry *module.Registry, recorder Recorder, claims GuestClaims) *Importer {
	return &Importer{repo: repo, registry: registry, recorder: recorder, claims: claims}
}

var _ zsync.MatchRecorder = (*Importer)(nil)

// ImportBundle checks a handed-up match and records it.
//
// The check is the replay. A node that simply announced an outcome would be
// asking to be believed; a node that hands over a shuffle and a list of moves
// is handing over something that can be played out again here, with the same
// rules, and compared. A bundle whose moves do not produce what it claims is
// refused, and the importer's caller keeps it with its reason rather than
// dropping it, because a refused match is usually a bug rather than a cheat
// and somebody will want to look at it.
func (i *Importer) ImportBundle(ctx context.Context, b zsync.Bundle) error {
	id, err := bson.ObjectIDFromHex(b.Match)
	if err != nil {
		return fmt.Errorf("bundle names %q, which is not a match id", b.Match)
	}
	var envelope models.Match
	if err := json.Unmarshal(b.Envelope, &envelope); err != nil {
		return fmt.Errorf("match %s: its envelope does not decode: %w", b.Match, err)
	}
	if envelope.ID != id {
		return fmt.Errorf("match %s: its envelope is match %s", b.Match, envelope.ID.Hex())
	}
	if envelope.Status != "completed" {
		return fmt.Errorf("match %s: handed up as %q rather than completed", b.Match, envelope.Status)
	}
	mod := i.registry.Get(b.Module)
	if mod == nil {
		return fmt.Errorf("match %s: this server has no %q", b.Match, b.Module)
	}

	moves := make([]models.MatchAction, 0, len(b.Moves))
	for n, raw := range b.Moves {
		var mv models.MatchAction
		if err := json.Unmarshal(raw, &mv); err != nil {
			return fmt.Errorf("match %s: move %d does not decode: %w", b.Match, n+1, err)
		}
		moves = append(moves, mv)
	}

	state, err := mod.NewMatch(
		module.MatchConfig{Variation: envelope.Variation, Options: envelope.Options},
		playerRefs(envelope.Players), b.Seed)
	if err != nil {
		return fmt.Errorf("match %s: dealing it again: %w", b.Match, err)
	}
	for _, mv := range moves {
		var a module.Action
		if err := json.Unmarshal(mv.Action, &a); err != nil {
			return fmt.Errorf("match %s: move %d does not decode: %w", b.Match, mv.Seq, err)
		}
		next, _, err := mod.Apply(state, mv.PlayerID, a)
		if err != nil {
			return fmt.Errorf("match %s: move %d does not apply: %w", b.Match, mv.Seq, err)
		}
		state = next
	}
	outcome := module.OutcomeOf(mod, state)
	if len(outcome.Standings) == 0 {
		return fmt.Errorf("match %s: replaying its moves does not finish the game", b.Match)
	}

	// Who sat in each seat is the node's word, and the one part of the bundle
	// that cannot be checked by replaying anything: the cloud was not in the
	// room. What it can do is refuse to credit an account that did not prove
	// it was there, which is what the seat attestations behind Seats are for,
	// and carry a guest's seat to the account that has since claimed it.
	credited, err := i.credit(ctx, envelope, b.Seats)
	if err != nil {
		return err
	}
	credited.Status = "completed"
	credited.State = nil

	if err := i.store(ctx, credited, moves, state); err != nil {
		return err
	}
	i.recorder.RecordMatchAsync(credited, outcome)
	return nil
}

// credit rewrites the envelope's seats from what the hosting node attested,
// applying any claim a guest has since made.
func (i *Importer) credit(ctx context.Context, envelope models.Match, seats map[string]string) (models.Match, error) {
	out := envelope
	out.Players = append([]models.Player(nil), envelope.Players...)
	for n, p := range out.Players {
		subject, ok := seats[p.ID]
		if !ok {
			continue
		}
		user, guest, persona := splitSubject(subject)
		// Who sat where is the hosting node's word, and a word is not a
		// shape: these ids become subject keys, document ids and the names of
		// namespaces, so a bundle naming something that is not an id is
		// refused here rather than further in, where the database's answer to
		// an impossible namespace name is to take the process down.
		if user != "" && !isAccountID(user) {
			return models.Match{}, fmt.Errorf("match %s: seat %s names %q, which is not an account", envelope.ID.Hex(), p.ID, user)
		}
		if guest != "" && !isGuestID(guest) {
			return models.Match{}, fmt.Errorf("match %s: seat %s names %q, which is not a guest", envelope.ID.Hex(), p.ID, guest)
		}
		switch {
		case user != "":
			out.Players[n].UserID, out.Players[n].GuestID = user, ""
		case guest != "":
			out.Players[n].UserID, out.Players[n].GuestID = "", guest
			if i.claims == nil {
				continue
			}
			account, claimed, err := i.claims.AccountForGuest(ctx, guest)
			if err != nil {
				return models.Match{}, err
			}
			if claimed {
				// The person who played this hand has signed in since. The
				// match is theirs, and waiting for a later reconciliation pass
				// would mean it appeared in their history hours after they
				// asked where it was.
				out.Players[n].UserID, out.Players[n].GuestID = account, ""
			}
		case persona != "":
			out.Players[n].IsAI = true
		}
	}
	return out, nil
}

// subjectKeyForUser is the subject key for a seat held by an account, whether
// the host wrote the id bare (the cloud's spelling) or already as a subject
// (an offline host's, see auth.OfflineSeatPrefix).
func subjectKeyForUser(userID string) string {
	if strings.HasPrefix(userID, "user:") {
		return userID
	}
	return "user:" + userID
}

// aiKey is a bot's durable identity: its persona where it has one, and its
// difficulty for a bot seated before personas existed.
func aiKey(p models.Player) string {
	if p.AIPersona != "" {
		return p.AIPersona
	}
	return p.AIDifficulty
}

// isAccountID and isGuestID are the shapes the cloud mints, and the only ones
// a handed-up seat may name: an account is an ObjectID hex, a guest id is
// thirty-two hex characters (see auth's sanitizeGuestID, which mints them).
func isAccountID(s string) bool { return isLowerHex(s, 24) }

func isGuestID(s string) bool { return isLowerHex(s, 32) }

func isLowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func splitSubject(subject string) (user, guest, persona string) {
	switch {
	case len(subject) > 5 && subject[:5] == "user:":
		return subject[5:], "", ""
	case len(subject) > 6 && subject[:6] == "guest:":
		return "", subject[6:], ""
	case len(subject) > 3 && subject[:3] == "ai:":
		return "", "", subject[3:]
	}
	return "", "", ""
}

// store writes the imported match where every other match lives, so that it is
// read, listed and replayed by exactly the same code as one played here.
func (i *Importer) store(ctx context.Context, m models.Match, moves []models.MatchAction, final module.State) error {
	switch _, err := i.repo.FindByID(ctx, m.ID); {
	case err == nil:
		// Already here: the node handed it up twice, which is what a node that
		// never heard an acknowledgement does.
		return nil
	case !db.IsNotFound(err):
		return err
	}
	stored, err := i.repo.Insert(ctx, m)
	if err != nil {
		return err
	}
	m.Version = stored.Version
	seq := len(moves)
	m.Snapshots = []int{seq}
	state, err := json.Marshal(final)
	if err != nil {
		return err
	}
	return i.repo.CommitSnapshot(ctx, m.ID, stored.Version, m, moves, seq, models.JSONDoc(state))
}
