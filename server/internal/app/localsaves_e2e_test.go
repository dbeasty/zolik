package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// A game played on a phone reaches the cloud only when its players say so,
// and each of them only for their own seat.
//
// Three people at the phone's table with no internet: its owner, a friend
// who has an account (seated with an offline pass), and a guest. The game
// ends, and nothing leaves the phone. The friend says yes, the guest says no,
// and the owner saves it. The cloud then has the match credited to the owner
// and the friend, with the guest as an anonymous "Player 3" nobody can claim;
// the phone sees it arrive in its owner's history and lets the bundle go; and
// the cloud importing the same bundle again records nothing twice.
func TestALocalGameReachesTheCloudOnlyWithItsPlayersConsent(t *testing.T) {
	signingKeyForTest(t, "https://cloud.test")
	phoneKey := nodeKeyForTest(t)
	ctx := context.Background()

	hub := newNode(t, Config{Sync: SyncConfig{Role: syncRoleHub, Advertise: "hub"}})
	hub.Start(ctx)
	hubRouter := chi.NewRouter()
	hub.RegisterRoutes(hubRouter)
	cloud := httptest.NewServer(hubRouter)
	t.Cleanup(cloud.Close)

	ada, err := hub.authStore.InsertUser(ctx, models.User{Username: "ada"})
	if err != nil {
		t.Fatalf("creating ada: %v", err)
	}
	bob, err := hub.authStore.InsertUser(ctx, models.User{Username: "bob"})
	if err != nil {
		t.Fatalf("creating bob: %v", err)
	}
	nodeID := auth.NodeIDFor(phoneKey)
	if err := hub.nodes.SaveNode(ctx, auth.Node{
		ID: nodeID, OwnerID: ada.ID.Hex(), Kind: auth.NodeKindPhone,
		PublicKey: auth.EncodeNodePublicKey(phoneKey), EnrolledAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("enrolling the phone: %v", err)
	}
	credential, err := auth.CreateNodeCredential(nodeID, ada.ID.Hex(), auth.NodeKindPhone, time.Hour)
	if err != nil {
		t.Fatalf("credential: %v", err)
	}

	// Ada's phone: an embedded host, enrolled and signed in as her. Its own
	// per-install secret, as NewMobile installs one.
	mobileSecretForTest(t)
	phone := newNode(t, Config{
		Env: EnvMobile,
		Sync: SyncConfig{
			Role:   syncRoleSpoke,
			HubURL: "ws://" + strings.TrimPrefix(cloud.URL, "http://") + "/kdb/sync",
			Token:  credential,
			Namespaces: []string{
				db.UserNS(ada.ID.Hex()),
				db.UserReadOnlyNS(ada.ID.Hex()),
				db.NodeOutboxNS(auth.NodeID()),
			},
		},
	})
	phone.replicaUser = ada.ID.Hex()
	phone.Auth().SetOfflineKeys(auth.LocalKeys())
	phone.Start(ctx)
	phoneRouter := chi.NewRouter()
	phone.RegisterNodeRoutes(phoneRouter)
	// The phone's own app, on loopback, and the room.
	own := httptest.NewUnstartedServer(phoneRouter)
	own.Config.ConnContext = func(ctx context.Context, _ net.Conn) context.Context { return MarkLoopback(ctx) }
	own.Start()
	t.Cleanup(own.Close)
	room := httptest.NewServer(phoneRouter)
	t.Cleanup(room.Close)

	// Everybody sits down the way the app does it.
	seat := func(path string, body any) (subject, token string) {
		t.Helper()
		var out struct {
			AccessToken string `json:"accessToken"`
			UserID      string `json:"userId"`
		}
		postJSON(t, room.URL+path, "", body, http.StatusOK, &out)
		return out.UserID, out.AccessToken
	}
	adaPass, _ := auth.CreateOfflinePass(ada.ID.Hex(), "ada", 0, time.Hour)
	bobPass, _ := auth.CreateOfflinePass(bob.ID.Hex(), "bob", 0, time.Hour)
	adaSeat, _ := seat("/auth/offline-pass", map[string]string{"offlinePass": adaPass})
	bobSeat, bobToken := seat("/auth/offline-pass", map[string]string{"offlinePass": bobPass})
	carolSeat, carolToken := seat("/auth/guest", map[string]string{"name": "Carol"})

	mgr := phone.matchManager()
	created, err := mgr.Create(ctx, "prsi", module.MatchConfig{},
		models.Player{ID: adaSeat, Name: "ada", UserID: adaSeat})
	if err != nil {
		t.Fatalf("opening the table: %v", err)
	}
	matchID := created.ID.Hex()
	for _, p := range []models.Player{
		{ID: bobSeat, Name: "bob", UserID: bobSeat},
		{ID: carolSeat, Name: "Carol", GuestID: carolSeat},
	} {
		if _, err := mgr.Join(ctx, matchID, p); err != nil {
			t.Fatalf("seating %s: %v", p.Name, err)
		}
	}
	if _, err := mgr.Start(ctx, matchID); err != nil {
		t.Fatalf("starting: %v", err)
	}
	playSeatsToCompletion(t, phone, matchID, []string{adaSeat, bobSeat, carolSeat})

	// The game waits on the phone, and nothing has been handed up.
	var list struct {
		Enrolled bool              `json:"enrolled"`
		Games    []match.LocalSave `json:"games"`
	}
	waitFor(t, "the finished game to wait on the phone", func() bool {
		getJSON(t, own.URL+"/local/saves", "", http.StatusOK, &list)
		return len(list.Games) == 1
	})
	if !list.Enrolled || list.Games[0].State != match.SavePending {
		t.Fatalf("the phone says enrolled=%v, state=%q", list.Enrolled, list.Games[0].State)
	}
	if pending, _ := phone.outbox().Pending(); len(pending) != 0 {
		t.Fatalf("%d games were handed up before anybody said yes", len(pending))
	}

	// The list and the save belong to the phone's owner, not the room.
	getJSON(t, room.URL+"/local/saves", "", http.StatusNotFound, nil)
	postJSON(t, room.URL+"/local/saves/"+matchID+"/save", "", nil, http.StatusNotFound, nil)

	// Each player answers for their own seat.
	var answer struct {
		Available bool  `json:"available"`
		Consent   *bool `json:"consent"`
	}
	getJSON(t, room.URL+"/matches/"+matchID+"/save-consent", bobToken, http.StatusOK, &answer)
	if !answer.Available || answer.Consent != nil {
		t.Fatalf("bob is offered %+v before answering", answer)
	}
	postJSON(t, room.URL+"/matches/"+matchID+"/save-consent", bobToken, map[string]bool{"save": true}, http.StatusOK, &answer)
	if answer.Consent == nil || !*answer.Consent {
		t.Fatalf("bob's yes reads back as %+v", answer)
	}
	postJSON(t, room.URL+"/matches/"+matchID+"/save-consent", carolToken, map[string]bool{"save": false}, http.StatusOK, nil)

	// The owner saves it.
	postJSON(t, own.URL+"/local/saves/"+matchID+"/save", "", nil, http.StatusOK, nil)
	if pending, _ := phone.outbox().Pending(); len(pending) != 1 {
		t.Fatalf("after saving, %d bundles are in the outbox", len(pending))
	}

	if err := phone.sync.SyncNow(); err != nil {
		t.Fatalf("syncing: %v", err)
	}
	waitFor(t, "the cloud to import the game", func() bool {
		if err := hub.importer.Pass(ctx); err != nil {
			t.Fatalf("importing: %v", err)
		}
		if refused := hub.importer.Refused(); len(refused) > 0 {
			t.Fatalf("the cloud refused the game: %v", refused)
		}
		_, err := hub.matchRepo.FindByID(ctx, created.ID)
		return err == nil
	})
	stored, err := hub.matchRepo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("reading the cloud's copy: %v", err)
	}
	byName := map[string]models.Player{}
	for _, p := range stored.Players {
		byName[p.Name] = p
	}
	if byName["ada"].UserID != ada.ID.Hex() {
		t.Errorf("ada's seat is credited to %q", byName["ada"].UserID)
	}
	if byName["bob"].UserID != bob.ID.Hex() {
		t.Errorf("bob said yes, and his seat is credited to %q", byName["bob"].UserID)
	}
	if _, named := byName["Carol"]; named {
		t.Errorf("carol said no, and her name reached the cloud")
	}
	anon, ok := byName["Player 3"]
	if !ok || anon.UserID != "" || anon.GuestID != match.AnonymousGuestID(matchID, carolSeat) {
		t.Errorf("carol's seat arrived as %+v, want an anonymous Player 3", anon)
	}

	// The phone sees the game in ada's history and lets the bundle go.
	waitFor(t, "the phone to see the game arrive", func() bool {
		_ = phone.sync.SyncNow()
		phone.localSaves.Reconcile(ctx, func(m string) bool {
			_, err := phone.kdb.Get(db.UserReadOnlyNS(ada.ID.Hex()), "matches/"+m)
			return err == nil
		})
		rec, err := phone.localSaves.Get(matchID)
		return err == nil && rec.State == match.SaveUploaded
	})
	if pending, _ := phone.outbox().Pending(); len(pending) != 0 {
		t.Fatalf("an uploaded game is still in the outbox")
	}

	// Importing what is already there records nothing twice.
	for range 3 {
		if err := hub.importer.Pass(ctx); err != nil {
			t.Fatalf("importing again: %v", err)
		}
	}
	waitFor(t, "ada's figures", func() bool {
		ps, err := hub.statsRepo.FindPlayerStats(ctx, "user:"+ada.ID.Hex())
		return err == nil && ps.Overall.Matches == 1
	})
	time.Sleep(200 * time.Millisecond)
	if ps, _ := hub.statsRepo.FindPlayerStats(ctx, "user:"+ada.ID.Hex()); ps.Overall.Matches != 1 {
		t.Fatalf("ada has %d matches after the same game was imported again", ps.Overall.Matches)
	}
}

// A game nobody saves never leaves the phone, and a discarded one is gone.
func TestALocalGameNobodySavesStaysOnThePhone(t *testing.T) {
	ctx := context.Background()
	mobileSecretForTest(t)
	phone := newNode(t, Config{Env: EnvMobile})
	phone.Start(ctx)
	r := chi.NewRouter()
	phone.RegisterMobileRoutes(r)
	own := httptest.NewUnstartedServer(r)
	own.Config.ConnContext = func(ctx context.Context, _ net.Conn) context.Context { return MarkLoopback(ctx) }
	own.Start()
	t.Cleanup(own.Close)

	var guest struct {
		UserID string `json:"userId"`
	}
	postJSON(t, own.URL+"/auth/guest", "", map[string]string{"name": "Dee"}, http.StatusOK, &guest)
	mgr := phone.matchManager()
	created, err := mgr.Create(ctx, "prsi", module.MatchConfig{}, models.Player{ID: guest.UserID, Name: "Dee", GuestID: guest.UserID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := mgr.Join(ctx, created.ID.Hex(), models.Player{ID: "bot-1", Name: "Bot", IsAI: true, AIDifficulty: "easy"}); err != nil {
		t.Fatalf("bot: %v", err)
	}
	if _, err := mgr.Start(ctx, created.ID.Hex()); err != nil {
		t.Fatalf("start: %v", err)
	}
	playSeatsToCompletion(t, phone, created.ID.Hex(), []string{guest.UserID})

	var list struct {
		Enrolled bool              `json:"enrolled"`
		Games    []match.LocalSave `json:"games"`
	}
	waitFor(t, "the game to wait", func() bool {
		getJSON(t, own.URL+"/local/saves", "", http.StatusOK, &list)
		return len(list.Games) == 1
	})
	if list.Enrolled {
		t.Fatal("a phone nobody signed in on says it can save games")
	}
	// Nowhere to save it until somebody signs in.
	postJSON(t, own.URL+"/local/saves/"+created.ID.Hex()+"/save", "", nil, http.StatusConflict, nil)
	postJSON(t, own.URL+"/local/saves/"+created.ID.Hex()+"/discard", "", nil, http.StatusNoContent, nil)
	getJSON(t, own.URL+"/local/saves", "", http.StatusOK, &list)
	if len(list.Games) != 0 {
		t.Fatalf("a discarded game is still listed: %+v", list.Games)
	}
}

func mobileSecretForTest(t *testing.T) {
	t.Helper()
	auth.SetAccessSecret("a-development-secret-of-at-least-32-bytes")
	t.Cleanup(func() { auth.SetAccessSecret("") })
}

// playSeatsToCompletion plays every listed seat until the match ends; any
// other seat is the bot loop's.
func playSeatsToCompletion(t *testing.T, a *App, matchID string, seats []string) models.Match {
	t.Helper()
	mgr := a.matchManager()
	ctx := context.Background()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		m, err := mgr.Current(ctx, matchID)
		if err != nil {
			t.Fatalf("reading the match: %v", err)
		}
		if m.Status == "completed" {
			return m
		}
		mod := mgr.Registry().Get(m.ModuleID)
		played := false
		for _, seat := range seats {
			awaited := module.AwaitedSeats(mod, module.State(m.State), seat, playerRefsForTest(m))
			if !containsString(awaited, seat) {
				continue
			}
			offers, err := mod.LegalActions(module.State(m.State), seat)
			if err != nil {
				continue
			}
			for _, action := range module.ChooseActions(offers, nil) {
				if err := mgr.HandleAction(ctx, matchID, seat, action); err == nil {
					played = true
					break
				}
			}
			if played {
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

func postJSON(t *testing.T, url, token string, body any, want int, out any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	doJSON(t, req, token, want, out)
}

func getJSON(t *testing.T, url, token string, want int, out any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	doJSON(t, req, token, want, out)
}

func doJSON(t *testing.T, req *http.Request, token string, want int, out any) {
	t.Helper()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		var b bytes.Buffer
		_, _ = b.ReadFrom(res.Body)
		t.Fatalf("%s %s: status %d, want %d: %s", req.Method, req.URL.Path, res.StatusCode, want, b.String())
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decoding: %v", req.Method, req.URL.Path, err)
		}
	}
}

// A host whose phone saves every game the moment it ends must not send it
// before the others at the table have answered: a "yes" that reaches the
// cloud after the game did counts for nothing, and the player arrives as an
// anonymous "Player 2". This was seen on devices with "Always save" on.
func TestASavedGameWaitsForThePlayersStillAnswering(t *testing.T) {
	ctx := context.Background()
	nodeKeyForTest(t)
	mobileSecretForTest(t)
	const owner = "6ab5a0b3d71e872332ed588e"
	phone := newNode(t, Config{
		Env: EnvMobile,
		Sync: SyncConfig{
			Role: syncRoleSpoke, HubURL: "ws://127.0.0.1:1/kdb/sync", Token: "unused",
			Namespaces: []string{db.NodeOutboxNS(auth.NodeID())},
		},
	})
	phone.replicaUser = owner
	phone.Start(ctx)
	saves := phone.LocalSaves()
	if saves == nil || !saves.Enrolled() {
		t.Fatal("the phone has no outbox to save to")
	}

	host, guest := "user:"+owner, "4f1c0ffee4f1c0ffee4f1c0ffee4f1c0"
	mgr := phone.matchManager()
	created, err := mgr.Create(ctx, "prsi", module.MatchConfig{}, models.Player{ID: host, Name: "Ada", UserID: host})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	matchID := created.ID.Hex()
	if _, err := mgr.Join(ctx, matchID, models.Player{ID: guest, Name: "Bea", GuestID: guest}); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, err := mgr.Start(ctx, matchID); err != nil {
		t.Fatalf("start: %v", err)
	}
	playSeatsToCompletion(t, phone, matchID, []string{host, guest})
	waitFor(t, "the game to wait", func() bool {
		_, err := saves.Get(matchID)
		return err == nil
	})

	// The host saves at once; Bea has not answered.
	if _, err := saves.Save(ctx, matchID); err != nil {
		t.Fatalf("save: %v", err)
	}
	if pending, _ := phone.outbox().Pending(); len(pending) != 0 {
		t.Fatal("the game went before Bea had answered")
	}
	saves.Reconcile(ctx, nil)
	if pending, _ := phone.outbox().Pending(); len(pending) != 0 {
		t.Fatal("the game went inside the grace period")
	}

	// Her answer is the last one it waited for.
	if _, err := saves.Consent(ctx, matchID, guest, true); err != nil {
		t.Fatalf("consent: %v", err)
	}
	pending, _ := phone.outbox().Pending()
	if len(pending) != 1 || pending[0].Seats[guest] != "guest:"+guest {
		t.Fatalf("after Bea's yes the outbox holds %+v", pending)
	}

	// A player who never answers does not hold the game back for ever.
	_ = phone.outbox().Drop(matchID)
	_ = saves.Discard(matchID)
	created2, _ := mgr.Create(ctx, "prsi", module.MatchConfig{}, models.Player{ID: host, Name: "Ada", UserID: host})
	_, _ = mgr.Join(ctx, created2.ID.Hex(), models.Player{ID: guest, Name: "Bea", GuestID: guest})
	_, _ = mgr.Start(ctx, created2.ID.Hex())
	playSeatsToCompletion(t, phone, created2.ID.Hex(), []string{host, guest})
	waitFor(t, "the second game to wait", func() bool {
		_, err := saves.Get(created2.ID.Hex())
		return err == nil
	})
	if _, err := saves.Save(ctx, created2.ID.Hex()); err != nil {
		t.Fatalf("save: %v", err)
	}
	saves.SetAnswerGrace(0)
	saves.Reconcile(ctx, nil)
	pending, _ = phone.outbox().Pending()
	if len(pending) != 1 || pending[0].Seats[guest] == "guest:"+guest {
		t.Fatalf("after the grace period the outbox holds %+v, want the game with Bea anonymous", pending)
	}
}
