package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
)

// Always-run KDB checks for the contracts the sign-in flow leans on. The
// full HTTP suite in handlers_test.go runs against this same store with
// ZOLIK_TEST_DB_ENGINE=kdb; these cover the load-bearing semantics on every
// plain `go test ./...`.

func newKDBStoreT(t *testing.T) (Store, SessionRepository) {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return NewKDBStore(k), NewKDBSessionRepository(k)
}

func TestKDBIdentityUniquenessIsTheInvariant(t *testing.T) {
	s, _ := newKDBStoreT(t)
	ctx := context.Background()

	first, err := s.InsertIdentity(ctx, models.Identity{UserID: "u1", Provider: "google", Subject: "sub-1"})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// The same external identity must never attach to a second account.
	if _, err := s.InsertIdentity(ctx, models.Identity{UserID: "u2", Provider: "google", Subject: "sub-1"}); !errors.Is(err, ErrIdentityTaken) {
		t.Fatalf("second insert: got %v, want ErrIdentityTaken", err)
	}
	got, err := s.FindIdentity(ctx, "google", "sub-1")
	if err != nil || got.UserID != "u1" {
		t.Fatalf("identity belongs to %q (%v), want u1", got.UserID, err)
	}
	// Same subject under another provider is a different identity.
	if _, err := s.InsertIdentity(ctx, models.Identity{UserID: "u2", Provider: "apple", Subject: "sub-1"}); err != nil {
		t.Fatalf("other-provider insert: %v", err)
	}
	if first.ID.IsZero() {
		t.Fatal("insert did not assign an id")
	}
}

func TestKDBInsertUserEnforcesUniqueness(t *testing.T) {
	s, _ := newKDBStoreT(t)
	ctx := context.Background()

	if _, err := s.InsertUser(ctx, models.User{Username: "ada", Email: "ada@x.test"}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := s.InsertUser(ctx, models.User{Username: "ada"}); !db.IsDuplicateKey(err) {
		t.Fatalf("duplicate username: got %v, want duplicate-key", err)
	}
	if _, err := s.InsertUser(ctx, models.User{Username: "grace", Email: "ada@x.test"}); !db.IsDuplicateKey(err) {
		t.Fatalf("duplicate email: got %v, want duplicate-key", err)
	}
	// Two accounts with no email at all are fine (the Mongo index is sparse).
	if _, err := s.InsertUser(ctx, models.User{Username: "grace"}); err != nil {
		t.Fatalf("second no-email account: %v", err)
	}
}

func TestKDBLoginCodeIsSingleUse(t *testing.T) {
	s, _ := newKDBStoreT(t)
	ctx := context.Background()

	now := time.Now().UTC()
	c := models.LoginCode{Email: "a@x.test", CodeHash: "h", CreatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
	if err := s.InsertLoginCode(ctx, c); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.LatestLoginCode(ctx, "a@x.test")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if err := s.ConsumeLoginCode(ctx, got.ID); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	// The second redemption must be told the code is spent.
	if err := s.ConsumeLoginCode(ctx, got.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second consume: got %v, want ErrNotFound", err)
	}
}

func TestKDBLatestLoginCodeIgnoresExpired(t *testing.T) {
	s, _ := newKDBStoreT(t)
	ctx := context.Background()

	now := time.Now().UTC()
	stale := models.LoginCode{Email: "b@x.test", CodeHash: "old", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-45 * time.Minute)}
	if err := s.InsertLoginCode(ctx, stale); err != nil {
		t.Fatalf("insert stale: %v", err)
	}
	if _, err := s.LatestLoginCode(ctx, "b@x.test"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired code still redeemable: %v", err)
	}
}

func TestKDBFlowExchangeCodeWorksExactlyOnce(t *testing.T) {
	s, _ := newKDBStoreT(t)
	ctx := context.Background()

	now := time.Now().UTC()
	f := models.OAuthFlow{State: "st-1", Nonce: "n", Provider: "google", CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute)}
	if err := s.InsertFlow(ctx, f); err != nil {
		t.Fatalf("insert: %v", err)
	}
	stored, err := s.FindFlowByState(ctx, "st-1")
	if err != nil {
		t.Fatalf("find by state: %v", err)
	}
	if err := s.CompleteFlow(ctx, stored.ID, "xc-1", models.OAuthFlowResult{UserID: "u1"}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	// A provider delivering the same callback twice must not complete twice.
	if err := s.CompleteFlow(ctx, stored.ID, "xc-2", models.OAuthFlowResult{UserID: "u1"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second complete: got %v, want ErrNotFound", err)
	}
	taken, err := s.TakeFlowResult(ctx, "xc-1")
	if err != nil || taken.Result == nil || taken.Result.UserID != "u1" {
		t.Fatalf("take: %+v (%v)", taken, err)
	}
	// Read-and-destroy: the replayed exchange finds nothing.
	if _, err := s.TakeFlowResult(ctx, "xc-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replayed take: got %v, want ErrNotFound", err)
	}
}

func TestKDBSessionExpiryIsEnforcedOnRead(t *testing.T) {
	_, sessions := newKDBStoreT(t)
	ctx := context.Background()

	now := time.Now().UTC()
	if err := sessions.CreateSession(ctx, models.Session{Token: "expired", GuestName: "g", CreatedAt: now.Add(-48 * time.Hour), ExpiresAt: now.Add(-24 * time.Hour)}); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	// Mongo's TTL monitor deletes these; here the read itself must refuse
	// them, because the sweeper is periodic just like Mongo's.
	if _, err := sessions.FindByToken(ctx, "expired"); err == nil {
		t.Fatal("an expired refresh token was accepted")
	}

	if err := sessions.CreateSession(ctx, models.Session{Token: "live", GuestName: "g", CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)}); err != nil {
		t.Fatalf("create live: %v", err)
	}
	if err := sessions.SetGuestID(ctx, "live", "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set guest id: %v", err)
	}
	got, err := sessions.FindByToken(ctx, "live")
	if err != nil || got.GuestID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("live session: %+v (%v)", got, err)
	}
	if err := sessions.DeleteByToken(ctx, "live"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := sessions.FindByToken(ctx, "live"); err == nil {
		t.Fatal("deleted session still readable")
	}
}
