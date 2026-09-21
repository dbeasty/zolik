package match

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/metrics"
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

	// metrics counts what the runtime did, for the operator's console. Never
	// nil after New — it defaults to a sink that discards — so nothing on the
	// game path has to guard a call.
	metrics metrics.Sink

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

	// inviteBaseURL is how the outside world reaches this deployment, and the
	// only thing standing between a join code and a link somebody can click.
	// Empty is the ordinary state under test, and means no link is offered —
	// see SetInviteBaseURL in invitelink.go.
	inviteBaseURL string

	// botThinkMin and botThinkMax bound a bot's cosmetic pause. Zero means
	// the defaults in bots.go, so a Manager built without SetBotPace behaves
	// exactly as it did before the pace was configurable.
	botThinkMin time.Duration
	botThinkMax time.Duration

	// live holds the state of every match in play; see live.go.
	live liveMatches

	// replayEnabled is the operator's half of whether a stopped game can be
	// stepped through; the store's half is asked of the store. Off by default,
	// which is what every test and every Mongo deployment runs with — see
	// ReplayAvailable, and SetReplayEnabled below.
	replayEnabled bool

	// lobbyObserver hears when a table stops taking players, and
	// personalRoom is the socket room each person's own connection is held
	// in. Both optional — see SetLobbyObserver and SetPersonalRoom.
	lobbyObserver LobbyObserver
	personalRoom  string
}

// LobbyObserver is told when a lobby stops being one somebody could join: it
// started, filled its last seat, or was deleted. It is how an invite already
// sent about the table is withdrawn. Must not block.
type LobbyObserver interface {
	LobbyClosed(matchID string)
}

// SetLobbyObserver attaches the observer. Optional.
func (m *Manager) SetLobbyObserver(o LobbyObserver) { m.lobbyObserver = o }

// SetPersonalRoom names the room every client's own, always-open connection
// is held in, keyed by subject key. With it, being picked up out of the
// waiting room reaches the player on whichever screen — and instance — they
// are on, not only on the waiting room's own socket. Optional.
func (m *Manager) SetPersonalRoom(roomID string) { m.personalRoom = roomID }

func (m *Manager) lobbyClosed(matchID string) {
	if m.lobbyObserver != nil {
		m.lobbyObserver.LobbyClosed(matchID)
	}
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

// SetReplayEnabled turns stepping back through stopped games on.
//
// Optional and off by default, like every other setter here. It costs the
// write path nothing either way: the moves and snapshots a replay folds are
// what every match is stored as.
//
// Only half the answer: the store still has to be one that keeps history, and
// still be keeping it. See ReplayAvailable.
func (m *Manager) SetReplayEnabled(on bool) { m.replayEnabled = on }

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
	//
	// A widening tuple rather than a struct, and deliberately so: the whole
	// point of this interface is that the runtime never learns what a waiting
	// room is, and returning the pool's own record would mean importing it.
	// Four values is uglier than a type and is the honest cost of that.
	IsWaiting(ctx context.Context, playerID string) (name string, isGuest bool, avatar string, ok bool)
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
	name, isGuest, avatar, stillWaiting := m.waiting.IsWaiting(ctx, playerID)
	if !stillWaiting {
		return models.Match{}, false, module.Error{Code: "NO_LONGER_WAITING"}
	}

	// The face they were waiting under, so being picked up out of the pool
	// seats the person the host was looking at. Sanitised again rather than
	// trusted: it entered the pool from a client, and this is a different
	// door into the same seat.
	seat := models.Player{ID: playerID, Name: name, Avatar: models.SanitizeAvatar(avatar)}
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
	invited := map[string]any{
		"type":     "lobby_invited",
		"matchId":  next.ID.Hex(),
		"joinCode": next.JoinCode,
	}
	if m.hub != nil && m.waitingRoom != "" {
		m.hub.WriteDirect(m.waitingRoom, playerID, invited)
	}
	// And on the personal socket, through Publish rather than WriteDirect:
	// that connection may be held by another instance, which WriteDirect
	// could never reach.
	if m.hub != nil && m.personalRoom != "" {
		key := "user:" + playerID
		if isGuest {
			key = "guest:" + playerID
		}
		m.hub.Publish(m.personalRoom, []ws.PlayerMessage{{PlayerID: key, Payload: invited}})
	}
	return next, false, nil
}

func NewManager(repo Repository, registry *module.Registry, hub *ws.Hub) *Manager {
	return &Manager{repo: repo, registry: registry, hub: hub, metrics: metrics.Nop()}
}

// Seat puts the table in the order the host wants, before it is dealt.
//
// This is how partnerships are chosen. In a game with sides the turn has to
// alternate between them — two partners can never play back to back — so a
// side *is* a position in the seating, and "who is my partner" and "where do I
// sit" are one question with one answer. Reordering the seats is therefore the
// whole mechanism, and no module needs to know it happened: Canasta already
// derives its partnerships from the order it is handed the players.
//
// The order must be a permutation of exactly who is already seated. Anything
// else — an unknown id, a duplicate, somebody left out — is refused rather than
// interpreted, because every one of those is a client bug whose kindest failure
// is a loud one.
func (m *Manager) Seat(ctx context.Context, idOrCode, hostID string, order []string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	defer e.mu.Unlock()
	match := e.match
	if match.HostID != hostID {
		return models.Match{}, module.Error{Code: "NOT_THE_HOST"}
	}
	// Only before the deal. Afterwards the seating is what the hands were dealt
	// against, and moving it would silently reassign cards already held.
	if match.Status != "lobby" {
		return models.Match{}, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}

	byID := make(map[string]models.Player, len(match.Players))
	for _, p := range match.Players {
		byID[p.ID] = p
	}
	if len(order) != len(match.Players) {
		return models.Match{}, module.Error{Code: "BAD_SEATING", Message: "the order must name every seat exactly once"}
	}
	seen := make(map[string]bool, len(order))
	players := make([]models.Player, 0, len(order))
	turn := make([]string, 0, len(order))
	for _, id := range order {
		p, ok := byID[id]
		if !ok || seen[id] {
			return models.Match{}, module.Error{Code: "BAD_SEATING", Message: "the order must name every seat exactly once"}
		}
		seen[id] = true
		players = append(players, p)
		turn = append(turn, id)
	}

	match.Players = players
	match.TurnOrder = turn
	if err := m.saveLocked(ctx, e, match); err != nil {
		return models.Match{}, err
	}
	m.Broadcast(e.match)
	return e.match, nil
}

func (m *Manager) Registry() *module.Registry { return m.registry }

// SetMetrics attaches the counter sink. Optional: a nil sink counts nothing
// and the runtime is unchanged, which is the rule for everything in this
// package that is about describing the game rather than playing it.
func (m *Manager) SetMetrics(s metrics.Sink) {
	if s == nil {
		s = metrics.Nop()
	}
	m.metrics = s
}

func (m *Manager) Repo() Repository { return m.repo }
func (m *Manager) Hub() *ws.Hub     { return m.hub }

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
	created, err := m.repo.Insert(ctx, match)
	if err != nil {
		return models.Match{}, err
	}
	// Counted on the insert rather than on the request, so a lobby that
	// failed to persist is not reported as one that existed. The gap between
	// this and matches.started is the report's "never started" figure — a
	// lobby nobody joined, which is not a game anybody failed to finish.
	m.metrics.Add(metrics.MatchesCreated, 1)
	return created, nil
}

// Join adds a player to a lobby.
func (m *Manager) Join(ctx context.Context, idOrCode string, p models.Player) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	joined, full, err := m.joinLocked(ctx, e, p)
	e.mu.Unlock()
	if err != nil {
		return models.Match{}, err
	}
	if full {
		// Told once the lock is released, so an observer can never be the
		// reason a table's lock is held.
		m.lobbyClosed(joined.ID.Hex())
	}
	return joined, nil
}

// joinLocked seats p, and reports whether that filled the table. e must be
// locked.
func (m *Manager) joinLocked(ctx context.Context, e *liveMatch, p models.Player) (models.Match, bool, error) {
	match := e.match
	if match.Status != "lobby" {
		return models.Match{}, false, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	for _, existing := range match.Players {
		if existing.ID == p.ID {
			return match, false, nil // idempotent: re-joining is not an error
		}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return models.Match{}, false, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	// The variation's range, not the module's: a table's size is a property of
	// the rules it was created under (module.SeatRange).
	if _, max := mod.Descriptor().SeatRange(match.Variation); len(match.Players) >= max {
		return models.Match{}, false, module.Error{Code: "MATCH_FULL"}
	}

	match.Players = append(append([]models.Player(nil), match.Players...), p)
	match.TurnOrder = append(append([]string(nil), match.TurnOrder...), p.ID)
	if err := m.saveLocked(ctx, e, match); err != nil {
		return models.Match{}, false, err
	}
	_, max := mod.Descriptor().SeatRange(match.Variation)
	return e.match, len(e.match.Players) >= max, nil
}

// Start deals the match through its module.
func (m *Manager) Start(ctx context.Context, idOrCode string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	started, err := m.startLocked(ctx, e)
	e.mu.Unlock()
	if err != nil {
		return models.Match{}, err
	}
	m.metrics.Add(metrics.MatchesStarted, 1)
	m.lobbyClosed(started.ID.Hex())
	m.Broadcast(started)
	// A bot may be first to act — in Hold'em it usually is, since the blinds
	// decide the order rather than who created the lobby.
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), started.ID.Hex())
	return started, nil
}

// startLocked deals the match and stores the deal as its first snapshot —
// the state every rebuild of this match starts from, so none of them ever
// calls NewMatch again. e must be locked.
func (m *Manager) startLocked(ctx context.Context, e *liveMatch) (models.Match, error) {
	match := e.match
	if match.Status != "lobby" {
		return models.Match{}, module.Error{Code: "MATCH_ALREADY_STARTED"}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return models.Match{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	d := mod.Descriptor()
	if min, _ := d.SeatRange(match.Variation); len(match.Players) < min {
		return models.Match{}, module.Error{
			Code:    "TOO_FEW_PLAYERS",
			Message: fmt.Sprintf("%s needs at least %d players", d.Label, min),
		}
	}

	state, err := mod.NewMatch(
		module.MatchConfig{Variation: match.Variation, Options: match.Options},
		playerRefs(match.Players), match.Seed)
	if err != nil {
		return models.Match{}, err
	}

	now := time.Now().UTC()
	match.Status = "active"
	match.StartedAt = &now
	e.seq = 0
	if err := m.snapshotLocked(ctx, e, match, state); err != nil {
		return models.Match{}, err
	}
	e.rounds = 0
	if r := module.RoundsFor(mod, state); r != nil {
		e.rounds = len(r.Rounds)
	}
	return e.match, nil
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
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		// A late action against a table that vanished under it — most often a
		// host's delete landing between two of a bot's own moves — otherwise
		// surfaces as the generic "ERROR" code: the lookup's error is a raw
		// store error, and module.CodeOf has no case for one. MATCH_NOT_FOUND
		// is wording the client already carries, for the identical situation
		// resumeMatch and getMatch answer with today.
		if db.IsNotFound(err) {
			return module.Error{Code: "MATCH_NOT_FOUND", Message: idOrCode}
		}
		return err
	}
	match, rounds, events, err := m.applyLocked(ctx, e, playerID, a)
	e.mu.Unlock()
	if err != nil {
		return err
	}
	mod := m.registry.Get(match.ModuleID)

	m.broadcastWith(match, rounds)
	m.publishEvents(match, events)

	// A finished match becomes a permanent record. Asynchronous and after the
	// broadcast on purpose: the player who just won should see the final board
	// without waiting on bookkeeping, and a bookkeeping failure must never
	// fail the move that won.
	if match.Status == "completed" && m.recorder != nil {
		m.recorder.RecordMatchAsync(match, module.OutcomeOf(mod, module.State(match.State)))
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

// snapshotRoundGap is how many moves must pass before a round boundary is
// worth a snapshot. Measured when snapshots were only replay landmarks: a
// board every round cost +313% at blackjack and +146% at hold'em, whose rounds
// are about nine moves, and fifty moves leaves the long-dealt games with one
// board per deal.
const snapshotRoundGap = 50

// snapshotEvery caps how many moves a rebuild ever re-applies, whatever a
// game's rounds look like — Prší keeps none, and a Žolíky deal runs long.
const snapshotEvery = 100

// snapshotDue says whether the board after move seq is stored, given the
// newest snapshot is at last and whether this move closed a round.
func snapshotDue(seq, last int, closedRound bool) bool {
	since := seq - last
	return since >= snapshotEvery || (closedRound && since >= snapshotRoundGap)
}

// touchEvery is how stale a playing match's UpdatedAt may get. The stranded
// sweep gives a match fifteen minutes and activity lists sort by it, so a
// minute is exact enough — and it is one envelope write a minute rather than
// one per move.
const touchEvery = time.Minute

// applyLocked plays one action on the match e holds and stores it: the move
// alone, most of the time, or the move with a snapshot and the envelope when
// the match finished or a snapshot is due. e must be locked.
func (m *Manager) applyLocked(ctx context.Context, e *liveMatch, playerID string, a module.Action) (
	models.Match, *module.RoundLog, []module.Event, error) {
	match := e.match
	if match.Status != "active" {
		return models.Match{}, nil, nil, module.Error{Code: "MATCH_NOT_ACTIVE"}
	}
	if e.stateErr != nil {
		return models.Match{}, nil, nil, e.stateErr
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return models.Match{}, nil, nil, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	next, events, err := mod.Apply(module.State(match.State), playerID, a)
	if err != nil {
		return models.Match{}, nil, nil, err // a refusal: nothing is stored, nothing is broadcast
	}

	entry := logEntry(e.seq+1, playerID, a)
	// The round log is decoded once here and used twice: to notice that this
	// move closed a round, and by the broadcast. Asking for it in both places
	// would decode the module's whole state twice on every single move.
	rounds := module.RoundsFor(mod, next)
	closed := e.rounds
	if rounds != nil && len(rounds.Rounds) > closed {
		closed = len(rounds.Rounds)
		entry.Rounds = closed
	}

	envelope := match
	finished := false
	if done, winners, err := mod.Finished(next); err == nil && done {
		finished = true
		now := time.Now().UTC()
		envelope.Status = "completed"
		envelope.Winners = winners
		// WinnerID stays on the document and the wire as the first winner, so
		// every client written against a single-winner match keeps working.
		envelope.WinnerID = ""
		if len(winners) > 0 {
			envelope.WinnerID = winners[0]
		}
		envelope.EndedAt = &now
	}

	last, _ := latestSnapshot(match)
	snapshot := finished || snapshotDue(entry.Seq, last, entry.Rounds > 0)
	if snapshot {
		envelope.Snapshots = withSnapshot(match.Snapshots, entry.Seq)
		err = m.repo.CommitSnapshot(ctx, match.ID, match.Version, envelope, []models.MatchAction{entry}, entry.Seq, models.JSONDoc(next))
	} else {
		err = m.repo.AppendMove(ctx, match.ID, entry)
	}
	if err != nil {
		if errors.Is(err, ErrVersionConflict) {
			// Another writer got here first; this entry is behind the store.
			e.loaded = false
		}
		log.Printf("match=%s player=%s action=%s persist failed: %v", match.ID.Hex(), playerID, a.Verb, err)
		return models.Match{}, nil, nil, err
	}

	now := time.Now().UTC()
	if snapshot {
		envelope.Version, envelope.UpdatedAt = match.Version+1, now
		e.touched = now
	}
	envelope.State = models.JSONDoc(next)
	e.match, e.seq, e.rounds = envelope, entry.Seq, closed

	if !snapshot && now.Sub(e.touched) > touchEvery {
		// Best effort: the move is stored; a missed touch costs a minute of
		// staleness in a listing, nothing more.
		if err := m.saveLocked(ctx, e, e.match); err != nil {
			log.Printf("match=%s touch failed: %v", match.ID.Hex(), err)
		}
	}
	return e.match, rounds, events, nil
}

// Broadcast sends every connected player their own view of the match.
//
// Per-viewer projection is unchanged from the rummy runtime, but the filtering
// itself has moved into the module: this loop asks each module what a given
// viewer may see and ships the answer, without knowing what was hidden.
func (m *Manager) Broadcast(match models.Match) {
	// Computed once for the whole broadcast rather than once per recipient: a
	// round log takes no viewer, so every seat would otherwise pay to decode
	// the same bytes into the same answer.
	m.broadcastWith(match, module.RoundsFor(m.registry.Get(match.ModuleID), module.State(match.State)))
}

// broadcastWith is Broadcast with the round log already in hand, for the one
// caller that had to decode it anyway — see HandleAction.
func (m *Manager) broadcastWith(match models.Match, rounds *module.RoundLog) {
	recipients := make([]string, 0, len(match.Players))
	for _, p := range match.Players {
		recipients = append(recipients, p.ID)
	}
	id := match.ID.Hex()
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
	return models.MatchAction{Seq: seq, PlayerID: playerID, Action: models.JSONDoc(raw), At: time.Now().UTC()}
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
