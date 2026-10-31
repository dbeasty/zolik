package stats

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

const (
	forgetAlice = "65ab000000000000000000a1"
	forgetBob   = "65ab000000000000000000b2"
)

// forgetTable records one finished match between Alice (seat 0), Bob
// (seat 1) and a hard bot (seat 2), with Alice winning.
func forgetTable(t *testing.T, r Repository, at time.Time) MatchResult {
	t.Helper()
	seats := []Standing{
		{PlayerID: forgetAlice, Seat: 0, Name: "alice", Subject: Subject{Kind: SubjectUser, ID: forgetAlice, Name: "alice"}, Score: 30, Rank: 1, Won: true},
		{PlayerID: forgetBob, Seat: 1, Name: "bob", Subject: Subject{Kind: SubjectUser, ID: forgetBob, Name: "bob"}, Score: 20, Rank: 2},
		{PlayerID: "bot-1", Seat: 2, Name: "Miroslav", Subject: Subject{Kind: SubjectAI, ID: "hard", Name: "Miroslav"}, Score: 10, Rank: 3},
	}
	m := MatchResult{
		MatchID:      bson.NewObjectID(),
		ModuleID:     "prsi",
		StartedAt:    at.Add(-10 * time.Minute),
		CompletedAt:  at,
		Participants: seats,
		SubjectKeys:  subjectKeysOf(seats),
		Winners:      []string{forgetAlice},
		RecordedAt:   at,
	}
	m, err := r.InsertMatch(context.Background(), m)
	if err != nil {
		t.Fatalf("insert match: %v", err)
	}
	return m
}

func TestForgetUserAnonymisesTheSeatAndKeepsEveryoneElsesRecord(t *testing.T) {
	ctx := context.Background()
	r := newKDBRepo(t)
	c := NewClaimer(r)

	now := time.Now().UTC()
	first := forgetTable(t, r, now.Add(-time.Hour))
	forgetTable(t, r, now)
	alice := Subject{Kind: SubjectUser, ID: forgetAlice, Name: "alice"}
	bob := Subject{Kind: SubjectUser, ID: forgetBob, Name: "bob"}
	bot := Subject{Kind: SubjectAI, ID: "hard", Name: "Miroslav"}
	for _, s := range []Subject{alice, bob, bot} {
		if err := c.RebuildPlayerStats(ctx, s); err != nil {
			t.Fatalf("rebuild %s: %v", s.Key(), err)
		}
	}
	if ps, _ := r.FindPlayerStats(ctx, bob.Key()); ps.HeadToHead[alice.Key()].Matches != 2 {
		t.Fatalf("setup: bob's head-to-head against alice = %+v, want 2 matches", ps.HeadToHead[alice.Key()])
	}

	if err := c.ForgetUser(ctx, forgetAlice); err != nil {
		t.Fatalf("ForgetUser: %v", err)
	}

	if _, err := r.FindPlayerStats(ctx, alice.Key()); !errors.Is(err, ErrNotFound) {
		t.Errorf("alice's lifetime record: err = %v, want ErrNotFound", err)
	}
	if n, err := r.CountMatchesForSubject(ctx, alice.Key()); err != nil || n != 0 {
		t.Errorf("matches still under alice = %d (%v), want 0", n, err)
	}

	// The match is still in Bob's history, with Alice's seat anonymous.
	rec, err := r.FindMatchByMatchID(ctx, first.MatchID)
	if err != nil {
		t.Fatalf("find match: %v", err)
	}
	seat := rec.Participants[0]
	wantID := models.AnonymousGuestID(first.MatchID.Hex(), forgetAlice)
	if seat.Subject.Kind != SubjectGuest || seat.Subject.ID != wantID {
		t.Errorf("alice's seat is now %+v, want the anonymous guest %s", seat.Subject, wantID)
	}
	if seat.Name != "Player 1" || seat.Subject.Name != "Player 1" {
		t.Errorf("alice's seat is called %q / %q, want Player 1", seat.Name, seat.Subject.Name)
	}
	if !seat.Won || seat.Score != 30 {
		t.Errorf("alice's seat lost its result: %+v", seat)
	}
	if rec.Participants[1].Subject != bob {
		t.Errorf("bob's seat changed: %+v", rec.Participants[1].Subject)
	}
	if contains(rec.SubjectKeys, alice.Key()) {
		t.Errorf("subjectKeys = %v, still names alice", rec.SubjectKeys)
	}

	// Bob keeps his totals but no longer has a line naming Alice.
	ps, err := r.FindPlayerStats(ctx, bob.Key())
	if err != nil {
		t.Fatalf("bob's record: %v", err)
	}
	if ps.Overall.Matches != 2 {
		t.Errorf("bob's matches = %d, want 2", ps.Overall.Matches)
	}
	if _, ok := ps.HeadToHead[alice.Key()]; ok {
		t.Errorf("bob's head-to-head still names alice: %v", ps.HeadToHead)
	}
	if n, _ := r.CountMatchesForSubject(ctx, bob.Key()); n != 2 {
		t.Errorf("bob's history has %d matches, want 2", n)
	}
	botStats, err := r.FindPlayerStats(ctx, bot.Key())
	if err != nil {
		t.Fatalf("bot's record: %v", err)
	}
	if _, ok := botStats.HeadToHead[alice.Key()]; ok {
		t.Errorf("the bot's head-to-head still names alice")
	}

	// A second run finds nothing left to do.
	if err := c.ForgetUser(ctx, forgetAlice); err != nil {
		t.Fatalf("second ForgetUser: %v", err)
	}
	again, _ := r.FindMatchByMatchID(ctx, first.MatchID)
	if again.Participants[0].Subject != seat.Subject {
		t.Errorf("second run changed the seat again: %+v", again.Participants[0].Subject)
	}
}
