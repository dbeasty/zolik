package match

import (
	"fmt"
	"testing"
	"time"

	"zolik/server/internal/models"
)

func action(seq int) models.MatchAction {
	return models.MatchAction{
		Seq: seq, PlayerID: "p1",
		Action: models.JSONDoc(fmt.Sprintf(`{"verb":"draw","n":%d}`, seq)),
		At:     time.UnixMilli(1789950000000 + int64(seq)).UTC(),
	}
}

func actions(n int) []models.MatchAction {
	out := make([]models.MatchAction, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, action(i))
	}
	return out
}

// A page holds seqs page*32+1 … page*32+32, and the boundary is where an
// off-by-one would hide.
func TestPageOfBoundaries(t *testing.T) {
	for seq, want := range map[int]int{1: 0, 32: 0, 33: 1, 64: 1, 65: 2} {
		if got := pageOf(seq); got != want {
			t.Errorf("pageOf(%d) = %d, want %d", seq, got, want)
		}
	}
}

// An entry past the match's count — an append whose match write never landed
// — is overwritten by the next append to that slot, not appended after.
func TestAppendToPageOverwritesAnUnrecordedSlot(t *testing.T) {
	p := logPage{Match: "m", Page: 0}
	for _, a := range actions(3) {
		p = appendToPage(p, a)
	}
	stray := action(4)
	stray.PlayerID = "never-recorded"
	p = appendToPage(p, stray)

	p = appendToPage(p, action(4))
	if len(p.Entries) != 4 {
		t.Fatalf("page holds %d entries, want 4", len(p.Entries))
	}
	if p.Entries[3].Player != "p1" {
		t.Errorf("slot 4 still holds the unrecorded entry")
	}
}

func TestPagesForSplitsAWholeLog(t *testing.T) {
	pages := pagesFor("m", actions(70))
	if len(pages) != 3 {
		t.Fatalf("70 actions made %d pages, want 3", len(pages))
	}
	for i, want := range []int{32, 32, 6} {
		if pages[i].Page != i || len(pages[i].Entries) != want {
			t.Errorf("page %d: number %d, %d entries, want %d", i, pages[i].Page, len(pages[i].Entries), want)
		}
	}
	if first := pages[1].Entries[0].Seq; first != 33 {
		t.Errorf("page 1 starts at seq %d, want 33", first)
	}
}

// A page entry is the action it was made from, bar sub-millisecond time —
// the precision the original layout's dates had anyway.
func TestPageEntryRoundTrip(t *testing.T) {
	a := action(7)
	if got := fromPageEntry(toPageEntry(a)); got.Seq != a.Seq || got.PlayerID != a.PlayerID ||
		string(got.Action) != string(a.Action) || !got.At.Equal(a.At) {
		t.Errorf("round trip changed the action: %+v -> %+v", a, got)
	}
}

func TestSliceLogStopsAtAGap(t *testing.T) {
	log := actions(10)
	if got := sliceLog(log, 3, 7); len(got) != 4 || got[0].Seq != 4 || got[3].Seq != 7 {
		t.Errorf("sliceLog(3,7) = %d entries from %v", len(got), got)
	}
	holed := append(append([]models.MatchAction(nil), log[:5]...), log[6:]...)
	if got := sliceLog(holed, 0, 10); len(got) != 5 {
		t.Errorf("a log missing seq 6 answered %d entries, want the 5 before the gap", len(got))
	}
	if got := sliceLog(log, 0, 50); len(got) != 10 {
		t.Errorf("asking past the end gave %d, want all 10", len(got))
	}
}

// settle is what makes the scans safe to hand to a writer: the count survives
// even though the inline log is dropped, and a stored board is still known
// to be there.
func TestSettleKeepsTheCountAndTheBoards(t *testing.T) {
	legacy := models.Match{
		LegacyActionLog: actions(5),
		Checkpoints: []models.MatchCheckpoint{
			{Seq: 2, Round: 1},
			{Seq: 5, Round: 2, LegacyState: models.JSONDoc(`{"board":true}`)},
		},
	}
	got := settle(legacy)
	if got.ActionCount != 5 || got.LegacyActionLog != nil {
		t.Errorf("legacy: count %d, inline log kept %v", got.ActionCount, got.LegacyActionLog != nil)
	}
	if got.Checkpoints[0].Board || !got.Checkpoints[1].Board || got.Checkpoints[1].LegacyState != nil {
		t.Errorf("legacy checkpoints settled wrong: %+v", got.Checkpoints)
	}
	if legacy.Checkpoints[1].LegacyState == nil {
		t.Error("settle modified the stored match it was given")
	}

	pages := settle(models.Match{LogFormat: models.LogFormatPages, ActionCount: 40})
	if pages.ActionCount != 40 {
		t.Errorf("pages: count %d, want 40", pages.ActionCount)
	}
}

// actionsIn is what BoardAfter searches on, so it must not step backwards at
// the version that moved a match from the original layout to pages.
func TestActionsInIsMonotonicAcrossTheLayoutSwitch(t *testing.T) {
	before := models.Match{LegacyActionLog: actions(12)}
	moved := models.Match{LogFormat: models.LogFormatPages, ActionCount: 12}
	after := models.Match{LogFormat: models.LogFormatPages, ActionCount: 13}
	if a, b, c := actionsIn(before), actionsIn(moved), actionsIn(after); !(a <= b && b <= c) || a != 12 {
		t.Errorf("actionsIn across the switch: %d, %d, %d", a, b, c)
	}
}
