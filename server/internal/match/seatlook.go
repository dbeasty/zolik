package match

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// A player's own face and name at a table.
//
// Both used to be settled the moment a seat was taken: the face came with the
// seating request and the name came out of the token, and nothing after that
// could change either. Which was fine until somebody sat down, saw the table,
// and wanted to be the fox rather than the owl — or found that the name the
// guest screen had suggested was a near-twin of the one across from them. The
// remedy then was to leave and come back, which is a great deal of ceremony
// for a cosmetic choice, and impossible once the cards are out.
//
// So a seated person may change their own look at any point until the match
// is over. Only their own: this is the seat's owner speaking, never the host
// dressing somebody else, and a bot's name is the host's strength picker's to
// change (UpdateBot). The change is for this table, carried into its rematch
// by the seat it rides on, and is not written back to the account or the
// token — a name is chosen for the people sitting here.
//
// The one rule a name has to keep is the one a default name was introduced
// for (see auth.GuestNameFor): it has to tell this seat apart from the others.
// Two seats spelling the same name, or the same name in a different case, is
// refused with NAME_TAKEN rather than quietly suffixed — the player asked for
// that name, and a "Lucky Fox 2" they did not choose is worse than being told.

// seatLookReq is what a player wants to change about their own seat. A field
// left out is left alone.
type seatLookReq struct {
	Name   *string `json:"name,omitempty"`
	Avatar *string `json:"avatar,omitempty"`
}

// setSeatLook changes the caller's own seat at this table.
func (h *Handlers) setSeatLook(w http.ResponseWriter, req *http.Request) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body seatLookReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var name, avatar *string
	if body.Name != nil {
		clean := auth.CleanDisplayName(*body.Name, auth.MaxGuestNameRunes)
		if clean == "" {
			http.Error(w, "a name is required", http.StatusBadRequest)
			return
		}
		name = &clean
	}
	if body.Avatar != nil {
		// An unrecognised slug is no slug, as everywhere a face arrives
		// (models.SanitizeAvatar): the seat falls back to its derived face.
		slug := models.SanitizeAvatar(*body.Avatar)
		avatar = &slug
	}
	updated, err := h.manager.UpdateOwnSeat(req.Context(), chi.URLParam(req, "id"), uc.UserID, name, avatar)
	if err != nil {
		writeModuleError(w, err)
		return
	}
	seat := playerByID(updated.Players, uc.UserID)
	writeJSON(w, map[string]any{"playerId": seat.ID, "name": seat.Name, "avatar": seat.Avatar})
}

// UpdateOwnSeat changes the name and face a person sits under, under the
// table's lock, and tells everyone at the table. A nil field is left as it is.
func (m *Manager) UpdateOwnSeat(ctx context.Context, idOrCode, playerID string, name, avatar *string) (models.Match, error) {
	e, err := m.lockMatch(ctx, idOrCode)
	if err != nil {
		return models.Match{}, err
	}
	match := e.match
	idx := -1
	for i, p := range match.Players {
		if p.ID == playerID && !p.IsAI {
			idx = i
		}
	}
	if idx < 0 {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "NOT_AT_THIS_TABLE", Message: playerID}
	}
	if match.Status == "completed" {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "MATCH_MOVED_ON"}
	}
	if name != nil && nameTaken(match, playerID, *name) {
		e.mu.Unlock()
		return models.Match{}, module.Error{Code: "NAME_TAKEN", Message: *name}
	}
	seat := match.Players[idx]
	if name != nil {
		seat.Name = *name
	}
	if avatar != nil {
		seat.Avatar = *avatar
	}
	if seat.Name == match.Players[idx].Name && seat.Avatar == match.Players[idx].Avatar {
		saved := e.match
		e.mu.Unlock()
		return saved, nil
	}
	match.Players = append([]models.Player(nil), match.Players...)
	match.Players[idx] = seat
	if err := m.saveLocked(ctx, e, match); err != nil {
		e.mu.Unlock()
		return models.Match{}, err
	}
	saved := e.match
	e.mu.Unlock()
	m.Broadcast(saved)
	return saved, nil
}

// nameTaken is whether anybody else at the table — seated, or with a seat held
// for them — already goes by name, ignoring case.
func nameTaken(match models.Match, playerID, name string) bool {
	for _, p := range match.Players {
		if p.ID != playerID && strings.EqualFold(p.Name, name) {
			return true
		}
	}
	for _, r := range match.Reserved {
		if r.PlayerID != playerID && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}
