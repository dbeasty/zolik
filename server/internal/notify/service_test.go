package notify

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/stats"
	"zolik/server/internal/ws"
)

// ---- fakes ---------------------------------------------------------------

type fakeHistory struct {
	matches map[string][]stats.MatchResult
}

func (f fakeHistory) ListMatchesForSubject(_ context.Context, key string, _ time.Time, _ int) ([]stats.MatchResult, error) {
	return f.matches[key], nil
}

type fakeAccounts struct{ users []models.User }

func (f fakeAccounts) FindByUsername(_ context.Context, name string) (models.User, error) {
	for _, u := range f.users {
		if u.Username == name {
			return u, nil
		}
	}
	return models.User{}, errors.New("no such user")
}

func (f fakeAccounts) FindByID(_ context.Context, id bson.ObjectID) (models.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return models.User{}, errors.New("no such user")
}

type fakeTables map[string]models.Match

func (f fakeTables) Resolve(_ context.Context, id string) (models.Match, error) {
	if m, ok := f[id]; ok {
		return m, nil
	}
	return models.Match{}, errors.New("no such table")
}

type sent struct {
	room, key string
	payload   map[string]any
}

type fakeHub struct {
	mu  sync.Mutex
	out []sent
}

func (h *fakeHub) Publish(room string, msgs []ws.PlayerMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range msgs {
		h.out = append(h.out, sent{room: room, key: m.PlayerID, payload: m.Payload.(map[string]any)})
	}
}

func (h *fakeHub) to(key, typ string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, s := range h.out {
		if s.key == key && s.payload["type"] == typ {
			n++
		}
	}
	return n
}

type fakeSender struct {
	mu    sync.Mutex
	got   []Push
	gone  map[string]bool
	calls chan struct{}
}

func (f *fakeSender) Send(_ context.Context, d Device, p Push) error {
	f.mu.Lock()
	f.got = append(f.got, p)
	gone := f.gone[d.ID]
	f.mu.Unlock()
	f.calls <- struct{}{}
	if gone {
		return ErrDeviceGone
	}
	return nil
}

// ---- fixture -------------------------------------------------------------

type fixture struct {
	svc    *Service
	repo   Repository
	hub    *fakeHub
	sender *fakeSender
	tables fakeTables
	clock  time.Time
}

var (
	anna  = auth.UserContext{UserID: bson.NewObjectID().Hex(), Username: "Anna"}
	bob   = auth.UserContext{UserID: bson.NewObjectID().Hex(), Username: "Bob"}
	carol = auth.UserContext{UserID: bson.NewObjectID().Hex(), Username: "Carol"}
	guest = auth.UserContext{UserID: "g-device-1", Username: "Guest 12", IsGuest: true}
)

func played(at time.Time, who ...auth.UserContext) stats.MatchResult {
	m := stats.MatchResult{ID: bson.NewObjectID(), MatchID: bson.NewObjectID(), CompletedAt: at}
	for _, uc := range who {
		kind := stats.SubjectUser
		if uc.IsGuest {
			kind = stats.SubjectGuest
		}
		sub := stats.Subject{Kind: kind, ID: uc.UserID, Name: uc.Username}
		m.Participants = append(m.Participants, stats.Standing{PlayerID: uc.UserID, Subject: sub, Name: uc.Username})
		m.SubjectKeys = append(m.SubjectKeys, sub.Key())
	}
	// A bot at the same table is never somebody to notify.
	bot := stats.Subject{Kind: stats.SubjectAI, ID: "hard:miroslav", Name: "Miroslav"}
	m.Participants = append(m.Participants, stats.Standing{PlayerID: "bot:1", Subject: bot})
	m.SubjectKeys = append(m.SubjectKeys, bot.Key())
	return m
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening kdb: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	repo := NewKDBRepository(k)

	at := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	game := played(at, anna, bob, guest)
	history := fakeHistory{matches: map[string][]stats.MatchResult{
		KeyFor(anna):  {game},
		KeyFor(bob):   {game},
		KeyFor(guest): {game},
	}}
	accounts := fakeAccounts{}
	for _, uc := range []auth.UserContext{anna, bob, carol} {
		id, _ := bson.ObjectIDFromHex(uc.UserID)
		accounts.users = append(accounts.users, models.User{ID: id, Username: uc.Username})
	}
	f := &fixture{
		repo:   repo,
		hub:    &fakeHub{},
		sender: &fakeSender{gone: map[string]bool{}, calls: make(chan struct{}, 100)},
		tables: fakeTables{},
		clock:  at.Add(24 * time.Hour),
	}
	f.svc = NewService(repo, history, accounts, f.tables, f.hub,
		Senders{DeviceExpo: f.sender, DeviceWebPush: f.sender},
		Config{PublicBaseURL: "https://jokerless.test", VAPIDPublicKey: "k", ExpoEnabled: true})
	f.svc.now = func() time.Time { return f.clock }
	return f
}

func (f *fixture) table(host auth.UserContext, seated ...auth.UserContext) string {
	id := bson.NewObjectID()
	m := models.Match{ID: id, ModuleID: "canasta", Status: "lobby", HostID: host.UserID, JoinCode: "ABC123"}
	for _, uc := range append([]auth.UserContext{host}, seated...) {
		p := models.Player{ID: uc.UserID, Name: uc.Username}
		if uc.IsGuest {
			p.GuestID = uc.UserID
		} else {
			p.UserID = uc.UserID
		}
		m.Players = append(m.Players, p)
	}
	f.tables[id.Hex()] = m
	return id.Hex()
}

func (f *fixture) waitPushes(t *testing.T, n int) {
	t.Helper()
	for range n {
		select {
		case <-f.sender.calls:
		case <-time.After(2 * time.Second):
			t.Fatalf("waited for %d pushes", n)
		}
	}
}

func code(err error) string {
	var me module.Error
	if errors.As(err, &me) {
		return me.Code
	}
	return ""
}

// ---- tests ---------------------------------------------------------------

func TestSuggestionsAreHumansYouPlayedMinusYourCircle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	got, err := f.svc.Suggestions(ctx, anna)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want Bob and the guest, got %+v", got)
	}
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	got, _ = f.svc.Suggestions(ctx, anna)
	if len(got) != 1 || got[0].Key != KeyFor(guest) {
		t.Fatalf("after adding Bob only the guest is left to suggest, got %+v", got)
	}
}

func TestAddingByKeyNeedsASharedTable(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.Add(context.Background(), anna, KeyFor(carol), "")
	if code(err) != "NOT_PLAYED_TOGETHER" {
		t.Fatalf("a stranger by key must be refused, got %v", err)
	}
	if _, err := f.svc.Add(context.Background(), anna, KeyFor(anna), ""); code(err) != "CANNOT_ADD_SELF" {
		t.Fatalf("adding yourself: %v", err)
	}
}

func TestAddingByUsernameIsARequestUntilAccepted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	e, err := f.svc.Add(ctx, anna, "", "Carol")
	if err != nil || e.Status != EdgePending {
		t.Fatalf("want a pending request, got %+v %v", e, err)
	}
	if f.hub.to(KeyFor(carol), "circle_changed") != 1 {
		t.Fatal("Carol should hear that a request arrived")
	}
	table := f.table(anna)
	if n, _, _ := f.svc.Announce(ctx, anna, table, nil); n != 0 {
		t.Fatalf("a pending request must not receive invites, told %d", n)
	}

	c, _ := f.svc.Circle(ctx, carol)
	if len(c.Requests) != 1 || c.Requests[0].Name != "Anna" {
		t.Fatalf("Carol's requests: %+v", c.Requests)
	}
	if _, err := f.svc.Accept(ctx, carol, KeyFor(anna)); err != nil {
		t.Fatal(err)
	}
	c, _ = f.svc.Circle(ctx, carol)
	if len(c.Members) != 1 || len(c.Notifiers) != 1 {
		t.Fatalf("accepting links both ways, got %+v", c)
	}
}

func TestFriendLinkLinksBothWays(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, err := f.svc.Profile(ctx, carol)
	if err != nil {
		t.Fatal(err)
	}
	if p.FriendURL != "https://jokerless.test/add/"+p.FriendCode {
		t.Fatalf("friend url: %q", p.FriendURL)
	}
	if name, _, err := f.svc.FriendPreview(ctx, p.FriendCode); err != nil || name != "Carol" {
		t.Fatalf("preview: %q %v", name, err)
	}
	if _, err := f.svc.AddByFriendCode(ctx, anna, p.FriendCode); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{KeyFor(anna), KeyFor(carol)}, {KeyFor(carol), KeyFor(anna)}} {
		e, err := f.repo.GetEdge(ctx, pair[0], pair[1])
		if err != nil || e.Status != EdgeActive {
			t.Fatalf("%s>%s: %+v %v", pair[0], pair[1], e, err)
		}
	}
	if _, err := f.svc.AddByFriendCode(ctx, carol, p.FriendCode); code(err) != "CANNOT_ADD_SELF" {
		t.Fatalf("own code: %v", err)
	}
}

func TestAnnounceReachesTheCircleOnceAndRespectsTheirSwitches(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, k := range []string{KeyFor(bob), KeyFor(guest)} {
		if _, err := f.svc.Add(ctx, anna, k, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.RegisterDevice(ctx, bob, DeviceRegistration{Kind: DeviceExpo, Token: "ExponentPushToken[bob]", Locale: "cs"}); err != nil {
		t.Fatal(err)
	}
	// The guest has turned invites off.
	off := InvitesOff
	if _, err := f.svc.UpdateProfile(ctx, guest, &off, nil); err != nil {
		t.Fatal(err)
	}

	table := f.table(anna)
	n, already, err := f.svc.Announce(ctx, anna, table, nil)
	if err != nil || n != 1 || already {
		t.Fatalf("want Bob alone told, got %d %v %v", n, already, err)
	}
	if f.hub.to(KeyFor(bob), "table_invite") != 1 || f.hub.to(KeyFor(guest), "table_invite") != 0 {
		t.Fatal("socket invites went to the wrong people")
	}
	f.waitPushes(t, 1)
	if got := f.sender.got[0]; got.Title == "" || got.URL != "https://jokerless.test/join/ABC123" {
		t.Fatalf("push: %+v", got)
	}

	n, already, _ = f.svc.Announce(ctx, anna, table, nil)
	if n != 0 || !already {
		t.Fatalf("a repeat must tell nobody new, got %d %v", n, already)
	}
}

func TestMutedNotifierIsSkippedAndTheMuteSurvivesRemoval(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Mute(ctx, bob, KeyFor(anna), true); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := f.svc.Announce(ctx, anna, f.table(anna), nil); n != 0 {
		t.Fatalf("muted, yet told %d", n)
	}
	// Anna removes and re-adds Bob: still muted.
	if err := f.svc.Remove(ctx, anna, KeyFor(bob)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(time.Hour)
	if n, _, _ := f.svc.Announce(ctx, anna, f.table(anna), nil); n != 0 {
		t.Fatalf("re-adding must not undo a mute, told %d", n)
	}
	c, _ := f.svc.Circle(ctx, bob)
	if len(c.Notifiers) != 1 || c.Notifiers[0].Muted == nil || !*c.Notifiers[0].Muted {
		t.Fatalf("Bob's notifiers: %+v", c.Notifiers)
	}
}

func TestAnnounceLimits(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Announce(ctx, bob, f.table(anna), nil); code(err) != "NOT_THE_HOST" {
		t.Fatalf("non-host: %v", err)
	}

	// A second table inside ten minutes does not reach the same person again.
	if n, _, _ := f.svc.Announce(ctx, anna, f.table(anna), nil); n != 1 {
		t.Fatal("first table should reach Bob")
	}
	f.clock = f.clock.Add(time.Minute)
	if n, _, _ := f.svc.Announce(ctx, anna, f.table(anna), nil); n != 0 {
		t.Fatal("second table within the cooldown should not")
	}
	// And a host cannot announce a seventh table in an hour.
	for range hostTablesPerHour - 2 {
		if _, _, err := f.svc.Announce(ctx, anna, f.table(anna), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := f.svc.Announce(ctx, anna, f.table(anna), nil); code(err) != "RATE_LIMITED" {
		t.Fatalf("want RATE_LIMITED, got %v", err)
	}
	f.clock = f.clock.Add(time.Hour)
	if _, _, err := f.svc.Announce(ctx, anna, f.table(anna), nil); err != nil {
		t.Fatalf("an hour later the host may announce again: %v", err)
	}
}

func TestSeatedPlayersAreNotInvited(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	if n, _, _ := f.svc.Announce(ctx, anna, f.table(anna, bob), nil); n != 0 {
		t.Fatal("Bob is already at the table")
	}
}

func TestLobbyClosedRevokesAndDropsDeadDevices(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Add(ctx, anna, KeyFor(bob), ""); err != nil {
		t.Fatal(err)
	}
	id, err := f.svc.RegisterDevice(ctx, bob, DeviceRegistration{Kind: DeviceExpo, Token: "ExponentPushToken[bob]"})
	if err != nil {
		t.Fatal(err)
	}
	table := f.table(anna)
	if n, _, _ := f.svc.Announce(ctx, anna, table, nil); n != 1 {
		t.Fatal("Bob should be told")
	}
	f.waitPushes(t, 1)

	f.sender.mu.Lock()
	f.sender.gone[id] = true
	f.sender.mu.Unlock()
	f.svc.LobbyClosed(table)
	if f.hub.to(KeyFor(bob), "invite_revoked") != 1 {
		t.Fatal("Bob's invite should be withdrawn over the socket")
	}
	f.waitPushes(t, 1)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if ds, _ := f.repo.DevicesFor(ctx, KeyFor(bob)); len(ds) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a device the push service disowned should be deleted")
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.svc.LobbyClosed(table) // twice is harmless
	if f.hub.to(KeyFor(bob), "invite_revoked") != 1 {
		t.Fatal("a second close must not revoke again")
	}
}

func TestRegisterDeviceRefusesWhatIsNotConfigured(t *testing.T) {
	f := newFixture(t)
	f.svc.vapidPublicKey = ""
	_, err := f.svc.RegisterDevice(context.Background(), bob, DeviceRegistration{
		Kind: DeviceWebPush, Subscription: &Subscription{Endpoint: "https://push.example/1"},
	})
	if code(err) != "PUSH_UNAVAILABLE" {
		t.Fatalf("want PUSH_UNAVAILABLE, got %v", err)
	}
	if f.svc.PublicConfig().VAPIDPublicKey != nil {
		t.Fatal("config must say browsers cannot subscribe")
	}
}

func TestRegisteringTheSameDeviceTwiceKeepsOneRow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	reg := DeviceRegistration{Kind: DeviceExpo, Token: "ExponentPushToken[x]"}
	a, _ := f.svc.RegisterDevice(ctx, guest, reg)
	// The same phone, now signed in as Bob, moves rather than duplicates.
	b, _ := f.svc.RegisterDevice(ctx, bob, reg)
	if a != b {
		t.Fatal("same token, same id")
	}
	if ds, _ := f.repo.DevicesFor(ctx, KeyFor(guest)); len(ds) != 0 {
		t.Fatal("the guest should no longer be reachable on Bob's phone")
	}
	if ds, _ := f.repo.DevicesFor(ctx, KeyFor(bob)); len(ds) != 1 {
		t.Fatal("Bob should be")
	}
}

func TestRekeyCarriesAGuestsCircleToTheirAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// Anna tells the guest about tables; the guest has muted Bob.
	if _, err := f.svc.Add(ctx, anna, KeyFor(guest), ""); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Mute(ctx, guest, KeyFor(bob), true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RegisterDevice(ctx, guest, DeviceRegistration{Kind: DeviceExpo, Token: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Rekey(ctx, guest.UserID, carol.UserID); err != nil {
		t.Fatal(err)
	}
	if e, err := f.repo.GetEdge(ctx, KeyFor(anna), KeyFor(carol)); err != nil || e.Status != EdgeActive {
		t.Fatalf("Anna's edge should now reach Carol: %+v %v", e, err)
	}
	if e, err := f.repo.GetEdge(ctx, KeyFor(bob), KeyFor(carol)); err != nil || !e.Muted {
		t.Fatalf("the mute should come along: %+v %v", e, err)
	}
	if _, err := f.repo.GetEdge(ctx, KeyFor(anna), KeyFor(guest)); !errors.Is(err, ErrNotFound) {
		t.Fatal("nothing should be left under the guest key")
	}
	if ds, _ := f.repo.DevicesFor(ctx, KeyFor(carol)); len(ds) != 1 {
		t.Fatal("the device should move too")
	}
}

func TestPushTextIsLocalised(t *testing.T) {
	if got := text("xx", "notify.push.inviteTitle", map[string]string{"host": "Anna"}); got == "notify.push.inviteTitle" || got == "" {
		t.Fatalf("unknown locale should fall back to English, got %q", got)
	}
	if baseLocale("pt-BR") != "pt" || baseLocale("de_AT") != "de" {
		t.Fatal("regional locales fall back to their language")
	}
	if gameName("en", "no-such-game") != "no-such-game" {
		t.Fatal("an unworded game is named by its id")
	}
}
