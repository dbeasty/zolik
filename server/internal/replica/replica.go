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
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

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
	writer func() *Writer
}

// NewHandlers returns the handlers, reading the account through fn so that
// signing in or out changes what is served without rebuilding the router.
func NewHandlers(fn func() *Reader) *Handlers { return &Handlers{reader: fn} }

// SetWriter makes the person's own data writable on this device. Without one
// the routes are read-only, which is what a device that has not been told who
// is signed in should be.
func (h *Handlers) SetWriter(fn func() *Writer) { h.writer = fn }

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/me/replica", h.status)
	r.Get("/me/prefs", h.document(func(rd *Reader) (json.RawMessage, error) { return rd.Prefs() }))
	r.Get("/me/profile", h.document(func(rd *Reader) (json.RawMessage, error) { return rd.Profile() }))
	r.Get("/me/stats", h.document(func(rd *Reader) (json.RawMessage, error) { return rd.Stats() }))
	r.Get("/me/circle", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Circle() }))
	r.Get("/me/devices", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Devices() }))
	r.Get("/me/scoring", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Scoring() }))
	r.Get("/me/matches", h.list(func(rd *Reader) ([]json.RawMessage, error) { return rd.Matches() }))

	r.Put("/me/prefs", h.write(func(wr *Writer, _ *http.Request, body json.RawMessage) error {
		return wr.PutPrefs(body)
	}))
	r.Put("/me/scoring/{id}", h.write(func(wr *Writer, req *http.Request, body json.RawMessage) error {
		return wr.PutScoring(chi.URLParam(req, "id"), body)
	}))
	r.Put("/me/circle/{member}", h.write(func(wr *Writer, req *http.Request, body json.RawMessage) error {
		return wr.PutCircleEntry(chi.URLParam(req, "member"), body)
	}))
	// What the cloud owns is not writable here, and says so rather than
	// answering 404 as if the route were a typo.
	for _, path := range []string{"/me/stats", "/me/matches"} {
		r.Put(path, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "this is written by the cloud", http.StatusForbidden)
		})
	}
}

func (h *Handlers) write(apply func(*Writer, *http.Request, json.RawMessage) error) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if h.writer == nil {
			http.Error(w, "this device is not signed in", http.StatusServiceUnavailable)
			return
		}
		wr := h.writer()
		body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
		if err != nil {
			http.Error(w, "could not read the document", http.StatusBadRequest)
			return
		}
		switch err := apply(wr, req, body); {
		case errors.Is(err, ErrNoReplica):
			http.Error(w, "this device is not signed in", http.StatusServiceUnavailable)
		case errors.Is(err, ErrNotYours):
			http.Error(w, "this is written by the cloud", http.StatusForbidden)
		case err != nil:
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
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

// Writing to the copy this device holds, rather than to the cloud.
//
// A person changing their settings, or scoring a hand on a pad, should not
// have to be online for it to take. The write goes into their own namespace
// here and replicates when there is a connection, which is the same document
// the cloud would have written and merges with whatever another device of
// theirs did in the meantime.
//
// What cannot be written this way is anything the cloud owns: their rating,
// their friend code, the history of matches they have played. Those live in
// the namespace this device only reads, and a device that could write them
// could award itself any of them.

// Writer applies a person's own changes to the copy this device holds.
type Writer struct {
	kdb  *db.KDB
	user string
}

// NewWriter returns a writer over an account's replica.
func NewWriter(k *db.KDB, userHex string) *Writer {
	return &Writer{kdb: k, user: strings.TrimSpace(userHex)}
}

// ErrNotYours is the refusal for a write into something the cloud owns.
var ErrNotYours = errors.New("replica: this is written by the cloud, not by this device")

// PutPrefs replaces the account's settings.
func (w *Writer) PutPrefs(body json.RawMessage) error {
	return w.put(db.PrefsKey, db.KindPrefs, body)
}

// PutScoring stores one scorepad. The id is the pad's own, so the same pad
// edited on two devices is one document that merges rather than two that both
// claim to be it.
func (w *Writer) PutScoring(id string, body json.RawMessage) error {
	if strings.TrimSpace(id) == "" || strings.Contains(id, "/") {
		return fmt.Errorf("replica: %q is not a scorepad id", id)
	}
	return w.put("scoring/"+id, db.KindScoring, body)
}

// PutCircleEntry stores one entry of the person's circle.
func (w *Writer) PutCircleEntry(member string, body json.RawMessage) error {
	if strings.TrimSpace(member) == "" || strings.Contains(member, "/") {
		return fmt.Errorf("replica: %q is not a member key", member)
	}
	return w.put("circle/"+member, db.KindCircle, body)
}

// put stamps a document with what it is and when it was written, and stores it
// in the account's own namespace.
//
// The time is what the merge rules decide by when two devices changed the same
// thing, so it is written here rather than trusted from the client: a phone
// whose clock is a day fast would otherwise win every disagreement it was ever
// part of.
func (w *Writer) put(key, kind string, body json.RawMessage) error {
	if w == nil || w.user == "" {
		return ErrNoReplica
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return fmt.Errorf("replica: that is not a document: %w", err)
	}
	stamped, err := json.Marshal(time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	fields["updatedAt"] = stamped
	kindValue, err := json.Marshal(kind)
	if err != nil {
		return err
	}
	fields[db.DocKindField] = kindValue
	doc, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return w.kdb.Put(db.UserNS(w.user), key, doc)
}
