package notify

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

// Somebody sitting down is news to everybody already at the table - on their
// socket, and as a push to a phone in a pocket - and to nobody else: not the
// person who sat down, and not a bot.
func TestEverybodyAtTheTableHearsWhoSatDown(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.RegisterDevice(ctx, anna, DeviceRegistration{Kind: DeviceExpo, Token: "ExponentPushToken[anna]", Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	table := models.Match{
		ID: bson.NewObjectID(), ModuleID: "canasta", Status: "lobby", JoinCode: "JOIN42",
		Players: []models.Player{seat(anna), {ID: "bot-1", Name: "Bot", IsAI: true}, seat(guest), seat(bob)},
	}
	f.svc.PlayerJoined(table, seat(bob))

	if f.hub.to(KeyFor(anna), "table_joined") != 1 || f.hub.to(KeyFor(guest), "table_joined") != 1 {
		t.Fatal("the people already seated should hear who sat down")
	}
	if f.hub.to(KeyFor(bob), "table_joined") != 0 {
		t.Fatal("Bob was told that he sat down")
	}
	f.waitPushes(t, 1)
	got := f.sender.got[0]
	if got.Title == "" || got.Title == "notify.push.joinedTitle" || got.URL != "https://jokerless.test/join/JOIN42" {
		t.Fatalf("push: %+v", got)
	}
	if got.Data["type"] != "table_joined" || got.Data["name"] != seat(bob).Name {
		t.Fatalf("push data: %+v", got.Data)
	}
}
