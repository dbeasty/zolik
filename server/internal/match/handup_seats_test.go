package match

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
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
	i := &Importer{}
	envelope := models.Match{ID: bson.NewObjectID(), Players: []models.Player{{ID: "p1"}}}

	for _, subject := range []string{"user:user:" + bson.NewObjectID().Hex(), "user:../../etc", "guest:not-a-guest-id"} {
		if _, err := i.credit(context.Background(), envelope, map[string]string{"p1": subject}); err == nil {
			t.Errorf("seat %q was credited; it should have been refused", subject)
		}
	}

	account := bson.NewObjectID().Hex()
	out, err := i.credit(context.Background(), envelope, map[string]string{"p1": "user:" + account})
	if err != nil {
		t.Fatalf("crediting an ordinary account seat: %v", err)
	}
	if out.Players[0].UserID != account {
		t.Fatalf("seat p1 credited to %q, want %q", out.Players[0].UserID, account)
	}
}

// noMoves is a repository for a match whose moves are of no interest here.
type noMoves struct{ Repository }

func (noMoves) Moves(context.Context, bson.ObjectID, int, int) ([]models.MatchAction, error) {
	return nil, nil
}
