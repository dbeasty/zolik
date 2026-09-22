package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// enrolHarness is the enrolment endpoint with nothing behind it but the
// in-memory repository: no database, because enrolling a node touches no
// account state, and the point of the stand-in repository is that it can be
// run exactly like the real one.
type enrolHarness struct {
	t      *testing.T
	router chi.Router
	nodes  *MemoryNodeRepository
}

func newEnrolHarness(t *testing.T) *enrolHarness {
	t.Helper()
	nodes := NewMemoryNodeRepository()
	h := NewHandlers(Deps{Nodes: nodes})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return &enrolHarness{t: t, router: r, nodes: nodes}
}

func (h *enrolHarness) enrol(token string, body map[string]any) *httptest.ResponseRecorder {
	h.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		h.t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/nodes/enroll", bytes.NewReader(raw))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func mustToken(t *testing.T, subject, username string, isGuest bool) string {
	t.Helper()
	token, err := CreateAccessToken(subject, username, isGuest, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func newEnrollingNode(t *testing.T) (ed25519.PublicKey, string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, base64.RawURLEncoding.EncodeToString(pub)
}

func TestEnrollNode(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	h := newEnrolHarness(t)
	pub, encoded := newEnrollingNode(t)

	rec := h.enrol(mustToken(t, testUserID, "alice", false), map[string]any{
		"pubkey":     encoded,
		"kind":       NodeKindPhone,
		"instanceId": "a1b2c3d4",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		NodeID     string `json:"nodeId"`
		Credential string `json:"credential"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.NodeID != NodeIDFor(pub) {
		t.Errorf("nodeId %q, want the id derived from the key %q", out.NodeID, NodeIDFor(pub))
	}

	claims, err := VerifyNodeCredential(out.Credential, LocalKeys())
	if err != nil {
		t.Fatalf("the credential this server just issued does not verify: %v", err)
	}
	if claims.NodeID() != out.NodeID {
		t.Errorf("credential names node %q, response says %q", claims.NodeID(), out.NodeID)
	}
	if claims.Subject != "node:"+out.NodeID {
		t.Errorf("sub %q, want node:<id>", claims.Subject)
	}
	if claims.Owner != testUserID {
		t.Errorf("owner %q, want the account that enrolled it", claims.Owner)
	}
	if claims.Kind != NodeKindPhone {
		t.Errorf("kind %q, want %q", claims.Kind, NodeKindPhone)
	}

	// A node credential is not an access token: it authenticates a replica
	// syncing, not a player playing.
	if _, err := ParseAccessClaims(out.Credential); err == nil {
		t.Fatal("a node credential was accepted as an access token")
	}

	stored, err := h.nodes.FindNode(context.Background(), out.NodeID)
	if err != nil {
		t.Fatalf("the enrolment was not recorded: %v", err)
	}
	if stored.OwnerID != testUserID || stored.Kind != NodeKindPhone || stored.InstanceID != "a1b2c3d4" {
		t.Fatalf("stored %+v", stored)
	}
	if stored.PublicKey != encoded {
		t.Errorf("stored key %q, want %q", stored.PublicKey, encoded)
	}

	owned, err := h.nodes.NodesForOwner(context.Background(), testUserID)
	if err != nil || len(owned) != 1 {
		t.Fatalf("the account's nodes are %v (%v)", owned, err)
	}
}

func TestEnrollNodeRefusals(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	_, encoded := newEnrollingNode(t)
	account := mustToken(t, testUserID, "alice", false)

	cases := []struct {
		name  string
		token string
		body  map[string]any
		want  int
	}{
		{"no token", "", map[string]any{"pubkey": encoded, "kind": NodeKindPhone}, http.StatusUnauthorized},
		{"a guest", mustToken(t, "0123456789abcdef0123456789abcdef", "Some Guest", true), map[string]any{"pubkey": encoded, "kind": NodeKindPhone}, http.StatusForbidden},
		{"no key", account, map[string]any{"kind": NodeKindPhone}, http.StatusBadRequest},
		{"a key of the wrong size", account, map[string]any{"pubkey": base64.RawURLEncoding.EncodeToString([]byte("short")), "kind": NodeKindPhone}, http.StatusBadRequest},
		{"an unknown kind", account, map[string]any{"pubkey": encoded, "kind": "toaster"}, http.StatusBadRequest},
		{"no kind", account, map[string]any{"pubkey": encoded}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newEnrolHarness(t)
			if rec := h.enrol(c.token, c.body); rec.Code != c.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
		})
	}
}

// The node id is derived from the key, so presenting a node that belongs to
// somebody else means holding their key. It is refused rather than taken
// over: the owner is who the sync side will authorise against.
func TestEnrollNodeWillNotTakeOverAnotherAccountsNode(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	h := newEnrolHarness(t)
	_, encoded := newEnrollingNode(t)

	body := map[string]any{"pubkey": encoded, "kind": NodeKindLAN}
	if rec := h.enrol(mustToken(t, testUserID, "alice", false), body); rec.Code != http.StatusOK {
		t.Fatalf("first enrolment: %d %s", rec.Code, rec.Body.String())
	}
	other := "652f1a2b3c4d5e6f70819299"
	if rec := h.enrol(mustToken(t, other, "mallory", false), body); rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}

	stored, err := h.nodes.FindNode(context.Background(), NodeIDFor(mustPublicKey(t, encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if stored.OwnerID != testUserID {
		t.Fatalf("the node changed hands: owner is now %q", stored.OwnerID)
	}
}

// Re-enrolment by the owner is ordinary, and keeps the date the node first
// appeared.
func TestEnrollNodeAgainKeepsTheOriginalEnrolment(t *testing.T) {
	newSigningKey(t, "https://zolik.test")
	h := newEnrolHarness(t)
	_, encoded := newEnrollingNode(t)
	account := mustToken(t, testUserID, "alice", false)
	body := map[string]any{"pubkey": encoded, "kind": NodeKindPhone}

	if rec := h.enrol(account, body); rec.Code != http.StatusOK {
		t.Fatalf("first enrolment: %d", rec.Code)
	}
	id := NodeIDFor(mustPublicKey(t, encoded))
	first, err := h.nodes.FindNode(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if rec := h.enrol(account, body); rec.Code != http.StatusOK {
		t.Fatalf("second enrolment: %d", rec.Code)
	}
	second, err := h.nodes.FindNode(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !second.EnrolledAt.Equal(first.EnrolledAt) {
		t.Errorf("enrolled at %v, want the original %v", second.EnrolledAt, first.EnrolledAt)
	}
	if second.LastSeenAt.IsZero() {
		t.Error("the second enrolment was not recorded as a sighting")
	}
}

// A deployment with no asymmetric key cannot issue a credential anything else
// could verify, and says so rather than handing out a token the node would
// believe it had enrolled with.
func TestEnrollNodeNeedsASigningKey(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "test_secret")
	resetKeys(t)
	h := newEnrolHarness(t)
	_, encoded := newEnrollingNode(t)

	rec := h.enrol(mustToken(t, testUserID, "alice", false), map[string]any{
		"pubkey": encoded,
		"kind":   NodeKindPhone,
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if _, err := h.nodes.FindNode(context.Background(), NodeIDFor(mustPublicKey(t, encoded))); err == nil {
		t.Fatal("a node was recorded as enrolled without being given a credential")
	}
}

func mustPublicKey(t *testing.T, encoded string) ed25519.PublicKey {
	t.Helper()
	pub, err := parseEd25519Public(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

// A receipt is checked against the key the node enrolled with, so a node the
// cloud has never enrolled proves nothing, and a receipt from one node cannot
// be passed off as another's.
func TestVerifySeatReceiptFromNode(t *testing.T) {
	resetKeys(t)
	pub := newNodeKey(t)
	guestID, err := NewRandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := CreateSeatReceipt(guestID)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	nodes := NewMemoryNodeRepository()
	if _, _, err := VerifySeatReceiptFromNode(ctx, nodes, receipt); err == nil {
		t.Fatal("a receipt from a node nobody enrolled was accepted")
	}

	if err := nodes.SaveNode(ctx, Node{
		ID:        NodeIDFor(pub),
		OwnerID:   testUserID,
		Kind:      NodeKindPhone,
		PublicKey: EncodeNodePublicKey(pub),
	}); err != nil {
		t.Fatal(err)
	}
	claims, node, err := VerifySeatReceiptFromNode(ctx, nodes, receipt)
	if err != nil {
		t.Fatalf("a receipt from an enrolled node was refused: %v", err)
	}
	if claims.GuestID != guestID || node.OwnerID != testUserID {
		t.Fatalf("receipt %+v from node %+v", claims, node)
	}

	// The same node id, enrolled with somebody else's key: the lookup
	// succeeds and the signature does not.
	stranger, _ := newEnrollingNode(t)
	if err := nodes.SaveNode(ctx, Node{
		ID:        NodeIDFor(pub),
		OwnerID:   testUserID,
		Kind:      NodeKindPhone,
		PublicKey: EncodeNodePublicKey(stranger),
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifySeatReceiptFromNode(ctx, nodes, receipt); err == nil {
		t.Fatal("a receipt verified against a key that did not sign it")
	}
}
