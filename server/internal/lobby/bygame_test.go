package lobby

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestIsWaitingAnswersPerGame(t *testing.T) {
	s, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	ctx := context.Background()
	s.Join(ctx, Entry{PlayerID: "holdem-only", Username: "Ana", ModuleIDs: []string{"holdem"}})
	s.Join(ctx, Entry{PlayerID: "anything", Username: "Petr"})

	for _, c := range []struct {
		player, module string
		want           bool
	}{
		{"holdem-only", "holdem", true},
		{"holdem-only", "canasta", false},
		{"holdem-only", "", true},
		{"anything", "canasta", true},
		{"anything", "", true},
	} {
		if _, _, _, ok := s.IsWaiting(ctx, c.player, c.module); ok != c.want {
			t.Errorf("IsWaiting(%s, %q) = %v, want %v", c.player, c.module, ok, c.want)
		}
	}
}

func TestForModuleKeepsPlayersWaitingForThatGameOrAny(t *testing.T) {
	all := []Entry{
		{PlayerID: "a", ModuleIDs: []string{"holdem"}},
		{PlayerID: "b", ModuleIDs: []string{"canasta"}},
		{PlayerID: "c"},
		{PlayerID: "d", ModuleIDs: []string{"canasta", "holdem"}},
	}
	ids := func(es []Entry) (out []string) {
		for _, e := range es {
			out = append(out, e.PlayerID)
		}
		return out
	}
	if got, want := ids(ForModule(all, "holdem")), []string{"a", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ForModule(holdem) = %v, want %v", got, want)
	}
	if got := ids(ForModule(all, "")); len(got) != 4 {
		t.Errorf("ForModule(\"\") = %v, want everyone", got)
	}
}

func TestPlayersSeeOnlyThoseTheyCouldShareATableWith(t *testing.T) {
	holdem := Entry{ModuleIDs: []string{"holdem"}}
	canasta := Entry{ModuleIDs: []string{"canasta"}}
	both := Entry{ModuleIDs: []string{"canasta", "holdem"}}
	anyGame := Entry{}
	for _, c := range []struct {
		name string
		a, b Entry
		want bool
	}{
		{"different games", holdem, canasta, false},
		{"same game", holdem, both, true},
		{"any game sees a hold'em player", anyGame, holdem, true},
		{"a hold'em player sees any game", holdem, anyGame, true},
	} {
		if got := c.a.sharesAGameWith(c.b); got != c.want {
			t.Errorf("%s: sharesAGameWith = %v, want %v", c.name, got, c.want)
		}
		if got := c.b.sharesAGameWith(c.a); got != c.want {
			t.Errorf("%s (reversed): sharesAGameWith = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestModuleIDsFromDropsBlanksDuplicatesAndOversizedIDs(t *testing.T) {
	got := moduleIDsFrom([]string{"holdem", "", "holdem", strings.Repeat("x", maxModuleIDLen+1), "canasta"})
	if want := []string{"holdem", "canasta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("moduleIDsFrom = %v, want %v", got, want)
	}
	many := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		many = append(many, "m"+strings.Repeat("i", i+1))
	}
	if got := moduleIDsFrom(many); len(got) != maxModuleIDs {
		t.Errorf("moduleIDsFrom kept %d ids, want at most %d", len(got), maxModuleIDs)
	}
	if got := moduleIDsFrom(nil); got != nil {
		t.Errorf("moduleIDsFrom(nil) = %v, want none (any game)", got)
	}
}

func TestRedisMirrorKeepsTheGamesAPlayerIsWaitingFor(t *testing.T) {
	a := newRedisBackedStore(t)
	b := newRedisBackedStore(t)
	ctx := context.Background()
	a.Join(ctx, Entry{PlayerID: "bygame-1", Username: "Ana", ModuleIDs: []string{"holdem"}})
	t.Cleanup(func() { a.Leave(context.Background(), "bygame-1") })

	if _, _, _, ok := b.IsWaiting(ctx, "bygame-1", "holdem"); !ok {
		t.Error("another instance does not see a hold'em player as waiting for hold'em")
	}
	if _, _, _, ok := b.IsWaiting(ctx, "bygame-1", "canasta"); ok {
		t.Error("another instance sees a hold'em-only player as waiting for canasta")
	}
	found := false
	for _, e := range ForModule(b.List(ctx), "holdem") {
		if e.PlayerID == "bygame-1" {
			found = true
			if !reflect.DeepEqual(e.ModuleIDs, []string{"holdem"}) {
				t.Errorf("mirrored ModuleIDs = %v, want [holdem]", e.ModuleIDs)
			}
		}
	}
	if !found {
		t.Error("the hold'em list on another instance is missing the player")
	}
}
