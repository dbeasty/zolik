package sync

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"zolik/server/internal/db"
)

// stubMatchHome is the match rules the handover policy asks, with everything a
// test needs to make a match busy or a player a stranger.
type stubMatchHome struct {
	seated   map[string][]string
	busy     map[string]bool
	released []string
	adopted  []string
}

func (s *stubMatchHome) Seated(_ context.Context, matchHex, userHex string) (bool, error) {
	for _, u := range s.seated[matchHex] {
		if u == userHex {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubMatchHome) Idle(_ context.Context, matchHex string) (bool, error) {
	return !s.busy[matchHex], nil
}

func (s *stubMatchHome) Release(_ context.Context, matchHex string) error {
	s.released = append(s.released, matchHex)
	return nil
}

func (s *stubMatchHome) Adopt(_ context.Context, matchHex string) error {
	s.adopted = append(s.adopted, matchHex)
	return nil
}

func wsURL(server *httptest.Server) string {
	return "ws://" + strings.TrimPrefix(server.URL, "http://") + "/kdb/sync"
}

func TestAMatchIsPickedUpOnAnotherDevice(t *testing.T) {
	const user = "65f0c0ffeec0ffeec0ffee01"
	const match = "65f0dead65f0dead65f0dead"
	verifier := stubVerifier{
		"phone-a": {NodeID: "phone-a", OwnerHex: user},
		"phone-b": {NodeID: "phone-b", OwnerHex: user},
	}
	seating := stubSeating{match: {user}}
	home := &stubMatchHome{seated: map[string][]string{match: {user}}}

	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, verifier, seating)
	hub.node.SetMatchHome(home)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	phoneA := openNode(t, Config{
		Role: RoleSpoke, HubURL: wsURL(server), Token: "phone-a",
		Namespaces: []string{db.MatchNS(match)},
	}, verifier, seating)
	phoneA.node.SetMatchHome(home)
	phoneB := openNode(t, Config{
		Role: RoleSpoke, HubURL: wsURL(server), Token: "phone-b",
		Namespaces: []string{db.MatchNS(match)},
	}, verifier, seating)
	phoneB.node.SetMatchHome(home)

	// The first device hosts the match and says so, then pushes it up.
	if err := phoneA.kdb.Put(db.MatchNS(match), "m/1", []byte(`{"seq":1,"action":"draw"}`)); err != nil {
		t.Fatalf("first move: %v", err)
	}
	if err := phoneA.node.ClaimMatch(match); err != nil {
		t.Fatalf("claiming the match: %v", err)
	}
	if err := phoneA.node.SyncNow(); err != nil {
		t.Fatalf("pushing the match up: %v", err)
	}
	if !phoneA.node.WritesMatches(match) {
		t.Fatal("the device hosting the match does not think it writes it")
	}

	// The player picks the match up on the hub, which is where a web client
	// plays: the first device hands it over.
	if err := phoneA.node.GiveMatch(context.Background(), match, hub.node.NodeID(), "hub"); err != nil {
		t.Fatalf("handing the match to the hub: %v", err)
	}
	if err := phoneA.node.SyncNow(); err != nil {
		t.Fatalf("syncing the handover: %v", err)
	}
	if phoneA.node.WritesMatches(match) {
		t.Fatal("the first device still thinks it writes the match it gave away")
	}

	// And now from the second device, which asks the hub for it.
	if err := phoneB.node.SyncNow(); err != nil {
		t.Fatalf("phone B first sync: %v", err)
	}
	if err := phoneB.node.TakeMatch(context.Background(), match, "the player opened it here"); err != nil {
		t.Fatalf("taking the match: %v", err)
	}
	if !phoneB.node.WritesMatches(match) {
		t.Fatal("the device that took the match does not write it")
	}
	// It has the first device's move, which is what makes carrying on
	// possible rather than starting again.
	if _, err := phoneB.kdb.Get(db.MatchNS(match), "m/1"); err != nil {
		t.Fatalf("the moves did not come with the match: %v", err)
	}
	if len(home.adopted) == 0 {
		t.Fatal("the match was taken without being loaded")
	}
}

func TestAMatchIsNotHandedOverMidMove(t *testing.T) {
	const user = "65f0c0ffeec0ffeec0ffee01"
	const match = "65f0dead65f0dead65f0dead"
	verifier := stubVerifier{"phone-b": {NodeID: "phone-b", OwnerHex: user}}
	seating := stubSeating{match: {user}}
	home := &stubMatchHome{seated: map[string][]string{match: {user}}, busy: map[string]bool{match: true}}

	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, verifier, seating)
	hub.node.SetMatchHome(home)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	if err := hub.kdb.Put(db.MatchNS(match), "m/1", []byte(`{"seq":1}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := hub.node.ClaimMatch(match); err != nil {
		t.Fatalf("claim: %v", err)
	}

	phoneB := openNode(t, Config{
		Role: RoleSpoke, HubURL: wsURL(server), Token: "phone-b",
		Namespaces: []string{db.MatchNS(match)},
	}, verifier, seating)
	phoneB.node.SetMatchHome(home)
	_ = phoneB.node.SyncNow()

	err := phoneB.node.TakeMatch(context.Background(), match, "opened here")
	if err == nil {
		t.Fatal("a match was handed over in the middle of a move")
	}
	if !strings.Contains(err.Error(), "move") {
		t.Fatalf("refusal = %v, want it to say why", err)
	}
	if phoneB.node.WritesMatches(match) {
		t.Fatal("the refused device thinks it writes the match")
	}
}

func TestOnlyASeatedPlayerMayTakeAMatch(t *testing.T) {
	const stranger = "65f0c0ffeec0ffeec0ffee09"
	const seated = "65f0c0ffeec0ffeec0ffee01"
	const match = "65f0dead65f0dead65f0dead"
	verifier := stubVerifier{"phone-x": {NodeID: "phone-x", OwnerHex: stranger}}
	// The stranger is allowed to sync nothing of this match, but the point
	// here is the handover: even a node that somehow reached it may not take
	// a table it is not sitting at.
	seating := stubSeating{match: {seated, stranger}}
	home := &stubMatchHome{seated: map[string][]string{match: {seated}}}

	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, verifier, seating)
	hub.node.SetMatchHome(home)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	if err := hub.kdb.Put(db.MatchNS(match), "m/1", []byte(`{"seq":1}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := hub.node.ClaimMatch(match); err != nil {
		t.Fatalf("claim: %v", err)
	}

	phone := openNode(t, Config{
		Role: RoleSpoke, HubURL: wsURL(server), Token: "phone-x",
		Namespaces: []string{db.MatchNS(match)},
	}, verifier, seating)
	phone.node.SetMatchHome(home)
	_ = phone.node.SyncNow()

	if err := phone.node.TakeMatch(context.Background(), match, "curious"); err == nil {
		t.Fatal("a stranger took a match over")
	}
}
