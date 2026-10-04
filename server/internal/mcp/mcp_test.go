package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/blackjack"
	"zolik/server/internal/db"
	"zolik/server/internal/holdem"
	"zolik/server/internal/match"
	"zolik/server/internal/mcp"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

type rig struct {
	t   *testing.T
	m   *match.Manager
	reg *mcp.Registry
	srv *httptest.Server
	now time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	m := match.NewManager(match.NewKDBRepository(k), module.NewRegistry(holdem.New(), blackjack.New()), hub)
	m.SetBotPace(time.Millisecond, 2*time.Millisecond)
	m.SetSitOutGrace(150 * time.Millisecond)

	r := &rig{t: t, m: m, reg: mcp.NewRegistry(), now: time.Now()}
	r.reg.SetClock(func() time.Time { return r.now })
	router := chi.NewRouter()
	mcp.NewHandlers(m, r.reg).RegisterRoutes(router)
	r.srv = httptest.NewServer(router)
	t.Cleanup(r.srv.Close)
	return r
}

func token(t *testing.T, id string) string {
	t.Helper()
	tok, err := auth.CreateAccessToken(id, id, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// rpc posts one JSON-RPC request as id and returns the decoded response.
func (r *rig) rpc(id, method string, params any) map[string]any {
	r.t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest("POST", r.srv.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token(r.t, id))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

// tool calls a tool and returns its decoded JSON text and whether it errored.
func (r *rig) tool(id, name string, args map[string]any) (map[string]any, bool) {
	r.t.Helper()
	out := r.rpc(id, "tools/call", map[string]any{"name": name, "arguments": args})
	res, _ := out["result"].(map[string]any)
	if res == nil {
		r.t.Fatalf("%s: no result: %v", name, out)
	}
	content := res["content"].([]any)[0].(map[string]any)["text"].(string)
	var v map[string]any
	_ = json.Unmarshal([]byte(content), &v)
	isErr, _ := res["isError"].(bool)
	return v, isErr
}

func (r *rig) tick(d time.Duration) { r.now = r.now.Add(d) }

func TestProtocolBasics(t *testing.T) {
	r := newRig(t)
	init := r.rpc("a1", "initialize", map[string]any{"protocolVersion": "2025-03-26"})
	if init["result"].(map[string]any)["serverInfo"] == nil {
		t.Fatalf("initialize: %v", init)
	}
	list := r.rpc("a1", "tools/list", nil)
	tools := list["result"].(map[string]any)["tools"].([]any)
	if len(tools) < 6 {
		t.Fatalf("tools/list returned %d tools", len(tools))
	}
	if out := r.rpc("a1", "nope", nil); out["error"] == nil {
		t.Fatal("an unknown method must be a JSON-RPC error")
	}
	// No token, no entry.
	resp, _ := http.Post(r.srv.URL+"/mcp", "application/json", bytes.NewReader([]byte(`{}`)))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST /mcp = %d, want 401", resp.StatusCode)
	}
	// Tools need registration first.
	if v, isErr := r.tool("a1", "list_tables", nil); !isErr || v["code"] != "NOT_REGISTERED" {
		t.Fatalf("unregistered call = %v, %v", v, isErr)
	}
}

// An agent joins a poker table by code, sees the board and its legal actions,
// and plays.
func TestAgentJoinsAndActs(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	host := models.Player{ID: "h1", Name: "Host", UserID: "h1"}
	created, err := r.m.Create(ctx, "holdem", module.MatchConfig{}, host)
	if err != nil {
		t.Fatal(err)
	}
	if _, isErr := r.tool("a1", "register_agent", map[string]any{"name": "Claude", "label": "claude-fable-5-1"}); isErr {
		t.Fatal("register failed")
	}
	if v, isErr := r.tool("a1", "join_table", map[string]any{"table": created.JoinCode}); isErr {
		t.Fatalf("join: %v", v)
	}
	if _, err := r.m.Start(ctx, created.ID.Hex()); err != nil {
		t.Fatal(err)
	}

	seated, _ := r.m.Current(ctx, created.ID.Hex())
	var agent *models.Player
	for i := range seated.Players {
		if seated.Players[i].ID == "a1" {
			agent = &seated.Players[i]
		}
	}
	if agent == nil || !agent.IsAgent || agent.Name != "Claude" || agent.AgentLabel != "claude-fable-5-1" {
		t.Fatalf("agent seat = %+v", agent)
	}

	// The wire message flags the seat for clients.
	msg := r.m.BuildStateMsg(seated, "h1")
	flagged := false
	for _, p := range msg.Players {
		if p.ID == "a1" && p.IsAgent {
			flagged = true
		}
	}
	if !flagged {
		t.Fatal("state message does not mark the agent seat")
	}

	id := created.ID.Hex()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, isErr := r.tool("a1", "wait_for_turn", map[string]any{"matchId": id, "timeoutSeconds": 2})
		if isErr {
			t.Fatalf("wait_for_turn: %v", st)
		}
		offers, _ := st["legalActions"].([]any)
		for _, o := range offers {
			offer := o.(map[string]any)
			if offer["enabled"] != true {
				continue
			}
			{
				v, isErr := r.tool("a1", "act", map[string]any{"matchId": id, "offerId": offer["id"], "verb": offer["verb"]})
				if isErr {
					t.Fatalf("act: %v", v)
				}
				return
			}
		}
	}
	t.Fatal("never got a turn to act on")
}

// A stranger cannot read or play a table they are not seated at.
func TestAgentCannotTouchAnotherTable(t *testing.T) {
	r := newRig(t)
	created, _ := r.m.Create(context.Background(), "holdem", module.MatchConfig{}, models.Player{ID: "h1", Name: "Host", UserID: "h1"})
	r.tool("a1", "register_agent", map[string]any{"name": "Claude"})
	if v, isErr := r.tool("a1", "get_state", map[string]any{"matchId": created.ID.Hex()}); !isErr || v["code"] != "NOT_AT_THIS_TABLE" {
		t.Fatalf("get_state at a foreign table = %v", v)
	}
}

// The point of drop-in: one agent going quiet does not stop the table.
func TestSilentAgentIsSatOutAtPoker(t *testing.T) {
	for _, game := range []string{"holdem", "blackjack"} {
		t.Run(game, func(t *testing.T) { silentAgentIsSatOut(t, game) })
	}
}

func silentAgentIsSatOut(t *testing.T, game string) {
	r := newRig(t)
	ctx := context.Background()
	created, _ := r.m.Create(ctx, game, module.MatchConfig{}, models.Player{ID: "h1", Name: "Host", UserID: "h1"})
	id := created.ID.Hex()
	for _, a := range []string{"a1", "a2"} {
		r.tool(a, "register_agent", map[string]any{"name": a})
	}
	// The host is a person with no socket, so they are sat out from the start.
	for _, a := range []string{"a1", "a2"} {
		if v, isErr := r.tool(a, "join_table", map[string]any{"table": created.JoinCode}); isErr {
			t.Fatalf("join %s: %v", a, v)
		}
	}
	if _, err := r.m.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	// Past the presence TTL, then only a1 is heard from.
	r.tick(2 * mcp.PresenceTTL)
	r.reg.Touch("a1")

	acted, sawSatOut := 0, false
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && acted < 3 {
		r.reg.Touch("a1")
		cur, _ := r.m.Current(ctx, id)
		if cur.Status != "active" {
			t.Fatalf("table status = %q; a drop-in table must never pause", cur.Status)
		}
		msg := r.m.BuildStateMsg(cur, "a1")
		for _, p := range msg.Players {
			if p.SatOut {
				sawSatOut = true
			}
		}
		if r.m.Awaits(ctx, id, "a1") {
			st, _ := r.tool("a1", "get_state", map[string]any{"matchId": id})
			if playable, _ := st["playable"].([]any); len(playable) > 0 {
				args := playable[0].(map[string]any)
				args["matchId"] = id
				if _, isErr := r.tool("a1", "act", args); !isErr {
					acted++
				}
			}
			continue
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !sawSatOut {
		t.Error("no seat was ever marked sat out")
	}
	if acted < 3 {
		t.Errorf("a1 acted %d times; the table stalled on its silent neighbours", acted)
	}
}

func TestHostSeatsAnAvailableAgent(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	created, _ := r.m.Create(ctx, "blackjack", module.MatchConfig{}, models.Player{ID: "h1", Name: "Host", UserID: "h1"})
	r.tool("a1", "register_agent", map[string]any{"name": "Claude", "available": true})

	post := func(path, as string, body any) *http.Response {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", r.srv.URL+path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token(t, as))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	path := "/matches/" + created.ID.Hex() + "/add-agent"
	if c := post(path, "stranger", map[string]string{"agentId": "a1"}).StatusCode; c != http.StatusForbidden {
		t.Fatalf("non-host add-agent = %d, want 403", c)
	}
	if c := post(path, "h1", map[string]string{"agentId": "nobody"}).StatusCode; c != http.StatusConflict {
		t.Fatalf("unknown agent = %d, want 409", c)
	}
	if c := post(path, "h1", map[string]string{"agentId": "a1"}).StatusCode; c != http.StatusOK {
		t.Fatalf("add-agent = %d, want 200", c)
	}
	v, _ := r.tool("a1", "list_tables", nil)
	if tables := v["tables"].([]any); len(tables) != 1 {
		t.Fatalf("the seated agent sees %d tables, want 1", len(tables))
	}
}
