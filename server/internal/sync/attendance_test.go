package sync

import (
	"net/http/httptest"
	"testing"
	"time"

	"zolik/server/internal/db"
)

func TestASelfHostedNodeFollowsWhoeverIsPlayingThere(t *testing.T) {
	const ada = "65f0c0ffeec0ffeec0ffee01"
	const grace = "65f0c0ffeec0ffeec0ffee02"
	verifier := stubVerifier{
		// A self-hosted node's credential: it serves whoever signs in, so it
		// is not bounded by one account the way a phone's is.
		"club": {NodeID: "club", Server: true},
	}
	seating := stubSeating{}

	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, verifier, seating)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	club := openNode(t, Config{
		Role: RoleSpoke, HubURL: wsURL(server), Token: "club",
		// Nothing configured: what it holds is decided by who turns up.
	}, verifier, seating)

	if got := club.node.Attending(); len(got) != 0 {
		t.Fatalf("a node nobody has used is holding %v", got)
	}

	// Ada signs in and plays. Her settings follow her to the machine in the
	// room, which is the point of the thing.
	if err := hub.kdb.Put(db.UserNS(ada), "prefs", []byte(`{"_kind":"prefs","cardStyle":"vector"}`)); err != nil {
		t.Fatalf("hub write: %v", err)
	}
	club.node.Attend(ada)
	if err := club.node.SyncNow(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	eventually(t, "Ada's settings to reach the club's node", func() bool {
		_, err := club.kdb.Get(db.UserNS(ada), "prefs")
		return err == nil
	})

	// Somebody who has not been there is not held.
	if err := hub.kdb.Put(db.UserNS(grace), "prefs", []byte(`{"_kind":"prefs","cardStyle":"large"}`)); err != nil {
		t.Fatalf("hub write: %v", err)
	}
	_ = club.node.SyncNow()
	time.Sleep(300 * time.Millisecond)
	if _, err := club.kdb.Get(db.UserNS(grace), "prefs"); err == nil {
		t.Fatal("the club's node holds somebody who has never played there")
	}

	// Attending again is not a second subscription, and does not churn the
	// replicator on every request.
	club.node.Attend(ada)
	if got := club.node.Attending(); len(got) != 1 || got[0] != ada {
		t.Fatalf("attending = %v, want just Ada", got)
	}
}

func TestASelfHostedNodeLetsGoOfPeopleWhoStopComing(t *testing.T) {
	const ada = "65f0c0ffeec0ffeec0ffee01"
	verifier := stubVerifier{"club": {NodeID: "club", Server: true}}
	seating := stubSeating{}

	hub := openNode(t, Config{Role: RoleHub, Advertise: "hub"}, verifier, seating)
	server := httptest.NewServer(hub.node.Handler())
	t.Cleanup(server.Close)

	club := openNode(t, Config{Role: RoleSpoke, HubURL: wsURL(server), Token: "club"}, verifier, seating)
	club.node.Attend(ada)
	if got := club.node.Attending(); len(got) != 1 {
		t.Fatalf("attending = %v", got)
	}

	// Still within the window: somebody who played yesterday evening should
	// find their things here tonight rather than waiting for a sync.
	if let := club.node.ForgetIdle(time.Now().Add(attendanceWindow - time.Minute)); let != 0 {
		t.Fatalf("let go of %d people who had just been here", let)
	}
	// Past it: a machine in somebody's house should not end up holding
	// everyone who has ever signed in on it.
	if let := club.node.ForgetIdle(time.Now().Add(attendanceWindow + time.Minute)); let != 1 {
		t.Fatalf("let go of %d people, want 1", let)
	}
	if got := club.node.Attending(); len(got) != 0 {
		t.Fatalf("still attending %v", got)
	}

	// What it already holds is not deleted: that is a decision for whoever
	// runs the machine, not for a timer.
	if err := club.kdb.Put(db.UserNS(ada), "prefs", []byte(`{"_kind":"prefs"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := club.kdb.Get(db.UserNS(ada), "prefs"); err != nil {
		t.Fatalf("the node dropped data it was holding: %v", err)
	}
}
