package lobby

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// testRedisURL points at the dev-stack Redis by default (the same instance
// docker-compose already runs for the game hub's own pub/sub), or wherever
// ZOLIK_TEST_REDIS_URL says. Any environment without a reachable Redis skips
// the tests that need one — see newRedisBackedStore.
func testRedisURL() string {
	if v := strings.TrimSpace(os.Getenv("ZOLIK_TEST_REDIS_URL")); v != "" {
		return v
	}
	return "redis://127.0.0.1:6379/0"
}

// newRedisBackedStore builds a Store against the real dev Redis and cleans up
// only the one hash key this package owns afterwards — never a broader flush,
// since that instance may be shared with other running services.
//
// Returns the concrete *redisStore, not the Store interface: a couple of
// tests below reach into the unexported redis field directly, to seed or
// inspect a record without going through the package's own read/write path
// — that white-box access needs the concrete type.
func newRedisBackedStore(t *testing.T) *redisStore {
	t.Helper()
	s, err := NewStore(testRedisURL())
	if err != nil {
		t.Skipf("no reachable redis at %s (set ZOLIK_TEST_REDIS_URL, or start the dev compose stack): %v",
			testRedisURL(), err)
	}
	rs := s.(*redisStore)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = rs.redis.Del(ctx, redisKey).Err()
		_ = rs.Close()
	})
	return rs
}

func TestLocalOnlyStoreTracksPresenceWithNoRedis(t *testing.T) {
	s, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore(\"\"): %v", err)
	}
	ctx := context.Background()

	if got := s.List(ctx); len(got) != 0 {
		t.Fatalf("List on an empty store = %v, want empty", got)
	}

	s.Join(ctx, Entry{PlayerID: "p1", Username: "Alice", IsGuest: true, Avatar: "p-violet"})
	s.Join(ctx, Entry{PlayerID: "p2", Username: "Bob"})

	list := s.List(ctx)
	if len(list) != 2 {
		t.Fatalf("List = %v, want 2 entries", list)
	}
	if list[0].PlayerID != "p1" || list[1].PlayerID != "p2" {
		t.Errorf("List order = %v, want join order (oldest first)", list)
	}
	if !list[0].IsGuest || list[1].IsGuest {
		t.Errorf("IsGuest not preserved: %+v", list)
	}

	name, isGuest, avatar, ok := s.IsWaiting(ctx, "p1")
	if !ok || name != "Alice" || !isGuest {
		t.Errorf("IsWaiting(p1) = (%q, %v, %v), want (Alice, true, true)", name, isGuest, ok)
	}
	// The face they are waiting under travels with them, so an invite seats
	// the person the host was looking at rather than a face derived from an id.
	if avatar != "p-violet" {
		t.Errorf("IsWaiting(p1) avatar = %q, want p-violet", avatar)
	}
	if _, _, _, ok := s.IsWaiting(ctx, "nobody"); ok {
		t.Error("IsWaiting reported a player who was never joined")
	}

	s.Leave(ctx, "p1")
	if _, _, _, ok := s.IsWaiting(ctx, "p1"); ok {
		t.Error("a player who left is still reported waiting")
	}
	if got := s.List(ctx); len(got) != 1 || got[0].PlayerID != "p2" {
		t.Errorf("List after Leave = %v, want just p2", got)
	}
}

func TestJoinResetsJoinedAtOnReconnect(t *testing.T) {
	s, _ := NewStore("")
	ctx := context.Background()

	s.Join(ctx, Entry{PlayerID: "p1", Username: "Alice"})
	first := s.List(ctx)[0].JoinedAt

	time.Sleep(2 * time.Millisecond)
	s.Join(ctx, Entry{PlayerID: "p1", Username: "Alice"})
	second := s.List(ctx)[0].JoinedAt

	if !second.After(first) {
		t.Errorf("JoinedAt did not advance on reconnect: first=%v second=%v", first, second)
	}
}

func TestPickupReportsWhetherThePlayerWasActuallyPresent(t *testing.T) {
	s, _ := NewStore("")
	ctx := context.Background()
	s.Join(ctx, Entry{PlayerID: "p1", Username: "Alice"})

	if !s.Pickup(ctx, "p1") {
		t.Error("Pickup(p1) = false, want true — they were waiting")
	}
	if s.Pickup(ctx, "p1") {
		t.Error("a second Pickup of the same player reported true — they already left")
	}
	if s.Pickup(ctx, "never-here") {
		t.Error("Pickup reported true for a player who was never in the pool")
	}
	if _, _, _, ok := s.IsWaiting(ctx, "p1"); ok {
		t.Error("a picked-up player is still reported waiting — they must not be invited twice")
	}
}

// listContains reports whether a player appears in a pool snapshot.
func listContains(entries []Entry, playerID string) bool {
	for _, e := range entries {
		if e.PlayerID == playerID {
			return true
		}
	}
	return false
}

// TestRedisMirroringMakesPresenceVisibleAcrossInstances is the property the
// whole Redis-backed half of Store exists for: two Store values that never
// talk to each other directly — modelling two server instances — must still
// agree on who is waiting, because they share the same Redis.
func TestRedisMirroringMakesPresenceVisibleAcrossInstances(t *testing.T) {
	instanceA := newRedisBackedStore(t)
	instanceB, err := NewStore(testRedisURL())
	if err != nil {
		t.Fatalf("building the second instance: %v", err)
	}
	t.Cleanup(func() { _ = instanceB.Close() })

	ctx := context.Background()
	instanceA.Join(ctx, Entry{PlayerID: "cross-1", Username: "Alice", Avatar: "m-brass"})

	// instanceB has no local connection for this player at all — the only
	// way it can know about them is via Redis.
	name, _, avatar, ok := instanceB.IsWaiting(ctx, "cross-1")
	if !ok || name != "Alice" {
		t.Fatalf("instanceB.IsWaiting(cross-1) = (%q, %v), want (Alice, true)", name, ok)
	}
	// Redis is the only path between these two instances, so this is also the
	// assertion that the face survives being serialised into the shared hash.
	if avatar != "m-brass" {
		t.Errorf("instanceB.IsWaiting(cross-1) avatar = %q, want m-brass", avatar)
	}
	// Present in the list, rather than the *only* thing in it.
	//
	// The store deliberately owns one shared Redis hash so that every instance
	// sees one pool — which means a developer's own running server, pointed at
	// the same dev Redis, legitimately appears here too. Asserting the pool
	// held exactly one entry made this test pass or fail on whether anything
	// else happened to be running, which is a property of the machine and not
	// of the code.
	if !listContains(instanceB.List(ctx), "cross-1") {
		t.Fatalf("instanceB.List() = %v, want it to contain cross-1", instanceB.List(ctx))
	}

	// Picking up from instanceB (the host's request landed on a different
	// pod than the waiting player's own connection) must still remove them
	// everywhere.
	if !instanceB.Pickup(ctx, "cross-1") {
		t.Error("instanceB.Pickup(cross-1) = false, want true")
	}
	if _, _, _, ok := instanceA.IsWaiting(ctx, "cross-1"); ok {
		t.Error("instanceA still reports the player waiting after instanceB picked them up")
	}
}

func TestStaleRedisEntriesAreFilteredOut(t *testing.T) {
	s := newRedisBackedStore(t)
	ctx := context.Background()

	// Write a record directly, bypassing Join, with a heartbeat old enough
	// to be stale — standing in for an instance that crashed without a
	// clean disconnect and therefore never called Leave.
	stale := redisRecord{
		Entry:    Entry{PlayerID: "ghost", Username: "Ghost", JoinedAt: time.Now().UTC().Add(-time.Hour)},
		LastSeen: time.Now().UTC().Add(-staleAfter - time.Minute),
	}
	raw, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if err := s.redis.HSet(ctx, redisKey, "ghost", raw).Err(); err != nil {
		t.Fatalf("seeding a stale record: %v", err)
	}

	if _, _, _, ok := s.IsWaiting(ctx, "ghost"); ok {
		t.Error("IsWaiting reported a stale entry as present")
	}
	for _, e := range s.List(ctx) {
		if e.PlayerID == "ghost" {
			t.Fatalf("List included a stale entry: %+v", e)
		}
	}
}

func TestHeartbeatKeepsARedisEntryFresh(t *testing.T) {
	s := newRedisBackedStore(t)
	ctx := context.Background()
	s.Join(ctx, Entry{PlayerID: "beats", Username: "Beats"})

	s.Heartbeat(ctx, "beats")

	raw, err := s.redis.HGet(ctx, redisKey, "beats").Result()
	if err != nil {
		t.Fatalf("reading back the heartbeat: %v", err)
	}
	var rec redisRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if time.Since(rec.LastSeen) > 5*time.Second {
		t.Errorf("lastSeen = %v, want a timestamp from just now", rec.LastSeen)
	}
}

func TestHeartbeatForAPlayerNotHeldLocallyIsANoOp(t *testing.T) {
	// A stray heartbeat for someone this instance never registered (e.g. a
	// racing disconnect) must not resurrect a stale or foreign entry.
	s, _ := NewStore("")
	s.Heartbeat(context.Background(), "nobody")
	if got := s.List(context.Background()); len(got) != 0 {
		t.Errorf("List = %v, want empty — heartbeat must not create an entry", got)
	}
}
