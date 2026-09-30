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
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/replica"
	zsync "zolik/server/internal/sync"
)

// The whole offline story, end to end, in one process: a person signs in on
// the cloud, enrols their phone, goes somewhere with no internet, plays a real
// match against a bot on the phone's own embedded server, comes back, and
// finds the match in their history with their lifetime figures updated.
//
// Nothing here is mocked except the room the phone is in. The phone runs the
// same server binary the cloud does, plays through the same runtime and the
// same rules, stores the match in its own namespace, hands it up through the
// outbox, and the cloud replays every move before it believes a word of it.

func newNode(t *testing.T, cfg Config) *App {
	t.Helper()
	cfg.DBEngine = db.EngineKDB
	cfg.KDBPath = t.TempDir()
	cfg.BotThinkMinMS, cfg.BotThinkMaxMS = 1, 2
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("starting a node: %v", err)
	}
	t.Cleanup(func() { _ = a.Close(context.Background()) })
	return a
}

// signingKeyForTest gives this process a cloud signing key, and puts the
// keyring back afterwards: it is process-wide, as a server's identity has to
// be, so a test that leaves one installed changes how every later test's
// tokens are signed.
func signingKeyForTest(t *testing.T, issuer string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	restore := auth.SnapshotKeysForTest()
	t.Cleanup(restore)
	if err := auth.ConfigureSigningKeys(auth.KeyConfig{
		Seed: hex.EncodeToString(priv.Seed()), Issuer: issuer,
	}); err != nil {
		t.Fatalf("installing the signing key: %v", err)
	}
}

// nodeKeyForTest gives this process a node identity, the way an install's
// host.json does, and restores what was there afterwards.
func nodeKeyForTest(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("node key: %v", err)
	}
	restore := auth.SnapshotKeysForTest()
	t.Cleanup(restore)
	if err := auth.SetNodeKey(hex.EncodeToString(priv.Seed())); err != nil {
		t.Fatalf("installing the node key: %v", err)
	}
	return pub
}

// playToCompletion plays the person's seat until the match ends, and lets the
// runtime's own bot loop play the others - which is what a real table does,
// and what makes this exercise the write path a player's move actually takes.
func playToCompletion(t *testing.T, a *App, matchID string, seat string) models.Match {
	t.Helper()
	mgr := a.matchManager()
	ctx := context.Background()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		m, err := mgr.Current(ctx, matchID)
		if err != nil {
			t.Fatalf("reading the match: %v", err)
		}
		if m.Status == "completed" {
			return m
		}
		mod := mgr.Registry().Get(m.ModuleID)
		awaited := module.AwaitedSeats(mod, module.State(m.State), seat, playerRefsForTest(m))
		if !containsString(awaited, seat) {
			// Somebody else's turn: the bot loop is playing it.
			time.Sleep(5 * time.Millisecond)
			continue
		}
		offers, err := mod.LegalActions(module.State(m.State), seat)
		if err != nil {
			t.Fatalf("legal actions: %v", err)
		}
		// A refusal is not always the offer list's fault - an offer is built
		// by probing the validator, so one can go stale between the probe and
		// the move - so every candidate is tried before giving up.
		played := false
		for _, action := range module.ChooseActions(offers, nil) {
			if err := mgr.HandleAction(ctx, matchID, seat, action); err == nil {
				played = true
				break
			}
		}
		if !played {
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("the match never finished")
	return models.Match{}
}

func playerRefsForTest(m models.Match) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(m.Players))
	for _, p := range m.Players {
		out = append(out, module.PlayerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}
	return out
}

func TestAMatchPlayedOfflineIsCreditedAfterItSyncs(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	phoneKey := nodeKeyForTest(t)

	// The cloud.
	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(context.Background())
	router := chi.NewRouter()
	hub.RegisterRoutes(router)
	cloud := httptest.NewServer(router)
	t.Cleanup(cloud.Close)

	ctx := context.Background()
	// A person with an account.
	player, err := hub.authStore.InsertUser(ctx, models.User{Username: "ada"})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	// And their phone, enrolled: the cloud knows which key that install signs
	// with, which is what lets it believe a match handed up by it later.
	nodeID := auth.NodeIDFor(phoneKey)
	if err := hub.nodes.SaveNode(ctx, auth.Node{
		ID: nodeID, OwnerID: player.ID.Hex(), Kind: auth.NodeKindPhone,
		PublicKey: auth.EncodeNodePublicKey(phoneKey), EnrolledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("enrolling the phone: %v", err)
	}
	credential, err := auth.CreateNodeCredential(nodeID, player.ID.Hex(), auth.NodeKindPhone, time.Hour)
	if err != nil {
		t.Fatalf("issuing the node credential: %v", err)
	}

	// The phone, which replicates its owner's data and nothing else.
	phone := newNode(t, Config{Sync: SyncConfig{
		Role:   syncRoleSpoke,
		HubURL: "ws://" + strings.TrimPrefix(cloud.URL, "http://") + "/kdb/sync",
		Token:  credential,
		Namespaces: []string{
			db.UserNS(player.ID.Hex()),
			db.UserReadOnlyNS(player.ID.Hex()),
			db.NodeOutboxNS(auth.NodeID()),
		},
	}})
	phone.Start(context.Background())

	// A table in a room with no internet: the person and one bot.
	mgr := phone.matchManager()
	host := models.Player{ID: "seat-ada", Name: "Ada", UserID: player.ID.Hex()}
	created, err := mgr.Create(ctx, "prsi", module.MatchConfig{}, host)
	if err != nil {
		t.Fatalf("opening the table: %v", err)
	}
	for _, bot := range []models.Player{
		{ID: "seat-bot1", Name: "Bot One", IsAI: true, AIDifficulty: "medium", AIPersona: "medium:bot-one"},
		{ID: "seat-bot2", Name: "Bot Two", IsAI: true, AIDifficulty: "medium", AIPersona: "medium:bot-two"},
	} {
		if _, err := mgr.Join(ctx, created.ID.Hex(), bot); err != nil {
			t.Fatalf("seating a bot: %v", err)
		}
	}
	if _, err := mgr.Start(ctx, created.ID.Hex()); err != nil {
		t.Fatalf("starting the match: %v", err)
	}
	finished := playToCompletion(t, phone, created.ID.Hex(), host.ID)
	if finished.Status != "completed" {
		t.Fatalf("the match ended as %q", finished.Status)
	}

	// The phone holds it, and nobody else does yet.
	if _, err := hub.matchRepo.FindByID(ctx, created.ID); err == nil {
		t.Fatal("the cloud knew about a match played with no internet")
	}

	// The phone comes back online: the match goes up, and the cloud checks it
	// by playing every move again before recording anything.
	waitFor(t, "the match to be handed up", func() bool {
		pending, err := phone.outbox().Pending()
		return err == nil && len(pending) == 1
	})
	if err := phone.sync.SyncNow(); err != nil {
		t.Fatalf("syncing: %v", err)
	}
	waitFor(t, "the cloud to import the match", func() bool {
		if err := hub.importer.Pass(ctx); err != nil {
			t.Fatalf("importing: %v", err)
		}
		if refused := hub.importer.Refused(); len(refused) > 0 {
			t.Fatalf("the cloud refused the match: %v", refused)
		}
		_, err := hub.matchRepo.FindByID(ctx, created.ID)
		return err == nil
	})

	// It is credited to the account that played it, with the same result the
	// phone saw.
	waitFor(t, "the result to be recorded", func() bool {
		_, err := hub.statsRepo.FindMatchByMatchID(ctx, created.ID)
		return err == nil
	})
	result, err := hub.statsRepo.FindMatchByMatchID(ctx, created.ID)
	if err != nil {
		t.Fatalf("reading the result: %v", err)
	}
	if !containsString(result.SubjectKeys, "user:"+player.ID.Hex()) {
		t.Fatalf("the match was credited to %v, not to the person who played it", result.SubjectKeys)
	}
	if result.ModuleID != "prsi" {
		t.Fatalf("recorded as %q", result.ModuleID)
	}

	// And it reaches the person's own history, which is what their phone reads
	// when they scroll back through their games with no connection at all.
	waitFor(t, "the match to reach the account's history", func() bool {
		_, err := hub.kdb.Get(db.UserReadOnlyNS(player.ID.Hex()), "matches/"+created.ID.Hex())
		return err == nil
	})
	// What the app reads on the phone: the account's own copy, served from
	// what this device holds rather than from the cloud.
	onPhone := replica.NewReader(phone.kdb, player.ID.Hex())
	waitFor(t, "the history to reach the phone", func() bool {
		if err := phone.sync.SyncNow(); err != nil {
			return false
		}
		rows, err := onPhone.Matches()
		return err == nil && len(rows) == 1
	})

	rows, err := onPhone.Matches()
	if err != nil {
		t.Fatalf("reading the phone's history: %v", err)
	}
	var entry struct {
		MatchID  string `json:"matchId"`
		ModuleID string `json:"moduleId"`
	}
	if err := json.Unmarshal(rows[0], &entry); err != nil {
		t.Fatalf("decoding the history entry: %v", err)
	}
	if entry.MatchID != created.ID.Hex() || entry.ModuleID != "prsi" {
		t.Fatalf("the phone's history says %+v", entry)
	}

	// The lifetime figures moved too, which is the thing a player actually
	// notices.
	ps, err := hub.statsRepo.FindPlayerStats(ctx, "user:"+player.ID.Hex())
	if err != nil {
		t.Fatalf("reading lifetime figures: %v", err)
	}
	if ps.Overall.Matches != 1 {
		t.Fatalf("lifetime matches = %d, want 1", ps.Overall.Matches)
	}
}

// TestAMatchFromAnUnenrolledPhoneIsRefused is the other half of believing a
// handed-up match: the cloud takes them from installs their owner enrolled,
// and from nothing else.
func TestAMatchFromAnUnenrolledPhoneIsRefused(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	nodeKeyForTest(t)

	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(context.Background())
	ctx := context.Background()

	// A bundle written straight into an outbox, as a phone that was never
	// enrolled could write one.
	bundle := sampleBundleForTest(t)
	doc, err := db.MarshalDoc(bundle)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if err := hub.kdb.Put(db.NodeOutboxNS(bundle.Node), "match/"+bundle.Match, doc); err != nil {
		t.Fatalf("writing the bundle: %v", err)
	}
	if err := hub.importer.Pass(ctx); err != nil {
		t.Fatalf("importing: %v", err)
	}
	if _, err := hub.matchRepo.FindByID(ctx, bson.NewObjectID()); err == nil {
		t.Fatal("a match from an install nobody enrolled was recorded")
	}
	if len(hub.importer.Refused()) != 1 {
		t.Fatalf("refused = %v, want the bundle kept with its reason", hub.importer.Refused())
	}
}

// sampleBundleForTest is a finished match as a node would hand one up, signed
// by this process's node key. What makes it refusable is not its contents but
// that nobody enrolled the node it names.
func sampleBundleForTest(t *testing.T) zsync.Bundle {
	t.Helper()
	b := zsync.Bundle{
		Kind:       "bundle",
		Match:      bson.NewObjectID().Hex(),
		Node:       auth.NodeID(),
		Module:     "prsi",
		Envelope:   json.RawMessage(`{"status":"completed"}`),
		Moves:      []json.RawMessage{},
		Seats:      map[string]string{"seat-1": "user:65f0"},
		FinishedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshalling the bundle: %v", err)
	}
	if b.Signature, err = auth.SignBundle(payload); err != nil {
		t.Fatalf("signing the bundle: %v", err)
	}
	return b
}

func waitFor(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
