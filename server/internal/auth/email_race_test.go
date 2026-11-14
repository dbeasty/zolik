package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"zolik/server/internal/models"
)

// A six-digit code is only safe while its guesses are capped. The cap used to
// be checked on a snapshot and counted after the compare, so a burst of
// parallel guesses all read "no attempts yet" and all got compared: sixty at
// once got sixty tries. Now each guess is paid for, atomically, before it is
// compared - so after a burst of wrong guesses the code is burned, and the
// right code typed afterwards is refused too.
func TestParallelWrongGuessesBurnTheCode(t *testing.T) {
	store, _ := newKDBStoreT(t)
	mailer := NewCapturingMailer(LogMailer{})
	e := NewEmailAuth(store, mailer, "TestApp")
	ctx := context.Background()
	const email = "race@example.com"

	if err := e.StartSignIn(ctx, email, "192.0.2.1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	code, ok := mailer.LastCode(email)
	if !ok {
		t.Fatal("no code was mailed")
	}
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}

	const guesses = 60
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	start := make(chan struct{})
	for range guesses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := e.VerifyCode(ctx, email, wrong); err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	if accepted != 0 {
		t.Fatalf("%d wrong guesses were accepted", accepted)
	}

	rec, err := store.LatestLoginCode(ctx, email)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if rec.Attempts != maxCodeAttempts {
		t.Fatalf("attempts = %d after %d parallel guesses, want exactly %d", rec.Attempts, guesses, maxCodeAttempts)
	}
	if _, err := e.VerifyCode(ctx, email, code); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("the right code after a burst of wrong ones: %v, want ErrInvalidCode", err)
	}
}

// Parallel guesses that include the right code: at most one sign-in, and
// never more comparisons than the cap allows.
func TestParallelGuessesIncludingTheRightCodeSignInAtMostOnce(t *testing.T) {
	store, _ := newKDBStoreT(t)
	mailer := NewCapturingMailer(LogMailer{})
	e := NewEmailAuth(store, mailer, "TestApp")
	ctx := context.Background()
	const email = "race2@example.com"
	if err := e.StartSignIn(ctx, email, "192.0.2.1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	code, _ := mailer.LastCode(email)

	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	start := make(chan struct{})
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := e.VerifyCode(ctx, email, code); err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("%d sign-ins from one code, want exactly 1", accepted)
	}
}

// The store's conditional updates, directly: an attempt is only taken while
// guesses remain, and a code whose guesses ran out cannot be consumed.
func TestKDBLoginCodeAttemptsAreConditional(t *testing.T) {
	s, _ := newKDBStoreT(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.InsertLoginCode(ctx, models.LoginCode{Email: "c@x.test", CodeHash: "h", CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	got, err := s.LatestLoginCode(ctx, "c@x.test")
	if err != nil {
		t.Fatal(err)
	}
	for n := range 3 {
		if err := s.TakeCodeAttempt(ctx, got.ID, 3); err != nil {
			t.Fatalf("attempt %d: %v", n+1, err)
		}
	}
	if err := s.TakeCodeAttempt(ctx, got.ID, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a fourth attempt at a three-attempt code: %v, want ErrNotFound", err)
	}
	if err := s.ConsumeLoginCode(ctx, got.ID, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("consuming a code past its attempts: %v, want ErrNotFound", err)
	}
	if err := s.ConsumeLoginCode(ctx, got.ID, 3); err != nil {
		t.Fatalf("consuming a code within its attempts: %v", err)
	}
	if err := s.TakeCodeAttempt(ctx, got.ID, 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an attempt at a consumed code: %v, want ErrNotFound", err)
	}
}
