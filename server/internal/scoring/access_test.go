package scoring

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Before: the routes were mounted bare. Anybody could create, read or
// overwrite any pad by id, and create stored no owner, so deleting an account
// never deleted its pads.

func created(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := call(h, http.MethodPost, "/scoring-sessions", `{"players":["A","B"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.ID == "" {
		t.Fatalf("create answered no id: %s", rec.Body)
	}
	return out.ID
}

func TestTheScorepadNeedsAnAccount(t *testing.T) {
	h, repo := scorepad(t)
	if rec := callWith(h, "", http.MethodPost, "/scoring-sessions", `{"players":["A","B"]}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous create: %d, want 401", rec.Code)
	}
	if rec := callWith(h, tokenFor(t, "0123456789abcdef0123456789abcdef", true), http.MethodPost, "/scoring-sessions", `{"players":["A","B"]}`); rec.Code != http.StatusForbidden {
		t.Errorf("guest create: %d, want 403", rec.Code)
	}
	if len(repo.s) != 0 {
		t.Fatal("a pad was stored without an account")
	}
	id := created(t, h)
	if rec := callWith(h, "", http.MethodGet, "/scoring-sessions/"+id, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous read: %d, want 401", rec.Code)
	}
}

func TestAScorepadBelongsToWhoeverMadeIt(t *testing.T) {
	h, repo := scorepad(t)
	id := created(t, h)
	oid, _ := bson.ObjectIDFromHex(id)
	if repo.s[oid].OwnerUser != ownerID {
		t.Fatalf("owner = %q, want %q", repo.s[oid].OwnerUser, ownerID)
	}

	stranger := tokenFor(t, bson.NewObjectID().Hex(), false)
	for _, r := range []struct{ method, path, body string }{
		{http.MethodGet, "/scoring-sessions/" + id, ""},
		{http.MethodGet, "/scoring-sessions/" + id + "/export", ""},
		{http.MethodPatch, "/scoring-sessions/" + id, `{"round":1,"scoresArr":[99,99]}`},
	} {
		if rec := callWith(h, stranger, r.method, r.path, r.body); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s by another account: %d, want 404", r.method, r.path, rec.Code)
		}
	}
	if repo.s[oid].Players[0].Scores[0] != 0 {
		t.Fatal("another account wrote to the pad")
	}

	// Deleting the account takes the pad with it.
	_ = repo.DeleteForOwner(context.Background(), ownerID)
	if len(repo.s) != 0 {
		t.Fatal("the owner's pad survived the owner")
	}
}

func TestAPatchAgainstAnOldReadingIsRefused(t *testing.T) {
	h, repo := scorepad(t)
	id := created(t, h) // version 1

	first := call(h, http.MethodPatch, "/scoring-sessions/"+id, `{"round":1,"version":1,"scoresArr":[5,6]}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first patch: %d %s", first.Code, first.Body)
	}
	var after struct {
		Version int `json:"version"`
	}
	_ = json.Unmarshal(first.Body.Bytes(), &after)
	if after.Version != 2 {
		t.Fatalf("version after one patch = %d, want 2", after.Version)
	}

	// A second device still holding version 1.
	stale := call(h, http.MethodPatch, "/scoring-sessions/"+id, `{"round":1,"version":1,"scoresArr":[0,0]}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale patch: %d, want 409", stale.Code)
	}
	oid, _ := bson.ObjectIDFromHex(id)
	if got := repo.s[oid].Players[0].Scores[0]; got != 5 {
		t.Fatalf("the stale patch overwrote the newer score: %d", got)
	}
}
