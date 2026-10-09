package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/auth"
	"zolik/server/internal/match"
)

// The routes a phone serves about its finished games.
//
// Two audiences, kept apart. The list of games waiting on this device, and the
// host's own save or discard, belong to whoever holds the phone: they answer
// only on the loopback listener the phone's own app talks to, never on the
// one the room reaches, and never through the Bluetooth tunnel. A player's
// answer for their own seat belongs to that player, wherever they are sitting:
// it is an ordinary authenticated call, and it can only ever change that one
// seat.

type loopbackKey struct{}

// MarkLoopback marks a connection as the phone's own. zolikcore sets it on
// every connection its loopback listener accepts, and on nothing else.
func MarkLoopback(ctx context.Context) context.Context {
	return context.WithValue(ctx, loopbackKey{}, true)
}

func isLoopback(r *http.Request) bool {
	v, _ := r.Context().Value(loopbackKey{}).(bool)
	return v
}

// ownerOnly refuses anything that did not arrive on the phone's own listener.
func ownerOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		next(w, r)
	}
}

// registerLocalSaves mounts the routes above, on an embedded host only.
func (a *App) registerLocalSaves(r chi.Router) {
	if a.localSaves == nil {
		// Built with the match manager; make sure it exists.
		a.matchManager()
	}
	saves := a.localSaves
	if saves == nil {
		return
	}
	r.Get("/local/saves", ownerOnly(func(w http.ResponseWriter, _ *http.Request) {
		list, err := saves.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if list == nil {
			list = []match.LocalSave{}
		}
		writeLocalJSON(w, http.StatusOK, map[string]any{
			"enrolled": saves.Enrolled(),
			"games":    list,
		})
	}))
	r.Post("/local/saves/{id}/save", ownerOnly(func(w http.ResponseWriter, r *http.Request) {
		rec, err := saves.Save(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeSaveError(w, err)
			return
		}
		writeLocalJSON(w, http.StatusOK, rec)
	}))
	r.Post("/local/saves/{id}/discard", ownerOnly(func(w http.ResponseWriter, r *http.Request) {
		if err := saves.Discard(chi.URLParam(r, "id")); err != nil {
			writeSaveError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	// A player's own seat: what they have said, and saying it.
	r.With(auth.AuthMiddleware).Get("/matches/{id}/save-consent", func(w http.ResponseWriter, r *http.Request) {
		uc, _ := auth.GetUserContext(r)
		rec, err := saves.Get(chi.URLParam(r, "id"))
		if err != nil {
			writeLocalJSON(w, http.StatusOK, map[string]any{"available": false})
			return
		}
		for _, seat := range rec.Seats {
			if seat.Subject != "" && seat.Subject == uc.UserID {
				writeLocalJSON(w, http.StatusOK, map[string]any{
					"available": true,
					"consent":   seat.Consent,
					"kind":      seat.Kind,
					"state":     rec.State,
				})
				return
			}
		}
		writeLocalJSON(w, http.StatusOK, map[string]any{"available": false})
	})
	r.With(auth.AuthMiddleware).Post("/matches/{id}/save-consent", func(w http.ResponseWriter, r *http.Request) {
		uc, _ := auth.GetUserContext(r)
		var body struct {
			Save *bool `json:"save"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Save == nil {
			http.Error(w, "save (true or false) is required", http.StatusBadRequest)
			return
		}
		rec, err := saves.Consent(r.Context(), chi.URLParam(r, "id"), uc.UserID, *body.Save)
		if err != nil {
			writeSaveError(w, err)
			return
		}
		for _, seat := range rec.Seats {
			if seat.Subject == uc.UserID {
				writeLocalJSON(w, http.StatusOK, map[string]any{
					"available": true,
					"consent":   seat.Consent,
					"kind":      seat.Kind,
					"state":     rec.State,
				})
				return
			}
		}
		writeLocalJSON(w, http.StatusOK, map[string]any{"available": true})
	})
}

func writeSaveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, match.ErrNoSuchSave):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, match.ErrNotSeated):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, match.ErrGameGone):
		http.Error(w, err.Error(), http.StatusGone)
	case errors.Is(err, match.ErrNotEnrolled):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeLocalJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
