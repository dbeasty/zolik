package game_test

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
	"zolik/server/internal/db"
	"zolik/server/internal/game"
)

// Invite is the "pick them up out of the waiting room" half of the lobby
// feature: a host seats a specific player directly, without a join code.
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

// fakeWaitingRoom is a minimal, in-test stand-in for *lobby.Store —
// game.WaitingLookup is a narrow, primitive-typed interface precisely so
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
	hub     *game.Hub
}

func newInviteHarness(t *testing.T) *inviteHarness {
	t.Helper()
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

	repo := game.NewRepository(m)
	hub, err := game.NewHub(game.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	manager := game.NewManager(repo, hub)
	waiting := newFakeWaitingRoom()

	h := game.NewGameRestHandlers(repo, hub, manager, nil, waiting, testWaitingRoom, false)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
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

func (h *inviteHarness) createGame(t *testing.T, hostToken string) (gameID string) {
	t.Helper()
	res := h.do(http.MethodPost, "/games", hostToken, map[string]any{})
	if res.status != http.StatusOK {
		t.Fatalf("create game: status %d body %s", res.status, res.raw)
	}
	return res.str("gameId")
}

func TestInviteSeatsAWaitingPlayerNotifiesThemAndRemovesThemFromThePool(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	gameID := h.createGame(t, hostToken)

	h.waiting.add("waiter-1", "Waiter", true)

	res := h.do(http.MethodPost, "/games/"+gameID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if res.status != http.StatusOK {
		t.Fatalf("invite: status %d body %s", res.status, res.raw)
	}
	if !res.boolean("invited") {
		t.Error("invited = false, want true")
	}
	if res.num("playerCount") != 2 {
		t.Errorf("playerCount = %v, want 2 (host + invited)", res.body["playerCount"])
	}

	lobby := h.do(http.MethodGet, "/games/"+gameID, "", nil)
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
	gameID := h.createGame(t, hostToken)
	h.waiting.add("waiter-1", "Waiter", false)

	otherToken := token(t, "someone-else", "Someone", false)
	res := h.do(http.MethodPost, "/games/"+gameID+"/invite", otherToken, map[string]any{"playerId": "waiter-1"})
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
	gameID := h.createGame(t, hostToken)

	res := h.do(http.MethodPost, "/games/"+gameID+"/invite", hostToken, map[string]any{"playerId": "ghost"})
	if res.status != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a player who is not actually waiting", res.status)
	}
}

func TestInvitingAnAlreadySeatedPlayerIsIdempotent(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	gameID := h.createGame(t, hostToken)
	h.waiting.add("waiter-1", "Waiter", false)

	first := h.do(http.MethodPost, "/games/"+gameID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if first.status != http.StatusOK {
		t.Fatalf("first invite: status %d body %s", first.status, first.raw)
	}

	// Re-add them to the fake pool (as if they reconnected) and invite again;
	// the handler's own already-joined check must short-circuit before ever
	// asking whether they're still waiting.
	h.waiting.add("waiter-1", "Waiter", false)
	second := h.do(http.MethodPost, "/games/"+gameID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if second.status != http.StatusOK {
		t.Fatalf("second invite: status %d body %s", second.status, second.raw)
	}
	if !second.boolean("alreadyJoined") {
		t.Error("alreadyJoined = false on a repeat invite of a seated player")
	}
}

func TestInviteRespectsLobbyCapacity(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	gameID := h.createGame(t, hostToken)

	// The host already holds one seat; fill the rest via ordinary self-join
	// so this test exercises invite's capacity check specifically, not a
	// second copy of the join logic.
	for i := 1; i < 8; i++ {
		playerToken := token(t, fmt.Sprintf("filler-%d", i), fmt.Sprintf("Filler%d", i), false)
		res := h.do(http.MethodPost, "/games/"+gameID+"/join", playerToken, nil)
		if res.status != http.StatusOK {
			t.Fatalf("filler %d join: status %d body %s", i, res.status, res.raw)
		}
	}

	h.waiting.add("waiter-1", "Waiter", false)
	res := h.do(http.MethodPost, "/games/"+gameID+"/invite", hostToken, map[string]any{"playerId": "waiter-1"})
	if res.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 — the lobby is already full", res.status)
	}
}

func TestInviteWithNoWaitingRoomWiredInIsUnavailable(t *testing.T) {
	h := newInviteHarness(t)
	hostToken := token(t, "host-1", "Host", false)
	gameID := h.createGame(t, hostToken)

	// A deployment that never wired the lobby package in passes nil for
	// WaitingLookup — see game.NewGameRestHandlers's doc comment.
	res := h.doAgainstHandlersWithNoWaitingRoom(t, gameID, hostToken)
	if res.status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 when no waiting room is configured", res.status)
	}
}

// doAgainstHandlersWithNoWaitingRoom stands up a second server sharing
// nothing with h but the Mongo connection, wired with waiting=nil, to prove
// the documented degrade-gracefully behaviour independent of any fake.
func (h *inviteHarness) doAgainstHandlersWithNoWaitingRoom(t *testing.T, gameID, hostToken string) apiResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(testMongoURI()))
	if err != nil {
		t.Skipf("could not build a mongo client: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("mongo unreachable: %v", err)
	}
	m := &db.Mongo{Client: client, DB: client.Database(fmt.Sprintf("zolik_invitetest_nowaiting_%d", time.Now().UnixNano()))}
	if err := m.EnsureIndexes(ctx); err != nil {
		t.Fatalf("ensuring indexes: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = m.DB.Drop(dropCtx)
		_ = client.Disconnect(dropCtx)
	})

	repo := game.NewRepository(m)
	hub, err := game.NewHub(game.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	manager := game.NewManager(repo, hub)

	handlers := game.NewGameRestHandlers(repo, hub, manager, nil, nil, "", false)
	r := chi.NewRouter()
	handlers.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	local := &inviteHarness{t: t, server: srv}
	create := local.do(http.MethodPost, "/games", hostToken, map[string]any{})
	if create.status != http.StatusOK {
		t.Fatalf("create game on the no-waiting-room server: status %d body %s", create.status, create.raw)
	}
	return local.do(http.MethodPost, "/games/"+create.str("gameId")+"/invite", hostToken, map[string]any{"playerId": "anyone"})
}
