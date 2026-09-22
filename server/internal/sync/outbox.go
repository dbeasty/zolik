package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"zolik/server/internal/db"
)

// A match played away from the cloud - on a phone hosting a table in a room
// with no internet, on a self-hosted server in a club - is finished long
// before the cloud hears about it. The node that hosted it writes what
// happened into its own outbox namespace, which is pushed up on the next
// sync, and the cloud verifies it and records it.
//
// The bundle carries the moves rather than the outcome, and the cloud replays
// them. That is the whole reason this is safe to accept at all: a node that
// simply announced "my player won" would be asking to be believed, whereas a
// node that hands over a seed and a list of moves is handing over something
// the cloud can check by playing the game itself. A bundle that does not
// replay to the outcome it claims is refused and kept for inspection.

// Bundle is one finished match as the node that hosted it hands it up.
type Bundle struct {
	Kind string `bson:"_kind" json:"kind"`
	// Match is the match's own id, as that node minted it.
	Match string `bson:"match" json:"match"`
	// Node is the node that hosted the match, and the owner of the outbox.
	Node string `bson:"node" json:"node"`
	// Module and Variation say which game's rules replay it.
	Module    string `bson:"module" json:"module"`
	Variation string `bson:"variation,omitempty" json:"variation,omitempty"`
	// Seed is the shuffle. With the moves it is the whole of what happened.
	Seed int64 `bson:"seed" json:"seed"`
	// Envelope is the match document as it stood when the match finished.
	Envelope json.RawMessage `bson:"envelope" json:"envelope"`
	// Moves are every move of the match, in order.
	Moves []json.RawMessage `bson:"moves" json:"moves"`
	// Seats maps each seat id to who sat in it: "user:<hex>" for a player who
	// signed in with an offline pass, "guest:<id>" for one who did not, and
	// "ai:<persona>" for a bot. A guest's seat becomes an account's when that
	// guest signs in and claims it.
	Seats map[string]string `bson:"seats" json:"seats"`
	// FinishedAt is when the hosting node saw the match end.
	FinishedAt time.Time `bson:"finishedAt" json:"finishedAt"`
	// Signature is the hosting node's signature over the bundle, so the cloud
	// can tell a bundle from the node it enrolled from one somebody wrote into
	// a namespace they happened to be able to push.
	Signature string `bson:"signature,omitempty" json:"signature,omitempty"`
}

// BundleKey is where a bundle sits inside its node's outbox.
func BundleKey(matchHex string) string { return "match/" + matchHex }

// Signer signs a node's outgoing bundles. internal/auth implements it with
// the node key; a node with no key signs nothing and the cloud will only
// accept its bundles if it is configured to (which it is not, in production).
type Signer interface {
	SignBundle(payload []byte) (string, error)
}

// Outbox is a node's own outbox: what it has finished and not yet had
// acknowledged.
type Outbox struct {
	kdb    *db.KDB
	nodeID string
	signer Signer
}

// NewOutbox returns the outbox of the node this process is.
func NewOutbox(k *db.KDB, nodeID string, signer Signer) *Outbox {
	return &Outbox{kdb: k, nodeID: nodeID, signer: signer}
}

// Hand writes a finished match into the outbox. It is idempotent: handing the
// same match up twice writes the same document, and the cloud's import is
// keyed by the match, so a node that never hears the acknowledgement can keep
// trying without recording the match twice.
func (o *Outbox) Hand(b Bundle) error {
	if o == nil {
		return nil
	}
	b.Kind = "bundle"
	b.Node = o.nodeID
	if o.signer != nil {
		payload, err := signingPayload(b)
		if err != nil {
			return err
		}
		if b.Signature, err = o.signer.SignBundle(payload); err != nil {
			return fmt.Errorf("sync: signing the bundle for match %s: %w", b.Match, err)
		}
	}
	doc, err := db.MarshalDoc(b)
	if err != nil {
		return err
	}
	return o.kdb.Put(db.NodeOutboxNS(o.nodeID), BundleKey(b.Match), doc)
}

// Drop removes a bundle the cloud has acknowledged. A phone holding every
// match it ever hosted would eventually be holding a database it has no use
// for: once the cloud has the match, the copy in the outbox is a duplicate of
// something that will come back as history anyway.
func (o *Outbox) Drop(matchHex string) error {
	if o == nil {
		return nil
	}
	_, err := o.kdb.Delete(db.NodeOutboxNS(o.nodeID), BundleKey(matchHex))
	return err
}

// Pending lists what this node is still waiting to have accepted, so the app
// can say "3 matches waiting to sync" rather than nothing at all.
func (o *Outbox) Pending() ([]Bundle, error) {
	if o == nil {
		return nil, nil
	}
	return bundlesIn(o.kdb, db.NodeOutboxNS(o.nodeID))
}

func bundlesIn(k *db.KDB, namespace string) ([]Bundle, error) {
	var out []Bundle
	err := k.Scan(namespace, func(doc []byte) error {
		var b Bundle
		if err := db.UnmarshalDoc(doc, &b); err != nil {
			return err
		}
		if b.Kind != "bundle" {
			return nil
		}
		out = append(out, b)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FinishedAt.Before(out[j].FinishedAt) })
	return out, nil
}

// signingPayload is the bytes a node signs and the cloud verifies.
//
// It is built field by field rather than by encoding the bundle, because the
// bundle does not travel as the bytes it was signed as: it is stored and
// replicated as a document, which means a round trip through another encoding
// before the cloud sees it again. Anything that does not survive that trip
// byte for byte - a timestamp's precision, most obviously - would make every
// signature fail verification for a reason that looks nothing like the cause.
//
// So the time goes in as milliseconds, the seats in sorted order, and the
// envelope and the moves as their own digests, which keeps the payload small
// as well as stable.
func signingPayload(b Bundle) ([]byte, error) {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			// Length-prefixed, so no combination of field values can be
			// rearranged into the same bytes as another.
			fmt.Fprintf(h, "%d:%s\n", len(p), p)
		}
	}
	write("kdb:zolik:bundle/1", b.Match, b.Node, b.Module, b.Variation)
	write(strconv.FormatInt(b.Seed, 10), strconv.FormatInt(b.FinishedAt.UnixMilli(), 10))
	write(string(digest(b.Envelope)))
	write(strconv.Itoa(len(b.Moves)))
	for _, mv := range b.Moves {
		write(string(digest(mv)))
	}
	seats := make([]string, 0, len(b.Seats))
	for seat := range b.Seats {
		seats = append(seats, seat)
	}
	sort.Strings(seats)
	for _, seat := range seats {
		write(seat, b.Seats[seat])
	}
	return h.Sum(nil), nil
}

// digest is one field's content hash, hex.
func digest(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	out := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(out, sum[:])
	return out
}

// Importer is the cloud side: it watches the outboxes that have been pushed to
// it, verifies each bundle and records the match.
type Importer struct {
	kdb      *db.KDB
	verify   BundleVerifier
	record   MatchRecorder
	interval time.Duration

	mu     sync.Mutex
	failed map[string]string

	stop chan struct{}
	done chan struct{}
	kick chan struct{}
}

// BundleVerifier checks that a bundle is what the node claims: the signature
// belongs to an enrolled node, and the owner may credit the seats it names.
// internal/auth implements it.
type BundleVerifier interface {
	VerifyBundle(ctx context.Context, nodeID string, payload []byte, signature string) error
}

// MatchRecorder replays a bundle and records the match. The replay is the
// check: a bundle whose moves do not produce the outcome it claims is
// refused. internal/match implements it, because replaying a match needs the
// game's own rules.
type MatchRecorder interface {
	ImportBundle(ctx context.Context, b Bundle) error
}

// NewImporter returns the cloud's importer.
func NewImporter(k *db.KDB, verify BundleVerifier, record MatchRecorder, interval time.Duration) *Importer {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Importer{
		kdb: k, verify: verify, record: record, interval: interval,
		failed: map[string]string{},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		kick:   make(chan struct{}, 1),
	}
}

func (i *Importer) Start() {
	if i == nil {
		return
	}
	go i.loop()
}

func (i *Importer) Close() {
	if i == nil {
		return
	}
	select {
	case <-i.stop:
	default:
		close(i.stop)
	}
	<-i.done
}

// Kick asks for a pass now, for when a push has just arrived.
func (i *Importer) Kick() {
	if i == nil {
		return
	}
	select {
	case i.kick <- struct{}{}:
	default:
	}
}

func (i *Importer) loop() {
	defer close(i.done)
	t := time.NewTicker(i.interval)
	defer t.Stop()
	for {
		select {
		case <-i.stop:
			return
		case <-t.C:
		case <-i.kick:
		}
		if err := i.Pass(context.Background()); err != nil {
			log.Printf("sync: importing finished matches: %v", err)
		}
	}
}

// Pass imports everything waiting in every outbox this node holds. A node that
// takes no handed-up matches - anything that is not the hub - has nothing to
// do here rather than nothing to be called on.
func (i *Importer) Pass(ctx context.Context) error {
	if i == nil {
		return nil
	}
	for _, name := range i.kdb.OpenNamespaces() {
		node, ok := db.NodeOfOutbox(name)
		if !ok {
			continue
		}
		bundles, err := bundlesIn(i.kdb, name)
		if err != nil {
			return err
		}
		for _, b := range bundles {
			if err := i.importOne(ctx, node, b); err != nil {
				// One bad bundle must not stop the rest: a node with a
				// corrupt match still has good ones behind it.
				i.remember(b, err)
				log.Printf("sync: refusing match %s from node %s: %v", b.Match, node, err)
			}
		}
	}
	return nil
}

func (i *Importer) importOne(ctx context.Context, node string, b Bundle) error {
	if b.Node != node {
		return fmt.Errorf("bundle claims node %s but sits in %s's outbox", b.Node, node)
	}
	if i.verify != nil {
		payload, err := signingPayload(b)
		if err != nil {
			return err
		}
		if err := i.verify.VerifyBundle(ctx, node, payload, b.Signature); err != nil {
			return fmt.Errorf("verifying the hosting node: %w", err)
		}
	}
	if err := i.record.ImportBundle(ctx, b); err != nil {
		return err
	}
	i.forget(b)
	return nil
}

// Refused lists the bundles this importer would not take and why, so an
// operator has somewhere to look when a player says their match never arrived.
func (i *Importer) Refused() map[string]string {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make(map[string]string, len(i.failed))
	for k, v := range i.failed {
		out[k] = v
	}
	return out
}

func (i *Importer) remember(b Bundle, err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.failed[b.Node+"/"+b.Match] = strings.TrimSpace(err.Error())
}

func (i *Importer) forget(b Bundle) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.failed, b.Node+"/"+b.Match)
}
