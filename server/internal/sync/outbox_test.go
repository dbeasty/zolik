package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"zolik/server/internal/db"
)

type stubSigner struct{ prefix string }

func (s stubSigner) SignBundle(payload []byte) (string, error) {
	return fmt.Sprintf("%s:%d", s.prefix, len(payload)), nil
}

type stubBundleVerifier struct{ refuse error }

func (v stubBundleVerifier) VerifyBundle(_ context.Context, _ string, _ []byte, signature string) error {
	if v.refuse != nil {
		return v.refuse
	}
	if signature == "" {
		return errors.New("unsigned")
	}
	return nil
}

type stubRecorder struct {
	imported []Bundle
	refuse   error
}

func (r *stubRecorder) ImportBundle(_ context.Context, b Bundle) error {
	if r.refuse != nil {
		return r.refuse
	}
	r.imported = append(r.imported, b)
	return nil
}

func openTestDB(t *testing.T) *db.KDB {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k
}

func sampleBundle(match string) Bundle {
	return Bundle{
		Match:      match,
		Module:     "canasta",
		Seed:       42,
		Envelope:   json.RawMessage(`{"status":"completed"}`),
		Moves:      []json.RawMessage{json.RawMessage(`{"seq":1}`)},
		Seats:      map[string]string{"p1": "user:65f0", "p2": "guest:abcd"},
		FinishedAt: time.Now().UTC(),
	}
}

func TestAFinishedMatchWaitsInTheOutboxUntilItIsTaken(t *testing.T) {
	k := openTestDB(t)
	out := NewOutbox(k, "phone-1", stubSigner{prefix: "sig"})

	if err := out.Hand(sampleBundle("65f0aa")); err != nil {
		t.Fatalf("hand: %v", err)
	}
	// Handing the same match up again is what a node with no acknowledgement
	// does, and it must not become two matches.
	if err := out.Hand(sampleBundle("65f0aa")); err != nil {
		t.Fatalf("hand again: %v", err)
	}
	pending, err := out.Pending()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d bundles, want 1", len(pending))
	}
	if pending[0].Node != "phone-1" {
		t.Fatalf("node = %q, want the node that hosted it", pending[0].Node)
	}
	if pending[0].Signature == "" {
		t.Fatal("the bundle was handed up unsigned")
	}

	if err := out.Drop("65f0aa"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	pending, err = out.Pending()
	if err != nil {
		t.Fatalf("pending after drop: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %d bundles after the cloud took it, want 0", len(pending))
	}
}

func TestTheCloudImportsWhatAPhonePushedUp(t *testing.T) {
	k := openTestDB(t)
	// The cloud's copy of a phone's outbox: what a push leaves behind.
	out := NewOutbox(k, "phone-1", stubSigner{prefix: "sig"})
	if err := out.Hand(sampleBundle("65f0aa")); err != nil {
		t.Fatalf("hand: %v", err)
	}

	recorder := &stubRecorder{}
	imp := NewImporter(k, stubBundleVerifier{}, recorder, time.Minute)
	if err := imp.Pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if len(recorder.imported) != 1 {
		t.Fatalf("imported %d matches, want 1", len(recorder.imported))
	}
	if recorder.imported[0].Seats["p2"] != "guest:abcd" {
		t.Fatalf("seats did not survive: %+v", recorder.imported[0].Seats)
	}
}

func TestABundleFromAnUnknownNodeIsRefusedAndRemembered(t *testing.T) {
	k := openTestDB(t)
	out := NewOutbox(k, "phone-1", stubSigner{prefix: "sig"})
	if err := out.Hand(sampleBundle("65f0aa")); err != nil {
		t.Fatalf("hand: %v", err)
	}

	recorder := &stubRecorder{}
	imp := NewImporter(k, stubBundleVerifier{refuse: errors.New("no such node")}, recorder, time.Minute)
	if err := imp.Pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if len(recorder.imported) != 0 {
		t.Fatal("a bundle from an unverifiable node was recorded")
	}
	refused := imp.Refused()
	if len(refused) != 1 {
		t.Fatalf("refused = %v, want the bundle kept with its reason", refused)
	}
}

func TestABundleThatDoesNotReplayIsRefused(t *testing.T) {
	k := openTestDB(t)
	out := NewOutbox(k, "phone-1", stubSigner{prefix: "sig"})
	if err := out.Hand(sampleBundle("65f0aa")); err != nil {
		t.Fatalf("hand: %v", err)
	}
	// The recorder is where the replay happens, so a node claiming an outcome
	// its moves do not produce is refused there.
	imp := NewImporter(k, stubBundleVerifier{}, &stubRecorder{refuse: errors.New("replay ended 40 points short")}, time.Minute)
	if err := imp.Pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	for key, reason := range imp.Refused() {
		if reason == "" {
			t.Fatalf("no reason kept for %s", key)
		}
		return
	}
	t.Fatal("the refusal was not remembered")
}

func TestABundleInSomebodyElsesOutboxIsRefused(t *testing.T) {
	k := openTestDB(t)
	// A node that can push its own outbox writing a bundle that claims to
	// come from another node: the namespace it sits in is the fact, and the
	// claim inside it is not.
	b := sampleBundle("65f0aa")
	b.Kind, b.Node = "bundle", "some-other-node"
	doc, err := db.MarshalDoc(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := k.Put(db.NodeOutboxNS("phone-1"), BundleKey(b.Match), doc); err != nil {
		t.Fatalf("put: %v", err)
	}

	recorder := &stubRecorder{}
	imp := NewImporter(k, stubBundleVerifier{}, recorder, time.Minute)
	if err := imp.Pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if len(recorder.imported) != 0 {
		t.Fatal("a bundle claiming another node was accepted")
	}
}
