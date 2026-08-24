package match

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"zolik/server/internal/game"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Manager is the single write path for module-hosted matches.
//
// It mirrors game.Manager's shape — load, apply, persist with a version check,
// broadcast per viewer — with the game-specific middle replaced by a call into
// whichever module owns the match. That symmetry is deliberate: the properties
// worth keeping (optimistic concurrency, per-viewer projection, one writer per
// socket) are runtime properties, not rummy ones.
type Manager struct {
	repo     *Repository
	registry *module.Registry
	hub      *game.Hub
}

func NewManager(repo *Repository, registry *module.Registry, hub *game.Hub) *Manager {
	return &Manager{repo: repo, registry: registry, hub: hub}
}

func (m *Manager) Registry() *module.Registry { return m.registry }
func (m *Manager) Repo() *Repository          { return m.repo }
func (m *Manager) Hub() *game.Hub             { return m.hub }

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
	return match, nil
}

// HandleAction is the single write path: load, apply through the module,
// persist with a version check, broadcast.
//
// The module decides everything about legality; this function decides nothing
// except whether the write raced.
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
	m.hub.BroadcastGameState(id, recipients, func(playerID string) interface{} {
		return m.BuildStateMsg(match, playerID)
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
