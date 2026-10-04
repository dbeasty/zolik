package match

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/module"
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
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		if db.IsNotFound(err) {
			return module.Action{}, module.Error{Code: "MATCH_NOT_FOUND", Message: idOrCode}
		}
		return module.Action{}, err
	}
	defer e.mu.Unlock()

	match := e.match
	if match.Status != "active" {
		return module.Action{}, module.Error{Code: "MATCH_NOT_ACTIVE"}
	}
	if e.stateErr != nil {
		return module.Action{}, e.stateErr
	}
	if playerByID(match.Players, playerID) == nil {
		return module.Action{}, module.Error{Code: "NOT_AT_THIS_TABLE"}
	}
	if !(module.MatchConfig{Variation: match.Variation, Options: match.Options}).HintsAllowed() {
		return module.Action{}, module.Error{Code: "HINTS_OFF"}
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return module.Action{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}

	state := module.State(match.State)
	offers, err := mod.LegalActions(state, playerID)
	if err != nil {
		return module.Action{}, err
	}
	live := false
	for _, o := range offers {
		if o.Enabled {
			live = true
			break
		}
	}
	if !live {
		return module.Action{}, module.Error{Code: "NOT_YOUR_TURN"}
	}

	seat := module.BotSeat{
		PlayerID: playerID,
		Skill:    module.SkillHard,
		Seed:     module.SeatSeed(match.Seed, playerID, "hint"),
	}
	bot := module.BotFor(mod)
	// A Hard seat may be playing a trained model (learn.HardModel), and a
	// hint asks as a Hard seat. Hints stay with the hand-written heuristic
	// regardless: the model switch is about who the opponents are, not about
	// what the game suggests to a person, and it can flip mid-match.
	if layered, ok := bot.(interface{ Heuristic() module.Bot }); ok {
		bot = layered.Heuristic()
	}
	a, ok := bot.Act(state, seat, offers)
	if !ok {
		return module.Action{}, module.Error{Code: "NO_HINT"}
	}
	// Named by the offer it goes through, so a client can point at the
	// control. Some bots answer in verbs alone, and a hint the client cannot
	// tie to a control on screen is no use to anyone.
	offer := module.OfferFor(offers, a)
	if offer == nil {
		return module.Action{}, module.Error{Code: "NO_HINT"}
	}
	a.OfferID = offer.ID
	return a, nil
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
		writeModuleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"action": a})
}
