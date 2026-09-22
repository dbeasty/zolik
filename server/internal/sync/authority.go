package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/limidus/kdb/go/kdb/codec"
	"github.com/limidus/kdb/go/kdb/peersync"
	kdbserver "github.com/limidus/kdb/go/kdb/server"

	"zolik/server/internal/db"
)

// The resolver authority is where the database stops being able to decide for
// itself and asks the application.
//
// The engine's own rules go a long way: a field-merge settles two devices
// editing different fields of the same document, which is most of what
// actually happens. What it cannot do is know that a scoring session's rounds
// are a set and not a value, that an edge removed on one device and renamed on
// another should end up removed, or that of two friend codes the one the
// server handed out first is the real one. Those are zolik's rules, and this
// is where they run.
//
// The authority is the hub, which is also where the conflicts are settled: a
// settlement is an ordinary commit, so it replicates to every node like any
// other write and each node closes its own copy of the entry.

// Every document in a user's namespace carries the kind it is (db.DocKindField),
// so a rule can be chosen for it without knowing its key: a document id is a
// hash of the key and says nothing about what is inside.

// rule merges one document. base is what both sides started from and may be
// nil when they each created the document independently. A rule is pure and
// must be deterministic: the same three values must give the same answer on
// every node and in either order, or two nodes settle the same conflict
// differently and the database has learned nothing.
type rule func(base, local, incoming map[string]json.RawMessage) (map[string]json.RawMessage, error)

var rules = map[string]rule{
	db.KindPrefs:   mergeByUpdatedAt,
	db.KindNotify:  mergeByUpdatedAt,
	db.KindProfile: mergeByUpdatedAt,
	db.KindDevice:  mergeByUpdatedAt,
	db.KindScoring: mergeScoringSession,
	db.KindCircle:  mergeCircleEdge,
}

// authority settles the conflicts on this node's queues that the chains hand
// to it.
type authority struct {
	node     *Node
	interval time.Duration
	// started is whether the loop is running. A node that is built and closed
	// without ever starting - a process that failed further along its own
	// startup, or a test - must not be waiting for a goroutine nobody ever
	// launched.
	started bool
	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	kick    chan struct{}
}

func newAuthority(n *Node, interval time.Duration) *authority {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &authority{
		node:     n,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		kick:     make(chan struct{}, 1),
	}
}

func (a *authority) start() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return
	}
	a.started = true
	go a.loop()
}

func (a *authority) close() {
	a.mu.Lock()
	started := a.started
	a.mu.Unlock()
	select {
	case <-a.stop:
	default:
		close(a.stop)
	}
	if started {
		<-a.done
	}
}

// Kick asks for a pass now. A conflict recorded while a person is looking at
// the screen should be settled before they look away, and the tick alone is
// too slow for that.
func (a *authority) Kick() {
	select {
	case a.kick <- struct{}{}:
	default:
	}
}

func (a *authority) loop() {
	defer close(a.done)
	t := time.NewTicker(a.interval)
	defer t.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-t.C:
		case <-a.kick:
		}
		a.pass()
	}
}

// Settle runs a settlement pass now and waits for it, for the moments where
// waiting for the tick would be visible: a sync that has just brought in a
// divergence, or a test that would otherwise be asserting against a timer.
func (n *Node) Settle() {
	if n == nil || n.authority == nil {
		return
	}
	n.authority.pass()
}

// OpenConflicts is what this node has not been able to settle, by namespace,
// as a line per conflict. For an operator looking at why two devices disagree,
// and for a test asserting that they do not.
func (n *Node) OpenConflicts() map[string][]string {
	if n == nil {
		return nil
	}
	out := map[string][]string{}
	for _, name := range n.kdb.OpenNamespaces() {
		queue, ok := n.node.Conflicts(db.Qualified(name))
		if !ok {
			continue
		}
		for _, e := range queue.List() {
			out[name] = append(out[name], fmt.Sprintf("%s kind=%s authority=%v items=%d",
				e.ID, e.Kind, e.Authority, len(e.Items)))
		}
	}
	return out
}

// pass settles everything settleable on every namespace this node holds.
func (a *authority) pass() {
	for _, name := range a.node.kdb.OpenNamespaces() {
		if _, ok := db.UserOfNamespace(name); !ok {
			continue
		}
		if err := a.settleNamespace(name); err != nil {
			log.Printf("sync: settling conflicts in %s: %v", name, err)
		}
	}
}

func (a *authority) settleNamespace(name string) error {
	rt, err := a.node.runtime(name)
	if err != nil {
		return err
	}
	queue, ok := a.node.node.Conflicts(db.Qualified(name))
	if !ok {
		return nil
	}
	for _, entry := range queue.List() {
		if !entry.Authority || len(entry.Items) == 0 {
			continue
		}
		choices, err := a.decide(rt, name, entry)
		if err != nil {
			log.Printf("sync: %s conflict %s: %v", name, entry.ID, err)
			continue
		}
		if len(choices) == 0 {
			continue
		}
		switch _, err := rt.ResolveConflict(entry.ID, choices, a.node.principal()); {
		case err == nil:
		case isSuperseded(err):
			// The document has moved on since this merge: another merge
			// settled it again, and that later entry is the one worth
			// deciding. This one describes a moment that no longer exists, so
			// it is dropped rather than retried for ever.
			if err := queue.Remove(entry.ID); err != nil {
				return fmt.Errorf("dropping the superseded conflict %s: %w", entry.ID, err)
			}
		default:
			return fmt.Errorf("resolving %s: %w", entry.ID, err)
		}
	}
	return nil
}

// decide turns one conflict entry into a settlement per document.
//
// A provisional entry says which value a merge kept and which it dropped, and
// the engine has been serving the kept one ever since. So the question is not
// "which of these two" but "what should the document hold now", and the answer
// has to be built against what it holds at this moment rather than against
// what it held when the merge happened: the person may have changed it again
// since, and the engine refuses a settlement decided against a value that has
// moved.
func (a *authority) decide(rt *kdbserver.KdbServerRuntime, name string, entry peersync.ConflictEntry) (map[codec.UUID]peersync.Choice, error) {
	bases := map[string]string{}
	for _, d := range entry.Details {
		if d.Base != nil {
			bases[d.DocumentID] = *d.Base
		}
	}
	out := map[codec.UUID]peersync.Choice{}
	for _, item := range entry.Items {
		id, err := codec.UUIDFromString(item.DocumentID)
		if err != nil {
			return nil, err
		}
		current, _, found, err := rt.ReadForUpdate(db.Qualified(name), id)
		if err != nil {
			return nil, err
		}
		kept := item.LocalDoc
		if found {
			kept = &current
		}
		// The side the merge dropped, which is the whole reason this is here.
		dropped := item.IncomingDoc
		if dropped != nil && kept != nil && sameJSON(*dropped, *kept) {
			dropped = item.LocalDoc
		}
		body, err := settle(bases[item.DocumentID], kept, dropped)
		if err != nil {
			return nil, err
		}
		if body == nil {
			// No rule for this document: leave the whole entry queued rather
			// than guess. A conflict nobody can explain is worth a human
			// looking at it, and a settlement has to answer for every
			// document the entry names or the engine refuses it anyway.
			return nil, nil
		}
		// Every item gets a choice, including the ones already holding the
		// right value: a settlement is all or nothing.
		merged := string(body)
		out[id] = peersync.Choice{Body: &merged}
	}
	return out, nil
}

// isSuperseded reports whether a settlement was refused because the documents
// it decided have changed since the merge that queued them.
func isSuperseded(err error) bool {
	var stale *kdbserver.ErrResolutionStale
	return errors.As(err, &stale)
}

// sameJSON compares two documents by their fields rather than their bytes, so
// a difference of key order or whitespace is not a difference.
func sameJSON(a, b string) bool {
	fa, err := fields(a)
	if err != nil {
		return a == b
	}
	fb, err := fields(b)
	if err != nil {
		return a == b
	}
	ca, err := marshalCanonical(fa)
	if err != nil {
		return a == b
	}
	cb, err := marshalCanonical(fb)
	if err != nil {
		return a == b
	}
	return string(ca) == string(cb)
}

// settle picks the rule for a document and applies it. A nil result means
// "this is not mine to decide".
func settle(base string, local, incoming *string) ([]byte, error) {
	// A document deleted on one side and edited on the other is not a merge of
	// values; it is a question about intent, and the rules that have one
	// answer it themselves. Anything else stays queued.
	if local == nil || incoming == nil {
		return nil, nil
	}
	l, err := fields(*local)
	if err != nil {
		return nil, err
	}
	i, err := fields(*incoming)
	if err != nil {
		return nil, err
	}
	var b map[string]json.RawMessage
	if base != "" {
		if b, err = fields(base); err != nil {
			return nil, err
		}
	}
	kind := kindOf(l)
	if kind == "" {
		kind = kindOf(i)
	}
	r, ok := rules[kind]
	if !ok {
		return nil, nil
	}
	// Order the sides canonically before the rule sees them. The engine hands
	// the two sides in the merge's own order, which is stable across nodes,
	// but a rule that is asked the same question twice with the sides swapped
	// must still answer the same thing, and the cheapest way to guarantee that
	// is to remove the asymmetry here.
	first, second := l, i
	if *local > *incoming {
		first, second = i, l
	}
	merged, err := r(b, first, second)
	if err != nil {
		return nil, err
	}
	if merged == nil {
		return nil, nil
	}
	return marshalCanonical(merged)
}

func fields(body string) (map[string]json.RawMessage, error) {
	var out map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func kindOf(doc map[string]json.RawMessage) string {
	raw, ok := doc[db.DocKindField]
	if !ok {
		return ""
	}
	var kind string
	if err := json.Unmarshal(raw, &kind); err != nil {
		return ""
	}
	return kind
}

// marshalCanonical writes the fields back with their keys in order, so two
// nodes that settle the same conflict write byte-identical documents.
func marshalCanonical(doc map[string]json.RawMessage) ([]byte, error) {
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(doc[k])
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// updatedAt reads a document's own idea of when it was last written. Documents
// that do not carry one are merged field by field instead.
func updatedAt(doc map[string]json.RawMessage) (time.Time, bool) {
	for _, field := range []string{"updatedAt", "seenAt", "createdAt"} {
		raw, ok := doc[field]
		if !ok {
			continue
		}
		if t, ok := parseTime(raw); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseTime reads the two shapes a time arrives in: a JSON string, and the
// extended-JSON $date the bson encoding writes.
func parseTime(raw json.RawMessage) (time.Time, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		t, err := time.Parse(time.RFC3339Nano, s)
		return t, err == nil
	}
	var wrapper struct {
		Date json.RawMessage `json:"$date"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Date == nil {
		return time.Time{}, false
	}
	var iso string
	if err := json.Unmarshal(wrapper.Date, &iso); err == nil {
		t, err := time.Parse(time.RFC3339Nano, iso)
		return t, err == nil
	}
	var millis struct {
		Number string `json:"$numberLong"`
	}
	if err := json.Unmarshal(wrapper.Date, &millis); err == nil && millis.Number != "" {
		var ms int64
		if _, err := fmt.Sscan(millis.Number, &ms); err == nil {
			return time.UnixMilli(ms).UTC(), true
		}
	}
	var ms int64
	if err := json.Unmarshal(wrapper.Date, &ms); err == nil {
		return time.UnixMilli(ms).UTC(), true
	}
	return time.Time{}, false
}

// mergeByUpdatedAt keeps the document that was written last, and falls back to
// a field-level merge when neither says when it was written. This is the rule
// for the documents that are a bag of settings: taking half of one device's
// preferences and half of another's would produce a state neither person asked
// for.
func mergeByUpdatedAt(base, local, incoming map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	lt, lok := updatedAt(local)
	it, iok := updatedAt(incoming)
	switch {
	case lok && iok && lt.After(it):
		return local, nil
	case lok && iok && it.After(lt):
		return incoming, nil
	case lok && !iok:
		return local, nil
	case iok && !lok:
		return incoming, nil
	}
	return mergeFields(base, local, incoming), nil
}

// mergeFields takes each side's changes against the base, and where both
// changed the same field keeps the one that sorts higher, which is arbitrary
// but identical on every node.
func mergeFields(base, local, incoming map[string]json.RawMessage) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range local {
		out[k] = v
	}
	for k, v := range incoming {
		cur, have := out[k]
		if !have {
			out[k] = v
			continue
		}
		if string(cur) == string(v) {
			continue
		}
		if b, ok := base[k]; ok {
			// Only one side moved: that side wins, which is what a field merge
			// means.
			if string(cur) == string(b) {
				out[k] = v
				continue
			}
			if string(v) == string(b) {
				continue
			}
		}
		if string(v) > string(cur) {
			out[k] = v
		}
	}
	return out
}

// mergeScoringSession unions the rounds of a pen-and-paper scorepad and keeps
// the later of everything else. Two people scoring the same game on two
// devices is the case this exists for: each writes the rounds they were there
// for, and dropping either side's would lose somebody's score.
func mergeScoringSession(base, local, incoming map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	merged, err := mergeByUpdatedAt(base, local, incoming)
	if err != nil {
		return nil, err
	}
	out := map[string]json.RawMessage{}
	for k, v := range merged {
		out[k] = v
	}
	rounds, err := unionRounds(local["rounds"], incoming["rounds"])
	if err != nil {
		return nil, err
	}
	if rounds != nil {
		out["rounds"] = rounds
	}
	// A closed scorepad stays closed: reopening somebody's finished game
	// because their other device had not heard yet is the wrong way round.
	if isTrue(local["closed"]) || isTrue(incoming["closed"]) {
		out["closed"] = json.RawMessage("true")
	}
	return out, nil
}

// unionRounds merges two round lists by each round's id, keeping the later
// version of a round both sides have.
func unionRounds(localRaw, incomingRaw json.RawMessage) (json.RawMessage, error) {
	if localRaw == nil && incomingRaw == nil {
		return nil, nil
	}
	type round struct {
		raw json.RawMessage
		id  string
		at  time.Time
		ok  bool
	}
	read := func(raw json.RawMessage) ([]round, error) {
		if raw == nil {
			return nil, nil
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		out := make([]round, 0, len(items))
		for _, item := range items {
			f, err := fields(string(item))
			if err != nil {
				return nil, err
			}
			r := round{raw: item}
			if id, ok := f["id"]; ok {
				_ = json.Unmarshal(id, &r.id)
			}
			if r.id == "" {
				// No id to merge on: the value itself is the identity.
				r.id = string(item)
			}
			r.at, r.ok = updatedAt(f)
			out = append(out, r)
		}
		return out, nil
	}
	l, err := read(localRaw)
	if err != nil {
		return nil, err
	}
	i, err := read(incomingRaw)
	if err != nil {
		return nil, err
	}
	byID := map[string]round{}
	order := []string{}
	for _, r := range append(l, i...) {
		cur, have := byID[r.id]
		if !have {
			byID[r.id] = r
			order = append(order, r.id)
			continue
		}
		switch {
		case r.ok && cur.ok && r.at.After(cur.at):
			byID[r.id] = r
		case r.ok && !cur.ok:
			byID[r.id] = r
		case string(r.raw) > string(cur.raw) && !cur.ok && !r.ok:
			byID[r.id] = r
		}
	}
	sort.Strings(order)
	items := make([]json.RawMessage, 0, len(order))
	for _, id := range order {
		items = append(items, byID[id].raw)
	}
	return json.Marshal(items)
}

// mergeCircleEdge decides between a removal and an edit of the same circle
// entry, and between a guest's edge and the account that guest became.
func mergeCircleEdge(base, local, incoming map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	// An edge to a member who has since signed in is the account's, not the
	// guest's: the guest key is what the person was called before they had a
	// name.
	lAccount := isAccountKey(local["memberKey"])
	iAccount := isAccountKey(incoming["memberKey"])
	if lAccount != iAccount {
		if lAccount {
			return local, nil
		}
		return incoming, nil
	}
	// Removal and edit: whichever the person did later is what they meant.
	lRemoved, lHas := removedAt(local)
	iRemoved, iHas := removedAt(incoming)
	if lHas != iHas {
		removed, other := local, incoming
		removedAtTime := lRemoved
		if iHas {
			removed, other = incoming, local
			removedAtTime = iRemoved
		}
		if at, ok := updatedAt(other); ok && at.After(removedAtTime) {
			return other, nil
		}
		return removed, nil
	}
	return mergeByUpdatedAt(base, local, incoming)
}

func removedAt(doc map[string]json.RawMessage) (time.Time, bool) {
	raw, ok := doc["removedAt"]
	if !ok {
		return time.Time{}, false
	}
	return parseTime(raw)
}

func isAccountKey(raw json.RawMessage) bool {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return false
	}
	return strings.HasPrefix(s, "user:")
}

func isTrue(raw json.RawMessage) bool {
	var b bool
	return json.Unmarshal(raw, &b) == nil && b
}

// declareResolution records the chains this database runs on, as patterns
// rather than per namespace: there is one rule for every user's data and one
// for every match, and a database with a million users cannot hold a million
// copies of the same sentence.
func (n *Node) declareResolution() error {
	meta := n.node.Meta()
	if meta == nil {
		return fmt.Errorf("sync: no metadata namespace")
	}
	// A person's own data merges: different devices edit different things,
	// and what is genuinely contested goes to the authority, which is the
	// hub. Provisional rather than held, because a phone that is offline for a
	// week should not have its settings frozen in the meantime: the merge
	// stands until the authority says otherwise.
	userChain := peersync.ResolutionChain{Rules: []peersync.ResolutionRule{
		{Kind: peersync.RuleFieldMerge},
		{
			Kind:    peersync.RuleAuthority,
			Node:    n.NodeID(),
			Pending: peersync.PendingProvisional,
			Timeout: "24h",
		},
	}}
	if err := meta.SetResolution(db.Qualified("u/*"), userChain); err != nil {
		return fmt.Errorf("sync: user resolution chain: %w", err)
	}
	// A match has one writer at a time, so a divergence in one is not a merge
	// to be made but a fence that failed. Queue it, so somebody sees it.
	matchChain := peersync.ResolutionChain{Rules: []peersync.ResolutionRule{{Kind: peersync.RuleQueue}}}
	if err := meta.SetResolution(db.Qualified("m/*"), matchChain); err != nil {
		return fmt.Errorf("sync: match resolution chain: %w", err)
	}
	// An outbox has one writer by construction: the node it belongs to. The
	// same reasoning, and the same answer.
	if err := meta.SetResolution(db.Qualified("node/*"), matchChain); err != nil {
		return fmt.Errorf("sync: outbox resolution chain: %w", err)
	}
	return nil
}

// resolutionRuntime is the runtime a chain is recorded against; kept separate
// so tests can settle a conflict without a whole node.
var _ = func(rt *kdbserver.KdbServerRuntime) {}
