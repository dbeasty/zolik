package app

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/replica"
)

// Two of one person's devices, both offline, both changing their own copy of
// the same things, and what the two of them agree on afterwards.
//
// This is the case the whole merge story exists for, and the one that is easy
// to get wrong in a way nobody notices until somebody's settings quietly
// revert. Neither device is the authority; the cloud is, and it decides with
// zolik's own rules rather than by taking whichever write happened to arrive
// second.
func TestTwoDevicesEditingOfflineConverge(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")

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
	hubURL := "ws://" + strings.TrimPrefix(cloud.URL, "http://") + "/kdb/sync"

	device := func(name string) (*App, *replica.Writer, *replica.Reader) {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatalf("%s key: %v", name, err)
		}
		// Each device has its own node key, and the keyring is process-wide,
		// so the credential is minted while that key is installed.
		restore := auth.SnapshotKeysForTest()
		if err := auth.SetNodeKey(hex.EncodeToString(priv.Seed())); err != nil {
			t.Fatalf("%s node key: %v", name, err)
		}
		nodeID := auth.NodeIDFor(pub)
		if err := hub.nodes.SaveNode(ctx, auth.Node{
			ID: nodeID, OwnerID: player.ID.Hex(), Kind: auth.NodeKindPhone,
			PublicKey: auth.EncodeNodePublicKey(pub), EnrolledAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("enrolling %s: %v", name, err)
		}
		credential, err := auth.CreateNodeCredential(nodeID, player.ID.Hex(), auth.NodeKindPhone, time.Hour)
		if err != nil {
			t.Fatalf("%s credential: %v", name, err)
		}
		restore()

		a := newNode(t, Config{Sync: SyncConfig{
			Role: syncRoleSpoke, HubURL: hubURL, Token: credential,
			Namespaces: []string{db.UserNS(player.ID.Hex()), db.UserReadOnlyNS(player.ID.Hex())},
		}})
		a.Start(context.Background())
		return a, replica.NewWriter(a.kdb, player.ID.Hex()), replica.NewReader(a.kdb, player.ID.Hex())
	}

	phone, phoneWrites, phoneReads := device("phone")
	tablet, tabletWrites, tabletReads := device("tablet")

	// Both start from the same settings.
	if err := phoneWrites.PutPrefs([]byte(`{"language":"cs","cardStyle":"classic"}`)); err != nil {
		t.Fatalf("phone first write: %v", err)
	}
	syncAll(t, phone, tablet)

	// Now each changes a different setting with no connection.
	if err := phoneWrites.PutPrefs([]byte(`{"language":"en","cardStyle":"classic"}`)); err != nil {
		t.Fatalf("phone: %v", err)
	}
	if err := tabletWrites.PutPrefs([]byte(`{"language":"cs","cardStyle":"vector"}`)); err != nil {
		t.Fatalf("tablet: %v", err)
	}
	// And both add a round to the same scorepad, which is the case where
	// taking one document whole would lose somebody's score.
	if err := phoneWrites.PutScoring("pad-1", []byte(`{"rounds":[{"id":"r1","score":10}]}`)); err != nil {
		t.Fatalf("phone pad: %v", err)
	}
	if err := tabletWrites.PutScoring("pad-1", []byte(`{"rounds":[{"id":"r2","score":20}]}`)); err != nil {
		t.Fatalf("tablet pad: %v", err)
	}

	// They come back online, in the order that makes the second one merge.
	syncAll(t, phone, tablet)
	syncAll(t, phone, tablet)

	waitFor(t, "both devices to agree about the settings", func() bool {
		a, aErr := phoneReads.Prefs()
		b, bErr := tabletReads.Prefs()
		return aErr == nil && bErr == nil && sameLanguageAndStyle(a, b)
	})

	waitFor(t, "both devices to hold every round of the scorepad", func() bool {
		// The cloud settles what the two devices disagreed about, and the
		// devices take the settlement on their next sync.
		hub.sync.Settle()
		syncAll(t, phone, tablet)
		return roundsOn(t, phoneReads) == 2 && roundsOn(t, tabletReads) == 2
	})

	// Neither device's change was dropped: the later of the two settings
	// wins as a whole document, but no round of the pad is lost, because a
	// scorepad's rounds are a set rather than a value.
	if got := roundsOn(t, phoneReads); got != 2 {
		t.Fatalf("the phone holds %d rounds, want both", got)
	}
	// And nothing is left for a person to look at: a conflict the rules can
	// settle should not also become an operator's problem.
	if open := hub.sync.OpenConflicts(); len(open) != 0 {
		t.Fatalf("conflicts left open after everything settled: %v", open)
	}
}

func syncAll(t *testing.T, nodes ...*App) {
	t.Helper()
	for _, n := range nodes {
		if err := n.sync.SyncNow(); err != nil {
			t.Fatalf("syncing: %v", err)
		}
	}
}

func sameLanguageAndStyle(a, b json.RawMessage) bool {
	var x, y struct {
		Language  string `json:"language"`
		CardStyle string `json:"cardStyle"`
	}
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return x.Language == y.Language && x.CardStyle == y.CardStyle && x.Language != "" && x.CardStyle != ""
}

func roundsOn(t *testing.T, rd *replica.Reader) int {
	t.Helper()
	pads, err := rd.Scoring()
	if err != nil || len(pads) == 0 {
		return 0
	}
	var pad struct {
		Rounds []struct {
			ID string `json:"id"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal(pads[0], &pad); err != nil {
		t.Fatalf("decoding the scorepad: %v", err)
	}
	return len(pad.Rounds)
}
