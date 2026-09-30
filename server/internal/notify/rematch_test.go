package notify

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/models"
)

func seat(uc auth.UserContext) models.Player {
	p := models.Player{ID: uc.UserID, Name: uc.Username}
	if uc.IsGuest {
		p.GuestID = uc.UserID
	} else {
		p.UserID = uc.UserID
	}
	return p
}

func rematchTable() models.Match {
	return models.Match{ID: bson.NewObjectID(), ModuleID: "canasta", Status: "lobby", JoinCode: "REM123", RematchOf: "old-table"}
}

// Somebody who already left the finished table still hears about the rematch
// holding their seat — with no circle between them, because they just played
// together — unless they switched invites off.
func TestARematchReachesEveryHeldSeatThatWantsInvites(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.RegisterDevice(ctx, bob, DeviceRegistration{Kind: DeviceExpo, Token: "ExponentPushToken[bob]", Locale: "de"}); err != nil {
		t.Fatal(err)
	}
	off := InvitesOff
	if _, err := f.svc.UpdateProfile(ctx, guest, &off, nil); err != nil {
		t.Fatal(err)
	}

	next := rematchTable()
	f.svc.RematchOpened(next, seat(anna), []models.Player{seat(bob), seat(guest)})

	if f.hub.to(KeyFor(bob), "table_invite") != 1 {
		t.Fatal("Bob is not in Anna's circle but played with her; he should be told")
	}
	if f.hub.to(KeyFor(guest), "table_invite") != 0 {
		t.Fatal("the guest switched invites off")
	}
	f.waitPushes(t, 1)
	got := f.sender.got[0]
	if got.Title == "" || got.Title == "notify.push.rematchTitle" || got.URL != "https://jokerless.test/join/REM123" {
		t.Fatalf("push: %+v", got)
	}
	inv, _ := got.Data["invite"].(Invite)
	if inv.RematchOf != "old-table" {
		t.Fatalf("the invite should say it is a rematch: %+v", got.Data)
	}
}

// A mute is the reader's standing answer to this host, rematch or not.
func TestARematchRespectsAMute(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Mute(ctx, bob, KeyFor(anna), true); err != nil {
		t.Fatal(err)
	}
	f.svc.RematchOpened(rematchTable(), seat(anna), []models.Player{seat(bob)})
	if f.hub.to(KeyFor(bob), "table_invite") != 0 {
		t.Fatal("Bob muted Anna")
	}
}

// Letting one seat go withdraws that person's invite and nobody else's; the
// table being dealt withdraws the rest.
func TestReleasingAHeldSeatWithdrawsOnlyThatInvite(t *testing.T) {
	f := newFixture(t)
	next := rematchTable()
	f.svc.RematchOpened(next, seat(anna), []models.Player{seat(bob), seat(carol)})

	f.svc.HeldSeatReleased(next.ID.Hex(), seat(bob))
	if f.hub.to(KeyFor(bob), "invite_revoked") != 1 || f.hub.to(KeyFor(carol), "invite_revoked") != 0 {
		t.Fatal("only Bob's invite should be withdrawn")
	}
	f.svc.HeldSeatReleased(next.ID.Hex(), seat(bob)) // twice is harmless
	if f.hub.to(KeyFor(bob), "invite_revoked") != 1 {
		t.Fatal("a second release must not revoke again")
	}

	f.svc.LobbyClosed(next.ID.Hex())
	if f.hub.to(KeyFor(carol), "invite_revoked") != 1 || f.hub.to(KeyFor(bob), "invite_revoked") != 1 {
		t.Fatal("dealing should withdraw Carol's invite, and not Bob's a second time")
	}
}

// The rematch's own invites are not the host's circle being told: announcing
// the table afterwards still reaches the circle, and a host with nobody in it
// hears "nobody to tell", not "they already know".
func TestARematchIsNotACircleAnnouncement(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.table(anna)
	next := f.tables[id]
	next.RematchOf = "old-table"
	f.svc.RematchOpened(next, seat(anna), []models.Player{seat(bob)})

	n, already, err := f.svc.Announce(ctx, anna, id, nil)
	if err != nil || n != 0 || already {
		t.Fatalf("a host with no circle: told %d, already %v, err %v; want 0, false", n, already, err)
	}
	if _, err := f.svc.Add(ctx, anna, KeyFor(guest), ""); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := f.svc.Announce(ctx, anna, id, nil); n != 1 {
		t.Fatalf("the guest joined the circle and should now be told, told %d", n)
	}
	// And Bob, told by the rematch, is not told twice.
	if f.hub.to(KeyFor(bob), "table_invite") != 1 {
		t.Fatal("bob was told twice")
	}
}
