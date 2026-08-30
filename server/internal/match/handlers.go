package match

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// Handlers expose the generic runtime over HTTP and WebSocket.
//
// The route set is small because there is almost nothing game-specific to
// expose: list the games, open one, join, start, and then a socket that
// carries actions in and views out.
type Handlers struct {
	manager  *Manager
	upgrader websocket.Upgrader
	// testEndpoints enables the dev-only state seeder. Off outside local
	// development, because it writes game state without validating it.
	testEndpoints bool
	// admission turns new players away before the box runs out of memory.
	// Nil means no gating, which is what tests and unconstrained deployments
	// get — see SetAdmission.
	admission *admission.Controller
}

// SetAdmission wires in the capacity gate.
//
// Injected rather than taken in NewHandlers so that the many call sites that
// build handlers without caring about capacity — every handler test — keep
// working unchanged, and so a deployment that has not configured a ceiling
// behaves exactly as it did before.
func (h *Handlers) SetAdmission(c *admission.Controller) { h.admission = c }

func NewHandlers(m *Manager, testEndpoints bool) *Handlers {
	return &Handlers{
		manager:       m,
		testEndpoints: testEndpoints,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Every game this server can host, and what each one lets a lobby set. A
	// client renders its whole game-picker and new-match form from this.
	r.Get("/modules", h.listModules)
	r.Get("/modules/{id}/rules", h.moduleRules)
	r.Get("/matches/{id}", h.getMatch)
	r.With(auth.AuthMiddleware).Post("/matches", h.createMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/join", h.joinMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/start", h.startMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/add-bot", h.addBot)
	// Seat a specific player out of the waiting room, instead of reading a
	// join code out to them.
	r.With(auth.AuthMiddleware).Post("/matches/{id}/invite", h.invite)
	r.Get("/ws/matches/{id}", h.handleWS)

	if h.testEndpoints {
		r.With(auth.AuthMiddleware).Post("/matches/{id}/debug-state", h.debugState)
	}
}

// debugState replaces a match's state wholesale, so a test can start from the
// position it wants to exercise instead of playing there turn by turn.
//
// Its predecessor took hands, melds, a phase and a discard pile — twenty-odd
// rummy fields, and a second place that had to learn a new one whenever the
// engine grew a field. This takes the module's own state verbatim and writes
// the bytes. It works for every game because it understands none of them, and
// a module that adds a field needs no change here.
//
// Dev-only, and it bypasses every rule: whatever is written is what the game
// becomes. That is the point, and the reason it is behind a flag.
func (h *Handlers) debugState(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := req.Context()
	m, err := h.manager.Repo().Resolve(ctx, chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Only somebody at the table may seed it, even in development: a debug
	// hatch that any caller can reach into is an authorisation bug waiting to
	// be shipped by an accidental flag.
	if playerByID(m.Players, uc.UserID) == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body struct {
		State  json.RawMessage `json:"state"`
		Status string          `json:"status,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.State) == 0 {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}
	// Round-tripped through the module before it is stored, so a malformed
	// blob is a 400 here rather than a panic on the next broadcast.
	mod := h.manager.Registry().Get(m.ModuleID)
	if mod == nil {
		writeModuleError(w, module.Error{Code: "UNKNOWN_MODULE", Message: m.ModuleID})
		return
	}
	if _, err := mod.View(module.State(body.State), uc.UserID); err != nil {
		http.Error(w, "state is not valid for module "+m.ModuleID+": "+err.Error(), http.StatusBadRequest)
		return
	}

	expected := m.Version
	m.State = module.State(body.State)
	if body.Status != "" {
		m.Status = body.Status
	}
	if err := h.manager.Repo().UpdateWithVersion(ctx, m.ID, expected, m); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	m.Version = expected + 1

	h.manager.Broadcast(m)
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "status": m.Status})
}

func (h *Handlers) listModules(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"modules": h.manager.Registry().Descriptors(),
	})
}

// moduleRules writes out one module's rules, resolved against the variation
// and option overrides a lobby has actually chosen — so a "see the rules"
// screen can reflect the table a player is looking at, not just the module's
// defaults.
//
// Unauthenticated, like /modules: this is descriptive metadata, the same
// trust level as the descriptor it is resolved against.
func (h *Handlers) moduleRules(w http.ResponseWriter, req *http.Request) {
	id := chi.URLParam(req, "id")
	mod := h.manager.Registry().Get(id)
	if mod == nil {
		writeModuleError(w, module.Error{Code: "UNKNOWN_MODULE", Message: id})
		return
	}
	d := mod.Descriptor()

	variation := req.URL.Query().Get("variation")
	if variation != "" && d.Variation(variation) == nil {
		writeModuleError(w, module.Error{Code: "UNKNOWN_VARIATION", Message: variation})
		return
	}

	// Same discipline as Manager.Create: the descriptor is authoritative, so a
	// value it does not declare is refused here rather than silently ignored.
	opts := module.Options{}
	validated := map[string]*int{}
	for key, vals := range req.URL.Query() {
		name, ok := strings.CutPrefix(key, "opt.")
		if !ok || len(vals) == 0 {
			continue
		}
		v, err := strconv.Atoi(vals[0])
		if err != nil {
			writeModuleError(w, module.Error{Code: "BAD_OPTION", Message: name})
			return
		}
		opts[name] = v
		validated[name] = &v
	}
	if err := d.ValidateOptions(validated); err != nil {
		writeModuleError(w, err)
		return
	}

	rp, ok := mod.(module.RulesProvider)
	if !ok {
		writeModuleError(w, module.Error{Code: "NO_RULES", Message: id})
		return
	}
	sections, err := rp.Rules(module.MatchConfig{Variation: variation, Options: opts})
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"moduleId":  id,
		"variation": variation,
		"options":   opts,
		"sections":  sections,
	})
}

type createMatchReq struct {
	ModuleID  string         `json:"moduleId"`
	Variation string         `json:"variation,omitempty"`
	Options   map[string]int `json:"options,omitempty"`
}

func (h *Handlers) createMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// A new match is a promise of sockets, module state and bot drivers for
	// the next half hour — refused here, before anything is committed, while
	// "try again in a moment" is still cheap for everyone.
	if err := h.admission.AllowMatchStart(); err != nil {
		admission.WriteBusy(w, err)
		return
	}
	var body createMatchReq
	_ = json.NewDecoder(req.Body).Decode(&body)
	if body.ModuleID == "" {
		http.Error(w, "moduleId is required", http.StatusBadRequest)
		return
	}

	// PlayerUserID/PlayerGuestID set exactly one of the two, which is what
	// makes a guest's play attributable to their device and therefore
	// claimable onto an account they create later.
	host := models.Player{
		ID:      uc.UserID,
		Name:    uc.Username,
		UserID:  uc.PlayerUserID(),
		GuestID: uc.PlayerGuestID(),
	}
	m, err := h.manager.Create(req.Context(), body.ModuleID,
		module.MatchConfig{Variation: body.Variation, Options: body.Options}, host)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "joinCode": m.JoinCode})
}

func (h *Handlers) joinMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p := models.Player{
		ID:      uc.UserID,
		Name:    uc.Username,
		UserID:  uc.PlayerUserID(),
		GuestID: uc.PlayerGuestID(),
	}
	m, err := h.manager.Join(req.Context(), chi.URLParam(req, "id"), p)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex()})
}

// addBot seats a non-human player.
//
// It used to seat a body that never moved, because driving one was rummy-only
// work living in the rummy runtime. It now seats an opponent that plays: the
// runtime drives every bot from the module's own offer list (see bots.go), so
// this works for a game nobody has written yet.
func (h *Handlers) addBot(w http.ResponseWriter, req *http.Request) {
	if _, ok := auth.GetUserContext(req); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := req.Context()
	m, err := h.manager.Repo().Resolve(ctx, chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	bot := models.Player{
		ID:   "bot:" + randomJoinCode(8),
		Name: "Bot " + randomJoinCode(2),
		IsAI: true,
	}
	if _, err := h.manager.Join(ctx, m.ID.Hex(), bot); err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"playerId": bot.ID})
}

// invite seats a player the host picked out of the waiting room.
//
// The alternative to reading a join code out loud. Everything about who is
// still available is decided in the manager, at the moment of seating — this
// handler only carries the request.
func (h *Handlers) invite(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		PlayerID string `json:"playerId"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.PlayerID == "" {
		http.Error(w, "playerId required", http.StatusBadRequest)
		return
	}

	m, already, err := h.manager.Invite(req.Context(), chi.URLParam(req, "id"), uc.UserID, body.PlayerID)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	if already {
		writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "alreadyJoined": true})
		return
	}
	writeJSON(w, map[string]any{
		"matchId": m.ID.Hex(), "invited": true, "playerCount": len(m.Players),
	})
}

func (h *Handlers) startMatch(w http.ResponseWriter, req *http.Request) {
	if _, ok := auth.GetUserContext(req); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Start is where the module's state is actually allocated. A refusal here
	// is retryable — the table keeps its seats and the host tries again once
	// the pressure passes — where an OOM after an unguarded start is not.
	if err := h.admission.AllowMatchStart(); err != nil {
		admission.WriteBusy(w, err)
		return
	}
	m, err := h.manager.Start(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "status": m.Status})
}

// getMatch returns a viewer's state over plain HTTP.
//
// The socket is the live path, but a plain GET makes the runtime testable and
// debuggable without opening one — and, unauthenticated, it deliberately
// returns the *spectator* view, which is the same projection with nobody's
// hand in it.
func (h *Handlers) getMatch(w http.ResponseWriter, req *http.Request) {
	m, err := h.manager.Repo().Resolve(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	viewer := req.URL.Query().Get("as")
	writeJSON(w, h.manager.BuildStateMsg(m, viewer))
}

// handleWS carries actions in and per-viewer state out.
//
// Compare game.handleWS: no phases, no undo verbs, no rules error taxonomy —
// it decodes a module.Action and hands it over. Everything that made the rummy
// socket long was rummy.
func (h *Handlers) handleWS(w http.ResponseWriter, req *http.Request) {
	matchID := chi.URLParam(req, "id")
	playerID, err := auth.SubjectFromToken(req.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		http.Error(w, "invalid match id", http.StatusBadRequest)
		return
	}

	// Capacity is checked before the upgrade, so a refused player gets a plain
	// HTTP 503 their client can read, rather than a socket that opens and then
	// dies for no stated reason.
	//
	// A player already registered in this room is admitted without asking:
	// their new socket displaces their old one in Add below, and refusing a
	// reconnect would strand them mid-hand. It still takes a slot — the
	// displaced handler releases its own on the way out — so the ledger keeps
	// matching the sockets actually open.
	var slot *admission.Release
	if h.manager.Hub().Registry().Has(matchID, playerID) {
		slot = h.admission.AdmitReconnect()
	} else {
		var err error
		if slot, err = h.admission.Admit(admission.ClassGameplay); err != nil {
			admission.WriteBusy(w, err)
			return
		}
	}
	defer slot.Release()

	conn, err := h.upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	wsConn, prev := h.manager.Hub().Registry().Add(matchID, playerID, ws.PingableConn{Conn: conn})
	if prev != nil {
		_ = prev.Close()
	}
	ctx := context.Background()
	// Leaving pauses the table, but only if it was waiting on them — and only
	// if this socket was still the player's when it died.
	//
	// The two halves have to be one deferred block, because the second is
	// conditional on the first. `Add` above closes whatever connection this
	// one displaced, and that closure runs the displaced handler's defers: it
	// is a socket ending, but it is not a player leaving, because the player
	// is right here on the newer socket. RemoveIfCurrent already draws exactly
	// that distinction and says so in its doc comment; suspending regardless
	// of its answer is what turned a client that reconnects too eagerly into a
	// table that paused and un-paused every couple of seconds for a quarter of
	// an hour, with every action sent in the paused half refused as
	// MATCH_NOT_ACTIVE and the bot loop giving up entirely.
	defer func() {
		if h.manager.Hub().Registry().RemoveIfCurrent(matchID, playerID, wsConn) {
			h.manager.SuspendOnDisconnect(context.WithoutCancel(ctx), matchID, playerID, "socket closed")
		}
	}()

	// Arriving may be a *return*: a match this player's disconnection paused
	// resumes the moment they are back, before they are sent anything.
	h.manager.ResumeIfReturning(ctx, matchID, playerID)
	if m, err := h.manager.Repo().FindByID(ctx, oid); err == nil {
		h.manager.Hub().WriteDirect(matchID, playerID, h.manager.BuildStateMsg(m, playerID))
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var a module.Action
		if err := json.Unmarshal(data, &a); err != nil {
			_ = wsConn.WriteJSON(map[string]any{
				"type": "error", "code": "BAD_JSON", "message": err.Error(),
			})
			continue
		}
		if err := h.manager.HandleAction(ctx, matchID, playerID, a); err != nil {
			log.Printf("match=%s player=%s verb=%s refused: %v", matchID, playerID, a.Verb, err)
			frame := map[string]any{
				"type": "error", "code": module.CodeOf(err), "message": err.Error(),
			}
			// The rules behind the refusal, so a submission refused on arrival
			// explains itself the same way a greyed-out control does. An
			// offer's own whyNot carries these too, but a composed submission
			// — a meld a person put together — has no offer of its own to
			// have been greyed out in advance.
			if ids := h.manager.ExplainRefusal(ctx, matchID, module.CodeOf(err)); len(ids) > 0 {
				frame["ruleIds"] = ids
			}
			_ = wsConn.WriteJSON(frame)
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeModuleError maps a module refusal onto a status code.
//
// The code travels in the body as well: it is the same stable vocabulary the
// offers use, so a client renders it from the same locale bundle rather than
// parsing prose out of an HTTP error.
func writeModuleError(w http.ResponseWriter, err error) {
	code := module.CodeOf(err)
	status := http.StatusBadRequest
	switch code {
	case "UNKNOWN_MODULE", "UNKNOWN_VARIATION", "NO_RULES":
		status = http.StatusNotFound
	case "NOT_THE_HOST":
		status = http.StatusForbidden
	case "NO_LONGER_WAITING", "MATCH_FULL":
		// A conflict rather than a bad request: the caller did nothing wrong,
		// the world moved under them.
		status = http.StatusConflict
	case "WAITING_ROOM_UNAVAILABLE", "SERVER_BUSY":
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "message": err.Error()})
}
