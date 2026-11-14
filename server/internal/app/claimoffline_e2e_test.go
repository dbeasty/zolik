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
	"github.com/golang-jwt/jwt/v5"

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

	// The table seats the guest - minting the id in its own namespace, which
	// is what lets its receipt vouch for that guest and nobody else - and
	// hands them a receipt, signed with the host's node key.
	guestID, err := auth.NewGuestID()
	if err != nil {
		t.Fatalf("minting the guest: %v", err)
	}
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
	strangersGuest, err := auth.NewGuestID()
	if err != nil {
		t.Fatalf("guest: %v", err)
	}
	receipt, err := auth.CreateSeatReceipt(strangersGuest)
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
	if _, ok, _ := claims.AccountForGuest(context.Background(), strangersGuest); ok {
		t.Fatal("a guest was claimed on the word of a table nobody enrolled")
	}
}

// A guest id is public: every table broadcasts it. An account that enrols a
// node of its own can sign whatever it likes with that node's key, so a
// receipt naming a guest the node never minted proves nothing - neither for an
// online guest, whose id the cloud minted, nor for a guest seated at somebody
// else's offline table. Either claim is refused, and nothing moves.
func TestAReceiptForSomebodyElsesGuestIsRefused(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	nodeKeyForTest(t)

	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(context.Background())
	router := chi.NewRouter()
	hub.RegisterRoutes(router)
	cloud := httptest.NewServer(router)
	t.Cleanup(cloud.Close)

	ctx := context.Background()
	mallory, err := hub.authStore.InsertUser(ctx, models.User{Username: "mallory"})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}

	// An honest table, enrolled by its owner, seats a guest.
	honestPub, honestPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	restore := auth.SnapshotKeysForTest()
	if err := auth.SetNodeKey(hex.EncodeToString(honestPriv.Seed())); err != nil {
		t.Fatalf("installing the honest table's key: %v", err)
	}
	offlineGuest, err := auth.NewGuestID()
	if err != nil {
		t.Fatalf("guest: %v", err)
	}
	restore()
	host, err := hub.authStore.InsertUser(ctx, models.User{Username: "host"})
	if err != nil {
		t.Fatalf("creating the host: %v", err)
	}
	if err := hub.nodes.SaveNode(ctx, auth.Node{
		ID: auth.NodeIDFor(honestPub), OwnerID: host.ID.Hex(), Kind: auth.NodeKindPhone,
		PublicKey: auth.EncodeNodePublicKey(honestPub), EnrolledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("enrolling the honest table: %v", err)
	}

	// Mallory enrols a node of her own, properly: it is her key and her
	// account, and the cloud has no reason to refuse it.
	malloryPub, malloryPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	malloryNode := auth.NodeIDFor(malloryPub)
	if err := hub.nodes.SaveNode(ctx, auth.Node{
		ID: malloryNode, OwnerID: mallory.ID.Hex(), Kind: auth.NodeKindPhone,
		PublicKey: auth.EncodeNodePublicKey(malloryPub), EnrolledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("enrolling mallory's node: %v", err)
	}

	// An online guest's id, as the cloud mints them and every table shows.
	const onlineGuest = "b8f0c0ffeec0ffeeb8f0c0ffeec0ffee"
	forge := func(guestID string) string {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, auth.SeatReceiptClaims{
			GuestID: guestID,
			NodeID:  malloryNode,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:  guestID,
				Issuer:   "node:" + malloryNode,
				Audience: jwt.ClaimStrings{auth.AudienceSeat},
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
		})
		signed, err := token.SignedString(malloryPriv)
		if err != nil {
			t.Fatalf("signing: %v", err)
		}
		return signed
	}

	token, err := auth.CreateAccessToken(mallory.ID.Hex(), mallory.Username, false, time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"receipts": []string{forge(onlineGuest), forge(offlineGuest)}})
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
			Claimed bool `json:"claimed"`
		} `json:"seats"`
	}
	if err := json.NewDecoder(res.Body).Decode(&claimed); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(claimed.Seats) != 2 || claimed.Seats[0].Claimed || claimed.Seats[1].Claimed {
		t.Fatalf("seats = %+v, want both forged receipts refused", claimed.Seats)
	}

	claims := auth.NewKDBGuestClaims(hub.kdb)
	for _, victim := range []string{onlineGuest, offlineGuest} {
		if account, ok, _ := claims.AccountForGuest(ctx, victim); ok {
			t.Errorf("guest %s was claimed by %s on a forged receipt", victim, account)
		}
		// ClaimGuest records the guest as an identity before it moves any
		// history; no identity means nothing was moved.
		if id, err := hub.authStore.FindIdentity(ctx, models.IdentityProviderGuest, victim); err == nil {
			t.Errorf("guest %s's history was moved to %s", victim, id.UserID)
		}
	}
}
