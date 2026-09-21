package match

import (
	"context"
	"testing"
	"time"

	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/ws"
)

// The runtime against a real store: every match here is created, dealt and
// played through the Manager, so what is stored is exactly what production
// stores.

type liveHarness struct {
	m    *Manager
	k    *db.KDB
	repo Repository
}

func newLiveHarness(t *testing.T) liveHarness {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Skipf("no embedded kdb available here: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	repo := NewKDBRepository(k)
	return liveHarness{m: NewManager(repo, replayManager().registry, hub), k: k, repo: repo}
}

// startGame opens a table of human seats for a module and deals it.
func (h liveHarness) startGame(t *testing.T, g replayable, seed int64) models.Match {
	t.Helper()
	ctx := context.Background()
	host := models.Player{ID: g.players[0].ID, Name: g.players[0].Name}
	cfg := g.cfg
	if g.name == "canasta" {
		// A value the lobby offers: the replay fixtures' 1500 is for folding a
		// state directly, which Create would refuse.
		cfg.Options = module.Options{"targetScore": 500}
	}
	m, err := h.m.Create(ctx, g.name, cfg, host)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, p := range g.players[1:] {
		if _, err := h.m.Join(ctx, m.ID.Hex(), models.Player{ID: p.ID, Name: p.Name}); err != nil {
			t.Fatalf("join: %v", err)
		}
	}
	// The seed decides the deal; set it before Start reads it.
	e, err := h.m.lockMatch(ctx, m.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	next := e.match
	next.Seed = seed
	if err := h.m.saveLocked(ctx, e, next); err != nil {
		t.Fatal(err)
	}
	e.mu.Unlock()
	started, err := h.m.Start(ctx, m.ID.Hex())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return started
}

// play makes up to n moves through HandleAction, choosing the way the offer
// driver does, and calls after with the board after every accepted move.
func (h liveHarness) play(t *testing.T, g replayable, id string, n int, after func(seq int, board module.State)) int {
	t.Helper()
	ctx := context.Background()
	moves := 0
	for moves < n {
		cur, err := h.m.current(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if cur.Status != "active" {
			break
		}
		state := module.State(cur.State)
		seats := module.AwaitedSeats(g.mod, state, "", refsOf(cur))
		if len(seats) == 0 {
			break
		}
		actor := seats[0]
		offers, err := g.mod.LegalActions(state, actor)
		if err != nil || len(offers) == 0 {
			break
		}
		played := false
		for _, c := range botCandidates(orderByPreference(offers, g.prefer)) {
			if err := h.m.HandleAction(ctx, id, actor, c.action); err == nil {
				played = true
				break
			}
		}
		if !played {
			break
		}
		moves++
		after(moves, module.State(h.board(t, id)))
	}
	return moves
}

func orderByPreference(offers []module.ActionOffer, prefer []string) []module.ActionOffer {
	out := make([]module.ActionOffer, 0, len(offers))
	for _, verb := range prefer {
		for _, o := range offers {
			if o.Verb == verb {
				out = append(out, o)
			}
		}
	}
	for _, o := range offers {
		found := false
		for _, verb := range prefer {
			if o.Verb == verb {
				found = true
			}
		}
		if !found {
			out = append(out, o)
		}
	}
	return out
}

func (h liveHarness) board(t *testing.T, id string) models.JSONDoc {
	t.Helper()
	cur, err := h.m.current(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return cur.State
}

// The property everything rests on: a match rebuilt from what is stored — its
// newest snapshot and the moves after it — is the board it was being played
// on, byte for byte, at every point in the game. The in-memory board is
// dropped every few moves, so the rebuild is exercised from every distance to
// a snapshot there is.
func TestARebuiltMatchIsTheBoardInPlay(t *testing.T) {
	for _, g := range replayables() {
		t.Run(g.name, func(t *testing.T) {
			for _, seed := range []int64{1, 2, 3, 4, 5} {
				h := newLiveHarness(t)
				m := h.startGame(t, g, seed)
				id := m.ID.Hex()
				made := h.play(t, g, id, 150, func(seq int, board module.State) {
					if seq%7 != 0 {
						return
					}
					h.m.live.forget(id)
					if rebuilt := h.board(t, id); string(rebuilt) != string(board) {
						t.Fatalf("seed %d, move %d: the rebuilt board differs from the one in play", seed, seq)
					}
				})
				if made < 10 {
					t.Fatalf("seed %d: only %d moves made", seed, made)
				}
				stored, err := h.repo.Moves(context.Background(), m.ID, 0, -1)
				if err != nil || len(stored) != made {
					t.Fatalf("seed %d: %d moves made, %d stored (%v)", seed, made, len(stored), err)
				}
			}
		})
	}
}

// An envelope write — here a suspension — must never touch the board: it is
// no longer on the match document at all, so there is nothing for a stale
// copy to overwrite.
func TestSuspendingKeepsTheBoard(t *testing.T) {
	g := withRounds(t, "canasta")
	h := newLiveHarness(t)
	m := h.startGame(t, g, 3)
	id := m.ID.Hex()
	h.play(t, g, id, 12, func(int, module.State) {})
	before := h.board(t, id)

	cur, _ := h.m.current(context.Background(), id)
	awaited := module.AwaitedSeats(g.mod, module.State(cur.State), "", refsOf(cur))
	h.m.SuspendOnDisconnect(context.Background(), id, awaited[0], "test")
	if got, _ := h.m.current(context.Background(), id); got.Status != "suspended" {
		t.Fatalf("status %q, want suspended", got.Status)
	}
	h.m.live.forget(id)
	if after := h.board(t, id); string(after) != string(before) {
		t.Error("the board changed across a suspension and a reload")
	}
}

// Moves do not write the match, so while a match is being played its
// UpdatedAt is kept fresh by a touch at most every touchEvery — which is what
// stops the stranded-match sweep from taking a game that is in play.
func TestAPlayingMatchStaysFresh(t *testing.T) {
	g := withRounds(t, "prsi")
	h := newLiveHarness(t)
	m := h.startGame(t, g, 1)
	id := m.ID.Hex()

	e, err := h.m.lockMatch(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	e.touched = time.Now().Add(-2 * touchEvery)
	e.mu.Unlock()

	h.play(t, g, id, 1, func(int, module.State) {})
	stored, err := h.repo.FindByID(context.Background(), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(stored.UpdatedAt) > time.Minute {
		t.Errorf("a move after a quiet spell left UpdatedAt at %v", stored.UpdatedAt)
	}
}

// Move N is stored once: a second writer with the same seq is refused, which
// is how a process holding a stale board finds out.
func TestAMoveIsStoredOnce(t *testing.T) {
	h := newLiveHarness(t)
	m := h.startGame(t, withRounds(t, "prsi"), 1)
	move := models.MatchAction{Seq: 1, PlayerID: "p1", Action: models.JSONDoc(`{"verb":"draw"}`), At: time.Now()}
	if err := h.repo.AppendMove(context.Background(), m.ID, move); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.AppendMove(context.Background(), m.ID, move); err != ErrVersionConflict {
		t.Errorf("a second move 1: %v, want ErrVersionConflict", err)
	}
}

// Deleting a match takes every move and snapshot with it.
func TestDeletingAMatchLeavesNothingBehind(t *testing.T) {
	g := withRounds(t, "canasta")
	h := newLiveHarness(t)
	m := h.startGame(t, g, 2)
	id := m.ID.Hex()
	h.play(t, g, id, 120, func(int, module.State) {})
	if err := h.m.DeleteAsHost(context.Background(), id, "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	left := 0
	if err := h.k.Scan(db.NSMatchLog, func([]byte) error { left++; return nil }); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d log records outlived their match", left)
	}
}

// What a move costs in the store, measured the way the store keeps it: the
// bytes each move commits, which is what history holds and a restart decodes.
func TestAMoveCommitsAFewHundredBytes(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a thousand moves")
	}
	g := withRounds(t, "canasta")
	h := newLiveHarness(t)
	m := h.startGame(t, g, 4)
	id := m.ID.Hex()
	var committed int64
	made := h.play(t, g, id, 1000, func(seq int, _ module.State) {
		raw, err := h.k.Get(db.NSMatchLog, logKey(m.ID, kindMove, seq))
		if err != nil {
			t.Fatal(err)
		}
		committed += int64(len(raw))
		cur, _ := h.m.current(context.Background(), id)
		if last, _ := latestSnapshot(cur); last == seq {
			snap, _ := h.k.Get(db.NSMatchLog, logKey(m.ID, kindSnapshot, seq))
			env, _ := h.k.Get(db.NSMatches, id)
			committed += int64(len(snap) + len(env))
		}
	})
	per := float64(committed) / float64(made)
	t.Logf("%d moves: %.0f bytes committed per move, snapshots included", made, per)
	if per > 400 {
		t.Errorf("%.0f bytes per move; the move log should cost a few hundred", per)
	}
}
