package gamemcp

import (
	"math/rand"
	"strings"
	"testing"
)

// observe and coach show a seat's hand, so a bot's seat is refused while the
// match is on — otherwise the caller reads the cards it is playing against.
// Once the match is over every seat may be looked at.
func TestObserveAndCoachRefuseABotSeatUntilTheEnd(t *testing.T) {
	svc := NewService()
	out := mustCall[NewTableOut](t, svc, "new_table", NewTableIn{
		Game: "sedma", Seats: 2, ClaudeSeats: []int{0}, Seed: seed(3),
	})
	if _, err := svc.Call("observe", mustJSON(t, ObserveIn{Table: out.Table, Seat: 0})); err != nil {
		t.Fatalf("observing your own seat: %v", err)
	}
	for _, tool := range []string{"observe", "coach"} {
		var args any = ObserveIn{Table: out.Table, Seat: 1}
		if tool == "coach" {
			args = CoachIn{Table: out.Table, Seat: 1}
		}
		_, err := svc.Call(tool, mustJSON(t, args))
		if err == nil || !strings.Contains(err.Error(), "played by a bot") {
			t.Fatalf("%s as the bot's seat: got %v, want a refusal", tool, err)
		}
	}

	// The match over, the bot's seat may be looked at.
	playRandomly(t, svc, out.Table, rand.New(rand.NewSource(1)), 5000, func(*Table) bool { return false })
	if done, _ := svc.tables[out.Table].finished(); !done {
		t.Fatal("could not finish the match")
	}
	if _, err := svc.Call("observe", mustJSON(t, ObserveIn{Table: out.Table, Seat: 1})); err != nil {
		t.Fatalf("observing the bot after the match: %v", err)
	}
}
