package match

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
	zsync "zolik/server/internal/sync"
)

// A host with no internet seats an account from its offline pass, and records
// that seat as a subject - "user:<hex>" - because the prefix is how it says
// "I decided you were this account". The bundle it hands up must carry that
// as the one subject key the cloud knows, not prefix it a second time: the
// cloud turns a subject key into a namespace name, and "user:user:<hex>" is
// not a name any database will take.
func TestAnOfflineSeatIsHandedUpAsOneSubject(t *testing.T) {
	id := bson.NewObjectID()
	account := bson.NewObjectID().Hex()

	for _, tc := range []struct {
		name   string
		userID string
	}{
		{"seated by the cloud", account},
		{"seated from an offline pass", "user:" + account},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := models.Match{
				ID:       id,
				ModuleID: "blackjack",
				Players:  []models.Player{{ID: "p1", UserID: tc.userID}},
			}
			b, err := BundleOf(context.Background(), noMoves{}, m)
			if err != nil {
				t.Fatalf("BundleOf: %v", err)
			}
			if got, want := b.Seats["p1"], "user:"+account; got != want {
				t.Fatalf("seat p1 handed up as %q, want %q", got, want)
			}
		})
	}
}

// The seats are the one part of a bundle no replay can check, so a node that
// names something which is not an id is refused rather than believed. Before
// this, such a name reached the recorder and became a namespace, and the
// database's answer to an impossible namespace name is to panic - which any
// enrolled phone could therefore do to the cloud.
func TestABundleNamingSomethingThatIsNotAnAccountIsRefused(t *testing.T) {
	account := bson.NewObjectID().Hex()
	i := &Importer{passes: fakePasses{owner: account}}
	envelope := models.Match{ID: bson.NewObjectID(), Players: []models.Player{{ID: "p1"}}}
	bundle := func(subject string) zsync.Bundle {
		return zsync.Bundle{Node: "phone", Seats: map[string]string{"p1": subject}}
	}

	for _, subject := range []string{"user:user:" + bson.NewObjectID().Hex(), "user:../../etc", "guest:not-a-guest-id"} {
		if _, err := i.credit(context.Background(), envelope, bundle(subject)); err == nil {
			t.Errorf("seat %q was credited; it should have been refused", subject)
		}
	}

	// The account that enrolled the hosting node, at its own table.
	out, err := i.credit(context.Background(), envelope, bundle("user:"+account))
	if err != nil {
		t.Fatalf("crediting an ordinary account seat: %v", err)
	}
	if out.Players[0].UserID != account {
		t.Fatalf("seat p1 credited to %q, want %q", out.Players[0].UserID, account)
	}
}

// A phone's word is not enough to put a match into somebody else's account.
// Another account's seat is credited only with the offline pass that seated
// it; without one, it arrives as an anonymous player.
func TestAnAccountSeatIsCreditedOnlyWithItsPass(t *testing.T) {
	owner, other := bson.NewObjectID().Hex(), bson.NewObjectID().Hex()
	i := &Importer{passes: fakePasses{owner: owner, valid: map[string]string{"pass-of-other": other}}}
	envelope := models.Match{ID: bson.NewObjectID(), Players: []models.Player{
		{ID: "p1", Name: "Ada", UserID: "user:" + other},
	}}

	b := zsync.Bundle{Node: "phone", Seats: map[string]string{"p1": "user:" + other}}
	out, err := i.credit(context.Background(), envelope, b)
	if err != nil {
		t.Fatalf("credit: %v", err)
	}
	if got := out.Players[0]; got.UserID != "" || got.GuestID != AnonymousGuestID(envelope.ID.Hex(), "p1") {
		t.Fatalf("a seat with no pass was credited to user %q / guest %q", got.UserID, got.GuestID)
	}

	b.Passes = map[string]string{"p1": "pass-of-other"}
	out, err = i.credit(context.Background(), envelope, b)
	if err != nil {
		t.Fatalf("credit: %v", err)
	}
	if out.Players[0].UserID != other {
		t.Fatalf("a seat with its pass was credited to %q, want %q", out.Players[0].UserID, other)
	}

	// A seat the node did not name at all is nobody, whatever the envelope
	// says it was.
	out, err = i.credit(context.Background(), envelope, zsync.Bundle{Node: "phone", Seats: map[string]string{}})
	if err != nil {
		t.Fatalf("credit: %v", err)
	}
	if out.Players[0].UserID != "" {
		t.Fatalf("an unnamed seat kept the envelope's account %q", out.Players[0].UserID)
	}
}

type fakePasses struct {
	owner string
	valid map[string]string
}

func (f fakePasses) NodeOwner(context.Context, string) (string, error) { return f.owner, nil }

func (f fakePasses) CheckSeatPass(pass, user string, _ time.Time) error {
	if f.valid[pass] != user {
		return errors.New("not a pass for that account")
	}
	return nil
}

// noMoves is a repository for a match whose moves are of no interest here.
type noMoves struct{ Repository }

func (noMoves) Moves(context.Context, bson.ObjectID, int, int) ([]models.MatchAction, error) {
	return nil, nil
}
