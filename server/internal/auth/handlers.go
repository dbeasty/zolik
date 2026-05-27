package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

type Handlers struct {
	users       *mongo.Collection
	statistics  *mongo.Collection
	sessionRepo *SessionRepository
}

func NewHandlers(m *db.Mongo) *Handlers {
	c := m.Collections()
	return &Handlers{
		users:       c.Users,
		statistics:  c.Statistics,
		sessionRepo: NewSessionRepository(m),
	}
}

func (h *Handlers) createUser(ctx context.Context, u models.User) (models.User, error) {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.LastSeenAt = now
	res, err := h.users.InsertOne(ctx, u)
	if err != nil {
		return models.User{}, err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		u.ID = oid
	}
	_, _ = h.statistics.UpdateOne(ctx,
		bson.M{"userId": u.ID},
		bson.M{"$setOnInsert": models.Statistics{UserID: u.ID}},
		nil,
	)
	return u, nil
}

func (h *Handlers) findUserByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	err := h.users.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	return u, err
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Post("/auth/guest", h.guest)
	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
}

type guestReq struct {
	GuestName string `json:"guestName,omitempty"`
}

func (h *Handlers) guest(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body guestReq
	_ = json.NewDecoder(req.Body).Decode(&body)

	guestName := body.GuestName
	if guestName == "" {
		guestName = "Guest"
	}

	refreshToken, err := CreateRefreshToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: guestName,
		UserID:    "",
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	accessToken, err := CreateAccessToken(refreshToken, guestName, true, 7*24*time.Hour)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"guestName":    guestName,
	})
}

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password"`
}

func (h *Handlers) register(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body registerReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Username == "" || body.Password == "" {
		http.Error(w, "username/password required", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	u := models.User{
		Username:     body.Username,
		Email:        body.Email,
		PasswordHash: string(hash),
		AuthProvider: "local",
		Preferences: models.UserPreferences{
			Language:  "en",
			CardStyle: "classic",
		},
	}

	u.CreatedAt = time.Now().UTC()
	u.LastSeenAt = time.Now().UTC()

	created, err := h.createUser(ctx, u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = created

	// Create refresh session.
	refreshToken, err := CreateRefreshToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	// Persist refresh token.
	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: body.Username,
		UserID:    created.ID.Hex(),
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	accessToken, err := CreateAccessToken(created.ID.Hex(), created.Username, false, 15*time.Minute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handlers) login(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body loginReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, err := h.findUserByUsername(ctx, body.Username)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	refreshToken, err := CreateRefreshToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: u.Username,
		UserID:    u.ID.Hex(),
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	accessToken, err := CreateAccessToken(u.ID.Hex(), u.Username, false, 15*time.Minute)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
	})
}

type refreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handlers) refresh(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body refreshReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.RefreshToken == "" {
		http.Error(w, "refreshToken required", http.StatusBadRequest)
		return
	}

	s, err := h.sessionRepo.FindByToken(ctx, body.RefreshToken)
	if err != nil {
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	newRefresh, err := CreateRefreshToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	newExpiresAt := now.Add(30 * 24 * time.Hour)

	// Rotate: delete old and insert new.
	_ = h.sessionRepo.DeleteByToken(ctx, body.RefreshToken)
	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     newRefresh,
		GuestName: s.GuestName,
		UserID:    s.UserID,
		CreatedAt: now,
		ExpiresAt: newExpiresAt,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// subject = userId for registered, token itself for guest
	subject := s.UserID
	isGuest := subject == ""
	if isGuest {
		subject = newRefresh
	}

	ttl := 15 * time.Minute
	if isGuest {
		ttl = 7 * 24 * time.Hour
	}

	accessToken, err := CreateAccessToken(subject, s.GuestName, isGuest, ttl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"accessToken":  accessToken,
		"refreshToken": newRefresh,
	})
}

type logoutReq struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handlers) logout(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body logoutReq
	_ = json.NewDecoder(req.Body).Decode(&body)
	if body.RefreshToken != "" {
		_ = h.sessionRepo.DeleteByToken(ctx, body.RefreshToken)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"loggedOut": true})
}

