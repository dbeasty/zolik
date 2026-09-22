// Package replica serves a person their own data out of the copy their device
// holds, rather than out of the cloud.
//
// A phone that has synced is not a cache of the server: it holds the user's
// namespace and the read-only one the cloud writes for them, which between
// them are their preferences, their circle, their devices, their scorepads,
// their lifetime figures and the index of every match they have played. All
// of that can be answered on a train with no signal, and none of it needs the
// server to be reachable.
//
// What it deliberately cannot answer is anything that is about other people:
// the waiting room, the leaderboard, finding a friend by code. Those are
// questions about data this device does not have and should not have, and a
// device that answered them from what it happens to hold would be answering
// them wrongly.
package replica

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
)

// Reader answers questions about one person from what this device holds.
type Reader struct {
	kdb  *db.KDB
	user string
}

// NewReader returns a reader over an account's replica. user is the account's
// ObjectID hex, which is what its namespaces are named after.
func NewReader(k *db.KDB, userHex string) *Reader {
	return &Reader{kdb: k, user: strings.TrimSpace(userHex)}
}

// ErrNoReplica is what every read answers with when this device holds nothing
// for the account: nobody has signed in, or the first sync has not finished.
// It is a different answer from "you have no friends yet", and the client
// shows it differently.
var ErrNoReplica = errors.New("replica: this device holds nothing for this account yet")

// Ready reports whether there is anything to serve.
func (r *Reader) Ready() bool {
	if r == nil || r.user == "" {
		return false
	}
	return r.kdb.HoldsNamespace(db.UserNS(r.user)) || r.kdb.HoldsNamespace(db.UserReadOnlyNS(r.user))
}

// Prefs is the account's settings as last synced.
func (r *Reader) Prefs() (json.RawMessage, error) {
	return r.one(db.UserNS(r.user), db.PrefsKey)
}

// Profile is the notify profile: the friend code, and what the person chose to
// be reachable as.
func (r *Reader) Profile() (json.RawMessage, error) {
	return r.one(db.UserNS(r.user), "profile")
}

// Stats is the lifetime figures the cloud computed. They are read-only here
// for a reason worth being explicit about: a device that could write its own
// rating could invent one.
func (r *Reader) Stats() (json.RawMessage, error) {
	return r.one(db.UserReadOnlyNS(r.user), "stats")
}

// Circle is everybody in the person's circle.
func (r *Reader) Circle() ([]json.RawMessage, error) {
	return r.many(db.UserNS(r.user), db.KindCircle)
}

// Devices is the person's registered devices.
func (r *Reader) Devices() ([]json.RawMessage, error) {
	return r.many(db.UserNS(r.user), db.KindDevice)
}

// Scoring is the person's scorepads.
func (r *Reader) Scoring() ([]json.RawMessage, error) {
	return r.many(db.UserNS(r.user), db.KindScoring)
}

// Matches is the index of matches this account has played, newest first. It is
// what the history screen lists offline, and what tells the device which match
// namespaces it may ask the hub for.
func (r *Reader) Matches() ([]json.RawMessage, error) {
	rows, err := r.many(db.UserReadOnlyNS(r.user), db.KindMatch)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return completedAt(rows[i]) > completedAt(rows[j])
	})
	return rows, nil
}

// completedAt is the sort key of a match index entry: whatever its completion
// time serialises to, compared as text. The encoding sorts chronologically for
// the two shapes a time is written in, and an entry with no time sorts last,
// which is where a match still being played belongs in a history list.
func completedAt(row json.RawMessage) string {
	var fields struct {
		CompletedAt json.RawMessage `json:"completedAt"`
	}
	if err := json.Unmarshal(row, &fields); err != nil {
		return ""
	}
	return string(fields.CompletedAt)
}

// asClientJSON turns a stored document into the JSON the rest of the API
// speaks.
//
// Documents are stored as the extended JSON the bson encoding writes, where an
// id is {"$oid":"..."} and a time is {"$date":...}. That is right for storage
// and wrong for a client: every other endpoint this app talks to answers with
// plain JSON, and a screen that had to understand both depending on whether
// the device was online would be a screen with two bugs in it.
func asClientJSON(doc []byte) (json.RawMessage, error) {
	var fields bson.M
	if err := db.UnmarshalDoc(doc, &fields); err != nil {
		return nil, err
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Reader) one(namespace, key string) (json.RawMessage, error) {
	if r == nil || r.user == "" {
		return nil, ErrNoReplica
	}
	if !r.kdb.HoldsNamespace(namespace) {
		return nil, ErrNoReplica
	}
	doc, err := r.kdb.Get(namespace, key)
	if err != nil {
		return nil, err
	}
	return asClientJSON(doc)
}

// many returns every document of one kind in a namespace. A scan of one
// person's namespace is a scan of a few dozen documents, which is what naming
// namespaces after people buys: no index, and no filtering of other people's
// rows, because there are none in here.
func (r *Reader) many(namespace, kind string) ([]json.RawMessage, error) {
	if r == nil || r.user == "" {
		return nil, ErrNoReplica
	}
	out := []json.RawMessage{}
	if !r.kdb.HoldsNamespace(namespace) {
		return out, nil
	}
	err := r.kdb.Scan(namespace, func(doc []byte) error {
		var probe struct {
			Kind string `json:"_kind"`
		}
		if err := json.Unmarshal(doc, &probe); err != nil {
			return err
		}
		if probe.Kind != kind {
			return nil
		}
		row, err := asClientJSON(doc)
		if err != nil {
			return err
		}
		out = append(out, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Handlers serves the replica over the device's own loopback listener.
//
// Who the account is comes from the session the device holds, not from the
// request: there is exactly one person signed in on a phone, and letting a
// request name somebody else would make the local listener a way to read any
// account whose data happened to be on the device.
type Handlers struct {
	reader func() *Reader
}

// NewHandlers returns the handlers, reading the account through fn so that
// signing in or out changes what is served without rebuilding the router.
func NewHandlers(fn func() *Reader) *Handlers { return &Handlers{reader: fn} }

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/me/replica", h.status)
	r.Get("/me/prefs", h.document(func(rd *Reader) (json.RawMessage, error) { return rd.Prefs() }))
	r.Get("/me/profile", h.document(func(rd *Reader) (json.RawMessage, error) { return rd.Profile() }))
	r.Get("/me/stats", h.document(func(rd *Reader) (json.RawMessage, error) { return rd.Stats() }))
	r.Get("/me/circle", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Circle() }))
	r.Get("/me/devices", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Devices() }))
	r.Get("/me/scoring", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Scoring() }))
	r.Get("/me/matches", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Matches() }))
}

func (h *Handlers) status(w http.ResponseWriter, _ *http.Request) {
	rd := h.reader()
	writeJSON(w, http.StatusOK, map[string]any{"ready": rd.Ready()})
}

func (h *Handlers) document(read func(*Reader) (json.RawMessage, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rd := h.reader()
		doc, err := read(rd)
		switch {
		case errors.Is(err, ErrNoReplica):
			http.Error(w, "nothing synced to this device yet", http.StatusServiceUnavailable)
		case db.IsNotFound(err):
			http.Error(w, "not found", http.StatusNotFound)
		case err != nil:
			http.Error(w, "could not read the local copy", http.StatusInternalServerError)
		default:
			writeRaw(w, doc)
		}
	}
}

func (h *Handlers) list(read func(*Reader) ([]json.RawMessage, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rd := h.reader()
		rows, err := read(rd)
		switch {
		case errors.Is(err, ErrNoReplica):
			http.Error(w, "nothing synced to this device yet", http.StatusServiceUnavailable)
		case err != nil:
			http.Error(w, "could not read the local copy", http.StatusInternalServerError)
		default:
			writeJSON(w, http.StatusOK, rows)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRaw(w http.ResponseWriter, doc json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(doc)
}
