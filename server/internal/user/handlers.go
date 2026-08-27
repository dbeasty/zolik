package user

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// IdentityLister supplies the sign-in methods attached to an account.
//
// An interface rather than a concrete dependency so this package keeps knowing
// nothing about how identities are proven — it renders the list, it does not
// interpret it. Satisfied by auth.Store.
type IdentityLister interface {
	ListIdentities(ctx context.Context, userID string) ([]models.Identity, error)
}

type Handlers struct {
	repo       Repository
	identities IdentityLister
}

func NewHandlers(repo Repository, identities IdentityLister) *Handlers {
	return &Handlers{repo: repo, identities: identities}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	// /leaderboard, /users/me/stats and /users/me/history live in
	// internal/stats, which owns the match records they are computed from.
	// Linking and unlinking live in internal/auth, which owns identities.
	r.With(auth.AuthMiddleware).Get("/users/me", h.me)
	r.With(auth.AuthMiddleware).Patch("/users/me", h.patchMe)
}

func (h *Handlers) requireUser(w http.ResponseWriter, req *http.Request) (auth.UserContext, bool) {
	uc, ok := auth.GetUserContext(req)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return auth.UserContext{}, false
	}
	if uc.IsGuest {
		// A guest has no account document to render. This is not an error the
		// client should ever provoke — it knows it is a guest from its own
		// session — so the plain status is enough.
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

	// The linked-account list is part of the profile rather than a separate
	// call, because every screen that shows the account also has to answer
	// "how do I get back in next time" — and a second round trip to answer it
	// invites clients to skip asking.
	identities := []models.Identity{}
	if h.identities != nil {
		if list, err := h.identities.ListIdentities(req.Context(), uc.UserID); err == nil {
			identities = list
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":            u.ID.Hex(),
		"username":      u.Username,
		"email":         u.Email,
		"emailVerified": u.EmailVerified,
		"avatarUrl":     u.AvatarURL,
		"createdAt":     u.CreatedAt,
		"prefs":         u.Preferences,
		"identities":    identities,
		// Whether a password still works, so the account screen can show the
		// legacy login without guessing from the identity list.
		"hasPassword": u.PasswordHash != "",
	})
}

type patchMeReq struct {
	Username    *string                 `json:"username,omitempty"`
	Preferences *models.UserPreferences `json:"preferences,omitempty"`
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
		name := strings.TrimSpace(*body.Username)
		if name == "" {
			http.Error(w, "a name is required", http.StatusBadRequest)
			return
		}
		if len([]rune(name)) > 24 {
			http.Error(w, "that name is too long", http.StatusBadRequest)
			return
		}
		update["username"] = name
	}
	if body.Preferences != nil {
		update["preferences"] = *body.Preferences
	}
	if len(update) == 0 {
		_ = json.NewEncoder(w).Encode(map[string]any{"updated": false})
		return
	}
	if err := h.repo.UpdateByID(req.Context(), oid, update); err != nil {
		// The unique index on username is what makes a clash a clash; report
		// it as one rather than as a server fault.
		if db.IsDuplicateKey(err) {
			http.Error(w, "that name is already taken", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"updated": true})
}
