package match_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/auth"
	"zolik/server/internal/canasta"
	"zolik/server/internal/db"
	"zolik/server/internal/match"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/ws"
	"zolik/server/internal/zolikmod"
)

// Invite is the "pick them up out of the waiting room" half of the lobby
// feature: a host seats a specific player directly, without a join code.
//
// Ported from the Žolíky-specific route it was written against. What it tests
// is unchanged, because none of it was ever about rummy — host-only gating,
// capacity, and whether the invite actually consulted the pool are runtime
// questions, and they now have one answer for every game.
// These are end-to-end HTTP tests against a real MongoDB, in the same style
// as internal/auth/handlers_test.go, because the behaviour under test —
// host-only gating, capacity, and "did the invite actually consult the
// waiting pool" — is exactly what a mock of the repository would have to
// fake.

func testMongoURI() string {
	if v := strings.TrimSpace(os.Getenv("ZOLIK_TEST_MONGO_URI")); v != "" {
		return v
	}
	return "mongodb://127.0.0.1:27018"
}

// newTestRepository builds the match repository these tests run on: the dev
// compose stack's MongoDB by default (skipping when unreachable), or the
// embedded KDB engine when ZOLIK_TEST_DB_ENGINE=kdb — same tests, same
// routes, other storage engine.
func newTestRepository(t *testing.T) match.Repository {
	t.Helper()
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ZOLIK_TEST_DB_ENGINE")), db.EngineKDB) {
		k, err := db.OpenKDB(t.TempDir())
		if err != nil {
			t.Fatalf("opening kdb: %v", err)
		}
		t.Cleanup(func() { _ = k.Close(context.Background()) })
		return match.NewKDBRepository(k)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI()))
	if err != nil {
		t.Skipf("could not build a mongo client: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("no reachable mongo at %s (set ZOLIK_TEST_MONGO_URI, or start the dev compose stack): %v",
			testMongoURI(), err)
	}
	dbName := fmt.Sprintf("zolik_invitetest_%d", time.Now().UnixNano())
	m := &db.Mongo{Client: client, DB: client.Database(dbName)}
	if err := m.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensuring indexes: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = m.DB.Drop(dropCtx)
		_ = client.Disconnect(dropCtx)
	})
	return match.NewRepository(m)
}

// fakeWaitingRoom is a minimal, in-test stand-in for *lobby.Store —
// match.WaitingLookup is a narrow, primitive-typed interface precisely so
// nothing here needs to depend on the real lobby package to exercise it.
type fakeWaitingRoom struct {
	mu      sync.Mutex
	waiting map[string]struct {
		name    string
		isGuest bool
	}
	pickedUp []string
}

func newFakeWaitingRoom() *fakeWaitingRoom {
	return &fakeWaitingRoom{waiting: map[string]struct {
		name    string
		isGuest bool
	}{}}
}

func (f *fakeWaitingRoom) add(playerID, name string, isGuest bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waiting[playerID] = struct {
		name    string
		isGuest bool
	}{name, isGuest}
}

func (f *fakeWaitingRoom) IsWaiting(_ context.Context, playerID string) (string, bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.waiting[playerID]
	return e.name, e.isGuest, ok
}

func (f *fakeWaitingRoom) Pickup(_ context.Context, playerID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.waiting[playerID]; !ok {
		return false
	}
	delete(f.waiting, playerID)
	f.pickedUp = append(f.pickedUp, playerID)
	return true
}

const testWaitingRoom = "test-lobby-room"

type inviteHarness struct {
	t       *testing.T
	server  *httptest.Server
	waiting *fakeWaitingRoom
	hub     *ws.Hub
}

func newInviteHarness(t *testing.T) *inviteHarness {
	t.Helper()

	repo := newTestRepository(t)

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })

	mods := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New())
	manager := match.NewManager(repo, mods, hub)
	waiting := newFakeWaitingRoom()
	manager.SetWaitingRoom(waiting, testWaitingRoom)

	r := chi.NewRouter()
	match.NewHandlers(manager, false).RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return &inviteHarness{t: t, server: srv, waiting: waiting, hub: hub}
}

type apiResponse struct {
	status int
	body   map[string]any
	raw    string
}

func (h *inviteHarness) do(method, path, bearer string, body any) apiResponse {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshalling request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := apiResponse{status: resp.StatusCode, raw: string(raw)}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func (r apiResponse) str(key string) string {
	v, _ := r.body[key].(string)
	return v
}

func (r apiResponse) num(key string) float64 {
	v, _ := r.body[key].(float64)
	return v
}

func (r apiResponse) boolean(key string) bool {
	v, _ := r.body[key].(bool)
	return v
}

// token mints a real access token, exactly what a signed-in client presents —
// the invite handler is reached only through auth.AuthMiddleware, same as in
// production.
func token(t *testing.T, subject, username string, isGuest bool) string {
	t.Helper()
	tok, err := auth.CreateAccessToken(subject, username, isGuest, time.Hour)
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}
	return tok
}

// createMatch opens a table. The module is named because *some* module has to
// be, not because invites care which: the seat being filled is an envelope
// property, and none of these tests mentions a rule of any game.
func (h *inviteHarness) createMatch(t *testing.T, hostToken string) (matchID string) {
	t.Helper()
	res := h.do(http.MethodPost, "/matches", hostToken, map[string]any{"moduleId": "prsi"})
	if res.status != http.StatusOK {
		t.Fatalf("create match: status %d body %s", res.status, res.raw)
	}
	return res.str("matchId")
}

func TestInviteSeatsAWaitingPlayerNotifiesThemAndRemovesThemFromThePool(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	h.waiting.add("waiter-1", "Waiter", true)

	res := h.do(http.MethodPost, "/matches/"+matchID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if res.status != http.StatusOK {
		t.Fatalf("invite: status %d body %s", res.status, res.raw)
	}
	if !res.boolean("invited") {
		t.Error("invited = false, want true")
	}
	if res.num("playerCount") != 2 {
		t.Errorf("playerCount = %v, want 2 (host + invited)", res.body["playerCount"])
	}

	lobby := h.do(http.MethodGet, "/matches/"+matchID, "", nil)
	players, _ := lobby.body["players"].([]any)
	found := false
	for _, p := range players {
		m := p.(map[string]any)
		if m["id"] == "waiter-1" {
			found = true
			if m["name"] != "Waiter" {
				t.Errorf("invited player's name = %v, want Waiter (taken from the waiting entry)", m["name"])
			}
		}
	}
	if !found {
		t.Fatalf("invited player is not in the lobby's player list: %v", players)
	}

	h.waiting.mu.Lock()
	pickedUp := append([]string(nil), h.waiting.pickedUp...)
	h.waiting.mu.Unlock()
	if len(pickedUp) != 1 || pickedUp[0] != "waiter-1" {
		t.Errorf("pickedUp = %v, want exactly [waiter-1] — the invite must remove them from the pool", pickedUp)
	}
}

func TestInviteIsRefusedForAnyoneButTheHost(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)
	h.waiting.add("waiter-1", "Waiter", false)

	otherToken := token(t, "someone-else", "Someone", false)
	res := h.do(http.MethodPost, "/matches/"+matchID+"/invite", otherToken, map[string]any{"playerId": "waiter-1"})
	if res.status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 — only the host may invite", res.status)
	}
}

func TestInviteIsRefusedWhenTheTargetHasStoppedWaiting(t *testing.T) {
	// The host's client may be showing a stale snapshot — the target could
	// have left, been picked up elsewhere, or never existed. The handler
	// must re-check rather than trust the request.
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	res := h.do(http.MethodPost, "/matches/"+matchID+"/invite", hostToken, map[string]any{"playerId": "ghost"})
	if res.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a player who is not actually waiting", res.status)
	}
}

func TestInvitingAnAlreadySeatedPlayerIsIdempotent(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)
	h.waiting.add("waiter-1", "Waiter", false)

	first := h.do(http.MethodPost, "/matches/"+matchID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if first.status != http.StatusOK {
		t.Fatalf("first invite: status %d body %s", first.status, first.raw)
	}

	// Re-add them to the fake pool (as if they reconnected) and invite again;
	// the handler's own already-joined check must short-circuit before ever
	// asking whether they're still waiting.
	h.waiting.add("waiter-1", "Waiter", false)
	second := h.do(http.MethodPost, "/matches/"+matchID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if second.status != http.StatusOK {
		t.Fatalf("second invite: status %d body %s", second.status, second.raw)
	}
	if !second.boolean("alreadyJoined") {
		t.Error("alreadyJoined = false on a repeat invite of a seated player")
	}
}

// maxPlayers reads a module's seat count out of its own descriptor.
func (h *inviteHarness) maxPlayers(t *testing.T, moduleID string) int {
	t.Helper()
	res := h.do(http.MethodGet, "/modules", "", nil)
	if res.status != http.StatusOK {
		t.Fatalf("GET /modules: status %d body %s", res.status, res.raw)
	}
	mods, _ := res.body["modules"].([]any)
	for _, raw := range mods {
		m, _ := raw.(map[string]any)
		if id, _ := m["id"].(string); id == moduleID {
			n, _ := m["maxPlayers"].(float64)
			return int(n)
		}
	}
	t.Fatalf("module %q is not hosted", moduleID)
	return 0
}

func TestInviteRespectsLobbyCapacity(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	matchID := h.createMatch(t, hostToken)

	// How many seats this game has is the *module's* answer, read from the
	// descriptor rather than written down here. Hard-coding it worked while
	// there was one game with eight seats; Prší has six, and a test that knows
	// a number the server owns is a test that breaks on the next game.
	capacity := h.maxPlayers(t, "prsi")

	// The host already holds one seat; fill the rest via ordinary self-join so
	// this test exercises invite's capacity check specifically, not a second
	// copy of the join logic.
	for i := 1; i < capacity; i++ {
		playerToken := token(t, fmt.Sprintf("filler-%d", i), fmt.Sprintf("Filler%d", i), false)
		res := h.do(http.MethodPost, "/matches/"+matchID+"/join", playerToken, nil)
		if res.status != http.StatusOK {
			t.Fatalf("filler %d join: status %d body %s", i, res.status, res.raw)
		}
	}

	h.waiting.add("waiter-1", "Waiter", false)
	res := h.do(http.MethodPost, "/matches/"+matchID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if res.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409 — the table is already full", res.status)
	}
	// And the player is still waiting: a refused seat must not quietly drop
	// them out of the pool.
	if _, _, ok := h.waiting.IsWaiting(context.Background(), "waiter-1"); !ok {
		t.Error("a refused invite removed the player from the waiting room")
	}
}

func TestInviteWithNoWaitingRoomWiredInIsUnavailable(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	// A deployment that never wired the lobby package in never calls
	// SetWaitingRoom — see match.Manager.SetWaitingRoom.
	res := h.doAgainstHandlersWithNoWaitingRoom(t, hostToken)
	if res.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when no waiting room is configured", res.status)
	}
}

// doAgainstHandlersWithNoWaitingRoom stands up a second server sharing
// nothing with h but the Mongo connection, wired with waiting=nil, to prove
// the documented degrade-gracefully behaviour independent of any fake.
func (h *inviteHarness) doAgainstHandlersWithNoWaitingRoom(t *testing.T, hostToken string) apiResponse {
	t.Helper()

	repo := newTestRepository(t)

	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mods := module.NewRegistry(zolikmod.New(), prsi.New(), canasta.New())
	// Deliberately no waiting room wired in.
	manager := match.NewManager(repo, mods, hub)
	r := chi.NewRouter()
	match.NewHandlers(manager, false).RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	local := &inviteHarness{t: t, server: srv}
	create := local.do(http.MethodPost, "/matches", hostToken, map[string]any{"moduleId": "prsi"})
	if create.status != http.StatusOK {
		t.Fatalf("create match on the no-waiting-room server: status %d body %s", create.status, create.raw)
	}
	return local.do(http.MethodPost, "/matches/"+create.str("matchId")+"/invite", hostToken, map[string]any{"playerId": "anyone"})
}
