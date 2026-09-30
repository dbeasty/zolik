package match_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"zolik/server/internal/match"
	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// abandonedMatch writes a match in the state the reaper leaves behind: a
// resolved status, an end time, and the module's own state untouched.
//
// `others` are the seats beside p1, so one test can build the bots-only table
// this feature is for and another the mixed one it must refuse.
func abandonedMatch(t *testing.T, repo match.Repository, others ...models.Player) models.Match {
	t.Helper()
	ended := time.Now().UTC().Add(-time.Hour)
	players := append([]models.Player{{ID: "p1", Name: "Ada"}}, others...)
	order := make([]string, 0, len(players))
	for _, p := range players {
		order = append(order, p.ID)
	}
	m, err := repo.Insert(context.Background(), models.Match{
		ModuleID:  "prsi",
		Status:    "abandoned",
		Players:   players,
		TurnOrder: order,
		HostID:    "p1",
		EndedAt:   &ended,
		CreatedAt: ended,
	})
	if err != nil {
		t.Fatalf("inserting an abandoned match: %v", err)
	}
	return m
}

func bot(id string) models.Player { return models.Player{ID: id, Name: id, IsAI: true} }

// The bug this closes: a table swept up while its only human was away came
// back as a board with every control refused and no way out — the reaper
// resolved it for the statistics and left the player stranded.
func TestResumeBringsBackABotsOnlyTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, bot("bot:a"), bot("bot:b"))

	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("resuming: %v", err)
	}

	got, err := repo.FindByID(ctx, saved.ID)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
	// A live match with an end time on it would read as finished to everything
	// downstream that asks "is this over" by looking for one.
	if got.EndedAt != nil {
		t.Errorf("EndedAt = %v, want nil on a resumed table", got.EndedAt)
	}
	if got.AbandonAt != nil || got.SuspendedAt != nil || got.SuspendedPlayer != "" {
		t.Errorf("suspension fields survived the resume: %+v", got)
	}
}

// The abandonment stands and the resumption is recorded beside it, because the
// table really was abandoned at the time the sweeper said so. completionRate
// nets the two rather than either counter being retracted.
func TestResumeCountsItselfWithoutRetractingTheAbandonment(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)
	saved := abandonedMatch(t, repo, bot("bot:a"))

	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("resuming: %v", err)
	}
	if got := sink.counts[metrics.MatchesResumed]; got != 1 {
		t.Errorf("%s = %d, want 1", metrics.MatchesResumed, got)
	}
	if got := sink.counts[metrics.MatchesResumedFor("prsi")]; got != 1 {
		t.Errorf("per-module resumed = %d, want 1", got)
	}
	if got := sink.counts[metrics.MatchesAbandoned]; got != 0 {
		t.Errorf("resume decremented the abandonment counter (%d); it should stand", got)
	}
}

// A player who is away was not asked. Bringing the table back for them would
// restart a game they have counted as over — possibly hours ago, possibly on
// a phone in a pocket — and that is not one player's decision to make.
func TestResumeRefusesATableWhoseOtherPlayersAreAway(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, models.Player{ID: "p2", Name: "Bo"}, bot("bot:a"))
	sitDown(t, m, saved.ID.Hex(), "p1") // p1 is looking at it; Bo is not.

	err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1")
	if module.CodeOf(err) != "TABLE_HAS_PLAYERS_AWAY" {
		t.Fatalf("code = %q, want TABLE_HAS_PLAYERS_AWAY (err %v)", module.CodeOf(err), err)
	}
	// Who, not just no: it is what turns the refusal into something the
	// player can act on.
	if !strings.Contains(err.Error(), "p2") {
		t.Errorf("refusal %q does not name the missing player", err)
	}
	got, err := repo.FindByID(ctx, saved.ID)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got.Status != "abandoned" {
		t.Errorf("status = %q, want the refusal to have left it abandoned", got.Status)
	}
}

// The bug this closes, and the reason the rule above is about absence rather
// than about bots.
//
// Two friends played a game. One of them lost their connection for two and a
// half minutes, the sweeper resolved the table, and from then on it could not
// be brought back by anybody — not by the player who had dropped, and not by
// the one who had never left and had watched it happen from an open tab. The
// hands, the pile and the score were all still on the server. Both of them
// were looking at the board. The only offer either screen made was "Back to
// games".
//
// Nobody is surprised by this resume: everyone it concerns is at the table.
func TestResumeBringsBackAHumanTableWhenEverybodyIsBack(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, models.Player{ID: "p2", Name: "Bo"}, bot("bot:a"))
	sitDown(t, m, saved.ID.Hex(), "p1")
	sitDown(t, m, saved.ID.Hex(), "p2")

	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("resuming a table both players are sitting at: %v", err)
	}
	got, err := repo.FindByID(ctx, saved.ID)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
	if got.EndedAt != nil {
		t.Errorf("EndedAt = %v, want nil on a resumed table", got.EndedAt)
	}
}

// Either of them may press it. The player who stayed has at least as much
// claim on the game as the one who dropped, and a rule that let only the
// returning player resume would stall on whoever happened to press first.
func TestEitherPlayerMayResumeOnceBothAreBack(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, models.Player{ID: "p2", Name: "Bo"})
	sitDown(t, m, saved.ID.Hex(), "p1")
	sitDown(t, m, saved.ID.Hex(), "p2")

	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p2"); err != nil {
		t.Fatalf("resuming as the second player: %v", err)
	}
	got, _ := repo.FindByID(ctx, saved.ID)
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
}

// And the table goes back to being unresumable the moment somebody leaves
// again, rather than staying open because it once was.
func TestResumeRefusesAgainAfterSomebodyLeaves(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, models.Player{ID: "p2", Name: "Bo"})
	sitDown(t, m, saved.ID.Hex(), "p1")
	leave := sitDown(t, m, saved.ID.Hex(), "p2")
	leave()

	err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1")
	if module.CodeOf(err) != "TABLE_HAS_PLAYERS_AWAY" {
		t.Fatalf("code = %q, want TABLE_HAS_PLAYERS_AWAY (err %v)", module.CodeOf(err), err)
	}
}

// A table of bots needs nobody else to be anywhere: the original feature,
// unchanged, and the case the old rule was written around.
func TestResumeStillNeedsNobodyPresentAtABotsOnlyTable(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, bot("bot:a"), bot("bot:b"))

	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("resuming a bots-only table with no socket open: %v", err)
	}
}

// "Abandoned" is not an invitation: a table is not a public resource just
// because nobody came back to it.
func TestResumeRefusesSomebodyWhoWasNotThere(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, bot("bot:a"))

	err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "stranger")
	if module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
		t.Fatalf("code = %q, want NOT_AT_THIS_TABLE (err %v)", module.CodeOf(err), err)
	}
}

// Resume undoes the sweeper and nothing else. A completed match is over
// because it was played to the end, and this must never reopen one.
func TestResumeRefusesAMatchThatIsNotAbandoned(t *testing.T) {
	ctx := t.Context()
	m, repo, _ := newReaperHarness(t)
	saved := abandonedMatch(t, repo, bot("bot:a"))

	saved.Status = "completed"
	if err := repo.UpdateWithVersion(ctx, saved.ID, saved.Version, saved); err != nil {
		t.Fatalf("marking completed: %v", err)
	}

	err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1")
	if module.CodeOf(err) != "MATCH_NOT_ABANDONED" {
		t.Fatalf("code = %q, want MATCH_NOT_ABANDONED (err %v)", module.CodeOf(err), err)
	}
}

// A double press — two sockets, or one impatient player on a slow link —
// resumes once. The version check is what decides it.
func TestResumeTwiceResumesOnce(t *testing.T) {
	ctx := t.Context()
	m, repo, sink := newReaperHarness(t)
	saved := abandonedMatch(t, repo, bot("bot:a"))

	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1"); err != nil {
		t.Fatalf("first resume: %v", err)
	}
	// The second finds it active rather than abandoned, which is the same
	// refusal a stale client would get.
	if err := m.ResumeAbandoned(ctx, saved.ID.Hex(), "p1"); module.CodeOf(err) != "MATCH_NOT_ABANDONED" {
		t.Fatalf("second resume: code = %q, want MATCH_NOT_ABANDONED", module.CodeOf(err))
	}
	if got := sink.counts[metrics.MatchesResumed]; got != 1 {
		t.Errorf("%s = %d, want 1", metrics.MatchesResumed, got)
	}
}
