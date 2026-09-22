package db

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/models"
)

func TestReservationKeyFoldsCaseForNamesAndAddresses(t *testing.T) {
	if got, want := ReservationKey(ReservationUsername, "Ada"), "uname:ada"; got != want {
		t.Fatalf("username key = %q, want %q", got, want)
	}
	if got, want := ReservationKey(ReservationEmail, "Ada@Example.COM"), "email:ada@example.com"; got != want {
		t.Fatalf("email key = %q, want %q", got, want)
	}
	// Friend codes are generated upper-case and read back literally; folding
	// them would make two codes one claim.
	if got, want := ReservationKey(ReservationFriendCode, "6XDYHB8C"), "friend:6XDYHB8C"; got != want {
		t.Fatalf("friend key = %q, want %q", got, want)
	}
}

func TestKDBInsertUserClaimsItsNameAndAddress(t *testing.T) {
	k := openTestKDB(t)
	ada := models.User{ID: bson.NewObjectID(), Username: "ada", Email: "ada@example.com"}
	if err := KDBInsertUser(k, ada); err != nil {
		t.Fatalf("insert: %v", err)
	}

	owner, err := KDBReservationOwner(k, ReservationUsername, "ADA")
	if err != nil {
		t.Fatalf("username reservation: %v", err)
	}
	if owner != ada.ID.Hex() {
		t.Fatalf("username owner = %q, want %q", owner, ada.ID.Hex())
	}

	// A second account cannot take either value, in any case.
	clash := models.User{ID: bson.NewObjectID(), Username: "Ada", Email: "other@example.com"}
	if err := KDBInsertUser(k, clash); !IsDuplicateKey(err) {
		t.Fatalf("username clash: got %v, want a duplicate-key error", err)
	}
	clash = models.User{ID: bson.NewObjectID(), Username: "grace", Email: "ADA@example.com"}
	if err := KDBInsertUser(k, clash); !IsDuplicateKey(err) {
		t.Fatalf("email clash: got %v, want a duplicate-key error", err)
	}

	// And the refused account must not be half-written: no user document, no
	// claim on the value it was refused.
	if _, err := k.Get(NSUsers, clash.ID.Hex()); !IsNotFound(err) {
		t.Fatalf("refused account was written: %v", err)
	}
	if _, err := KDBReservationOwner(k, ReservationUsername, "grace"); !IsNotFound(err) {
		t.Fatalf("refused account kept a claim on its username: %v", err)
	}
}

func TestKDBFindUserByUsernameReadsTheReservation(t *testing.T) {
	k := openTestKDB(t)
	ada := models.User{ID: bson.NewObjectID(), Username: "ada"}
	if err := KDBInsertUser(k, ada); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := KDBFindUserByUsername(k, "ada")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != ada.ID {
		t.Fatalf("found %s, want %s", got.ID.Hex(), ada.ID.Hex())
	}
	if _, err := KDBFindUserByUsername(k, "grace"); !IsNotFound(err) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestKDBFindUserByUsernameStillScansUnmigratedAccounts(t *testing.T) {
	k := openTestKDB(t)
	// Written the way a pre-reservation database holds it: the account
	// document alone, with nothing claiming its name.
	ada := models.User{ID: bson.NewObjectID(), Username: "ada"}
	doc, err := MarshalDoc(ada)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := k.Put(NSUsers, ada.ID.Hex(), doc); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := KDBFindUserByUsername(k, "ada")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != ada.ID {
		t.Fatalf("found %s, want %s", got.ID.Hex(), ada.ID.Hex())
	}
}

func TestKDBUpdateUserFieldsMovesTheClaimWithTheName(t *testing.T) {
	k := openTestKDB(t)
	ada := models.User{ID: bson.NewObjectID(), Username: "ada"}
	if err := KDBInsertUser(k, ada); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := KDBUpdateUserFields(k, ada.ID, bson.M{"username": "grace"}); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if _, err := KDBReservationOwner(k, ReservationUsername, "ada"); !IsNotFound(err) {
		t.Fatalf("the old name is still claimed: %v", err)
	}
	owner, err := KDBReservationOwner(k, ReservationUsername, "grace")
	if err != nil {
		t.Fatalf("new name: %v", err)
	}
	if owner != ada.ID.Hex() {
		t.Fatalf("new name owner = %q, want %q", owner, ada.ID.Hex())
	}

	// Somebody else may now take the name this account gave up, and may not
	// take the one it holds.
	other := models.User{ID: bson.NewObjectID(), Username: "ada"}
	if err := KDBInsertUser(k, other); err != nil {
		t.Fatalf("taking the freed name: %v", err)
	}
	if err := KDBUpdateUserFields(k, other.ID, bson.M{"username": "grace"}); !IsDuplicateKey(err) {
		t.Fatalf("got %v, want a duplicate-key error", err)
	}
	// A refused rename leaves the account as it was, still holding its name.
	owner, err = KDBReservationOwner(k, ReservationUsername, "ada")
	if err != nil || owner != other.ID.Hex() {
		t.Fatalf("owner of the old name = %q (%v), want %q", owner, err, other.ID.Hex())
	}
}

func TestAccountDataIsMirroredIntoTheAccountsOwnNamespace(t *testing.T) {
	k := openTestKDB(t)
	ada := models.User{
		ID:          bson.NewObjectID(),
		Username:    "ada",
		Preferences: models.UserPreferences{Language: "cs", CardStyle: "classic"},
	}
	if err := KDBInsertUser(k, ada); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// A phone syncs a namespace, not a row, so what this person's devices can
	// hold is exactly what is written under their own name.
	doc, err := k.Get(UserNS(ada.ID.Hex()), PrefsKey)
	if err != nil {
		t.Fatalf("preferences in the account namespace: %v", err)
	}
	var prefs struct {
		Language string `bson:"language"`
		Kind     string `bson:"_kind"`
	}
	if err := UnmarshalDoc(doc, &prefs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if prefs.Language != "cs" {
		t.Fatalf("language = %q, want cs", prefs.Language)
	}
	// The kind is what a merge rule is chosen by on the other side of a sync.
	if prefs.Kind != KindPrefs {
		t.Fatalf("kind = %q, want %q", prefs.Kind, KindPrefs)
	}

	if err := KDBUpdateUserFields(k, ada.ID, bson.M{"preferences": models.UserPreferences{Language: "en"}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	doc, err = k.Get(UserNS(ada.ID.Hex()), PrefsKey)
	if err != nil {
		t.Fatalf("preferences after update: %v", err)
	}
	if err := UnmarshalDoc(doc, &prefs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if prefs.Language != "en" {
		t.Fatalf("language = %q, want en after the update", prefs.Language)
	}
}

func TestGuestsHaveNoAccountNamespace(t *testing.T) {
	if ns, ok := UserNSForSubject("guest:abcd"); ok {
		t.Fatalf("a guest was given the namespace %q", ns)
	}
	ns, ok := UserNSForSubject("user:65f0")
	if !ok || ns != UserNS("65f0") {
		t.Fatalf("subject namespace = %q, %v", ns, ok)
	}
}
