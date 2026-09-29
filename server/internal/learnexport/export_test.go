package learnexport_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	_ "zolik/server/internal/canasta"
	"zolik/server/internal/db"
	_ "zolik/server/internal/holdem"
	"zolik/server/internal/learn"
	"zolik/server/internal/learnexport"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// The secrets a stored match carries that must never reach a record. Each is
// distinctive enough that finding it anywhere in the output is proof of a
// leak, not a coincidence.
const (
	secretUser  = "user-6b1f0c2e9d-secret"
	secretGuest = "guest-5a2c77e1-secret"
	secretName  = "Zdenka Secretova"
	testSalt    = "test-salt"
)

// played is a match as the store holds it, plus the deal it started from.
type played struct {
	match models.Match
	moves []models.MatchAction
	deal  module.State
}

func (p played) snapshot(seq int) (models.JSONDoc, error) {
	if seq != 0 {
		return nil, fmt.Errorf("no snapshot %d", seq)
	}
	return models.JSONDoc(p.deal), nil
}

// playRecorded plays a match out with the game's heuristic bot in every seat
// and records every move exactly as the runtime stores it (manager.go's
// logEntry): seq from 1, the seat's player id, the action as JSON.
//
// The seats in humans are marked IsAI = false. They are still played by the
// heuristic — which is the point: every move they make came from an offer, so
// every one of them should be found among the candidates, and a decision that
// is not is the exporter's fault, not the player's.
func playRecorded(t *testing.T, g learn.Game, seats int, variation string, humans map[int]bool, seed int64) played {
	t.Helper()
	mod := g.Module()
	cfg := g.Config(seats, variation)

	players := make([]models.Player, seats)
	refs := make([]module.PlayerRef, seats)
	for i := range players {
		p := models.Player{ID: fmt.Sprintf("bot:%d", i), Name: fmt.Sprintf("Bot %d", i), IsAI: true}
		if humans[i] {
			// A person's seat, carrying everything that identifies them.
			p = models.Player{
				ID:     fmt.Sprintf("%s-%d", secretUser, i),
				Name:   secretName,
				UserID: secretUser,
			}
			if i%2 == 1 {
				p.UserID, p.GuestID = "", secretGuest
			}
		}
		players[i] = p
		refs[i] = module.PlayerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI}
	}

	deal, err := mod.NewMatch(cfg, refs, seed)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	state := deal
	bot := g.Heuristic()
	var moves []models.MatchAction
	start := time.Date(2026, 9, 1, 12, 34, 56, 0, time.UTC)
	for n := 0; ; n++ {
		if n > 20000 {
			t.Fatalf("seed %d: match did not finish", seed)
		}
		if done, _, err := mod.Finished(state); err != nil {
			t.Fatal(err)
		} else if done {
			break
		}
		actor := module.ActiveSeat(mod, state, refs[0].ID, refs)
		if actor == "" {
			t.Fatalf("seed %d: nobody on turn", seed)
		}
		offers, err := mod.LegalActions(state, actor)
		if err != nil {
			t.Fatal(err)
		}
		var tries []module.Action
		if a, ok := bot.Act(state, module.BotSeat{PlayerID: actor, Seed: module.SeatSeed(seed, actor, "bot")}, offers); ok {
			tries = append(tries, a)
		}
		tries = append(tries, module.ChooseActions(offers, nil)...)
		applied := false
		for _, a := range tries {
			next, _, err := mod.Apply(state, actor, a)
			if err != nil {
				continue
			}
			raw, _ := json.Marshal(a)
			moves = append(moves, models.MatchAction{
				Seq: len(moves) + 1, PlayerID: actor, Action: models.JSONDoc(raw),
				At: start.Add(time.Duration(len(moves)) * time.Second),
			})
			state, applied = next, true
			break
		}
		if !applied {
			t.Fatalf("seed %d: no move for %s", seed, actor)
		}
	}

	ended := start.Add(time.Duration(len(moves)) * time.Second)
	return played{
		match: models.Match{
			ID:        bson.NewObjectID(),
			ModuleID:  g.Name(),
			Variation: cfg.Variation,
			Options:   cfg.Options,
			Status:    "completed",
			Players:   players,
			HostID:    players[0].ID,
			JoinCode:  "SECRETJC",
			Seed:      seed,
			Snapshots: []int{0},
			CreatedAt: start,
			StartedAt: &start,
			EndedAt:   &ended,
		},
		moves: moves,
		deal:  deal,
	}
}

func game(t *testing.T, name string) learn.Game {
	t.Helper()
	g, err := learn.LookupGame(name)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// humanMoves counts the log entries made by seats not marked IsAI.
func humanMoves(p played) int {
	human := map[string]bool{}
	for _, pl := range p.match.Players {
		human[pl.ID] = !pl.IsAI
	}
	n := 0
	for _, mv := range p.moves {
		if human[mv.PlayerID] {
			n++
		}
	}
	return n
}

// checkRecords is everything every export must satisfy, whatever the game.
func checkRecords(t *testing.T, g learn.Game, p played, recs []learnexport.Record) {
	t.Helper()
	if want := humanMoves(p); len(recs) != want {
		t.Fatalf("%d records for %d human decisions", len(recs), want)
	}
	lastSeq := 0
	for _, r := range recs {
		if len(r.Obs) != g.StateDim() {
			t.Fatalf("seq %d: len(obs) = %d, StateDim = %d", r.Seq, len(r.Obs), g.StateDim())
		}
		for i, c := range r.Cands {
			if len(c) != g.CandDim() {
				t.Fatalf("seq %d: candidate %d is %d wide, CandDim = %d", r.Seq, i, len(c), g.CandDim())
			}
		}
		if r.Chosen >= len(r.Cands) {
			t.Fatalf("seq %d: chosen %d of %d candidates", r.Seq, r.Chosen, len(r.Cands))
		}
		if (r.Chosen < 0) != (r.Unmatched != "") {
			t.Fatalf("seq %d: chosen %d with unmatched %q", r.Seq, r.Chosen, r.Unmatched)
		}
		if r.Bot {
			t.Fatalf("seq %d: a bot decision without IncludeBots", r.Seq)
		}
		if r.Seq <= lastSeq {
			t.Fatalf("records out of order: %d after %d", r.Seq, lastSeq)
		}
		lastSeq = r.Seq
		if r.Date != "2026-09-01" {
			t.Fatalf("seq %d: date %q, want the day only", r.Seq, r.Date)
		}
		if !r.Outcome.MatchFinished {
			t.Fatalf("seq %d: a finished match reported unfinished", r.Seq)
		}
	}

	// Nothing that identifies anybody, anywhere in the bytes written.
	raw, err := json.Marshal(recs)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	for _, secret := range []string{secretUser, secretGuest, secretName, p.match.ID.Hex(), "SECRETJC", "12:34"} {
		if strings.Contains(out, secret) {
			t.Fatalf("the output contains %q", secret)
		}
	}
	for _, pl := range p.match.Players {
		if strings.Contains(out, pl.ID) && !pl.IsAI {
			t.Fatalf("the output contains the seat id %q", pl.ID)
		}
	}
}

// rate reports a match rate for the test log, which is what the plan asks to
// know. Every heuristic move comes from an offer, so anything under 100% is a
// candidate the adapter does not build.
func rate(t *testing.T, label string, st learnexport.Stats) {
	t.Helper()
	pct := 0.0
	if st.Human > 0 {
		pct = 100 * float64(st.Matched) / float64(st.Human)
	}
	t.Logf("%s: %d/%d human decisions matched (%.1f%%), %d approximate, %d forced, %d plan starts (%d made whole), %d within a plan, unmatched %v, dropped %v",
		label, st.Matched, st.Human, pct, st.Approximate, st.Forced, st.PlanStarts, st.PlansWhole, st.WithinPlan, st.Unmatched, st.Dropped)
	for v, s := range st.ByVerb {
		t.Logf("  %-14s %4d/%4d", v, s.Matched, s.Decisions)
	}
}

func seedsFor(short, long int) int {
	if testing.Short() {
		return short
	}
	return long
}

func TestHoldemExportMatchesEveryHumanDecision(t *testing.T) {
	g := game(t, "holdem")
	var total learnexport.Stats
	for seed := int64(1); seed <= int64(seedsFor(3, 12)); seed++ {
		p := playRecorded(t, g, 4, "", map[int]bool{0: true, 2: true}, seed)
		recs, st, err := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: testSalt})
		if err != nil {
			t.Fatal(err)
		}
		checkRecords(t, g, p, recs)
		total.Add(st)
	}
	rate(t, "holdem", total)
	if total.Matched != total.Human || total.Human == 0 {
		t.Fatalf("matched %d of %d: %v", total.Matched, total.Human, total.Unmatched)
	}
}

// Canasta is matched exactly everywhere but in one place: an opening the
// adapter's search did not find.
//
// Before a partnership has opened, the adapter offers whole openings, found by
// a bounded search (canasta/learn.go: at most maxOpenings plans of at most
// maxOpeningSteps actions). The heuristic opens greedily and a person opens
// however they like, so an opening can be legal, played, and not among the
// plans offered — and then it is recorded unmatched rather than forced onto a
// plan it was not. Everything outside an opening must match, and openings must
// mostly match: the rate is logged, because it is what the trainer loses.
func TestCanastaExportMatchesEveryHumanDecision(t *testing.T) {
	g := game(t, "canasta")
	var total learnexport.Stats
	openings, openingsMatched := 0, 0
	// An opening turn is counted by its first move, which is where the plan
	// is chosen; the rest are that plan's continuations.
	turns, turnsMatched := 0, 0
	for _, variation := range []string{"classic", "samba"} {
		seats := 4
		if variation == "samba" {
			seats = 6
		}
		for seed := int64(1); seed <= int64(seedsFor(1, 4)); seed++ {
			p := playRecorded(t, g, seats, variation, map[int]bool{0: true, 1: true}, seed)
			recs, st, err := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: testSalt})
			if err != nil {
				t.Fatal(err)
			}
			checkRecords(t, g, p, recs)
			total.Add(st)

			opening := canastaOpeningMoves(t, g, p)
			for _, r := range recs {
				if opening[r.Seq] {
					openings++
					if r.Chosen >= 0 {
						openingsMatched++
					}
					if !opening[r.Seq-1] {
						turns++
						if r.Chosen >= 0 {
							turnsMatched++
						}
					}
					continue
				}
				if r.Chosen < 0 {
					t.Errorf("%s seed %d seq %d: a %s outside an opening unmatched: %s", variation, seed, r.Seq, r.Verb, r.Unmatched)
				}
			}
		}
	}
	rate(t, "canasta", total)
	t.Logf("canasta openings: %d/%d moves matched (%.1f%%); %d/%d opening turns matched on their first move (%.1f%%)",
		openingsMatched, openings, 100*float64(openingsMatched)/float64(max(openings, 1)),
		turnsMatched, turns, 100*float64(turnsMatched)/float64(max(turns, 1)))
	if total.Human == 0 || total.PlanStarts == 0 || openings == 0 {
		t.Fatalf("no openings exported: %+v", total)
	}
	// A floor, not a target: it is the adapter's search that sets the rate,
	// and this only catches the exporter losing openings it used to find.
	if openingsMatched*2 < openings {
		t.Fatalf("only %d of %d opening moves matched", openingsMatched, openings)
	}
}

// canastaOpeningMoves are the seqs of the moves that put cards on the table for
// a partnership that had not yet opened: the moves an opening is made of.
func canastaOpeningMoves(t *testing.T, g learn.Game, p played) map[int]bool {
	t.Helper()
	out := map[int]bool{}
	var prev module.State
	_, err := match.FoldMoves(g.Module(), p.match, p.moves, p.snapshot,
		func(step int, entry *models.MatchAction, a module.Action, s module.State) (bool, error) {
			if step > 0 && (a.Verb == "lay_meld" || a.Verb == "lay_off" || a.Verb == "take_pile") {
				var st struct {
					Teams []struct {
						Players   []string `json:"players"`
						HasMelded bool     `json:"hasMelded"`
					} `json:"teams"`
				}
				if err := json.Unmarshal(prev, &st); err != nil {
					return false, err
				}
				for _, team := range st.Teams {
					for _, id := range team.Players {
						if id == entry.PlayerID && !team.HasMelded {
							out[entry.Seq] = true
						}
					}
				}
			}
			prev = s
			return true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A person lays off whichever natural they like; the adapter offers one per
// group. The seven of hearts on the sevens is the candidate the seven of
// spades stands for (learn.Equivalence), and nothing else is.
func TestCanastaLayOffOfAnotherSuitIsTheSameMove(t *testing.T) {
	g := game(t, "canasta")
	mod := g.Module()
	tried := 0
	for seed := int64(1); seed <= 3 && tried == 0; seed++ {
		p := playRecorded(t, g, 4, "classic", map[int]bool{0: true}, seed)
		var prev module.State
		_, err := match.FoldMoves(mod, p.match, p.moves, p.snapshot,
			func(step int, entry *models.MatchAction, a module.Action, s module.State) (bool, error) {
				defer func() { prev = s }()
				if step == 0 || a.Verb != "lay_off" || len(a.Cards) != 1 {
					return true, nil
				}
				offers, err := mod.LegalActions(prev, entry.PlayerID)
				if err != nil {
					return false, err
				}
				cands, err := g.Candidates(prev, entry.PlayerID, offers)
				if err != nil {
					return false, err
				}
				exact := learnexport.Choose(g, prev, entry.PlayerID, cands, a, nil)
				if exact.Index < 0 || len(cands[exact.Index].Then) > 0 {
					return true, nil
				}
				// Another card the offer would take, of the same rank but not
				// the same card: the same move by another suit.
				var hand struct {
					Hands map[string][]string `json:"hands"`
				}
				if err := json.Unmarshal(prev, &hand); err != nil {
					return false, err
				}
				want := a.Cards[0]
				for _, c := range hand.Hands[entry.PlayerID] {
					if c == want || c[:len(c)-1] != want[:len(want)-1] || strings.HasPrefix(c, "JOKER") || strings.HasPrefix(c, "2") {
						continue
					}
					other := a
					other.Cards = []string{c}
					if _, _, err := mod.Apply(prev, entry.PlayerID, other); err != nil {
						continue
					}
					got := learnexport.Choose(g, prev, entry.PlayerID, cands, other, nil)
					if got.Index != exact.Index {
						t.Fatalf("seq %d: %s laid off as %v matched candidate %d, %v matched %d", entry.Seq, c, other, got.Index, a, exact.Index)
					}
					tried++
					break
				}
				return true, nil
			})
		if err != nil {
			t.Fatal(err)
		}
	}
	if tried == 0 {
		t.Skip("no lay-off with a second natural of its rank in these seeds")
	}
}

// The deal a match starts from is stored, but a match stored before snapshots
// existed has none and is rebuilt from its seed. Both must be the same game.
func TestExportRebuildsTheDealFromTheSeed(t *testing.T) {
	g := game(t, "holdem")
	p := playRecorded(t, g, 3, "", map[int]bool{1: true}, 7)
	withSnap, _, err := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: testSalt})
	if err != nil {
		t.Fatal(err)
	}
	p.match.Snapshots = nil
	fromSeed, _, err := learnexport.Export(g, p.match, p.moves, nil, learnexport.Options{Salt: testSalt})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(withSnap)
	b, _ := json.Marshal(fromSeed)
	if string(a) != string(b) {
		t.Fatal("the stored deal and the seed's deal export differently")
	}
}

func TestExportSkipsBotsUnlessAsked(t *testing.T) {
	g := game(t, "holdem")
	p := playRecorded(t, g, 3, "", map[int]bool{0: true}, 3)
	recs, st, err := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: testSalt, IncludeBots: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != len(p.moves) || st.Bot+st.Human != len(p.moves) {
		t.Fatalf("%d records (%d bot, %d human) for %d moves", len(recs), st.Bot, st.Human, len(p.moves))
	}
	bots := 0
	for _, r := range recs {
		if r.Bot {
			bots++
		}
	}
	if bots != st.Bot || bots == 0 {
		t.Fatalf("%d records marked bot, %d counted", bots, st.Bot)
	}
}

// The salt is what stands between a hash and the id under it: the same match
// under two salts must not be linkable, and one salt must group consistently.
func TestHashesAreSaltedAndStable(t *testing.T) {
	g := game(t, "holdem")
	p := playRecorded(t, g, 2, "", map[int]bool{0: true}, 5)
	a, _, _ := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: "one"})
	b, _, _ := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: "two"})
	if len(a) == 0 {
		t.Fatal("no records")
	}
	if a[0].MatchHash == b[0].MatchHash || a[0].SeatHash == b[0].SeatHash {
		t.Fatal("a different salt gave the same hash")
	}
	for _, r := range a {
		if r.MatchHash != a[0].MatchHash || r.SeatHash != a[0].SeatHash {
			t.Fatal("one match's one human seat hashed two ways")
		}
	}
	if _, _, err := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{}); err == nil {
		t.Fatal("exported without a salt")
	}
}

// No peek, through the whole pipeline: change cards another seat holds in the
// stored deal — cards nobody else can see — and every decision the human made
// before the first episode ended must encode exactly as before.
//
// The swapped-in cards come from the bottom of the stock, which nothing deals
// from in the first hand, so the public game is untouched: the same board, the
// same bets. After a showdown or a scored deal the cards are public and the
// observations may rightly differ, so only the first episode is compared; a
// fold that stops because the other seat plays a card it no longer holds only
// shortens what is compared.
func TestExportDoesNotPeek(t *testing.T) {
	cases := []struct {
		game      string
		seats     int
		variation string
		// seeds: a hold'em hand is short and cheap, so many; a Canasta
		// match is neither.
		seeds  int
		tamper func(t *testing.T, st map[string]any, seat string)
	}{
		{"holdem", 4, "", 8, func(t *testing.T, st map[string]any, seat string) {
			deck := st["deck"].([]any)
			for _, s := range st["seats"].([]any) {
				sm := s.(map[string]any)
				if sm["playerId"] != seat {
					continue
				}
				hole := sm["hole"].([]any)
				hole[0], deck[0] = deck[0], hole[0]
				hole[1], deck[1] = deck[1], hole[1]
				return
			}
			t.Fatalf("no seat %s", seat)
		}},
		{"canasta", 4, "classic", seedsFor(2, 6), func(t *testing.T, st map[string]any, seat string) {
			stock := st["drawPile"].([]any)
			hand := st["hands"].(map[string]any)[seat].([]any)
			for i := 0; i < 3; i++ {
				hand[i], stock[i] = stock[i], hand[i]
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.game, func(t *testing.T) {
			g := game(t, tc.game)
			// Bots included, so that the seat whose cards were changed can
			// show the change is one Encode reads when it is entitled to: a
			// tamper nobody's observation noticed would prove nothing.
			opt := learnexport.Options{Salt: testSalt, IncludeBots: true}
			compared, noticed := 0, 0
			for seed := int64(11); seed < 11+int64(tc.seeds); seed++ {
				p := playRecorded(t, g, tc.seats, tc.variation, map[int]bool{0: true}, seed)
				orig, _, err := learnexport.Export(g, p.match, p.moves, p.snapshot, opt)
				if err != nil {
					t.Fatal(err)
				}

				var st map[string]any
				if err := json.Unmarshal(p.deal, &st); err != nil {
					t.Fatal(err)
				}
				other := p.match.Players[1].ID
				tc.tamper(t, st, other)
				tampered, err := json.Marshal(st)
				if err != nil {
					t.Fatal(err)
				}
				q := p
				q.deal = tampered
				changed, _, err := learnexport.Export(g, q.match, q.moves, q.snapshot, opt)
				if err != nil {
					t.Fatal(err)
				}

				bySeq := map[int]learnexport.Record{}
				for _, r := range changed {
					bySeq[r.Seq] = r
				}
				for _, r := range orig {
					c, ok := bySeq[r.Seq]
					if r.Episode > 0 || !ok {
						break
					}
					same := sameObs(r.Obs, c.Obs)
					switch {
					case r.SeatIndex == 1 && !same:
						noticed++
					case !r.Bot && !same:
						t.Fatalf("seed %d seq %d: the human's observation moved with another seat's cards", seed, r.Seq)
					case !r.Bot:
						compared++
					}
				}
			}
			if compared < 5 || noticed == 0 {
				t.Fatalf("%d human decisions compared, %d of the changed seat's noticed the change", compared, noticed)
			}
			t.Logf("%s: %d first-episode human decisions unchanged; the changed seat's own changed %d times", tc.game, compared, noticed)
		})
	}
}

func sameObs(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// store writes a played match the way the runtime does: the envelope, the
// deal as snapshot 0, and then every move with the final board.
func store(t *testing.T, repo match.Repository, p played, finalState module.State) models.Match {
	t.Helper()
	ctx := t.Context()
	m := p.match
	m.Snapshots = nil
	saved, err := repo.Insert(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	next := saved
	next.Snapshots = []int{0}
	if err := repo.CommitSnapshot(ctx, saved.ID, saved.Version, next, nil, 0, models.JSONDoc(p.deal)); err != nil {
		t.Fatal(err)
	}
	last := len(p.moves)
	final := next
	final.Snapshots = []int{0, last}
	if err := repo.CommitSnapshot(ctx, saved.ID, saved.Version+1, final, p.moves, last, models.JSONDoc(finalState)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByID(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// finalState folds a played match to its end.
func finalState(t *testing.T, g learn.Game, p played) module.State {
	t.Helper()
	var last module.State
	if _, err := match.FoldMoves(g.Module(), p.match, p.moves, p.snapshot,
		func(_ int, _ *models.MatchAction, _ module.Action, s module.State) (bool, error) {
			last = s
			return true, nil
		}); err != nil {
		t.Fatal(err)
	}
	return last
}

// Through the store, as cmd/export-games reads it: EachFinished finds the
// finished matches of one game inside a window, Moves and Snapshot hand back
// what was written, and the export of the stored match is the export of the
// match as it was played.
func TestExportThroughTheRepository(t *testing.T) {
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	repo := match.NewKDBRepository(k)
	ctx := t.Context()

	g := game(t, "holdem")
	var want [][]learnexport.Record
	for i, seed := range []int64{21, 22, 23} {
		p := playRecorded(t, g, 3, "", map[int]bool{0: true}, seed)
		ended := time.Date(2026, 9, 1+i, 10, 0, 0, 0, time.UTC)
		p.match.EndedAt = &ended
		recs, _, err := learnexport.Export(g, p.match, p.moves, p.snapshot, learnexport.Options{Salt: testSalt})
		if err != nil {
			t.Fatal(err)
		}
		p.match.ID = bson.ObjectID{} // the store assigns it
		stored := store(t, repo, p, finalState(t, g, p))
		// The hashes are of the id the store gave it.
		for j := range recs {
			recs[j].MatchHash = learnexport.Hash(testSalt, "match", stored.ID.Hex())
			recs[j].Date = ended.Format(time.DateOnly)
		}
		want = append(want, recs)
	}
	// Neither of these is a finished hold'em match, and neither may be listed.
	other := playRecorded(t, game(t, "canasta"), 4, "classic", nil, 1)
	other.match.ID = bson.ObjectID{}
	store(t, repo, other, finalState(t, game(t, "canasta"), other))
	active := playRecorded(t, g, 2, "", nil, 30)
	active.match.ID, active.match.Status, active.match.EndedAt = bson.ObjectID{}, "active", nil
	store(t, repo, active, finalState(t, g, active))

	export := func(f match.FinishedFilter) [][]learnexport.Record {
		var got [][]learnexport.Record
		err := repo.EachFinished(ctx, f, func(m models.Match) error {
			moves, err := repo.Moves(ctx, m.ID, 0, -1)
			if err != nil {
				return err
			}
			recs, _, err := learnexport.Export(g, m, moves,
				func(seq int) (models.JSONDoc, error) { return repo.Snapshot(ctx, m.ID, seq) },
				learnexport.Options{Salt: testSalt})
			if err != nil {
				return err
			}
			got = append(got, recs)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	all := export(match.FinishedFilter{ModuleID: "holdem"})
	if len(all) != 3 {
		t.Fatalf("listed %d matches, want the 3 finished hold'em ones", len(all))
	}
	for i := range all {
		a, _ := json.Marshal(all[i])
		b, _ := json.Marshal(want[i])
		if string(a) != string(b) {
			t.Fatalf("match %d exports differently from the store than as played", i)
		}
	}
	window := export(match.FinishedFilter{
		ModuleID:    "holdem",
		EndedFrom:   time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		EndedBefore: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
	})
	if len(window) != 1 || window[0][0].Date != "2026-09-02" {
		t.Fatalf("the window of 2 September listed %d matches", len(window))
	}
	if got := export(match.FinishedFilter{ModuleID: "holdem", Limit: 2}); len(got) != 2 || got[0][0].Date != "2026-09-01" {
		t.Fatalf("a limit of 2 listed %d, oldest first", len(got))
	}
}
