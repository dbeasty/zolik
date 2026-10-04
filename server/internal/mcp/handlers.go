package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

const protocolVersion = "2025-03-26"

const instructions = `You can play card games at Zolik tables. Authenticate with a bearer token (POST /auth/guest returns one). ` +
	`Call register_agent first. Then either join_table with a join code, or set available=true and wait for a host to seat you, ` +
	`and find the table with list_tables. Play a table with wait_for_turn, get_state and act: only play offers whose enabled is true. ` +
	`Poker and blackjack carry on if you go quiet for about twenty seconds (your seat checks or folds, stands and takes no insurance until you return); ` +
	`other games wait for you, and are abandoned after two minutes.`

// Handlers serves the MCP endpoint and the host-facing routes beside it.
type Handlers struct {
	manager  *match.Manager
	registry *Registry
}

func NewHandlers(m *match.Manager, r *Registry) *Handlers {
	m.SetAgentPresence(r)
	return &Handlers{manager: m, registry: r}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.With(auth.AuthMiddleware).Post("/mcp", h.serve)
	// Streamable HTTP lets a client open a GET stream for server-initiated
	// messages. There are none here, which the spec allows us to say with 405.
	r.Get("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", "POST")
		http.Error(w, "this server answers POST only", http.StatusMethodNotAllowed)
	})
	// The agents a host may seat, and seating one. {id} and not {matchId}:
	// chi keys a segment by position, so a different name would silently
	// replace the match group's own route.
	r.With(auth.AuthMiddleware).Get("/agents/available", h.listAvailable)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/add-agent", h.addAgent)
}

func (h *Handlers) serve(w http.ResponseWriter, req *http.Request) {
	uc, _ := auth.GetUserContext(req)
	var rq rpcRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&rq); err != nil {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{errParse, "parse error"}})
		return
	}
	if rq.JSONRPC != "2.0" || rq.Method == "" {
		writeRPC(w, rpcResponse{JSONRPC: "2.0", ID: nullID(rq.ID), Error: &rpcError{errInvalidRequest, "invalid request"}})
		return
	}
	h.registry.Touch(uc.UserID)
	if rq.isNotification() {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: rq.ID}
	switch rq.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(rq.Params, &p)
		v := p.ProtocolVersion
		if v == "" {
			v = protocolVersion
		}
		resp.Result = map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "zolik", "version": "1"},
			"instructions":    instructions,
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = json.RawMessage(toolList())
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(rq.Params, &p); err != nil || p.Name == "" {
			resp.Error = &rpcError{errInvalidParams, "tools/call needs a name"}
			break
		}
		resp.Result = h.call(req.Context(), uc, p.Name, p.Arguments)
	default:
		resp.Error = &rpcError{errNoMethod, "unknown method " + rq.Method}
	}
	writeRPC(w, resp)
}

func nullID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return id
}

func writeRPC(w http.ResponseWriter, r rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r)
}

// toolResult is MCP's tools/call result: text content, and isError when the
// call failed in a way the model should read and recover from, as opposed to a
// protocol error.
type toolResult struct {
	Content []map[string]string `json:"content"`
	IsError bool                `json:"isError,omitempty"`
}

func ok(v any) toolResult {
	b, _ := json.Marshal(v)
	return toolResult{Content: []map[string]string{{"type": "text", "text": string(b)}}}
}

func fail(code, msg string, extra map[string]any) toolResult {
	body := map[string]any{"code": code, "message": msg}
	for k, v := range extra {
		body[k] = v
	}
	r := ok(body)
	r.IsError = true
	return r
}

func (h *Handlers) call(ctx context.Context, uc auth.UserContext, name string, args json.RawMessage) toolResult {
	if name == "register_agent" {
		return h.register(uc, args)
	}
	if _, known := h.registry.Get(uc.UserID); !known {
		return fail("NOT_REGISTERED", "call register_agent first", nil)
	}
	switch name {
	case "set_available":
		var a struct {
			Available bool `json:"available"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return fail("BAD_ARGUMENTS", err.Error(), nil)
		}
		h.registry.SetAvailable(uc.UserID, a.Available)
		return ok(map[string]any{"available": a.Available})
	case "list_tables":
		return h.listTables(ctx, uc)
	case "join_table":
		return h.join(ctx, uc, args)
	case "get_state":
		return h.state(ctx, uc, args)
	case "wait_for_turn":
		return h.wait(ctx, uc, args)
	case "act":
		return h.act(ctx, uc, args)
	}
	return fail("UNKNOWN_TOOL", "no tool named "+name, nil)
}

func (h *Handlers) register(uc auth.UserContext, args json.RawMessage) toolResult {
	var a struct {
		Name      string `json:"name"`
		Label     string `json:"label"`
		Available bool   `json:"available"`
	}
	if err := json.Unmarshal(args, &a); err != nil || strings.TrimSpace(a.Name) == "" {
		return fail("BAD_ARGUMENTS", "register_agent needs a name", nil)
	}
	ag := h.registry.Register(uc.UserID, a.Name, a.Label, a.Available, uc.IsGuest)
	return ok(map[string]any{"agentId": ag.ID, "name": ag.Name, "available": ag.Available})
}

// agentSeat is the player an agent sits as.
func agentSeat(ag Agent) func(models.Match) models.Player {
	uc := auth.UserContext{UserID: ag.ID, IsGuest: ag.Guest}
	return func(models.Match) models.Player {
		return models.Player{
			ID:         ag.ID,
			Name:       ag.Name,
			IsAgent:    true,
			AgentLabel: ag.Label,
			Avatar:     "agent",
			UserID:     uc.PlayerUserID(),
			GuestID:    uc.PlayerGuestID(),
		}
	}
}

func (h *Handlers) listTables(ctx context.Context, uc auth.UserContext) toolResult {
	rows, err := h.manager.Repo().FindForPlayer(ctx, uc.UserID, match.PlayerMatchFilter{Statuses: match.UnfinishedStatuses})
	if err != nil {
		return fail("ERROR", err.Error(), nil)
	}
	out := []map[string]any{}
	for _, m := range rows {
		id := m.ID.Hex()
		h.manager.TrackAgent(id, uc.UserID)
		out = append(out, map[string]any{
			"matchId": id, "moduleId": m.ModuleID, "status": m.Status, "joinCode": m.JoinCode,
			"yourTurn": h.manager.Awaits(ctx, id, uc.UserID),
		})
	}
	return ok(map[string]any{"tables": out})
}

func (h *Handlers) join(ctx context.Context, uc auth.UserContext, args json.RawMessage) toolResult {
	var a struct {
		Table string `json:"table"`
	}
	if err := json.Unmarshal(args, &a); err != nil || a.Table == "" {
		return fail("BAD_ARGUMENTS", "join_table needs a table", nil)
	}
	ag, _ := h.registry.Get(uc.UserID)
	m, _, err := h.manager.JoinWith(ctx, a.Table, agentSeat(ag))
	if err != nil {
		return fail(module.CodeOf(err), err.Error(), nil)
	}
	h.manager.TrackAgent(m.ID.Hex(), uc.UserID)
	return ok(map[string]any{"matchId": m.ID.Hex(), "moduleId": m.ModuleID, "status": m.Status})
}

func (h *Handlers) state(ctx context.Context, uc auth.UserContext, args json.RawMessage) toolResult {
	var a struct {
		MatchID string `json:"matchId"`
	}
	if err := json.Unmarshal(args, &a); err != nil || a.MatchID == "" {
		return fail("BAD_ARGUMENTS", "get_state needs a matchId", nil)
	}
	return h.stateOf(ctx, uc, a.MatchID)
}

func (h *Handlers) stateOf(ctx context.Context, uc auth.UserContext, matchID string) toolResult {
	m, err := h.manager.Current(ctx, matchID)
	if err != nil {
		return fail("MATCH_NOT_FOUND", matchID, nil)
	}
	if !seated(m, uc.UserID) {
		return fail("NOT_AT_THIS_TABLE", "you are not seated here", nil)
	}
	h.manager.TrackAgent(m.ID.Hex(), uc.UserID)
	h.manager.ResumeIfReturning(ctx, m.ID.Hex(), uc.UserID)
	msg := h.manager.BuildStateMsg(m, uc.UserID)
	out := agentState{MatchStateMsg: msg, Playable: []module.Action{}}
	for _, o := range msg.LegalActions {
		if a, ok := module.SubmissionFor(o); ok {
			out.Playable = append(out.Playable, a)
		}
	}
	return ok(out)
}

// agentState is what get_state returns: the state message every client gets,
// plus Playable — for each enabled offer, the action that sends it as the
// offer itself describes it (a bet at its minimum, a raise at the minimum
// raise). A model can echo one straight back to act; it is a starting point,
// and the offer's params say how far it may be changed.
type agentState struct {
	match.MatchStateMsg
	Playable []module.Action `json:"playable"`
}

func seated(m models.Match, id string) bool {
	for _, p := range m.Players {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (h *Handlers) wait(ctx context.Context, uc auth.UserContext, args json.RawMessage) toolResult {
	var a struct {
		MatchID string `json:"matchId"`
		Timeout int    `json:"timeoutSeconds"`
	}
	if err := json.Unmarshal(args, &a); err != nil || a.MatchID == "" {
		return fail("BAD_ARGUMENTS", "wait_for_turn needs a matchId", nil)
	}
	if a.Timeout <= 0 {
		a.Timeout = 20
	}
	if a.Timeout > 25 {
		a.Timeout = 25
	}
	deadline := time.Now().Add(time.Duration(a.Timeout) * time.Second)
	for {
		m, err := h.manager.Current(ctx, a.MatchID)
		if err != nil {
			return fail("MATCH_NOT_FOUND", a.MatchID, nil)
		}
		if !seated(m, uc.UserID) {
			return fail("NOT_AT_THIS_TABLE", "you are not seated here", nil)
		}
		h.registry.Touch(uc.UserID)
		h.manager.ResumeIfReturning(ctx, m.ID.Hex(), uc.UserID)
		if m.Status == "completed" || m.Status == "abandoned" || h.manager.Awaits(ctx, a.MatchID, uc.UserID) || !time.Now().Before(deadline) {
			return h.stateOf(ctx, uc, a.MatchID)
		}
		select {
		case <-ctx.Done():
			return fail("CANCELLED", "request cancelled", nil)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (h *Handlers) act(ctx context.Context, uc auth.UserContext, args json.RawMessage) toolResult {
	var a struct {
		MatchID string `json:"matchId"`
		module.Action
	}
	if err := json.Unmarshal(args, &a); err != nil || a.MatchID == "" || a.Verb == "" {
		return fail("BAD_ARGUMENTS", "act needs a matchId and a verb", nil)
	}
	h.manager.ResumeIfReturning(ctx, a.MatchID, uc.UserID)
	if err := h.manager.HandleAction(ctx, a.MatchID, uc.UserID, a.Action); err != nil {
		extra := map[string]any{}
		if ids := h.manager.ExplainRefusal(ctx, a.MatchID, module.CodeOf(err)); len(ids) > 0 {
			extra["ruleIds"] = ids
		}
		return fail(module.CodeOf(err), err.Error(), extra)
	}
	return h.stateOf(ctx, uc, a.MatchID)
}

func (h *Handlers) listAvailable(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"agents": h.registry.Available()})
}

// addAgent seats an available agent at the caller's own table.
func (h *Handlers) addAgent(w http.ResponseWriter, req *http.Request) {
	uc, okc := auth.GetUserContext(req)
	if !okc {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		AgentID string `json:"agentId"`
	}
	_ = json.NewDecoder(req.Body).Decode(&body)
	m, err := h.manager.Repo().Resolve(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if m.HostID != uc.UserID {
		jsonError(w, http.StatusForbidden, "NOT_THE_HOST")
		return
	}
	ag, found := h.registry.Get(body.AgentID)
	if !found || !ag.Available || !h.registry.Present(ag.ID) {
		jsonError(w, http.StatusConflict, "AGENT_UNAVAILABLE")
		return
	}
	joined, _, err := h.manager.JoinWith(req.Context(), m.ID.Hex(), agentSeat(ag))
	if err != nil {
		jsonError(w, http.StatusBadRequest, module.CodeOf(err))
		return
	}
	h.manager.TrackAgent(joined.ID.Hex(), ag.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"playerId": ag.ID, "name": ag.Name, "isAgent": true})
}

func jsonError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"code":%q}`, code)
}
