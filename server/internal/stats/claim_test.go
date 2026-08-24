package stats

import (
	"testing"
)

// A guest with a device id keeps a findable record of their games but still
// earns no lifetime aggregate. Both halves matter: the first is what makes
// signing in later worth anything, the second is the rule that keeps a shared
// device off the leaderboard.
func TestGuestWithADeviceIDIsFindableButNotDurable(t *testing.T) {
	s := Subject{Kind: SubjectGuest, ID: "abc123"}

	if got := s.Key(); got != "guest:abc123" {
		t.Errorf("Key() = %q, want guest:abc123 — without a key the games cannot be found again", got)
	}
	if s.Durable() {
		t.Error("a guest was reported durable; that would put a per-device identity on the leaderboard")
	}
	if !s.IsHuman() {
		t.Error("a guest must still count as a person for everyone else's vs-humans split")
	}
}

func TestGuestWithoutADeviceIDHasNoKeyAtAll(t *testing.T) {
	// Seats played before guest ids existed. They simply stay unclaimable,
	// exactly as they were before.
	s := Subject{Kind: SubjectGuest, Name: "Guest"}
	if s.Key() != "" {
		t.Errorf("Key() = %q, want empty", s.Key())
	}
	if s.Durable() {
		t.Error("Durable() = true for an identity-less guest")
	}
}

func TestSubjectKeyRoundTripIncludesGuests(t *testing.T) {
	s := Subject{Kind: SubjectGuest, ID: "device-1"}
	back, err := ParseSubjectKey(s.Key())
	if err != nil {
		t.Fatalf("ParseSubjectKey(%q): %v", s.Key(), err)
	}
	if back.Kind != SubjectGuest || back.ID != "device-1" {
		t.Errorf("round trip gave %+v, want a guest with id device-1", back)
	}
}

// The claim itself: a guest's seat becomes the account's seat, and the match's
// indexed key list follows.
func TestRewriteSubjectMovesASeatToTheAccount(t *testing.T) {
	m := recordMatch(t, []string{"guest:device-1", "ai:hard"}, [][]int{{5}, {40}})

	if !contains(m.SubjectKeys, "guest:device-1") {
		t.Fatalf("subjectKeys = %v, want the guest's key so the match can be found", m.SubjectKeys)
	}

	user := Subject{Kind: SubjectUser, ID: "65ab00000000000000000001", Name: "New Account"}
	if !rewriteSubject(&m, "guest:device-1", user) {
		t.Fatal("rewriteSubject reported no change")
	}

	if contains(m.SubjectKeys, "guest:device-1") {
		t.Errorf("subjectKeys = %v, still lists the guest after the claim", m.SubjectKeys)
	}
	if !contains(m.SubjectKeys, user.Key()) {
		t.Errorf("subjectKeys = %v, want the account's key", m.SubjectKeys)
	}

	seat := seatFor(t, m, user.Key())
	if seat.Subject.Kind != SubjectUser {
		t.Errorf("seat kind = %q, want user", seat.Subject.Kind)
	}
	// The standings are a snapshot of that evening's table — rewriting the
	// name shown to the other players would be rewriting history.
	if seat.Subject.Name != "guest:device-1" {
		t.Errorf("seat name = %q, want the name the game was played under", seat.Subject.Name)
	}
}

func TestRewriteSubjectLeavesOtherSeatsAlone(t *testing.T) {
	m := recordMatch(t, []string{"guest:device-1", "guest:device-2", "ai:easy"},
		[][]int{{5}, {20}, {40}})

	user := Subject{Kind: SubjectUser, ID: "65ab00000000000000000001"}
	rewriteSubject(&m, "guest:device-1", user)

	if !contains(m.SubjectKeys, "guest:device-2") {
		t.Errorf("subjectKeys = %v, the other guest's key was lost", m.SubjectKeys)
	}
	if !contains(m.SubjectKeys, "ai:easy") {
		t.Errorf("subjectKeys = %v, the bot's key was lost", m.SubjectKeys)
	}
	if len(m.Participants) != 3 {
		t.Errorf("participants = %d, want 3", len(m.Participants))
	}
}

func TestRewriteSubjectIsANoOpWhenTheSeatIsAbsent(t *testing.T) {
	m := recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{5}, {40}})
	before := append([]string(nil), m.SubjectKeys...)

	if rewriteSubject(&m, "guest:not-at-this-table", Subject{Kind: SubjectUser, ID: "x"}) {
		t.Error("rewriteSubject reported a change it did not make")
	}
	if len(m.SubjectKeys) != len(before) {
		t.Errorf("subjectKeys changed from %v to %v", before, m.SubjectKeys)
	}
}

// Rebuilding from the match records has to produce exactly what folding them
// in live produced, or a claim would quietly change somebody's history. This
// checks the fold itself is order-faithful, which is the property the rebuild
// depends on.
func TestReplayingMatchesInOrderReproducesTheLiveRecord(t *testing.T) {
	me := Subject{Kind: SubjectUser, ID: "me"}

	matches := []MatchResult{
		recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{5}, {40}}),  // win
		recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{50}, {10}}), // loss
		recordMatch(t, []string{"user:me", "ai:easy"}, [][]int{{5}, {60}}),  // win
	}

	// Folded in as the games finished.
	live := ZeroStats(me)
	for _, m := range matches {
		live = applyOne(t, live, m, me.Key())
	}

	// Rebuilt afterwards from the same records, oldest first.
	rebuilt := ZeroStats(me)
	for _, m := range matches {
		rebuilt = applyOne(t, rebuilt, m, me.Key())
	}

	if live.Overall != rebuilt.Overall {
		t.Errorf("overall differs\n  live:    %+v\n  rebuilt: %+v", live.Overall, rebuilt.Overall)
	}
	if live.CurrentStreak != rebuilt.CurrentStreak || live.LongestWinStreak != rebuilt.LongestWinStreak {
		t.Errorf("streaks differ: live %d/%d, rebuilt %d/%d",
			live.CurrentStreak, live.LongestWinStreak, rebuilt.CurrentStreak, rebuilt.LongestWinStreak)
	}
	if live.Overall.Wins != 2 {
		t.Errorf("wins = %d, want 2 — the fixture itself is wrong", live.Overall.Wins)
	}
}

// A claimed guest becomes a real opponent for everyone else, so the person who
// beat them gains a head-to-head row that did not exist while they were a
// guest. That is correct — the row now names somebody who owns it.
func TestClaimedGuestBecomesAHeadToHeadOpponent(t *testing.T) {
	m := recordMatch(t, []string{"user:me", "guest:device-1"}, [][]int{{5}, {40}})

	before := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, "user:me")
	if len(before.HeadToHead) != 0 {
		t.Errorf("head-to-head = %+v, want empty while the opponent is a guest", before.HeadToHead)
	}

	them := Subject{Kind: SubjectUser, ID: "65ab00000000000000000002"}
	rewriteSubject(&m, "guest:device-1", them)

	after := applyOne(t, ZeroStats(Subject{Kind: SubjectUser, ID: "me"}), m, "user:me")
	rec, ok := after.HeadToHead[them.Key()]
	if !ok {
		t.Fatalf("head-to-head = %+v, want a row against the claimed account", after.HeadToHead)
	}
	if rec.Matches != 1 || rec.Ahead != 1 {
		t.Errorf("head-to-head row = %+v, want 1 match and 1 ahead", rec)
	}
}

func TestSubjectKeysOfDropsBlanksAndDuplicates(t *testing.T) {
	got := subjectKeysOf([]Standing{
		{Subject: Subject{Kind: SubjectAI, ID: "hard"}},
		{Subject: Subject{Kind: SubjectAI, ID: "hard"}}, // one difficulty, two seats
		{Subject: Subject{Kind: SubjectGuest}},          // no device id
		{Subject: Subject{Kind: SubjectUser, ID: "me"}},
	})
	want := []string{"ai:hard", "user:me"}
	if len(got) != len(want) {
		t.Fatalf("subjectKeysOf = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("subjectKeysOf = %v, want %v (sorted, deduplicated)", got, want)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
