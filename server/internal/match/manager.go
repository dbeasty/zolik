package match

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// Manager is the single write path for module-hosted matches.
//
// It mirrors game.Manager's shape — load, apply, persist with a version check,
// broadcast per viewer — with the game-specific middle replaced by a call into
// whichever module owns the match. That symmetry is deliberate: the properties
// worth keeping (optimistic concurrency, per-viewer projection, one writer per
// socket) are runtime properties, not rummy ones.
type Manager struct {
	repo     Repository
	registry *module.Registry
	hub      *ws.Hub

	// recorder is optional; nil simply means no statistics are kept, which is
	// what the tests and any statistics-free deployment run with.
	recorder Recorder

	// waiting is the pool a host may seat a player out of, and waitingRoom is
	// the socket room a pick-up notification goes to. Both optional: without
	// them, invites report themselves unavailable and nothing else changes.
	waiting     WaitingLookup
	waitingRoom string

	// botMu guards botRunning, which is one flag per match saying a bot loop
	// is already driving it. Without it, every human action would start a
	// second loop and the bots would race each other.
	botMu      sync.Mutex
	botRunning map[string]bool

	// botThinkMin and botThinkMax bound a bot's cosmetic pause. Zero means
	// the defaults in bots.go, so a Manager built without SetBotPace behaves
	// exactly as it did before the pace was configurable.
	botThinkMin time.Duration
	botThinkMax time.Duration
}

// Recorder is notified when a match finishes, so its result can be recorded
// and folded into lifetime statistics.
//
// An interface held here rather than a direct dependency on internal/stats,
// which keeps the arrow one-way and leaves the runtime testable without a
// database. It takes the module's own outcome — its standings and its round
// history — because only the module knows how its game is scored, which is what
// makes statistics work for every game rather than only for the one whose
// arithmetic the stats package used to hard-code.
type Recorder interface {
	// RecordMatchAsync must not block the action that completed the match,
	// and must not fail it: a bookkeeping problem is never a reason to reject
	// a legal move.
	RecordMatchAsync(m models.Match, out module.Outcome)
}

// SetBotPace bounds how long a bot pauses before answering. Optional; zero or
// an inverted range falls back to the defaults in bots.go.
//
// It exists because the pause is not really about the bot: it is about giving
// the client time to finish showing the *previous* move before the next state
// arrives. That is a property of how fast the board animates, which is a
// client decision, so the server has to be able to be told rather than
// guessing once at compile time.
func (m *Manager) SetBotPace(min, max time.Duration) { m.botThinkMin, m.botThinkMax = min, max }

// SetRecorder attaches statistics recording. Optional.
func (m *Manager) SetRecorder(r Recorder) { m.recorder = r }

// WaitingLookup answers whether a player is currently in the waiting-room
// pool, and lets them be picked up out of it. Satisfied by *lobby.Store.
//
// Named here as a narrow, primitive-typed interface rather than importing
// internal/lobby, so the runtime never learns what a waiting room is — the
// same reason the statistics recorder is an interface. (It was originally
// written this way to avoid an import cycle, which no longer exists now the
// socket hub lives in internal/ws; the narrowness is worth keeping on its own
// merits.)
type WaitingLookup interface {
	// IsWaiting reports the display details of a waiting player, so an invite
	// can build their seat without a second round trip.
	IsWaiting(ctx context.Context, playerID string) (name string, isGuest bool, ok bool)
	// Pickup removes a player from the pool, reporting whether they were
	// actually present. Called only after they have been seated — a failed
	// seat attempt must leave them waiting, not silently drop them.
	Pickup(ctx context.Context, playerID string) bool
}

// SetWaitingRoom attaches the pool a host may invite from, and the socket room
// a pick-up notification is delivered on. Optional: without it, invites are
// unavailable and every other path is unaffected.
func (m *Manager) SetWaitingRoom(w WaitingLookup, roomID string) {
	m.waiting, m.waitingRoom = w, roomID
}

// Invite seats a specific waiting player, without a join code.
//
// The order matters and is the same as the version this replaces: the seat is
// committed first, and only then does the player leave the pool. Picking up
// before the seat was confirmed would risk losing them from the waiting room
// over a write that then failed.
func (m *Manager) Invite(ctx context.Context, idOrCode, hostID, playerID string) (models.Match, bool, error) {
	if m.waiting == nil {
		return models.Match{}, false, module.Error{Code: "WAITING_ROOM_UNAVAILABLE"}
	}
	match, err := m.repo.Resolve(ctx, idOrCode)
	if err != nil {
		return models.Match{}, false, err
	}
	if match.HostID != hostID {
		return models.Match{}, false, module.Error{Code: "NOT_THE_HOST"}
	}
	if match.Status != "lobby" {
		return models.Match{}, false, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	for _, p := range match.Players {
		if p.ID == playerID {
			// Idempotent: inviting somebody already at the table is a no-op,
			// not an error. A host double-tapping is not a mistake worth a 400.
			return match, true, nil
		}
	}

	// Re-checked here rather than trusted from whatever snapshot the host's
	// client last polled: the target may have left, been picked up elsewhere,
	// or disconnected in the meantime, and this is the only point that gets to
	// decide whether they are still actually available.
	name, isGuest, stillWaiting := m.waiting.IsWaiting(ctx, playerID)
	if !stillWaiting {
		return models.Match{}, false, module.Error{Code: "NO_LONGER_WAITING"}
	}

	seat := models.Player{ID: playerID, Name: name}
	if isGuest {
		seat.GuestID = playerID
	} else {
		seat.UserID = playerID
	}
	next, err := m.Join(ctx, match.ID.Hex(), seat)
	if err != nil {
		return models.Match{}, false, err
	}

	m.waiting.Pickup(ctx, playerID)
	if m.hub != nil && m.waitingRoom != "" {
		m.hub.WriteDirect(m.waitingRoom, playerID, map[string]any{
			"type":     "lobby_invited",
			"matchId":  next.ID.Hex(),
			"joinCode": next.JoinCode,
		})
	}
	return next, false, nil
}

func NewManager(repo Repository, registry *module.Registry, hub *ws.Hub) *Manager {
	return &Manager{repo: repo, registry: registry, hub: hub}
}

func (m *Manager) Registry() *module.Registry { return m.registry }
func (m *Manager) Repo() Repository           { return m.repo }
func (m *Manager) Hub() *ws.Hub               { return m.hub }

// Create opens a lobby for a module.
func (m *Manager) Create(ctx context.Context, moduleID string, cfg module.MatchConfig, host models.Player) (models.Match, error) {
	mod := m.registry.Get(moduleID)
	if mod == nil {
		return models.Match{}, module.Error{Code: "UNKNOWN_MODULE", Message: moduleID}
	}
	d := mod.Descriptor()

	// The descriptor is authoritative here too: a value it does not declare is
	// refused at the door rather than by whichever client happens to behave.
	opts := map[string]*int{}
	for name, v := range cfg.Options {
		v := v
		opts[name] = &v
	}
	if err := d.ValidateOptions(opts); err != nil {
		return models.Match{}, err
	}
	if cfg.Variation != "" && d.Variation(cfg.Variation) == nil {
		return models.Match{}, module.Error{Code: "UNKNOWN_VARIATION", Message: cfg.Variation}
	}

	match := models.Match{
		ModuleID:  moduleID,
		Variation: cfg.Variation,
		Options:   cfg.Options,
		Status:    "lobby",
		Players:   []models.Player{host},
		TurnOrder: []string{host.ID},
		HostID:    host.ID,
		JoinCode:  randomJoinCode(6),
		Seed:      time.Now().UnixNano(),
		CreatedAt: time.Now().UTC(),
	}
	return m.repo.Insert(ctx, match)
}

// Join adds a player to a lobby.
func (m *Manager) Join(ctx context.Context, idOrCode string, p models.Player) (models.Match, error) {
	match, err := m.repo.Resolve(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	if match.Status != "lobby" {
		return models.Match{}, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	for _, existing := range match.Players {
		if existing.ID == p.ID {
			return match, nil // idempotent: re-joining is not an error
		}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return models.Match{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	if len(match.Players) >= mod.Descriptor().MaxPlayers {
		return models.Match{}, module.Error{Code: "MATCH_FULL"}
	}

	match.Players = append(match.Players, p)
	match.TurnOrder = append(match.TurnOrder, p.ID)
	if err := m.repo.UpdateWithVersion(ctx, match.ID, match.Version, match); err != nil {
		return models.Match{}, err
	}
	match.Version++
	return match, nil
}

// Start deals the match through its module.
func (m *Manager) Start(ctx context.Context, idOrCode string) (models.Match, error) {
	match, err := m.repo.Resolve(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	if match.Status != "lobby" {
		return models.Match{}, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return models.Match{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	d := mod.Descriptor()
	if len(match.Players) < d.MinPlayers {
		return models.Match{}, module.Error{
			Code:    "TOO_FEW_PLAYERS",
			Message: fmt.Sprintf("%s needs at least %d players", d.Label, d.MinPlayers),
		}
	}

	state, err := mod.NewMatch(
		module.MatchConfig{Variation: match.Variation, Options: match.Options},
		playerRefs(match.Players), match.Seed)
	if err != nil {
		return models.Match{}, err
	}

	now := time.Now().UTC()
	match.State = state
	match.Status = "active"
	match.StartedAt = &now
	if err := m.repo.UpdateWithVersion(ctx, match.ID, match.Version, match); err != nil {
		return models.Match{}, err
	}
	match.Version++
	m.Broadcast(match)
	// A bot may be first to act — in Hold'em it usually is, since the blinds
	// decide the order rather than who created the lobby.
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), match.ID.Hex())
	return match, nil
}

// HandleAction is the single write path: load, apply through the module,
// persist with a version check, broadcast.
//
// The module decides everything about legality; this function decides nothing
// except whether the write raced.
// ExplainRefusal names the written rules behind a refusal at this table, or
// nil when the module has no rule index or nothing explains the code.
//
// Best-effort by design: a lookup failure here must never turn a refusal into
// a different error, because the refusal is the thing the player is waiting
// to hear about.
func (m *Manager) ExplainRefusal(ctx context.Context, idOrCode, code string) []string {
	match, err := m.repo.Resolve(ctx, idOrCode)
	if err != nil {
		return nil
	}
	return module.ExplainRefusalFor(
		m.registry.Get(match.ModuleID),
		module.MatchConfig{Variation: match.Variation, Options: match.Options},
		code,
	)
}

func (m *Manager) HandleAction(ctx context.Context, idOrCode, playerID string, a module.Action) error {
	match, err := m.repo.Resolve(ctx, idOrCode)
	if err != nil {
		return err
	}
	if match.Status != "active" {
		return module.Error{Code: "MATCH_NOT_ACTIVE"}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}

	next, events, err := mod.Apply(match.State, playerID, a)
	if err != nil {
		return err // a refusal: nothing is persisted, nothing is broadcast
	}

	expected := match.Version
	match.State = next
	match.ActionLog = append(match.ActionLog, logEntry(len(match.ActionLog)+1, playerID, a))

	if done, winners, err := mod.Finished(next); err == nil && done {
		now := time.Now().UTC()
		match.Status = "completed"
		match.Winners = winners
		// WinnerID stays on the document and the wire as the first winner, so
		// every client written against a single-winner match keeps working. It
		// is derived from Winners rather than computed separately: one
		// implementation, two spellings.
		match.WinnerID = ""
		if len(winners) > 0 {
			match.WinnerID = winners[0]
		}
		match.EndedAt = &now
	}

	if err := m.repo.UpdateWithVersion(ctx, match.ID, expected, match); err != nil {
		log.Printf("match=%s player=%s action=%s persist failed: %v", match.ID.Hex(), playerID, a.Verb, err)
		return err
	}
	match.Version = expected + 1

	m.Broadcast(match)
	m.publishEvents(match, events)

	// A finished match becomes a permanent record. Asynchronous and after the
	// broadcast on purpose: the player who just won should see the final board
	// without waiting on bookkeeping, and a bookkeeping failure must never
	// fail the move that won.
	if match.Status == "completed" && m.recorder != nil {
		m.recorder.RecordMatchAsync(match, module.OutcomeOf(mod, match.State))
	}

	// Whoever is on turn now might be a bot. The loop is a no-op when it is
	// not, and refuses to start twice for the same match, so calling it after
	// every action is both correct and cheap.
	//
	// WithoutCancel because the loop outlives the request that triggered it:
	// the human's HTTP call or socket frame is finished, and the bots' turns
	// are not.
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), match.ID.Hex())
	return nil
}

// Broadcast sends every connected player their own view of the match.
//
// Per-viewer projection is unchanged from the rummy runtime, but the filtering
// itself has moved into the module: this loop asks each module what a given
// viewer may see and ships the answer, without knowing what was hidden.
func (m *Manager) Broadcast(match models.Match) {
	recipients := make([]string, 0, len(match.Players))
	for _, p := range match.Players {
		recipients = append(recipients, p.ID)
	}
	id := match.ID.Hex()
	// Computed once for the whole broadcast rather than once per recipient: a
	// round log takes no viewer, so every seat would otherwise pay to decode
	// the same bytes into the same answer.
	rounds := module.RoundsFor(m.registry.Get(match.ModuleID), match.State)
	m.hub.BroadcastGameState(id, recipients, func(playerID string) interface{} {
		return m.buildStateMsg(match, playerID, rounds)
	})
}

func (m *Manager) publishEvents(match models.Match, events []module.Event) {
	if len(events) == 0 {
		return
	}
	id := match.ID.Hex()
	for _, ev := range events {
		payload := map[string]any{"type": ev.Type}
		for k, v := range ev.Data {
			payload[k] = v
		}
		for _, p := range match.Players {
			m.hub.WriteDirect(id, p.ID, payload)
		}
	}
}

func logEntry(seq int, playerID string, a module.Action) models.MatchAction {
	raw, err := json.Marshal(a)
	if err != nil {
		raw = []byte("{}")
	}
	return models.MatchAction{Seq: seq, PlayerID: playerID, Action: raw, At: time.Now().UTC()}
}

func playerRefs(players []models.Player) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(players))
	for _, p := range players {
		out = append(out, module.PlayerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}
	return out
}

var joinCodeAlphabet = []rune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

func randomJoinCode(n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = joinCodeAlphabet[randInt(len(joinCodeAlphabet))]
	}
	return string(out)
}
