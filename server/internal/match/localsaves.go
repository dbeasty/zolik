package match

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	zsync "zolik/server/internal/sync"
)

// A match played on a phone stays on the phone until somebody says otherwise.
//
// Handing a finished match to the cloud puts the people who played it into
// somebody else's database: their names, their moves, and - for anybody who
// sat down with an account - a line in that account's history. So nothing is
// handed up on its own. The match waits here, the host decides whether the
// game goes up at all, and each player decides for their own seat only. A
// seat nobody said yes for travels as an anonymous "Player N" that no account
// can ever claim.

// Local save states.
const (
	SavePending  = "pending"
	SaveSaved    = "saved"
	SaveUploaded = "uploaded"
)

// Seat kinds, as the app shows them.
const (
	SeatHost    = "host"
	SeatAccount = "account"
	SeatGuest   = "guest"
	SeatBot     = "bot"
)

// ErrNotEnrolled is returned when a game is saved on a phone that has no
// account to save it to: nobody is signed in, or the install was never
// enrolled. The game stays waiting, and can be saved after signing in.
var ErrNotEnrolled = errors.New("sign in to save games to your account")

// ErrNoSuchSave is returned for a match that is not waiting on this device.
var ErrNoSuchSave = errors.New("no such game on this device")

// ErrGameGone is returned for a game the phone no longer holds: finished
// matches are kept for a while and then reclaimed (see RetentionWindows).
var ErrGameGone = errors.New("this game is no longer on this device")

// ErrNotSeated is returned when somebody answers for a seat that is not theirs.
var ErrNotSeated = errors.New("you did not play in this game")

// LocalSave is one finished match and what its players have said about it.
type LocalSave struct {
	Kind       string     `bson:"_kind" json:"-"`
	Match      string     `bson:"match" json:"matchId"`
	Module     string     `bson:"module" json:"moduleId"`
	Variation  string     `bson:"variation,omitempty" json:"variation,omitempty"`
	FinishedAt time.Time  `bson:"finishedAt" json:"finishedAt"`
	State      string     `bson:"state" json:"state"`
	Seats      []SaveSeat `bson:"seats" json:"seats"`
	// SavedAt is when the host said yes. The game is held back from then
	// until every player has answered, or AnswerGrace has passed.
	SavedAt time.Time `bson:"savedAt,omitempty" json:"savedAt,omitempty"`
}

// AnswerGrace is how long a saved game waits for the players who have not
// answered yet. A host whose phone saves every game the moment it ends would
// otherwise send it before anybody else at the table had read the question,
// and a "yes" that arrives after the cloud has the game counts for nothing.
const AnswerGrace = 2 * time.Minute

// answered reports whether every person at the table has said yes or no.
func (rec LocalSave) answered() bool {
	for _, seat := range rec.Seats {
		if seat.Kind != SeatBot && seat.Consent == nil {
			return false
		}
	}
	return true
}

// SaveSeat is one seat of a waiting match.
type SaveSeat struct {
	Seat string `bson:"seat" json:"seat"`
	Name string `bson:"name" json:"name"`
	Kind string `bson:"kind" json:"kind"`
	// Consent is nil until the player has answered.
	Consent *bool `bson:"consent,omitempty" json:"consent,omitempty"`
	// Subject is who held the seat: the id their session signs in as, which
	// is also the seat's own id on this server. Never sent to the app: it is
	// how an answer is matched to its seat, not something to show.
	Subject string `bson:"subject,omitempty" json:"-"`
}

func saveKey(matchHex string) string { return "save/" + matchHex }
func passKey(userHex string) string  { return "pass/" + userHex }

// LocalSaves is the recorder of an embedded host. It keeps every finished
// match waiting for its players' answers, and hands up only what they agreed
// to.
type LocalSaves struct {
	kdb  *db.KDB
	repo Repository
	// out is nil on a phone that is not enrolled: there games wait, and
	// saving one says to sign in first.
	out *zsync.Outbox
	// inner also records the match, where this host keeps figures of its own.
	inner Recorder
	// uploadable says whether the cloud would take a game at all. A game
	// dealt on the server only is never offered for saving.
	uploadable func(moduleID string) bool
	// grace is AnswerGrace, shorter in tests.
	grace time.Duration
	now   func() time.Time

	mu sync.Mutex
}

// NewLocalSaves returns the recorder an embedded host uses. out and inner may
// be nil.
func NewLocalSaves(k *db.KDB, repo Repository, out *zsync.Outbox, inner Recorder, uploadable func(string) bool) *LocalSaves {
	return &LocalSaves{kdb: k, repo: repo, out: out, inner: inner, uploadable: uploadable,
		grace: AnswerGrace, now: time.Now}
}

// SetAnswerGrace changes how long a saved game waits for unanswered seats.
func (s *LocalSaves) SetAnswerGrace(d time.Duration) { s.grace = d }

// due reports whether a saved game may go now: everybody has answered, or
// nobody else is going to.
func (s *LocalSaves) due(rec LocalSave) bool {
	return rec.answered() || !s.now().Before(rec.SavedAt.Add(s.grace))
}

var _ Recorder = (*LocalSaves)(nil)

// Enrolled reports whether saved games have anywhere to go.
func (s *LocalSaves) Enrolled() bool { return s != nil && s.out != nil }

func (s *LocalSaves) RecordMatchAsync(m models.Match, out module.Outcome) {
	if s.inner != nil {
		s.inner.RecordMatchAsync(m, out)
	}
	if s.uploadable != nil && !s.uploadable(m.ModuleID) {
		return
	}
	go func() {
		if err := s.remember(m); err != nil {
			log.Printf("match=%s: keeping the finished match for its players to save: %v", m.ID.Hex(), err)
		}
	}()
}

// remember writes a waiting record for a finished match, once.
func (s *LocalSaves) remember(m models.Match) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.kdb.Get(db.NSLocalSaves, saveKey(m.ID.Hex())); err == nil {
		return nil
	}
	finished := time.Now().UTC()
	if m.EndedAt != nil {
		finished = *m.EndedAt
	}
	rec := LocalSave{
		Kind:       "save",
		Match:      m.ID.Hex(),
		Module:     m.ModuleID,
		Variation:  m.Variation,
		FinishedAt: finished,
		State:      SavePending,
	}
	for _, p := range m.Players {
		seat := SaveSeat{Seat: p.ID, Name: p.Name}
		if !p.IsAI {
			seat.Subject = p.ID
		}
		switch {
		case p.IsAI:
			seat.Kind = SeatBot
		case p.ID == m.HostID:
			seat.Kind = SeatHost
		case p.UserID != "":
			seat.Kind = SeatAccount
		default:
			seat.Kind = SeatGuest
		}
		rec.Seats = append(rec.Seats, seat)
	}
	return s.put(rec)
}

func (s *LocalSaves) put(rec LocalSave) error {
	rec.Kind = "save"
	doc, err := db.MarshalDoc(rec)
	if err != nil {
		return err
	}
	return s.kdb.Put(db.NSLocalSaves, saveKey(rec.Match), doc)
}

func (s *LocalSaves) get(matchHex string) (LocalSave, error) {
	raw, err := s.kdb.Get(db.NSLocalSaves, saveKey(matchHex))
	if err != nil {
		if db.IsNotFound(err) {
			return LocalSave{}, ErrNoSuchSave
		}
		return LocalSave{}, err
	}
	var rec LocalSave
	if err := db.UnmarshalDoc(raw, &rec); err != nil {
		return LocalSave{}, err
	}
	return rec, nil
}

// List is every game on this device that is waiting or saved, newest first.
func (s *LocalSaves) List() ([]LocalSave, error) {
	var out []LocalSave
	err := s.kdb.Scan(db.NSLocalSaves, func(doc []byte) error {
		var rec LocalSave
		if err := db.UnmarshalDoc(doc, &rec); err != nil {
			return err
		}
		if rec.Kind == "save" {
			out = append(out, rec)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FinishedAt.After(out[j].FinishedAt) })
	return out, nil
}

// Get is one game's record.
func (s *LocalSaves) Get(matchHex string) (LocalSave, error) { return s.get(matchHex) }

// RememberPass keeps the pass an account sat down with, for the matches it
// plays here to carry to the cloud as evidence.
func (s *LocalSaves) RememberPass(subject, pass string) {
	user := strings.TrimPrefix(subject, "user:")
	if user == "" || pass == "" {
		return
	}
	doc, err := db.MarshalDoc(map[string]any{"_kind": "pass", "pass": pass})
	if err == nil {
		err = s.kdb.Put(db.NSLocalSaves, passKey(user), doc)
	}
	if err != nil {
		log.Printf("keeping an offline pass: %v", err)
	}
}

func (s *LocalSaves) passOf(user string) string {
	raw, err := s.kdb.Get(db.NSLocalSaves, passKey(user))
	if err != nil {
		return ""
	}
	var doc struct {
		Pass string `bson:"pass"`
	}
	if db.UnmarshalDoc(raw, &doc) != nil {
		return ""
	}
	return doc.Pass
}

// Save is the host saying yes: the game goes to the cloud, with the host's own
// seat and every seat whose player has said yes so far. A player who answers
// later still counts, until the cloud has the game.
func (s *LocalSaves) Save(ctx context.Context, matchHex string) (LocalSave, error) {
	if s.out == nil {
		return LocalSave{}, ErrNotEnrolled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.get(matchHex)
	if err != nil {
		return LocalSave{}, err
	}
	if rec.State == SaveUploaded {
		return rec, nil
	}
	yes := true
	for i := range rec.Seats {
		if rec.Seats[i].Kind == SeatHost {
			rec.Seats[i].Consent = &yes
		}
	}
	rec.State = SaveSaved
	if rec.SavedAt.IsZero() {
		rec.SavedAt = s.now().UTC()
	}
	if err := s.put(rec); err != nil {
		return LocalSave{}, err
	}
	// Somebody who has not answered yet still gets to: see AnswerGrace.
	// Reconcile sends it once they have, or once the wait is over.
	if !s.due(rec) {
		return rec, nil
	}
	return rec, s.hand(ctx, rec)
}

// Discard forgets a game: it will never be offered for saving again.
func (s *LocalSaves) Discard(matchHex string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.get(matchHex)
	if err != nil {
		return err
	}
	if rec.State == SaveSaved && s.out != nil {
		_ = s.out.Drop(matchHex)
	}
	_, err = s.kdb.Delete(db.NSLocalSaves, saveKey(matchHex))
	return err
}

// Consent records one player's answer for their own seat. subject is who is
// asking, as their session names them.
func (s *LocalSaves) Consent(ctx context.Context, matchHex, subject string, yes bool) (LocalSave, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := s.get(matchHex)
	if err != nil {
		return LocalSave{}, err
	}
	found := false
	for i := range rec.Seats {
		if subject != "" && rec.Seats[i].Subject == subject && rec.Seats[i].Kind != SeatBot {
			answer := yes
			rec.Seats[i].Consent = &answer
			found = true
		}
	}
	if !found {
		return LocalSave{}, ErrNotSeated
	}
	if err := s.put(rec); err != nil {
		return LocalSave{}, err
	}
	// Saved already: send it if this was the last answer it waited for, or
	// send it again with the new answer if it had gone. The outbox is keyed by
	// match, so this replaces a copy the cloud has not taken yet.
	if rec.State == SaveSaved && (s.due(rec) || s.out.Has(rec.Match)) {
		return rec, s.hand(ctx, rec)
	}
	return rec, nil
}

// Reconcile settles saved games against what the cloud has confirmed. acked
// reports whether the cloud has a match; one it has is dropped from the
// outbox, and one that went missing from the outbox without being confirmed is
// handed again.
func (s *LocalSaves) Reconcile(ctx context.Context, acked func(matchHex string) bool) {
	if s == nil || s.out == nil {
		return
	}
	list, err := s.List()
	if err != nil {
		log.Printf("local saves: %v", err)
		return
	}
	for _, rec := range list {
		if rec.State != SaveSaved {
			continue
		}
		s.mu.Lock()
		switch {
		case acked != nil && acked(rec.Match):
			if err := s.out.Drop(rec.Match); err != nil {
				log.Printf("match=%s: dropping the uploaded bundle: %v", rec.Match, err)
			}
			rec.State = SaveUploaded
			if err := s.put(rec); err != nil {
				log.Printf("match=%s: %v", rec.Match, err)
			}
		case !s.out.Has(rec.Match) && s.due(rec):
			err := s.hand(ctx, rec)
			if errors.Is(err, ErrGameGone) {
				// Reclaimed before it ever went: nothing left to send.
				_, _ = s.kdb.Delete(db.NSLocalSaves, saveKey(rec.Match))
				break
			}
			if err != nil {
				log.Printf("match=%s: handing the saved match up again: %v", rec.Match, err)
			}
		}
		s.mu.Unlock()
	}
}

// hand builds the bundle with only what the players agreed to, and puts it in
// the outbox.
func (s *LocalSaves) hand(ctx context.Context, rec LocalSave) error {
	id, err := bson.ObjectIDFromHex(rec.Match)
	if err != nil {
		return err
	}
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return ErrGameGone
		}
		return fmt.Errorf("match %s: %w", rec.Match, err)
	}
	b, err := BundleOf(ctx, s.repo, m)
	if err != nil {
		return err
	}
	if err := s.applyConsent(&b, m, rec); err != nil {
		return err
	}
	return s.out.Hand(b)
}

// applyConsent rewrites a bundle so that it says no more than its players
// agreed to. A seat whose player said yes keeps who they are; any other human
// seat becomes an anonymous player, with a name that is only a number and a
// guest id nobody holds a receipt for.
func (s *LocalSaves) applyConsent(b *zsync.Bundle, m models.Match, rec LocalSave) error {
	agreed := map[string]bool{}
	for _, seat := range rec.Seats {
		if seat.Consent != nil && *seat.Consent {
			agreed[seat.Seat] = true
		}
	}
	envelope := m
	envelope.Players = append([]models.Player(nil), m.Players...)
	b.Passes = nil
	for n, p := range envelope.Players {
		if p.IsAI {
			continue
		}
		if agreed[p.ID] {
			if p.UserID != "" {
				user := strings.TrimPrefix(subjectKeyForUser(p.UserID), "user:")
				if pass := s.passOf(user); pass != "" {
					if b.Passes == nil {
						b.Passes = map[string]string{}
					}
					b.Passes[p.ID] = pass
				}
			}
			continue
		}
		anon := AnonymousGuestID(m.ID.Hex(), p.ID)
		envelope.Players[n].UserID = ""
		envelope.Players[n].GuestID = anon
		envelope.Players[n].Name = "Player " + strconv.Itoa(n+1)
		envelope.Players[n].Avatar = ""
		b.Seats[p.ID] = "guest:" + anon
	}
	if agreed := hostAgreed(envelope, rec); !agreed {
		envelope.HostID = ""
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	b.Envelope = raw
	return nil
}

// hostAgreed is whether the seat the match names as its host is one whose
// player said yes; otherwise the host's id would name them in the cloud copy.
func hostAgreed(m models.Match, rec LocalSave) bool {
	for _, seat := range rec.Seats {
		if seat.Kind == SeatHost {
			return seat.Consent != nil && *seat.Consent
		}
	}
	return true
}

// AnonymousGuestID is the guest id a seat travels under when its player did
// not agree to be named. It is derived rather than random, so handing the
// same match up twice says the same thing, and it is the shape of a guest id
// so the cloud stores it like any other - but no device holds a receipt for
// it, so no account can ever claim it.
func AnonymousGuestID(matchHex, seat string) string {
	sum := sha256.Sum256([]byte("zolik:anonymous-seat:" + matchHex + "/" + seat))
	return hex.EncodeToString(sum[:16])
}
