package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
)

// A guest at a table with no internet is a guest of that table's host, not of
// the cloud: nobody there has ever heard of them. What they leave with is a
// receipt the host signed, saying this guest id sat here. When they sign in
// afterwards, that receipt is how the seat becomes theirs - and a match that
// reaches the cloud weeks later, from a phone that was in a drawer, is still
// credited to the person who played it.
func TestAGuestSeatBecomesTheirsWhenTheySignIn(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	phoneKey := nodeKeyForTest(t)

	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(context.Background())
	router := chi.NewRouter()
	hub.RegisterRoutes(router)
	cloud := httptest.NewServer(router)
	t.Cleanup(cloud.Close)

	ctx := context.Background()
	player, err := hub.authStore.InsertUser(ctx, models.User{Username: "ada"})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	nodeID := auth.NodeIDFor(phoneKey)
	if err := hub.nodes.SaveNode(ctx, auth.Node{
		ID: nodeID, OwnerID: player.ID.Hex(), Kind: auth.NodeKindPhone,
		PublicKey: auth.EncodeNodePublicKey(phoneKey), EnrolledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("enrolling the phone: %v", err)
	}

	// The table hands the guest a receipt, signed with the host's node key.
	const guestID = "b8f0c0ffeec0ffeeb8f0c0ffeec0ffee"
	receipt, err := auth.CreateSeatReceipt(guestID)
	if err != nil {
		t.Fatalf("issuing the seat receipt: %v", err)
	}

	// Later, online, they sign in and their device hands the receipt up.
	token, err := auth.CreateAccessToken(player.ID.Hex(), player.Username, false, time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"receipts": []string{receipt}})
	req, _ := http.NewRequest(http.MethodPost, cloud.URL+"/auth/claim-offline", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("claim-offline: status %d", res.StatusCode)
	}
	var claimed struct {
		Seats []struct {
			GuestID string `json:"guestId"`
			NodeID  string `json:"nodeId"`
			Claimed bool   `json:"claimed"`
			Reason  string `json:"reason"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(res.Body).Decode(&claimed); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(claimed.Seats) != 1 || !claimed.Seats[0].Claimed {
		t.Fatalf("seats = %+v, want the seat claimed", claimed.Seats)
	}
	if claimed.Seats[0].NodeID != nodeID {
		t.Fatalf("the seat was attributed to node %q, want %q", claimed.Seats[0].NodeID, nodeID)
	}

	// The claim is what a match arriving afterwards is credited by, however
	// long afterwards that is.
	claims := auth.NewKDBGuestClaims(hub.kdb)
	account, ok, err := claims.AccountForGuest(ctx, guestID)
	if err != nil || !ok || account != player.ID.Hex() {
		t.Fatalf("the guest belongs to %q (%v, %v), want %q", account, ok, err, player.ID.Hex())
	}
}

func TestASeatReceiptFromAnUnenrolledTableIsRefused(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	nodeKeyForTest(t)

	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(context.Background())
	router := chi.NewRouter()
	hub.RegisterRoutes(router)
	cloud := httptest.NewServer(router)
	t.Cleanup(cloud.Close)

	ctx := context.Background()
	player, err := hub.authStore.InsertUser(ctx, models.User{Username: "ada"})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	// A receipt signed by a key the cloud has never enrolled: anybody can
	// mint one of these, so it proves nothing until the node behind it is
	// known.
	_, stranger, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	restore := auth.SnapshotKeysForTest()
	if err := auth.SetNodeKey(hex.EncodeToString(stranger.Seed())); err != nil {
		t.Fatalf("installing a stranger's key: %v", err)
	}
	receipt, err := auth.CreateSeatReceipt("b8f0c0ffeec0ffeeb8f0c0ffeec0ffee")
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	restore()

	token, err := auth.CreateAccessToken(player.ID.Hex(), player.Username, false, time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"receipts": []string{receipt}})
	req, _ := http.NewRequest(http.MethodPost, cloud.URL+"/auth/claim-offline", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	defer res.Body.Close()

	var claimed struct {
		Seats []struct {
			Claimed bool   `json:"claimed"`
			Reason  string `json:"reason"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(res.Body).Decode(&claimed); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	// Reported rather than fatal: somebody with five good receipts and one
	// bad one should get the other four.
	if len(claimed.Seats) != 1 || claimed.Seats[0].Claimed {
		t.Fatalf("seats = %+v, want the receipt refused", claimed.Seats)
	}
	if claimed.Seats[0].Reason == "" {
		t.Fatal("a refused receipt was returned without saying why")
	}
	claims := auth.NewKDBGuestClaims(hub.kdb)
	if _, ok, _ := claims.AccountForGuest(context.Background(), "b8f0c0ffeec0ffeeb8f0c0ffeec0ffee"); ok {
		t.Fatal("a guest was claimed on the word of a table nobody enrolled")
	}
}
