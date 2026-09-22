package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/identity"
	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
)

// Token lifetimes. A guest's access token is long-lived because a guest has no
// way to sign back in: expiring it would end their game and lose the device's
// play history along with it. A real account refreshes often, since it can.
const (
	accessTokenTTL      = 15 * time.Minute
	guestAccessTokenTTL = 7 * 24 * time.Hour
	refreshTokenTTL     = 30 * 24 * time.Hour
	// refreshReuseGrace is how long a refresh token still answers after it has
	// been exchanged, with the token it was exchanged for. Long enough for the
	// requests a client already had in the air, short enough that a leaked
	// token is not a second session.
	refreshReuseGrace = time.Minute
)

// Deps is everything the auth handlers need from the outside.
type Deps struct {
	// Store is the persistence behind accounts and their identities.
	Store Store
	// Sessions is the persistence behind refresh-token sessions.
	Sessions SessionRepository
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
	// Nodes is the persistence behind enrolled replicas. Optional: without
	// one, /nodes/enroll answers that this deployment does not enrol nodes
	// rather than enrolling them into nothing.
	Nodes NodeRepository
	// Credentials reports the version an account's credentials are at, which
	// is the `sv` an offline pass carries. Optional; without one every
	// account is at version one, which is true until something revokes.
	Credentials CredentialVersions
	// OfflineKeys are the cloud's public keys as this node holds them, and
	// are what an offline host verifies a pass with. Nil on the cloud itself,
	// which verifies passes it signed with its own keyring, and nil on a host
	// that has never been online, where no pass can be checked and none is
	// accepted.
	OfflineKeys PublicKeys
	// TestEndpointsEnabled gates /auth/dev/last-code, the same way
	// app.Config.TestEndpointsEnabled gates game's debugState — see
	// devLastCode's doc comment. Off by default; a real deployment should
	// never set this.
	TestEndpointsEnabled bool
}

type Handlers struct {
	sessionRepo SessionRepository
	store       Store
	accounts    *Accounts
	email       *EmailAuth
	providers   *identity.Registry
	nodes       NodeRepository
	credentials CredentialVersions
	offlineKeys PublicKeys

	publicBaseURL     string
	allowedReturnURLs []string

	testEndpoints bool
	// devMailer is non-nil only when TestEndpointsEnabled is set — see
	// CapturingMailer's doc comment for why this must never exist outside
	// local/e2e use.
	devMailer *CapturingMailer

	// metrics counts first-time guests. Never nil after NewHandlers.
	metrics metrics.Sink
}

// SetOfflineKeys gives an embedded host the cloud keys it verifies offline
// passes with. It is a setter rather than another field on Deps because the
// cache belongs to the install rather than to the deployment: the phone host
// builds it from its own data directory and hands it over once the server it
// wraps exists.
func (h *Handlers) SetOfflineKeys(keys PublicKeys) { h.offlineKeys = keys }

// SetMetrics attaches the counter sink, here and on the accounts behind it.
func (h *Handlers) SetMetrics(s metrics.Sink) {
	if s == nil {
		s = metrics.Nop()
	}
	h.metrics = s
	if h.accounts != nil {
		h.accounts.SetMetrics(s)
	}
}

func NewHandlers(d Deps) *Handlers {
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
	// Both of these default to the in-memory stand-ins rather than to nil, so
	// the endpoints behind them work on a deployment that has not wired the
	// persistent implementations yet. What is lost is durability, not
	// correctness: a restart forgets enrolments and revocations, which is
	// visible, where a nil would have been a silent 503.
	nodes := d.Nodes
	if nodes == nil {
		nodes = NewMemoryNodeRepository()
	}
	credentials := d.Credentials
	if credentials == nil {
		credentials = NewMemoryCredentialVersions()
	}
	return &Handlers{
		sessionRepo:       d.Sessions,
		store:             d.Store,
		accounts:          NewAccounts(d.Store, d.Claimer),
		email:             NewEmailAuth(d.Store, mailer, d.AppName),
		providers:         providers,
		nodes:             nodes,
		credentials:       credentials,
		offlineKeys:       d.OfflineKeys,
		publicBaseURL:     d.PublicBaseURL,
		allowedReturnURLs: d.AllowedReturnURLs,
		testEndpoints:     d.TestEndpointsEnabled,
		devMailer:         devMailer,
		metrics:           metrics.Nop(),
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/auth/providers", h.listProviders)

	// The public half of whatever this server signs with. Unauthenticated,
	// because the keys are public and because every node that will later
	// verify a token without a connection has to have fetched them while it
	// still had one.
	r.Get("/.well-known/jwks.json", h.jwks)

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

	// Replicas. Enrolling one is something an account does, not something a
	// node does on its own, so it is authenticated as the person whose node
	// it will be.
	r.With(AuthMiddleware).Post("/nodes/enroll", h.enrollNode)
	r.With(AuthMiddleware).Get("/nodes", h.listNodes)

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

// RegisterLocalRoutes mounts the part of RegisterRoutes that an offline table
// hosted on a phone needs: a guest seat, and keeping it. There are no
// accounts, no mail and no identity provider on a table with no internet, and
// every route left out is one fewer reachable from the local network.
func (h *Handlers) RegisterLocalRoutes(r chi.Router) {
	// A host publishes its node key for the same reason the cloud publishes
	// its signing key: the tokens it hands the phones around the table are
	// signed with it, and a guest that can fetch the key can check them
	// rather than take them on trust. The key is public by construction, and
	// it is what the cloud is given at enrolment.
	r.Get("/.well-known/jwks.json", h.jwks)
	r.Post("/auth/guest", h.guest)
	// The one way a real account gets a seat at a table with no internet: a
	// pass the cloud signed, checked against the keys this host cached while
	// it last had a connection.
	r.Post("/auth/offline-pass", h.offlinePass)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
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
		internalError(w, "guest", err)
		return
	}

	out := map[string]any{
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
	}
	// Only where a node signed one. A client keeps it beside the guest id and
	// hands it back when the person eventually signs in, which is how seats
	// taken at an offline table are reconciled with an account.
	if tokens.SeatReceipt != "" {
		out["seatReceipt"] = tokens.SeatReceipt
	}
	writeJSON(w, out)
}

type offlinePassReq struct {
	OfflinePass string `json:"offlinePass"`
}

// offlinePass seats a signed-in cloud account at a host with no internet.
//
// The pass is the whole of the evidence. This host cannot ask the cloud
// anything, has no copy of the account and never will, so what it does is
// check the cloud's signature against keys it cached earlier and then seat
// the person as "user:<hex>" rather than as a guest. Their play is recorded
// against their real account id from the first hand, and syncs as theirs when
// the host next sees a connection.
//
// The local session is cut short to the pass's own expiry where that comes
// first. A host that let the session outlive the pass would be extending a
// credential the cloud issued for thirty days into an indefinite one, which
// is precisely the bound the pass exists to impose.
func (h *Handlers) offlinePass(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if h.offlineKeys == nil {
		http.Error(w, "this host has no copy of the cloud's signing keys", http.StatusServiceUnavailable)
		return
	}
	var body offlinePassReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || strings.TrimSpace(body.OfflinePass) == "" {
		http.Error(w, "offlinePass required", http.StatusBadRequest)
		return
	}
	claims, err := VerifyOfflinePass(body.OfflinePass, h.offlineKeys)
	if err != nil {
		// One answer for an unknown key, a wrong audience, a bad signature
		// and an expired pass alike. Which of them it was is a detail about
		// this host's key cache, and is of interest only to somebody probing
		// it.
		http.Error(w, "that pass is not valid here", http.StatusUnauthorized)
		return
	}

	username := claims.Username
	if username == "" {
		// A pass with no name still seats its holder. The name is for the
		// other players at the table, and one derived from the id is better
		// than an empty chair.
		username = GuestNameFor(claims.Subject)
	}
	subject := claims.SeatSubject()

	refreshToken, err := CreateRefreshToken()
	if err != nil {
		internalError(w, "offlinePass", err)
		return
	}
	now := time.Now().UTC()
	expiresAt := now.Add(refreshTokenTTL)
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(expiresAt) {
		expiresAt = claims.ExpiresAt.Time
	}
	if err := h.sessionRepo.CreateSession(ctx, models.Session{
		Token:     refreshToken,
		GuestName: username,
		UserID:    subject,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}); err != nil {
		internalError(w, "offlinePass", err)
		return
	}
	accessToken, err := CreateAccessToken(subject, username, false, accessTokenTTL)
	if err != nil {
		internalError(w, "offlinePass", err)
		return
	}
	writeJSON(w, map[string]any{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"userId":       subject,
		"username":     username,
		"isGuest":      false,
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
		internalError(w, "listIdentities", err)
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
		internalError(w, "claimGuest", err)
		return
	}
	// The guest session is retired: its history now belongs to the account,
	// and leaving it usable would let the device keep playing as a guest whose
	// results land nowhere.
	_ = h.sessionRepo.DeleteByToken(ctx, body.GuestRefreshToken)

	writeJSON(w, map[string]any{"claimedMatches": claimed})
}

// --- legacy username/password ---

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
		internalError(w, "register", err)
		return
	}

	created, err := h.store.InsertUser(ctx, models.User{
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
		internalError(w, "register", err)
		return
	}

	tokens, err := h.issueUserSession(ctx, created)
	if err != nil {
		internalError(w, "register", err)
		return
	}
	writeSessionJSON(w, tokens)
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
	u, err := h.store.FindUserByUsername(ctx, body.Username)
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
		internalError(w, "login", err)
		return
	}
	writeSessionJSON(w, tokens)
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
	now := time.Now().UTC()

	var newRefresh string
	if s.ReplacedBy != "" {
		// Already exchanged — usually by a second request from the same
		// client that was rejected at the same moment. Inside the grace
		// period it gets the same successor; after it, or once the successor
		// itself is gone (logged out, or rotated past its own grace), it is
		// refused. Mongo's TTL monitor sweeps only once a minute, so the
		// expiry is checked here rather than trusted to have happened.
		next, err := h.sessionRepo.FindByToken(ctx, s.ReplacedBy)
		if now.After(s.ExpiresAt) || err != nil || next.ReplacedBy != "" {
			http.Error(w, "invalid refresh token", http.StatusUnauthorized)
			return
		}
		s, newRefresh = next, next.Token
	} else {
		if newRefresh, err = CreateRefreshToken(); err != nil {
			internalError(w, "refresh", err)
			return
		}
		// A seat an offline host granted on the strength of a pass may not be
		// refreshed past the pass that justified it. Rotating it to a full
		// thirty days on every refresh would turn a bounded credential into
		// an unbounded one, and the bound is the only thing the cloud still
		// controls once the host is out of reach.
		expiresAt := now.Add(refreshTokenTTL)
		if strings.HasPrefix(s.UserID, OfflineSeatPrefix) {
			if now.After(s.ExpiresAt) {
				http.Error(w, "invalid refresh token", http.StatusUnauthorized)
				return
			}
			if s.ExpiresAt.Before(expiresAt) {
				expiresAt = s.ExpiresAt
			}
		}
		// Rotate: issue a new token, then retire the old one.
		if err := h.sessionRepo.CreateSession(ctx, models.Session{
			Token:     newRefresh,
			GuestName: s.GuestName,
			UserID:    s.UserID,
			GuestID:   s.GuestID,
			CreatedAt: now,
			ExpiresAt: expiresAt,
		}); err != nil {
			internalError(w, "refresh", err)
			return
		}
		_ = h.sessionRepo.Retire(ctx, body.RefreshToken, newRefresh, now.Add(refreshReuseGrace))
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
				internalError(w, "refresh", err)
				return
			}
			_ = h.sessionRepo.SetGuestID(ctx, newRefresh, subject)
		}
	}

	accessToken, err := CreateAccessToken(subject, s.GuestName, isGuest, ttl)
	if err != nil {
		internalError(w, "refresh", err)
		return
	}
	out := map[string]any{
		"accessToken":  accessToken,
		"refreshToken": newRefresh,
		"userId":       subject,
		"isGuest":      isGuest,
	}
	// Refreshed alongside the session, so a player who stays signed in never
	// carries a pass much older than the session it came with, and one who
	// opens the app before a trip leaves with a full thirty days.
	if !isGuest {
		if pass := h.offlinePassFor(ctx, subject, s.GuestName); pass != "" {
			out["offlinePass"] = pass
		}
	}
	writeJSON(w, out)
}

// writeSessionJSON is the answer to every sign-in that mints a session. It is
// one function so that a pass cannot be handed out by one path and quietly
// forgotten by another.
func writeSessionJSON(w http.ResponseWriter, tokens SessionTokens) {
	out := map[string]any{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
		"userId":       tokens.UserID,
		"username":     tokens.Username,
		"isGuest":      tokens.IsGuest,
	}
	if tokens.OfflinePass != "" {
		out["offlinePass"] = tokens.OfflinePass
	}
	writeJSON(w, out)
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
