package match_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/canasta"
	"zolik/server/internal/klondike"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	zsync "zolik/server/internal/sync"
	"zolik/server/internal/ws"
)

// Solitaire is played against the deck, so the deal is the whole game: it
// must never reach a device, nor be chosen by one (docs/solitaire-plan.md §4a).

func soloManager(t *testing.T) (*match.Manager, match.Repository, *module.Registry) {
	t.Helper()
	repo := newTestRepository(t)
	hub, err := ws.NewHub(ws.NewConnRegistry(), "")
	if err != nil {
		t.Fatalf("building a hub: %v", err)
	}
	t.Cleanup(func() { _ = hub.Close() })
	mods := module.NewRegistry(canasta.New(), klondike.New())
	return match.NewManager(repo, mods, hub), repo, mods
}

const soloUser = "0123456789abcdef01234567"

func TestADeviceIsNeverSeatedAtAServerDealtMatch(t *testing.T) {
	m, _, _ := soloManager(t)
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "Solo", UserID: soloUser}

	solo, err := m.Create(ctx, "klondike", module.MatchConfig{}, host)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.Start(ctx, solo.ID.Hex()); err != nil {
		t.Fatalf("start: %v", err)
	}
	seated, err := m.Seated(ctx, solo.ID.Hex(), soloUser)
	if err != nil {
		t.Fatal(err)
	}
	if seated {
		t.Fatal("the solitaire player's device may pull the match namespace, and every face-down card with it")
	}

	// A game with other players at it is unchanged.
	duo, err := m.Create(ctx, "canasta", module.MatchConfig{}, host)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if seated, _ := m.Seated(ctx, duo.ID.Hex(), soloUser); !seated {
		t.Fatal("a canasta host is no longer seated at their own table")
	}
}

func TestTwoMatchesCreatedTogetherDealDifferently(t *testing.T) {
	m, _, _ := soloManager(t)
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "Solo"}
	seen := map[int64]bool{}
	for i := 0; i < 8; i++ {
		created, err := m.Create(ctx, "klondike", module.MatchConfig{}, host)
		if err != nil {
			t.Fatal(err)
		}
		if seen[created.Seed] {
			t.Fatalf("seed %d dealt twice", created.Seed)
		}
		seen[created.Seed] = true
	}
}

func TestAServerDealtBundleIsRefused(t *testing.T) {
	_, repo, mods := soloManager(t)
	imp := match.NewImporter(repo, mods, nil, nil)
	m := models.Match{ModuleID: "klondike", Status: "completed", Players: []models.Player{{ID: "p1"}}}
	m.ID = bson.NewObjectID()
	env, _ := json.Marshal(m)
	err := imp.ImportBundle(context.Background(), zsync.Bundle{
		Match: m.ID.Hex(), Module: "klondike", Seed: 1, Envelope: env,
	})
	if err == nil || !strings.Contains(err.Error(), "server only") {
		t.Fatalf("a phone-dealt solitaire game was accepted: %v", err)
	}
}

// A solitaire table left behind is a saved game: suspended to wait for its
// player for a month, never swept up as abandoned two minutes later.
func TestALeftSolitaireGameIsSaved(t *testing.T) {
	m, repo, _ := soloManager(t)
	ctx := context.Background()
	solo, err := m.Create(ctx, "klondike", module.MatchConfig{}, models.Player{ID: "p1", Name: "Solo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, solo.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	m.SuspendOnDisconnect(ctx, solo.ID.Hex(), "p1", "test")
	got, err := repo.FindByID(ctx, solo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "suspended" || got.AbandonAt == nil {
		t.Fatalf("not suspended: %s", got.Status)
	}
	if wait := got.AbandonAt.Sub(*got.SuspendedAt); wait < match.SavedGameWindow {
		t.Fatalf("a saved game waits only %v", wait)
	}
	if n := m.ReapAbandoned(ctx); n != 0 {
		t.Fatalf("the reaper abandoned %d", n)
	}
	m.ResumeIfReturning(ctx, solo.ID.Hex(), "p1")
	if back, _ := repo.FindByID(ctx, solo.ID); back.Status != "active" {
		t.Fatalf("the player came back to a %s game", back.Status)
	}
}

// "Play this deal again" copies the deal on the server: the new table is dealt
// the very same cards, and nobody is ever shown the seed that did it.
func TestADealPlayedAgainIsTheSameDeal(t *testing.T) {
	m, repo, _ := soloManager(t)
	ctx := context.Background()
	host := models.Player{ID: "p1", Name: "Solo"}
	first, err := m.Create(ctx, "klondike", module.MatchConfig{Variation: "draw3"}, host)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, first.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	opening := tableauOf(t, m, first.ID.Hex())

	// Not while it is still being played.
	if _, err := m.DealAgain(ctx, first.ID.Hex(), "p1"); module.CodeOf(err) != "MATCH_NOT_OVER" {
		t.Fatalf("dealt again mid-game: %v", err)
	}
	if err := m.HandleAction(ctx, first.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}
	// Not for somebody who never sat there.
	if _, err := m.DealAgain(ctx, first.ID.Hex(), "stranger"); module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
		t.Fatalf("a stranger was dealt somebody else's game: %v", err)
	}

	again, err := m.DealAgain(ctx, first.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != "active" || again.ID == first.ID {
		t.Fatalf("the deal again is %s, id %s", again.Status, again.ID.Hex())
	}
	if again.Variation != "draw3" || again.DealFrom != first.ID.Hex() {
		t.Fatalf("variation %q, dealFrom %q", again.Variation, again.DealFrom)
	}
	if got := tableauOf(t, m, again.ID.Hex()); got != opening {
		t.Fatalf("dealt differently:\n%s\n%s", got, opening)
	}
	stored, _ := repo.FindByID(ctx, again.ID)
	if stored.Seed != first.Seed {
		t.Fatal("the seed was not carried over")
	}

	// Played again from the repeat, it still names the first table.
	if err := m.HandleAction(ctx, again.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}
	third, err := m.DealAgain(ctx, again.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if third.DealFrom != first.ID.Hex() {
		t.Fatalf("a repeat of a repeat names %q", third.DealFrom)
	}
}

// tableauOf is the face-up board as the player sees it: the same deal reads
// the same.
func tableauOf(t *testing.T, m *match.Manager, matchID string) string {
	t.Helper()
	cur, err := m.Current(context.Background(), matchID)
	if err != nil {
		t.Fatal(err)
	}
	vm, err := klondike.New().View(module.State(cur.State), "p1")
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(vm.Zones)
	return string(blob)
}

// "Send this deal": a link the player who played a game may hand to anybody,
// which deals them the same cards and tells them nothing about where from.
func TestASentDealIsTheSameDealAndNamesNoMatch(t *testing.T) {
	m, repo, _ := soloManager(t)
	ctx := context.Background()
	sender := models.Player{ID: "p1", Name: "Sender", UserID: soloUser}
	first, err := m.Create(ctx, "klondike", module.MatchConfig{}, sender)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, first.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	opening := tableauOf(t, m, first.ID.Hex())

	// Not while it is being played, and not by somebody who did not play it.
	if _, err := m.DealLink(ctx, first.ID.Hex(), "p1"); module.CodeOf(err) != "MATCH_NOT_OVER" {
		t.Fatalf("a link to a game in play: %v", err)
	}
	if err := m.HandleAction(ctx, first.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DealLink(ctx, first.ID.Hex(), "stranger"); module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
		t.Fatalf("a stranger sent somebody else's deal: %v", err)
	}

	token, err := m.DealLink(ctx, first.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(token, first.ID.Hex()) || strings.Contains(token, first.JoinCode) {
		t.Fatal("the link names the match it came from")
	}
	cfg, moduleID, err := m.SentDeal(ctx, token)
	if err != nil || moduleID != "klondike" || cfg.Variation != first.Variation {
		t.Fatalf("the link offers %q %+v: %v", moduleID, cfg, err)
	}

	friend := models.Player{ID: "p2", Name: "Friend", UserID: "fedcba9876543210fedcba98"}
	theirs, err := m.PlaySentDeal(ctx, token, friend)
	if err != nil {
		t.Fatal(err)
	}
	if got := tableauOf(t, m, theirs.ID.Hex()); got != opening {
		t.Fatalf("the friend was dealt differently:\n%s\n%s", got, opening)
	}
	stored, _ := repo.FindByID(ctx, theirs.ID)
	if stored.DealRepeat || stored.DealFrom != first.ID.Hex() {
		t.Fatalf("repeat %v, dealFrom %q", stored.DealRepeat, stored.DealFrom)
	}
	// Nothing the friend is sent carries the match it came from.
	blob, _ := json.Marshal(m.BuildStateMsg(stored, "p2"))
	if strings.Contains(string(blob), first.ID.Hex()) {
		t.Fatal("the friend's board names the match the deal came from")
	}

	// The sender playing their own link has seen it.
	mine, err := m.PlaySentDeal(ctx, token, models.Player{ID: "p1", Name: "Sender", UserID: soloUser})
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := repo.FindByID(ctx, mine.ID); !again.DealRepeat {
		t.Fatal("the sender's own deal is counted a first attempt")
	}

	// A forged or mangled link opens nothing.
	if _, err := m.PlaySentDeal(ctx, token[:len(token)-2]+"xx", friend); module.CodeOf(err) != "DEAL_LINK_INVALID" {
		t.Fatalf("a mangled link: %v", err)
	}
}

// Everybody who played one deal, side by side: the original, a friend it was
// sent to, and the original player's own second go.
func TestSameDealComparesEveryAttemptAndNamesNoMatch(t *testing.T) {
	m, _, _ := soloManager(t)
	ctx := context.Background()
	sender := models.Player{ID: "p1", Name: "Sender", UserID: soloUser}
	first, err := m.Create(ctx, "klondike", module.MatchConfig{}, sender)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, first.ID.Hex()); err != nil {
		t.Fatal(err)
	}
	// Not before you have finished it: the others' results would be a hint.
	if _, err := m.SameDeal(ctx, first.ID.Hex(), "p1"); module.CodeOf(err) != "MATCH_NOT_OVER" {
		t.Fatalf("compared mid-game: %v", err)
	}
	// One move, so the two attempts differ, then give up.
	if err := m.HandleAction(ctx, first.ID.Hex(), "p1", module.Action{Verb: "draw"}); err != nil {
		t.Fatal(err)
	}
	if err := m.HandleAction(ctx, first.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}

	token, err := m.DealLink(ctx, first.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	friend := models.Player{ID: "p2", Name: "Friend", UserID: "fedcba9876543210fedcba98"}
	theirs, err := m.PlaySentDeal(ctx, token, friend)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.HandleAction(ctx, theirs.ID.Hex(), "p2", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}
	again, err := m.DealAgain(ctx, first.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.HandleAction(ctx, again.ID.Hex(), "p1", module.Action{Verb: "giveup"}); err != nil {
		t.Fatal(err)
	}

	// Seen from the friend's table: three attempts, one of them theirs.
	results, err := m.SameDeal(ctx, theirs.ID.Hex(), "p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("%d attempts: %+v", len(results), results)
	}
	mine, repeats := 0, 0
	for _, r := range results {
		if r.You {
			mine++
			if r.Name != "Friend" {
				t.Fatalf("the friend is shown %q as theirs", r.Name)
			}
		}
		if r.Repeat {
			repeats++
		}
		if len(r.Facts) == 0 {
			t.Fatalf("%s's attempt has no numbers", r.Name)
		}
	}
	if mine != 1 || repeats != 1 {
		t.Fatalf("%d marked yours, %d repeats", mine, repeats)
	}
	// Seen from the sender's: both of theirs are theirs.
	fromSender, err := m.SameDeal(ctx, again.ID.Hex(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	own := 0
	for _, r := range fromSender {
		if r.You {
			own++
		}
	}
	if own != 2 {
		t.Fatalf("the sender sees %d of their attempts as theirs", own)
	}
	// No match is named, theirs or anybody else's.
	blob, _ := json.Marshal(results)
	for _, id := range []string{first.ID.Hex(), theirs.ID.Hex(), again.ID.Hex()} {
		if strings.Contains(string(blob), id) {
			t.Fatal("the comparison names a match")
		}
	}
	// And somebody who played none of it sees none of it.
	if _, err := m.SameDeal(ctx, theirs.ID.Hex(), "p1"); module.CodeOf(err) != "NOT_AT_THIS_TABLE" {
		t.Fatalf("read another player's table: %v", err)
	}
}
