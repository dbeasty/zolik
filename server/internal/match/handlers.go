package match

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/admission"
	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
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
	// Bringing a swept-up table back. Separate from start, which allocates a
	// module's state: this one only undoes an envelope.
	r.With(auth.AuthMiddleware).Post("/matches/{id}/resume", h.resumeMatch)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/add-bot", h.addBot)
	// Seat a specific player out of the waiting room, instead of reading a
	// join code out to them.
	r.With(auth.AuthMiddleware).Post("/matches/{id}/invite", h.invite)
	r.With(auth.AuthMiddleware).Post("/matches/{id}/seats", h.seatTable)
	r.Get("/ws/matches/{id}", h.handleWS)
	// A stored-games list: every table this caller is seated at. Registered
	// as /users/me/tables rather than /matches/mine — "table" is this
	// package's own word for a live envelope (see NOT_AT_THIS_TABLE,
	// TABLE_HAS_PLAYERS_AWAY), and /matches/{id} above is deliberately
	// unauthenticated and answers a spectator view; a caller-scoped list
	// belongs with the rest of "about me", not on that same segment.
	r.With(auth.AuthMiddleware).Get("/users/me/tables", h.myTables)
	// Ending a table outright, at its host's request. Same placeholder name
	// as the GET above so there is only one idea of what {id} means on this
	// path; chi keys the two by method, not by name, so they cannot collide.
	r.With(auth.AuthMiddleware).Delete("/matches/{id}", h.deleteMatch)
	// Stepping back through a game that has stopped. Authenticated and
	// seated-only, unlike the spectator GET above: this answers with every
	// board the match passed through, which is a great deal more than the one
	// it is sitting on.
	r.With(auth.AuthMiddleware).Get("/matches/{id}/replay", h.replayMatch)

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
	// Avatar is the face the host wants at the table. Sent per seating rather
	// than read from their account, because a guest has no account to read
	// and the client always knows its own current choice — including the one
	// made a second ago, which a token's claims would not yet carry.
	Avatar string `json:"avatar,omitempty"`
}

// joinMatchReq is everything a player brings to a seat they are taking. Only
// decoration so far, which is why an absent body is not an error.
type joinMatchReq struct {
	Avatar string `json:"avatar,omitempty"`
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
		Avatar:  models.SanitizeAvatar(body.Avatar),
		UserID:  uc.PlayerUserID(),
		GuestID: uc.PlayerGuestID(),
	}
	m, err := h.manager.Create(req.Context(), body.ModuleID,
		module.MatchConfig{Variation: body.Variation, Options: body.Options}, host)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	// The link goes back with the code, not instead of it: a host reads one
	// out over the phone and pastes the other into a chat, and which of the
	// two is the convenient one is theirs to decide.
	resp := map[string]any{"matchId": m.ID.Hex(), "joinCode": m.JoinCode}
	if link := h.manager.InviteURL(m.JoinCode); link != "" {
		resp["inviteUrl"] = link
	}
	writeJSON(w, resp)
}

func (h *Handlers) joinMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// A body is optional here — it carries only the face this player wants —
	// so a client that sends none joins exactly as it always did.
	var body joinMatchReq
	_ = json.NewDecoder(req.Body).Decode(&body)
	p := models.Player{
		ID:      uc.UserID,
		Name:    uc.Username,
		Avatar:  models.SanitizeAvatar(body.Avatar),
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

// addBotReq is what the caller may say about the opponent it wants. Everything
// in it is optional: a client that sends no body at all gets a bot at the
// table's own setting, which is what every existing client does.
type addBotReq struct {
	// Skill overrides the table's setting for this one seat, which is how a
	// deliberately mixed table is built one bot at a time.
	Skill string `json:"skill,omitempty"`
}

// addBot seats a non-human player.
//
// It used to seat a body that never moved, because driving one was rummy-only
// work living in the rummy runtime. It now seats an opponent that plays: the
// runtime drives every bot from the module's own offer list (see bots.go), so
// this works for a game nobody has written yet.
//
// Two things are decided here rather than later, and both are decided *once*,
// at seating, so that they are the same for the whole match:
//
//	how well it plays  the request's skill, or the table's botSkill option,
//	                   or — under Mixed — a strength drawn for this seat
//	                   alone, which is the only way two bots at one table
//	                   differ.
//	who it is          a persona off the roster for that strength, avoiding
//	                   the ones already sitting down. This is what gives the
//	                   bot a name a player recognises and a lifetime record
//	                   that survives the lobby it was created in.
func (h *Handlers) addBot(w http.ResponseWriter, req *http.Request) {
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
	// The host's table, and only the host's to fill. This used to check that
	// the caller was signed in and then discard who they were, which made the
	// six characters a host pastes into a chat enough for any passer-by to
	// seat a bot at their table and deal it — the invited friend then arriving
	// to MATCH_ALREADY_STARTED. Seat, Invite and DeleteAsHost have all always
	// asked; these two were the pair that did not.
	if err := requireHost(m, uc.UserID); err != nil {
		writeModuleError(w, err)
		return
	}
	var body addBotReq
	_ = json.NewDecoder(req.Body).Decode(&body)

	bot := models.Player{
		ID:   "bot:" + randomJoinCode(8),
		IsAI: true,
	}
	persona := h.personaFor(m, body.Skill)
	bot.Name = persona.Name
	bot.AIDifficulty = string(persona.Skill)
	bot.AIPersona = persona.Key()

	if _, err := h.manager.Join(ctx, m.ID.Hex(), bot); err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"playerId":  bot.ID,
		"name":      bot.Name,
		"skill":     bot.AIDifficulty,
		"aiPersona": bot.AIPersona,
	})
}

// personaFor decides which opponent sits down at this table.
//
// The seed is the match's own, salted with how many seats are already taken —
// so seating three bots at one table draws three different answers, and
// re-creating the same match with the same seed draws the same three. Nothing
// about that is stored: it is recomputed identically or not at all.
func (h *Handlers) personaFor(m models.Match, want string) module.Persona {
	skill, auto := module.ParseSkill(want)
	if auto {
		// Nothing asked for by name: fall back to the table's own setting,
		// which is itself allowed to be Mixed.
		skill, auto = module.MatchConfig{Options: m.Options}.BotSkill(h.defaultSkill(m.ModuleID))
	}
	seed := module.SeatSeed(m.Seed, strconv.Itoa(len(m.Players)), "seat")
	skill = module.ResolveSkill(skill, auto, seed)

	taken := make([]string, 0, len(m.Players))
	for _, p := range m.Players {
		if p.IsAI {
			taken = append(taken, p.AIPersona)
		}
	}
	return module.PickPersona(skill, module.TakenPersonas(taken), seed)
}

// defaultSkill is the strength a module wants when the lobby said nothing.
//
// Medium, universally, and deliberately not read from the descriptor: the
// descriptor declares what a lobby *may* choose, and every module's option
// carries its own default already. This is only the floor under a match
// created before the option existed.
func (h *Handlers) defaultSkill(string) module.Skill { return module.SkillMedium }

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

// seatTable puts the table in the order the host wants, before it is dealt.
//
// The whole of "form teams": in a game with sides the turn alternates between
// them, so a side is a position in the seating and reordering the seats is how
// partners are chosen. See Manager.Seat.
func (h *Handlers) seatTable(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Order []string `json:"order"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.Order) == 0 {
		http.Error(w, "order required", http.StatusBadRequest)
		return
	}

	m, err := h.manager.Seat(req.Context(), chi.URLParam(req, "id"), uc.UserID, body.Order)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"matchId": m.ID.Hex(),
		"order":   m.TurnOrder,
		"sides":   h.sidesFor(m),
	})
}

// sidesFor is who would be playing with whom if this table were dealt now, or
// nil for a game where everybody plays for themselves.
func (h *Handlers) sidesFor(m models.Match) [][]string {
	mod := h.manager.Registry().Get(m.ModuleID)
	if mod == nil {
		return nil
	}
	return module.SidesOf(mod, module.MatchConfig{Variation: m.Variation, Options: m.Options}, playerRefs(m.Players))
}

func (h *Handlers) startMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := req.Context()
	// Resolved before anything else so there is a host to compare against —
	// see the note on addBot for what an unguarded deal let a stranger do.
	// The check lives here rather than inside Manager.Start for the same
	// reason debugState's does: Start is a runtime operation with no caller,
	// driven by tests and by the bot loop as well as by a person.
	m, err := h.manager.Repo().Resolve(ctx, chi.URLParam(req, "id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := requireHost(m, uc.UserID); err != nil {
		writeModuleError(w, err)
		return
	}
	// Start is where the module's state is actually allocated. A refusal here
	// is retryable — the table keeps its seats and the host tries again once
	// the pressure passes — where an OOM after an unguarded start is not.
	if err := h.admission.AllowMatchStart(); err != nil {
		admission.WriteBusy(w, err)
		return
	}
	m, err = h.manager.Start(ctx, chi.URLParam(req, "id"))
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "status": m.Status})
}

// resumeMatch brings an abandoned bots-only table back, for the player who
// was at it.
//
// Every rule about who may do this lives in the manager rather than here, so
// the socket path and any future caller get the same answers; this end is only
// the door. The resolved match is returned so the client can route on the
// status it actually got rather than assuming the one it asked for.
func (h *Handlers) resumeMatch(w http.ResponseWriter, req *http.Request) {
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
	// A resumed table is a live one, and a live one costs a slot the same way
	// a freshly started match does. Checked before the write rather than
	// after, so a server under pressure refuses the table instead of reviving
	// one it cannot then run bots for.
	if err := h.admission.AllowMatchStart(); err != nil {
		admission.WriteBusy(w, err)
		return
	}
	if err := h.manager.ResumeAbandoned(ctx, m.ID.Hex(), uc.UserID); err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": m.ID.Hex(), "status": "active"})
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

// storedTable is one row of a "my games" list: the envelope, worded for a
// player rather than for a socket, and nothing the module owns.
type storedTable struct {
	MatchID     string      `json:"matchId"`
	ModuleID    string      `json:"moduleId"`
	Variation   string      `json:"variation,omitempty"`
	Status      string      `json:"status"`
	JoinCode    string      `json:"joinCode,omitempty"`
	IsHost      bool        `json:"isHost"`
	Players     []PlayerMsg `json:"players"`
	HumanCount  int         `json:"humanCount"`
	BotCount    int         `json:"botCount"`
	CreatedAt   time.Time   `json:"createdAt"`
	StartedAt   *time.Time  `json:"startedAt,omitempty"`
	EndedAt     *time.Time  `json:"endedAt,omitempty"`
	SuspendedAt *time.Time  `json:"suspendedAt,omitempty"`
	UpdatedAt   time.Time   `json:"updatedAt,omitempty"`
	// CanResume mirrors resumableBy exactly, so the button this list offers
	// is never one ResumeAbandoned would then refuse.
	CanResume bool `json:"canResume"`
	// CanDelete is just IsHost today. Named as its own capability rather than
	// left for the client to derive from IsHost, so the rule can change here
	// without a client release.
	CanDelete bool `json:"canDelete"`
	// CanReplay is whether this table has a game in it to step through.
	//
	// Keyed off StartedAt because the list projection strips the action log,
	// so nothing here could count moves even if it wanted to — and because a
	// dealt match always has at least the deal to show, which is exactly where
	// BuildReplay draws the same line.
	CanReplay bool `json:"canReplay"`
}

func (h *Handlers) storedTableOf(m models.Match, viewerID string) storedTable {
	// UpdatedAt is unset on a match nothing has yet written back through
	// UpdateWithVersion — a lobby nobody has touched since it was created is
	// the ordinary case. CreatedAt is the honest answer for "last activity"
	// there; the alternative is a zero-value 0001-01-01 reaching the client,
	// which is no player's idea of when anything happened.
	updatedAt := m.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = m.CreatedAt
	}
	out := storedTable{
		MatchID:     m.ID.Hex(),
		ModuleID:    m.ModuleID,
		Variation:   m.Variation,
		Status:      m.Status,
		JoinCode:    m.JoinCode,
		IsHost:      m.HostID == viewerID,
		CreatedAt:   m.CreatedAt,
		StartedAt:   m.StartedAt,
		EndedAt:     m.EndedAt,
		SuspendedAt: m.SuspendedAt,
		UpdatedAt:   updatedAt,
	}
	out.CanResume = m.Status == "abandoned" && h.manager.resumableBy(m, viewerID)
	out.CanDelete = out.IsHost
	out.CanReplay = m.StartedAt != nil
	for _, p := range m.Players {
		out.Players = append(out.Players, PlayerMsg{ID: p.ID, Name: p.Name, IsAI: p.IsAI, Avatar: p.Avatar})
		if p.IsAI {
			out.BotCount++
		} else {
			out.HumanCount++
		}
	}
	return out
}

// myTables lists the stored games the caller is seated at.
//
// Guests are served exactly like accounts: players.id is the JWT subject
// either way (an account's object id hex, or the device's guest id), so one
// query answers both and a guest — the common case in this app — is not
// turned away the way the separate lifetime-statistics endpoints turn them
// away. Neither State nor ActionLog reaches the response; storedTable has no
// field to carry them in.
func (h *Handlers) myTables(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	statuses := UnfinishedStatuses
	if req.URL.Query().Get("status") == "finished" {
		statuses = FinishedStatuses
	}
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))

	rows, err := h.manager.Repo().FindForPlayer(req.Context(), uc.UserID,
		PlayerMatchFilter{Statuses: statuses, Limit: limit})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]storedTable, 0, len(rows))
	for _, m := range rows {
		out = append(out, h.storedTableOf(m, uc.UserID))
	}
	writeJSON(w, map[string]any{"tables": out})
}

// replayMatch plays a stopped game back, frame by frame.
//
// Paged rather than whole: a long canasta match is a megabyte and a half of
// boards, and nothing on this server compresses a response. Paging costs one
// extra fold per page and buys a first frame that arrives immediately.
//
// The viewer is always the caller's own seat. There is deliberately no
// ?as=<somebody else> here, unlike getMatch: that route is unauthenticated and
// spectator-ish by intent, whereas this one would hand a seated player every
// board their opponent ever held.
func (h *Handlers) replayMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(req, "id")
	m, err := h.manager.Repo().Resolve(req.Context(), id)
	if err != nil {
		if db.IsNotFound(err) {
			// Also how a match that outlived its retention window answers,
			// which is the same answer every other route on a swept match
			// already gives.
			writeModuleError(w, module.Error{Code: "MATCH_NOT_FOUND", Message: id})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if playerByID(m.Players, uc.UserID) == nil {
		writeModuleError(w, module.Error{Code: "NOT_AT_THIS_TABLE"})
		return
	}

	from, _ := strconv.Atoi(req.URL.Query().Get("from"))
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	rep, err := h.manager.BuildReplay(m, uc.UserID, ReplayOptions{
		From: from, Limit: limit,
		// Asking is not getting: BuildReplay grants this only to a finished
		// match, and every other status is projected per viewer as usual.
		Open: true,
	})
	if err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, rep)
}

// deleteMatch ends a table at its host's request. Every rule lives in
// Manager.DeleteAsHost; this door does no policy of its own.
func (h *Handlers) deleteMatch(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(req, "id")
	if err := h.manager.DeleteAsHost(req.Context(), id, uc.UserID); err != nil {
		writeModuleError(w, err)
		return
	}
	writeJSON(w, map[string]any{"matchId": id, "status": "deleted"})
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
			// A seat leaving is also what takes a resume offer away from the
			// people still looking at a swept-up table. Same call as the
			// arrival below, and a no-op for every other status.
			h.manager.AnnouncePresence(context.WithoutCancel(ctx), matchID)
		}
	}()

	// Arriving may be a *return*: a match this player's disconnection paused
	// resumes the moment they are back, before they are sent anything.
	h.manager.ResumeIfReturning(ctx, matchID, playerID)
	m, err := h.manager.Repo().FindByID(ctx, oid)
	if err != nil {
		// Said out loud rather than left as a silence.
		//
		// This used to be an `if err == nil` with no else: a link to a table
		// that no longer exists opened a socket, was sent nothing at all, and
		// left the screen on "Waiting for the table…" for ever — connected,
		// so not even a reconnect spinner, just a sentence that would never
		// stop being true. Indistinguishable from a server that had gone
		// quiet, for the one case where the answer is short and certain.
		//
		// It matters more now than it did: retention deletes resolved matches
		// on a schedule (see retention.go), so "this table is gone" stops
		// being a rarity and becomes the expected end state of every old link
		// somebody saved.
		_ = wsConn.WriteJSON(map[string]any{
			"type": "error", "code": "MATCH_NOT_FOUND", "message": matchID,
		})
		// Closed explicitly. Returning from the handler does not do it — this
		// connection was hijacked out of net/http at the upgrade, so nothing
		// upstream owns it any more — and a socket left open after a final
		// refusal is the same hang in a new place: the client has stopped
		// reconnecting on this code, so it would sit on a connection that is
		// never going to say anything else.
		_ = conn.Close()
		return
	}
	h.manager.Hub().WriteDirect(matchID, playerID, h.manager.BuildStateMsg(m, playerID))
	// And, on a swept-up table, tell everybody else that somebody just sat
	// down: their own resume offer may have become available because of it.
	// After the direct write above, so the arriving player's first message is
	// still their own state.
	//
	// Gated on the status already in hand rather than left to AnnouncePresence
	// to discover, so that opening a socket onto an ordinary match — which is
	// every socket, nearly all the time — costs no extra read.
	if m.Status == string(rules.StatusAbandoned) {
		h.manager.AnnouncePresence(ctx, matchID)
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

// requireHost is the rule that shaping a table belongs to whoever opened it.
//
// The same rule Manager.Seat and Manager.Invite have always carried, written
// once here for the two handlers whose work is done before the manager is
// reached — adding a seat, and dealing. Host rather than merely seated,
// because that is what both clients already offer: the table screen shows
// "Add a bot" and "Start" to the host and "waiting for the host" to everybody
// else, so a non-host reaching either of these is not a player using the app.
func requireHost(m models.Match, playerID string) error {
	if m.HostID != playerID {
		return module.Error{Code: "NOT_THE_HOST", Message: playerID}
	}
	return nil
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
	case "UNKNOWN_MODULE", "UNKNOWN_VARIATION", "NO_RULES", "MATCH_NOT_FOUND":
		status = http.StatusNotFound
	case "NOT_AT_THIS_TABLE", "TABLE_HAS_PLAYERS_AWAY":
		status = http.StatusForbidden
	case "NOT_THE_HOST":
		status = http.StatusForbidden
	case "NO_LONGER_WAITING", "MATCH_FULL", "MATCH_NOT_ABANDONED", "MATCH_MOVED_ON", "NOTHING_TO_REPLAY":
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
