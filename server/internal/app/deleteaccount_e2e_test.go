package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/stats"
)

// Deleting an account, through the routes a phone calls: the person signs in
// by email, finishes a game, and deletes the account. Afterwards nothing
// signs in as it, nothing counts for it, and the game is still there for the
// bots it was played against, with the seat renamed.
func TestDeletingAnAccountRemovesItAndAnonymisesItsGames(t *testing.T) {
	a := newNode(t, Config{TestEndpointsEnabled: true})
	a.Start(context.Background())
	router := chi.NewRouter()
	a.RegisterRoutes(router)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	ctx := context.Background()

	session := emailSignIn(t, srv.URL, "ada@example.test")
	userID, access, refresh := session.UserID, session.AccessToken, session.RefreshToken

	// A finished game against two bots.
	mgr := a.matchManager()
	host := models.Player{ID: userID, Name: "ada", UserID: userID}
	created, err := mgr.Create(ctx, "prsi", module.MatchConfig{}, host)
	if err != nil {
		t.Fatalf("opening the table: %v", err)
	}
	for _, bot := range []models.Player{
		{ID: "seat-bot1", Name: "Bot One", IsAI: true, AIDifficulty: "medium", AIPersona: "medium:bot-one"},
		{ID: "seat-bot2", Name: "Bot Two", IsAI: true, AIDifficulty: "medium", AIPersona: "medium:bot-two"},
	} {
		if _, err := mgr.Join(ctx, created.ID.Hex(), bot); err != nil {
			t.Fatalf("seating a bot: %v", err)
		}
	}
	if _, err := mgr.Start(ctx, created.ID.Hex()); err != nil {
		t.Fatalf("starting: %v", err)
	}
	playToCompletion(t, a, created.ID.Hex(), host.ID)
	userKey := "user:" + userID
	waitFor(t, "the game to be recorded", func() bool {
		_, err := a.statsRepo.FindPlayerStats(ctx, userKey)
		return err == nil
	})

	if _, err := a.kdb.Get(db.UserReadOnlyNS(userID), "stats"); err != nil {
		t.Fatalf("setup: no stats copy for the account's devices to check: %v", err)
	}

	// A guest has nothing to delete.
	var guest signedIn
	postJSON(t, srv.URL+"/auth/guest", "", map[string]string{"guestName": "Guest"}, http.StatusOK, &guest)
	deleteAccount(t, srv.URL, guest.AccessToken, http.StatusForbidden)

	deleteAccount(t, srv.URL, access, http.StatusOK)

	oid, _ := bson.ObjectIDFromHex(userID)
	if _, err := a.authStore.FindUserByID(ctx, userID); err == nil {
		t.Error("the account is still there")
	}
	if ids, err := a.authStore.ListIdentities(ctx, userID); err != nil || len(ids) != 0 {
		t.Errorf("sign-in identities left: %v (%v)", ids, err)
	}
	if _, err := a.userRepo.FindByID(ctx, oid); err == nil {
		t.Error("the profile still answers")
	}
	postJSON(t, srv.URL+"/auth/refresh", "", map[string]string{"refreshToken": refresh}, http.StatusUnauthorized, nil)
	if _, err := a.statsRepo.FindPlayerStats(ctx, userKey); !errors.Is(err, stats.ErrNotFound) {
		t.Errorf("the lifetime record is still there: %v", err)
	}
	if n, _ := a.statsRepo.CountMatchesForSubject(ctx, userKey); n != 0 {
		t.Errorf("%d games still recorded under the account", n)
	}
	rec, err := a.statsRepo.FindMatchByMatchID(ctx, created.ID)
	if err != nil {
		t.Fatalf("the game's record went too: %v", err)
	}
	for _, seat := range rec.Participants {
		if seat.PlayerID == userID && (seat.Subject.Kind != stats.SubjectGuest || seat.Name != "Player 1") {
			t.Errorf("the account's seat in the record is %+v", seat)
		}
	}
	m, err := mgr.Current(ctx, created.ID.Hex())
	if err != nil {
		t.Fatalf("the game went: %v", err)
	}
	if p := m.Players[0]; p.UserID != "" || p.Name != "Player 1" {
		t.Errorf("the account's seat at the table is %+v", p)
	}

	// Nothing is left where the account's own devices would sync it from.
	if _, err := a.kdb.Get(db.UserReadOnlyNS(userID), "stats"); !db.IsNotFound(err) {
		t.Errorf("the stats copy for the account's devices is still there: %v", err)
	}
	for _, ns := range []string{db.UserNS(userID), db.UserReadOnlyNS(userID)} {
		if !a.kdb.HoldsNamespace(ns) {
			continue
		}
		left := 0
		_ = a.kdb.Scan(ns, func([]byte) error { left++; return nil })
		if left != 0 {
			t.Errorf("%s still holds %d documents", ns, left)
		}
	}

	// The address is free: signing in with it again makes a new account.
	again := emailSignIn(t, srv.URL, "ada@example.test")
	if again.UserID == userID {
		t.Error("signing in again found the deleted account")
	}
}

type signedIn struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	UserID       string `json:"userId"`
}

func emailSignIn(t *testing.T, base, email string) signedIn {
	t.Helper()
	postJSON(t, base+"/auth/email/start", "", map[string]string{"email": email}, http.StatusOK, nil)
	var code struct {
		Code string `json:"code"`
	}
	getJSON(t, base+"/auth/dev/last-code?email="+url.QueryEscape(email), "", http.StatusOK, &code)
	var out signedIn
	postJSON(t, base+"/auth/email/verify", "", map[string]string{"email": email, "code": code.Code}, http.StatusOK, &out)
	return out
}

func deleteAccount(t *testing.T, base, token string, want int) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, base+"/auth/account", nil)
	doJSON(t, req, token, want, nil)
}
