package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"zolik/server/internal/db"
	"zolik/server/internal/identity"
	"zolik/server/internal/models"
)

// Token lifetimes. A guest's access token is long-lived because a guest has no
// way to sign back in: expiring it would end their game and lose the device's
// play history along with it. A real account refreshes often, since it can.
const (
	accessTokenTTL      = 15 * time.Minute
	guestAccessTokenTTL = 7 * 24 * time.Hour
	refreshTokenTTL     = 30 * 24 * time.Hour
)

// Deps is everything the auth handlers need from the outside.
type Deps struct {
	Mongo *db.Mongo
	// Providers is the configured set of external sign-in methods. An empty
	// registry is fine — the server then offers guest and email sign-in only,
	// and the clients render exactly that.
	Providers *identity.Registry
	// Mailer delivers one-time sign-in codes.
	Mailer Mailer
	// Claimer moves a guest's play history onto an account. Optional; without
	// one, sign-in works and history simply is not claimed.
	Claimer GuestClaimer
	// PublicBaseURL is this server's externally reachable base, used to build
	// the OAuth redirect URI that providers must have registered.
	PublicBaseURL string
	// AllowedReturnURLs are the prefixes a browser flow may return to. The
	// first is the default when a client sends none.
	AllowedReturnURLs []string
	// AppName appears in the sign-in email.
	AppName string
	// TestEndpointsEnabled gates /auth/dev/last-code, the same way
	// app.Config.TestEndpointsEnabled gates game's debugState — see
	// devLastCode's doc comment. Off by default; a real deployment should
	// never set this.
	TestEndpointsEnabled bool
}

type Handlers struct {
	users       *mongo.Collection
	sessionRepo *SessionRepository
	store       *Store
	accounts    *Accounts
	email       *EmailAuth
	providers   *identity.Registry

	publicBaseURL     string
	allowedReturnURLs []string

	testEndpoints bool
	// devMailer is non-nil only when TestEndpointsEnabled is set — see
	// CapturingMailer's doc comment for why this must never exist outside
	// local/e2e use.
	devMailer *CapturingMailer
}

func NewHandlers(d Deps) *Handlers {
	c := d.Mongo.Collections()
	store := NewStore(d.Mongo)
	providers := d.Providers
	if providers == nil {
		providers = identity.NewRegistry()
	}
	var mailer Mailer = d.Mailer
	if mailer == nil {
		mailer = LogMailer{}
	}
	var devMailer *CapturingMailer
	if d.TestEndpointsEnabled {
		devMailer = NewCapturingMailer(mailer)
		mailer = devMailer
	}
	return &Handlers{
		users:             c.Users,
		sessionRepo:       NewSessionRepository(d.Mongo),
		store:             store,
		accounts:          NewAccounts(store, d.Claimer),
		email:             NewEmailAuth(store, mailer, d.AppName),
		providers:         providers,
		publicBaseURL:     d.PublicBaseURL,
		allowedReturnURLs: d.AllowedReturnURLs,
		testEndpoints:     d.TestEndpointsEnabled,
		devMailer:         devMailer,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/auth/providers", h.listProviders)

	r.Post("/auth/guest", h.guest)

	// Passwordless email sign-in. Verification takes an optional Authorization
	// header: when it carries a guest token, the guest's play history is
	// claimed as part of signing in.
	r.Post("/auth/email/start", h.emailStart)
	r.With(OptionalAuthMiddleware).Post("/auth/email/verify", h.emailVerify)

	// Browser redirect flow. Start and the native-token endpoint both take an
	// optional Authorization header, for the same reason.
	r.With(OptionalAuthMiddleware).Post("/auth/oauth/{provider}/start", h.oauthStart)
	r.Get("/auth/oauth/{provider}/callback", h.oauthCallback)
	// Apple posts its callback as a form when name/email scopes are requested.
	r.Post("/auth/oauth/{provider}/callback", h.oauthCallback)
	r.Post("/auth/oauth/exchange", h.oauthExchange)
	r.With(OptionalAuthMiddleware).Post("/auth/oauth/{provider}/token", h.oauthNativeToken)

	// Account maintenance.
	r.With(AuthMiddleware).Get("/auth/identities", h.listIdentities)
	r.With(AuthMiddleware).Delete("/auth/identities/{provider}", h.unlinkIdentity)
	r.With(AuthMiddleware).Post("/auth/claim-guest", h.claimGuest)
	r.With(AuthMiddleware).Get("/auth/guest-summary", h.guestSummary)

	// Legacy username/password. Kept because the SSH/TUI client can neither
	// open a browser nor receive mail, so it has no other way in. New accounts
	// should use the flows above; nothing here grows.
	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)

	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)

	if h.testEndpoints {
		r.Get("/auth/dev/last-code", h.devLastCode)
	}
}

// devLastCode returns the most recently mailed sign-in code for an address —
// the passwordless-email equivalent of game's debugState: a way for an
// automated test to complete a flow that, for a real user, requires reading
// an actual inbox. Only mounted when TestEndpointsEnabled is set (default on
// for local dev, same as ENABLE_TEST_ENDPOINTS elsewhere; must be explicitly
// opted into anywhere APP_ENV is set) — see devMailer.
func (h *Handlers) devLastCode(w http.ResponseWriter, req *http.Request) {
	email := NormalizeEmail(req.URL.Query().Get("email"))
	if email == "" || h.devMailer == nil {
		http.Error(w, "no code available", http.StatusNotFound)
		return
	}
	code, ok := h.devMailer.LastCode(email)
	if !ok {
		http.Error(w, "no code available", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"code": code})
}

// listProviders tells a client which sign-in methods this deployment offers,
// so the sign-in screen is built from server configuration rather than from a
// list compiled into each client. Enabling Apple later lights up the button
// everywhere without shipping a new app build.
func (h *Handlers) listProviders(w http.ResponseWriter, _ *http.Request) {
	methods := []identity.Descriptor{
		{ID: "guest", DisplayName: "Play as guest", Kind: "guest"},
		{ID: models.IdentityProviderEmail, DisplayName: "Email", Kind: "email"},
	}
	methods = append(methods, h.providers.Descriptors()...)
	writeJSON(w, map[string]any{"providers": methods})
}

// --- guest ---

type guestReq struct {
	GuestName string `json:"guestName,omitempty"`
	// GuestID is the device's existing guest identity, sent back on every
	// subsequent guest sign-in so the same device keeps one identity and its
	// play history keeps accumulating in one place. Absent on first run.
	GuestID string `json:"guestId,omitempty"`
}

func (h *Handlers) guest(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var body guestReq
	_ = json.NewDecoder(req.Body).Decode(&body)

	tokens, err := h.GuestSessionWithID(ctx, body.GuestName, body.GuestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"guestName":    tokens.Username,
		"guestId":      tokens.GuestID,
		"userId":       tokens.UserID,
		"isGuest":      true,
		// How many finished matches are already recorded against this device,
		// so the client can offer "sign in to keep your N games" with a real
		// number instead of a vague promise.
		"claimableMatches": h.accounts.GuestMatchCount(ctx, tokens.GuestID),
	})
}

// guestSummary reports what a guest stands to keep by signing in. Authenticated
// as the guest, so the guest id never has to be supplied — and cannot be
// guessed at — by the caller.
func (h *Handlers) guestSummary(w http.ResponseWriter, req *http.Request) {
	uc, _ := GetUserContext(req)
	if !uc.IsGuest {
		http.Error(w, "not a guest session", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"guestId":          uc.UserID,
		"claimableMatches": h.accounts.GuestMatchCount(req.Context(), uc.UserID),
	})
}

// --- passwordless email ---

type emailStartReq struct {
	Email string `json:"email"`
}

func (h *Handlers) emailStart(w http.ResponseWriter, req *http.Request) {
	var body emailStartReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err := h.email.StartSignIn(req.Context(), body.Email, clientIP(req))
	switch {
	case errors.Is(err, ErrTooManyCodes):
		http.Error(w, "too many codes requested for that address — try again shortly", http.StatusTooManyRequests)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// The response says nothing about whether that address has an account,
	// which would otherwise make this endpoint a membership oracle.
	writeJSON(w, map[string]any{"sent": true, "expiresInSeconds": int(codeTTL.Seconds())})
}

type emailVerifyReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *Handlers) emailVerify(w http.ResponseWriter, req *http.Request) {
	var body emailVerifyReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	claims, err := h.email.VerifyCode(req.Context(), body.Email, body.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	opts := SignInOptions{}
	if uc, ok := GetUserContext(req); ok {
		if uc.IsGuest {
			opts.GuestID = uc.UserID
		} else {
			opts.LinkToUserID = uc.UserID
		}
	}
	h.completeSignIn(w, req, claims, opts, models.IdentityProviderEmail)
}

// --- account maintenance ---

func (h *Handlers) listIdentities(w http.ResponseWriter, req *http.Request) {
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "guests have no linked accounts", http.StatusForbidden)
		return
	}
	ids, err := h.accounts.Identities(req.Context(), uc.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"identities": ids})
}

func (h *Handlers) unlinkIdentity(w http.ResponseWriter, req *http.Request) {
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "guests have no linked accounts", http.StatusForbidden)
		return
	}
	err := h.accounts.Unlink(req.Context(), uc.UserID, chi.URLParam(req, "provider"))
	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, "no such linked account", http.StatusNotFound)
	case err != nil:
		// The "last way in" refusal lands here, and its message is written to
		// be shown to the player as-is.
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		writeJSON(w, map[string]any{"unlinked": true})
	}
}

type claimGuestReq struct {
	// GuestRefreshToken is the guest session's refresh token, which is proof
	// that this device really is that guest.
	//
	// The guest id alone would not do: it travels in match records and in
	// game state, so treating it as a claim ticket would let anyone who has
	// seen one walk off with somebody else's history. Possession of the
	// session is the thing that actually distinguishes the owner.
	GuestRefreshToken string `json:"guestRefreshToken"`
}

// claimGuest absorbs a device's guest play history into the signed-in account.
//
// This is the "I signed in first and only then remembered my old games" path;
// the common case is handled inline by whichever sign-in the person used.
func (h *Handlers) claimGuest(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "sign in with an account first", http.StatusForbidden)
		return
	}

	var body claimGuestReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.GuestRefreshToken == "" {
		http.Error(w, "guestRefreshToken required", http.StatusBadRequest)
		return
	}
	session, err := h.sessionRepo.FindByToken(ctx, body.GuestRefreshToken)
	if err != nil || session.GuestID == "" {
		http.Error(w, "that guest session is not valid", http.StatusUnauthorized)
		return
	}

	u, err := h.store.FindUserByID(ctx, uc.UserID)
	if err != nil {
		http.Error(w, "unknown account", http.StatusUnauthorized)
		return
	}
	claimed, err := h.accounts.ClaimGuest(ctx, session.GuestID, u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// The guest session is retired: its history now belongs to the account,
	// and leaving it usable would let the device keep playing as a guest whose
	// results land nowhere.
	_ = h.sessionRepo.DeleteByToken(ctx, body.GuestRefreshToken)

	writeJSON(w, map[string]any{"claimedMatches": claimed})
}

// --- legacy username/password ---

func (h *Handlers) createUser(ctx context.Context, u models.User) (models.User, error) {
	return h.store.InsertUser(ctx, u)
}

func (h *Handlers) findUserByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	err := h.users.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	return u, err
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

	created, err := h.createUser(ctx, models.User{
		Username:     body.Username,
		Email:        NormalizeEmail(body.Email),
		PasswordHash: string(hash),
		AuthProvider: models.IdentityProviderLocal,
		Preferences: models.UserPreferences{
			Language:  "en",
			CardStyle: "classic",
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// The password itself is the identity's proof, so the identity row records
	// only that this account has a local login — which is what stops Unlink
	// from stranding somebody whose password is their only way in.
	if _, err := h.store.InsertIdentity(ctx, models.Identity{
		UserID:      created.ID.Hex(),
		Provider:    models.IdentityProviderLocal,
		Subject:     created.ID.Hex(),
		DisplayName: created.Username,
	}); err != nil && !errors.Is(err, ErrIdentityTaken) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokens, err := h.issueUserSession(ctx, created)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"userId":       tokens.UserID,
		"username":     tokens.Username,
		"isGuest":      false,
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
	if u.PasswordHash == "" ||
		bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(body.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	tokens, err := h.issueUserSession(ctx, u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"userId":       tokens.UserID,
		"username":     tokens.Username,
		"isGuest":      false,
	})
}

// --- session lifecycle ---

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

	// Rotate: retire the old token and issue a new one.
	_ = h.sessionRepo.DeleteByToken(ctx, body.RefreshToken)
	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     newRefresh,
		GuestName: s.GuestName,
		UserID:    s.UserID,
		GuestID:   s.GuestID,
		CreatedAt: now,
		ExpiresAt: now.Add(refreshTokenTTL),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// The subject is the account for a registered player and the *guest id*
	// for a guest — never the refresh token, which rotates. A rotating subject
	// used to change the player's in-game id mid-session and made their play
	// impossible to attribute afterwards.
	subject, isGuest := s.UserID, s.UserID == ""
	ttl := accessTokenTTL
	if isGuest {
		subject, ttl = s.GuestID, guestAccessTokenTTL
		if subject == "" {
			// A guest session from before guest ids existed. Mint one now so
			// the session gains a durable identity rather than staying
			// unattributable forever.
			if subject, err = NewRandomToken(16); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = h.sessionRepo.SetGuestID(ctx, newRefresh, subject)
		}
	}

	accessToken, err := CreateAccessToken(subject, s.GuestName, isGuest, ttl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"accessToken":  accessToken,
		"refreshToken": newRefresh,
		"userId":       subject,
		"isGuest":      isGuest,
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
	writeJSON(w, map[string]any{"loggedOut": true})
}

// clientIP is best-effort, for abuse investigation only. It trusts
// X-Forwarded-For, which is fine for that purpose and would not be for
// anything security-relevant — nothing here makes a decision on it.
func clientIP(req *http.Request) string {
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		if first, _, ok := strings.Cut(fwd, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(fwd)
	}
	host, _, ok := strings.Cut(req.RemoteAddr, ":")
	if !ok {
		return req.RemoteAddr
	}
	return host
}
