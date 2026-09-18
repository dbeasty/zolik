package match

import (
	"encoding/json"
	"strings"
	"testing"

	"zolik/server/internal/blackjack"
	"zolik/server/internal/canasta"
	"zolik/server/internal/ginrummy"
	"zolik/server/internal/holdem"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
	"zolik/server/internal/rummytiles"
	"zolik/server/internal/zolikmod"
)

// Replay's own tests. The load-bearing one is determinism: if a folded log does
// not reach the board the match was actually played to, every frame before it
// is fiction too, and no amount of screen polish saves the feature.

// replayable is one hosted game, played out and then replayed.
//
// A local copy of internal/module/allmodules_test.go's table rather than an
// import: that one lives in package module_test and cannot be reached from
// here. The prefer lists are copied from it, and the reasons they are what they
// are are recorded there.
type replayable struct {
	name    string
	mod     module.GameModule
	players []module.PlayerRef
	cfg     module.MatchConfig
	prefer  []string
	// volatile names dotted paths in this module's state that hold a wall
	// clock, and so cannot survive a replay — nor a second replay of
	// themselves. Listed per module rather than tolerated globally, so that
	// any *other* field which stops folding cleanly still fails the test.
	volatile []string
}

func replayables() []replayable {
	refs := func(ids ...string) []module.PlayerRef {
		out := make([]module.PlayerRef, 0, len(ids))
		for _, id := range ids {
			out = append(out, module.PlayerRef{ID: id, Name: id})
		}
		return out
	}
	return []replayable{
		// zolik wraps internal/rules, whose GameState.Created is a time.Time
		// set from the clock (rules/round.go) and serialised into the state.
		// A replay therefore re-deals at a different instant. Verified to be
		// the *only* field that moves: every card, pile and score folds byte
		// for byte. Removing it from the persisted state would let this entry
		// go, and is worth doing — but replay is correct either way, because
		// the board a player sees is identical.
		{"zolik", zolikmod.New(), refs("p1", "p2"), module.MatchConfig{},
			[]string{"draw", "discard"}, []string{"rules.Created"}},
		{"prsi", prsi.New(), refs("p1", "p2", "p3"), module.MatchConfig{},
			[]string{"play_card", "pass", "draw"}, nil},
		{"canasta", canasta.New(), refs("p1", "p2"),
			module.MatchConfig{Options: module.Options{"targetScore": 1500}},
			[]string{"lay_meld", "lay_off", "take_pile", "draw", "discard"}, nil},
		{"holdem", holdem.New(), refs("p1", "p2", "p3"),
			module.MatchConfig{Variation: "timed"},
			[]string{"call", "check", "raise", "fold"}, nil},
		{"ginrummy", ginrummy.New(), refs("p1", "p2"), module.MatchConfig{},
			[]string{"knock", "lay_off", "finish_layoff", "draw", "discard", "pass"}, nil},
		{"blackjack", blackjack.New(), refs("p1", "p2", "p3"), module.MatchConfig{},
			[]string{"bet", "decline_insurance", "stand", "hit"}, nil},
		{"rummytiles", rummytiles.New(), refs("p1", "p2"), module.MatchConfig{},
			[]string{"swap_joker", "commit", "reset_turn", "draw"}, nil},
	}
}

// playOut drives a match with the offer-only driver and returns the match a
// replay would be built from: the final state, and the log the runtime would
// have written on the way there.
//
// Capped well under MaxReplayFrames so a single BuildReplay call covers the
// whole game and the last frame really is the last frame.
const replayTestActions = 200

func playOut(t *testing.T, g replayable, seed int64) (models.Match, module.State) {
	t.Helper()
	start, err := g.mod.NewMatch(g.cfg, g.players, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}

	log := []models.MatchAction{}
	final, _, err := module.PlayWithOffers(g.mod, start, g.players, module.DriverOptions{
		MaxActions: replayTestActions,
		Prefer:     g.prefer,
		// Exactly the pair HandleAction logs, through exactly the same
		// constructor — so if logEntry ever became lossy, this test breaks
		// rather than the feature.
		OnAction: func(playerID string, a module.Action) {
			log = append(log, logEntry(len(log)+1, playerID, a))
		},
	})
	if err != nil {
		t.Fatalf("PlayWithOffers: %v", err)
	}
	if len(log) == 0 {
		t.Fatalf("driver made no moves, so there is nothing to replay")
	}

	players := make([]models.Player, 0, len(g.players))
	for _, p := range g.players {
		players = append(players, models.Player{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}
	return models.Match{
		ModuleID:  g.name,
		Variation: g.cfg.Variation,
		Options:   g.cfg.Options,
		Status:    "active",
		Players:   players,
		Seed:      seed,
		State:     final,
		ActionLog: log,
	}, final
}

func replayManager() *Manager {
	return &Manager{registry: module.NewRegistry(
		zolikmod.New(), prsi.New(), canasta.New(), holdem.New(),
		ginrummy.New(), blackjack.New(), rummytiles.New(),
	)}
}

// TestReplayReachesTheStateTheMatchWasPlayedTo is the whole feature in one
// assertion: fold the log back and you are standing on the same board.
func TestReplayReachesTheStateTheMatchWasPlayedTo(t *testing.T) {
	for _, g := range replayables() {
		t.Run(g.name, func(t *testing.T) {
			m := replayManager()
			match, final := playOut(t, g, 7)

			rep, err := m.BuildReplay(match, "p1", ReplayOptions{Limit: MaxReplayFrames})
			if err != nil {
				t.Fatalf("BuildReplay: %v", err)
			}
			if rep.Truncated {
				t.Fatalf("fold stopped at frame %d (%s)", rep.TruncatedAt, rep.TruncatedCode)
			}
			if want := len(match.ActionLog) + 1; rep.Total != want {
				t.Errorf("total = %d, want %d", rep.Total, want)
			}
			if len(rep.Frames) != rep.Total {
				t.Fatalf("got %d frames, want all %d", len(rep.Frames), rep.Total)
			}

			// The board the replay ends on must be the board the match is on.
			last := rep.Frames[len(rep.Frames)-1]
			wantView, err := g.mod.View(final, "p1")
			if err != nil {
				t.Fatalf("View: %v", err)
			}
			if a, b := mustJSON(t, last.View), mustJSON(t, wantView); a != b {
				t.Errorf("last frame's board is not the match's board\n got: %s\nwant: %s", a, b)
			}
			if a, b := mustJSON(t, last.Standings), mustJSON(t, module.StandingsFor(g.mod, final)); a != b {
				t.Errorf("last frame's standings differ\n got: %s\nwant: %s", a, b)
			}
			done, _, err := g.mod.Finished(final)
			if err != nil {
				t.Fatalf("Finished: %v", err)
			}
			wantStatus := "active"
			if done {
				wantStatus = "completed"
			}
			if last.Status != wantStatus {
				t.Errorf("last frame status = %q, want %q", last.Status, wantStatus)
			}

			// Frame 0 is the deal, and carries no move.
			if rep.Frames[0].Index != 0 || rep.Frames[0].PlayerID != "" || rep.Frames[0].Verb != "" {
				t.Errorf("frame 0 should be the deal, got %+v", rep.Frames[0])
			}
		})
	}
}

// TestReplayFoldsToTheSameBytes is the stronger form: not merely the same
// board, but the same state, byte for byte.
func TestReplayFoldsToTheSameBytes(t *testing.T) {
	for _, g := range replayables() {
		t.Run(g.name, func(t *testing.T) {
			match, final := playOut(t, g, 11)

			var folded module.State
			_, err := foldActions(g.mod, match, func(_ int, _ *models.MatchAction, _ module.Action, s module.State) (bool, error) {
				folded = s
				return true, nil
			})
			if err != nil {
				t.Fatalf("fold: %v", err)
			}
			got := strip(t, folded, g.volatile)
			want := strip(t, final, g.volatile)
			if got != want {
				t.Errorf("folded state differs from the played state\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// strip removes dotted paths from a state before comparing it, and re-encodes
// through a map so key order cannot be the thing that differs.
func strip(t *testing.T, s module.State, paths []string) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(s, &v); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	for _, p := range paths {
		parts := strings.Split(p, ".")
		cur := v
		for i, part := range parts {
			if i == len(parts)-1 {
				if _, ok := cur[part]; !ok {
					t.Fatalf("volatile path %q is not in the state; drop it from the table", p)
				}
				delete(cur, part)
				break
			}
			next, ok := cur[part].(map[string]any)
			if !ok {
				t.Fatalf("volatile path %q is not in the state; drop it from the table", p)
			}
			cur = next
		}
	}
	return mustJSON(t, v)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestReplayShowsExactlyWhatTheLiveBoardWould is the security test.
//
// It is not "a viewer never sees another hand", because that is not true and
// should not be: gin rummy lays the knocker's hand face up so the opponent can
// lay off against it, and blackjack turns the dealer over. What must hold is
// the narrower thing — a replay frame reveals neither more nor less than the
// live projection of that same position — and it must hold on every frame, not
// just the last, because a game passes through positions its final board no
// longer shows.
//
// Per-frame, and per module, so that a replay can never acquire a projection
// path of its own that has to be kept honest separately.
func TestReplayShowsExactlyWhatTheLiveBoardWould(t *testing.T) {
	for _, g := range replayables() {
		t.Run(g.name, func(t *testing.T) {
			m := replayManager()
			match, _ := playOut(t, g, 3)

			for _, viewer := range []string{"p1", "p2"} {
				rep, err := m.BuildReplay(match, viewer, ReplayOptions{Limit: MaxReplayFrames})
				if err != nil {
					t.Fatalf("BuildReplay: %v", err)
				}
				if rep.Open {
					t.Fatalf("an unfinished match was replayed with hands face up")
				}

				live := map[int]string{}
				if _, err := foldActions(g.mod, match, func(step int, _ *models.MatchAction, _ module.Action, s module.State) (bool, error) {
					vm, err := g.mod.View(s, viewer)
					if err != nil {
						return false, err
					}
					live[step] = mustJSON(t, vm)
					return true, nil
				}); err != nil {
					t.Fatalf("fold: %v", err)
				}

				for _, f := range rep.Frames {
					if got, want := mustJSON(t, f.View), live[f.Index]; got != want {
						t.Fatalf("frame %d is not the board %s would have seen\n got: %s\nwant: %s",
							f.Index, viewer, got, want)
					}
				}
			}
		})
	}
}

// TestReplayRefusesToOpenAnUnfinishedMatch pins the whole security story of
// the feature: hands come up only for a game that is over.
//
// A suspended, set-aside or still-running table can be resumed by somebody, so
// revealing its hidden cards — to anyone, the player asking included — would
// turn replay into a way to see what you are not entitled to.
func TestReplayRefusesToOpenAnUnfinishedMatch(t *testing.T) {
	g := replayables()[1] // prsi: shortest game that still deals hands
	for _, status := range []string{"active", "suspended", "abandoned", "lobby"} {
		t.Run(status, func(t *testing.T) {
			m := replayManager()
			match, _ := playOut(t, g, 3)
			match.Status = status

			rep, err := m.BuildReplay(match, "p1", ReplayOptions{Limit: 10, Open: true})
			if err != nil {
				t.Fatalf("BuildReplay: %v", err)
			}
			if rep.Open {
				t.Fatalf("a %q match was replayed with every hand face up", status)
			}
			for _, f := range rep.Frames {
				for _, z := range f.View.Zones {
					if z.Kind == module.ZoneHand && z.OwnerID != "" && z.OwnerID != "p1" && len(z.Cards) > 0 {
						t.Fatalf("frame %d leaked %s's hand on a %q match", f.Index, z.OwnerID, status)
					}
				}
			}
		})
	}
}

// TestReplayTruncatesWhenTheLogNoLongerFolds: a module whose rules have moved
// since a game was played refuses a move it once accepted. The frames before
// it are still exactly what happened, so they are still served.
func TestReplayTruncatesWhenTheLogNoLongerFolds(t *testing.T) {
	m := replayManager()
	g := replayables()[1] // prsi
	match, _ := playOut(t, g, 5)

	good := len(match.ActionLog)
	match.ActionLog = append(match.ActionLog, models.MatchAction{
		Seq: good + 1, PlayerID: "p1", Action: []byte(`{"verb":"nonsense"}`),
	})

	rep, err := m.BuildReplay(match, "p1", ReplayOptions{Limit: MaxReplayFrames})
	if err != nil {
		t.Fatalf("BuildReplay: %v", err)
	}
	if !rep.Truncated {
		t.Fatalf("a log that no longer folds was reported as complete")
	}
	if rep.TruncatedAt != good+1 {
		t.Errorf("truncatedAt = %d, want %d", rep.TruncatedAt, good+1)
	}
	if rep.TruncatedCode == "" {
		t.Errorf("truncation did not say why")
	}
	// Everything up to the bad step survived.
	if len(rep.Frames) != good+1 {
		t.Errorf("got %d frames, want the %d that still fold", len(rep.Frames), good+1)
	}
}

// TestReplayPagingMatchesOneFold pins the early-stop optimisation: a page is
// folded, not sliced, so it had better agree with folding the whole thing.
func TestReplayPagingMatchesOneFold(t *testing.T) {
	m := replayManager()
	g := replayables()[1] // prsi
	match, _ := playOut(t, g, 9)

	whole, err := m.BuildReplay(match, "p1", ReplayOptions{Limit: MaxReplayFrames})
	if err != nil {
		t.Fatalf("BuildReplay: %v", err)
	}

	var paged []ReplayFrame
	for from := 0; from < whole.Total; from += 7 {
		page, err := m.BuildReplay(match, "p1", ReplayOptions{From: from, Limit: 7})
		if err != nil {
			t.Fatalf("page at %d: %v", from, err)
		}
		if page.Total != whole.Total {
			t.Errorf("page at %d reports total %d, want %d", from, page.Total, whole.Total)
		}
		paged = append(paged, page.Frames...)
	}
	if a, b := mustJSON(t, paged), mustJSON(t, whole.Frames); a != b {
		t.Errorf("paged frames differ from one fold\n got: %s\nwant: %s", a, b)
	}
}

// TestReplayRefusesATableNobodyPlayed: a lobby has no game in it to step
// through, which is not an error the caller made.
func TestReplayRefusesATableNobodyPlayed(t *testing.T) {
	m := replayManager()
	_, err := m.BuildReplay(models.Match{ModuleID: "prsi", Status: "lobby"}, "p1", ReplayOptions{})
	if got := module.CodeOf(err); got != "NOTHING_TO_REPLAY" {
		t.Errorf("code = %q, want NOTHING_TO_REPLAY", got)
	}
}

// TestReplayFrameOmitsLegalActions: nobody is playing a replay, and offer
// enumeration is the most verbose thing a projection does.
func TestReplayFrameOmitsLegalActions(t *testing.T) {
	m := replayManager()
	g := replayables()[1]
	match, _ := playOut(t, g, 4)

	rep, err := m.BuildReplay(match, "p1", ReplayOptions{Limit: 5})
	if err != nil {
		t.Fatalf("BuildReplay: %v", err)
	}
	if strings.Contains(mustJSON(t, rep), "legalActions") {
		t.Errorf("a replay frame shipped offers")
	}
}

// TestReplayCarriesRoundBoundaries: the round log rides only on the deal and
// on the frames that closed a round, and a client carries it forward.
func TestReplayCarriesRoundBoundaries(t *testing.T) {
	m := replayManager()
	var g replayable
	for _, r := range replayables() {
		if r.name == "canasta" {
			g = r
		}
	}
	match, _ := playOut(t, g, 21)

	rep, err := m.BuildReplay(match, "p1", ReplayOptions{Limit: MaxReplayFrames})
	if err != nil {
		t.Fatalf("BuildReplay: %v", err)
	}
	if rep.Frames[0].Rounds == nil {
		t.Errorf("the deal did not carry the round log")
	}
	for _, f := range rep.Frames[1:] {
		if f.RoundEnded && f.Rounds == nil {
			t.Errorf("frame %d ended a round but carried no round log", f.Index)
		}
		if !f.RoundEnded && f.Rounds != nil {
			t.Errorf("frame %d repeated the round log for nothing", f.Index)
		}
	}
	// Prší keeps no rounds, and says so by absence rather than by an empty log.
	prsiMatch, _ := playOut(t, replayables()[1], 4)
	prsiRep, err := m.BuildReplay(prsiMatch, "p1", ReplayOptions{Limit: 5})
	if err != nil {
		t.Fatalf("BuildReplay: %v", err)
	}
	if prsiRep.Frames[0].Rounds != nil {
		t.Errorf("prsi invented a round log")
	}
}
