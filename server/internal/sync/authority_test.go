package sync

import (
	"encoding/json"
	"testing"
)

// settleBoth runs a merge with the sides given both ways round. A rule that
// answers differently depending on which side it was handed would have two
// nodes settling the same conflict differently, and nothing after that
// converges, so every rule is tested for it rather than trusted.
func settleBoth(t *testing.T, base, local, incoming string) string {
	t.Helper()
	var basePtr string
	if base != "" {
		basePtr = base
	}
	one, err := settle(basePtr, &local, &incoming)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	other, err := settle(basePtr, &incoming, &local)
	if err != nil {
		t.Fatalf("settle reversed: %v", err)
	}
	if string(one) != string(other) {
		t.Fatalf("not symmetric:\n  %s\n  %s", one, other)
	}
	return string(one)
}

func field(t *testing.T, body, name string) string {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return string(doc[name])
}

func TestPreferencesKeepTheLastWrite(t *testing.T) {
	base := `{"_kind":"prefs","cardStyle":"classic","updatedAt":"2026-09-01T10:00:00Z"}`
	local := `{"_kind":"prefs","cardStyle":"vector","updatedAt":"2026-09-01T11:00:00Z"}`
	incoming := `{"_kind":"prefs","cardStyle":"large","updatedAt":"2026-09-01T12:00:00Z"}`
	got := settleBoth(t, base, local, incoming)
	if style := field(t, got, "cardStyle"); style != `"large"` {
		t.Fatalf("cardStyle = %s, want the last write", style)
	}
}

func TestPreferencesWithoutTimesMergeFieldByField(t *testing.T) {
	// Each device changed a different setting. Keeping one document whole
	// would silently undo the other's change.
	base := `{"_kind":"prefs","lang":"cs","cardStyle":"classic"}`
	local := `{"_kind":"prefs","lang":"en","cardStyle":"classic"}`
	incoming := `{"_kind":"prefs","lang":"cs","cardStyle":"vector"}`
	got := settleBoth(t, base, local, incoming)
	if field(t, got, "lang") != `"en"` {
		t.Fatalf("lang = %s, want \"en\"", field(t, got, "lang"))
	}
	if field(t, got, "cardStyle") != `"vector"` {
		t.Fatalf("cardStyle = %s, want \"vector\"", field(t, got, "cardStyle"))
	}
}

func TestScoringSessionUnionsItsRounds(t *testing.T) {
	base := `{"_kind":"scoring","rounds":[{"id":"r1","score":10}]}`
	local := `{"_kind":"scoring","updatedAt":"2026-09-01T11:00:00Z","rounds":[{"id":"r1","score":10},{"id":"r2","score":20}]}`
	incoming := `{"_kind":"scoring","updatedAt":"2026-09-01T10:00:00Z","rounds":[{"id":"r1","score":10},{"id":"r3","score":30}]}`
	got := settleBoth(t, base, local, incoming)

	var doc struct {
		Rounds []struct {
			ID    string `json:"id"`
			Score int    `json:"score"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(doc.Rounds) != 3 {
		t.Fatalf("rounds = %v, want all three kept", doc.Rounds)
	}
	seen := map[string]bool{}
	for _, r := range doc.Rounds {
		seen[r.ID] = true
	}
	for _, id := range []string{"r1", "r2", "r3"} {
		if !seen[id] {
			t.Fatalf("round %s was dropped: %s", id, got)
		}
	}
}

func TestAClosedScorepadStaysClosed(t *testing.T) {
	base := `{"_kind":"scoring","closed":false,"rounds":[]}`
	local := `{"_kind":"scoring","closed":true,"updatedAt":"2026-09-01T10:00:00Z","rounds":[]}`
	incoming := `{"_kind":"scoring","closed":false,"updatedAt":"2026-09-01T12:00:00Z","rounds":[]}`
	got := settleBoth(t, base, local, incoming)
	if field(t, got, "closed") != "true" {
		t.Fatalf("closed = %s, want true even though the later write says otherwise", field(t, got, "closed"))
	}
}

func TestACircleEdgePrefersTheAccountOverTheGuestItWas(t *testing.T) {
	base := `{"_kind":"circle","memberKey":"guest:abcd","updatedAt":"2026-09-01T10:00:00Z"}`
	local := `{"_kind":"circle","memberKey":"guest:abcd","updatedAt":"2026-09-01T12:00:00Z"}`
	incoming := `{"_kind":"circle","memberKey":"user:65f0","updatedAt":"2026-09-01T11:00:00Z"}`
	got := settleBoth(t, base, local, incoming)
	if field(t, got, "memberKey") != `"user:65f0"` {
		t.Fatalf("memberKey = %s, want the account", field(t, got, "memberKey"))
	}
}

func TestRemovingACircleEntryBeatsAnOlderEdit(t *testing.T) {
	base := `{"_kind":"circle","memberKey":"user:65f0","updatedAt":"2026-09-01T10:00:00Z"}`
	removed := `{"_kind":"circle","memberKey":"user:65f0","removedAt":"2026-09-01T12:00:00Z"}`
	edited := `{"_kind":"circle","memberKey":"user:65f0","updatedAt":"2026-09-01T11:00:00Z"}`
	got := settleBoth(t, base, removed, edited)
	if field(t, got, "removedAt") == "" {
		t.Fatalf("the removal was lost: %s", got)
	}

	// And the other way round: an edit made after the removal means the person
	// added them back.
	edited = `{"_kind":"circle","memberKey":"user:65f0","updatedAt":"2026-09-01T13:00:00Z"}`
	got = settleBoth(t, base, removed, edited)
	if field(t, got, "removedAt") != "" {
		t.Fatalf("a later edit did not beat the removal: %s", got)
	}
}

func TestADocumentWithNoRuleIsLeftAlone(t *testing.T) {
	local := `{"_kind":"something-new","v":1}`
	incoming := `{"_kind":"something-new","v":2}`
	got, err := settle("", &local, &incoming)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got != nil {
		t.Fatalf("got %s, want the conflict left queued for a human", got)
	}
}

func TestADeletionAgainstAnEditIsLeftQueued(t *testing.T) {
	local := `{"_kind":"prefs","lang":"en"}`
	got, err := settle("", &local, nil)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got != nil {
		t.Fatalf("got %s, want the conflict left queued", got)
	}
}

func TestExtendedJSONDatesAreUnderstood(t *testing.T) {
	// Documents are stored as the bson extended JSON the rest of the server
	// speaks, so a time arrives as {"$date":...} rather than as a string.
	base := `{"_kind":"prefs","lang":"cs"}`
	local := `{"_kind":"prefs","lang":"en","updatedAt":{"$date":"2026-09-01T11:00:00Z"}}`
	incoming := `{"_kind":"prefs","lang":"de","updatedAt":{"$date":{"$numberLong":"1756720800000"}}}`
	got := settleBoth(t, base, local, incoming)
	if field(t, got, "lang") == "" {
		t.Fatalf("no language survived: %s", got)
	}
}
