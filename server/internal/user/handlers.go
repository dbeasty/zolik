package user

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
)

type Handlers struct {
	repo *Repository
}

func NewHandlers(repo *Repository) *Handlers {
	return &Handlers{repo: repo}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// /leaderboard, /users/me/stats and /users/me/history live in
	// internal/stats, which owns the match records they are computed from.
	r.With(auth.AuthMiddleware).Get("/users/me", h.me)
	r.With(auth.AuthMiddleware).Patch("/users/me", h.patchMe)
}

func hctx(req *http.Request) (auth.UserContext, bool) {
	return auth.GetUserContext(req)
}

func (h *Handlers) requireUser(w http.ResponseWriter, req *http.Request) (auth.UserContext, bool) {
	uc, ok := hctx(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return auth.UserContext{}, false
	}
	if uc.IsGuest {
		http.Error(w, "forbidden for guests", http.StatusForbidden)
		return auth.UserContext{}, false
	}
	return uc, true
}

func (h *Handlers) me(w http.ResponseWriter, req *http.Request) {
	uc, ok := h.requireUser(w, req)
	if !ok {
		return
	}
	oid, err := bson.ObjectIDFromHex(uc.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	u, err := h.repo.FindByID(req.Context(), oid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       u.ID.Hex(),
		"username": u.Username,
		"email":    u.Email,
		"prefs":    u.Preferences,
	})
}

type patchMeReq struct {
	Username     *string           `json:"username,omitempty"`
	Preferences  *models.UserPreferences `json:"preferences,omitempty"`
}

func (h *Handlers) patchMe(w http.ResponseWriter, req *http.Request) {
	uc, ok := h.requireUser(w, req)
	if !ok {
		return
	}
	oid, err := bson.ObjectIDFromHex(uc.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var body patchMeReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	update := bson.M{}
	if body.Username != nil {
		update["username"] = *body.Username
	}
	if body.Preferences != nil {
		update["preferences"] = *body.Preferences
	}
	if len(update) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]any{"updated": false})
		return
	}
	if err := h.repo.UpdateByID(req.Context(), oid, update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"updated": true})
}
