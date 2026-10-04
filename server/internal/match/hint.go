package match

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/botstats"
	"zolik/server/internal/db"
	"zolik/server/internal/module"
	"zolik/server/internal/ratelimit"
)

// Hint is the move this player's own seat would make if a strong bot were
// sitting in it, worked out and never made.
//
// It reuses the bot rather than a separate adviser, because the bot is already
// the thing that knows how to play each game and already limits itself to
// what its seat can see (see allmodules_test's hint check). The caller
// still has to press the button: a hint that played itself would be a bot
// sitting in a person's seat.
func (m *Manager) Hint(ctx context.Context, idOrCode, playerID string) (module.Action, error) {
	in, err := m.hintInput(ctx, idOrCode, playerID)
	if err != nil {
		return module.Action{}, err
	}
	// Counted only once there is a decision to make: the refusals above cost
	// no more than any state read, and a player asking "is it my turn yet?"
	// should not use up the hints they then want.
	if !m.hintLimiter().Allow(playerID, time.Now()) {
		return module.Action{}, module.Error{Code: "HINT_TOO_SOON"}
	}

	// Worked out with the table unlocked. A hint is advice about a position,
	// not a move in it, so nothing here needs the table to hold still — and
	// holding it would make every other seat wait on this player's adviser.
	// If the table moves on meanwhile the hint is about a position that has
	// passed, which is what the player's press would find out anyway: it goes
	// through the same validation as any move.
	end := m.botStats.Begin(botstats.Key{Module: in.moduleID, Skill: skillLabel(in.seat.Skill), Source: botstats.SourceHint, Engine: botstats.EngineRule})
	a, ok := in.bot.Act(in.state, in.seat, in.offers)
	end()
	if !ok {
		return module.Action{}, module.Error{Code: "NO_HINT"}
	}
	// Named by the offer it goes through, so a client can point at the
	// control. Some bots answer in verbs alone, and a hint the client cannot
	// tie to a control on screen is no use to anyone.
	offer := module.OfferFor(in.offers, a)
	if offer == nil {
		return module.Action{}, module.Error{Code: "NO_HINT"}
	}
	a.OfferID = offer.ID
	return a, nil
}

// hintPerWindow hints a player may ask for in hintWindow. A person reading the
// board never comes near it; a client asking in a loop does, and each ask is a
// Hard decision on this server's CPU.
const (
	hintPerWindow = 5
	hintWindow    = 10 * time.Second
)

func (m *Manager) hintLimiter() *ratelimit.Limiter {
	m.hintsOnce.Do(func() { m.hints = ratelimit.New(hintWindow, hintPerWindow) })
	return m.hints
}

// hintRequest is everything a hint is worked out from, copied out of the
// table so the table can be let go of first.
type hintRequest struct {
	moduleID string
	bot      module.Bot
	state    module.State
	seat     module.BotSeat
	offers   []module.ActionOffer
}

// hintInput checks the player may have a hint now and copies out what it
// needs, holding the table only for that.
func (m *Manager) hintInput(ctx context.Context, idOrCode, playerID string) (hintRequest, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		if db.IsNotFound(err) {
			return hintRequest{}, module.Error{Code: "MATCH_NOT_FOUND", Message: idOrCode}
		}
		return hintRequest{}, err
	}
	defer e.mu.Unlock()

	match := e.match
	if match.Status != "active" {
		return hintRequest{}, module.Error{Code: "MATCH_NOT_ACTIVE"}
	}
	if e.stateErr != nil {
		return hintRequest{}, e.stateErr
	}
	if playerByID(match.Players, playerID) == nil {
		return hintRequest{}, module.Error{Code: "NOT_AT_THIS_TABLE"}
	}
	if !(module.MatchConfig{Variation: match.Variation, Options: match.Options}).HintsAllowed() {
		return hintRequest{}, module.Error{Code: "HINTS_OFF"}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return hintRequest{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}

	// A copy, because the table's state is the table's: the next move
	// replaces it while the bot is still reading this one.
	state := module.State(bytes.Clone(match.State))
	offers, err := mod.LegalActions(state, playerID)
	if err != nil {
		return hintRequest{}, err
	}
	live := false
	for _, o := range offers {
		if o.Enabled {
			live = true
			break
		}
	}
	if !live {
		return hintRequest{}, module.Error{Code: "NOT_YOUR_TURN"}
	}

	bot := module.BotFor(mod)
	// A Hard seat may be playing a trained model (learn.HardModel), and a
	// hint asks as a Hard seat. Hints stay with the hand-written heuristic
	// regardless: the model switch is about who the opponents are, not about
	// what the game suggests to a person, and it can flip mid-match.
	if layered, ok := bot.(interface{ Heuristic() module.Bot }); ok {
		bot = layered.Heuristic()
	}

	return hintRequest{
		moduleID: match.ModuleID,
		bot:      bot,
		state:    state,
		seat: module.BotSeat{
			PlayerID: playerID,
			Skill:    module.SkillHard,
			Seed:     module.SeatSeed(match.Seed, playerID, "hint"),
		},
		offers: offers,
	}, nil
}

// hint answers POST /matches/{id}/hint with the suggested action.
func (h *Handlers) hint(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	a, err := h.manager.Hint(req.Context(), chi.URLParam(req, "id"), uc.UserID)
	if err != nil {
		if module.CodeOf(err) == "HINT_TOO_SOON" {
			w.Header().Set("Retry-After", strconv.Itoa(int(hintWindow/time.Second)))
		}
		writeModuleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"action": a})
}
