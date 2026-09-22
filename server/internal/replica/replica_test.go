package replica

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/db"
)

const user = "65f0c0ffeec0ffeec0ffee01"

func openReplica(t *testing.T) (*db.KDB, *Reader) {
	t.Helper()
	k, err := db.OpenKDB(t.TempDir())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = k.Close(context.Background()) })
	return k, NewReader(k, user)
}

func seed(t *testing.T, k *db.KDB, namespace, key, doc string) {
	t.Helper()
	if err := k.Put(namespace, key, []byte(doc)); err != nil {
		t.Fatalf("seeding %s/%s: %v", namespace, key, err)
	}
}

func TestADeviceWithNothingSyncedSaysSoRatherThanSayingEmpty(t *testing.T) {
	k, rd := openReplica(t)
	if rd.Ready() {
		t.Fatal("a device that has never synced reported a replica")
	}
	if _, err := rd.Prefs(); err != ErrNoReplica {
		t.Fatalf("prefs = %v, want ErrNoReplica", err)
	}
	// The difference matters: "we have not synced" and "you have nobody in
	// your circle" are different things to put on a screen.
	seed(t, k, db.UserNS(user), db.PrefsKey, `{"_kind":"prefs","language":"cs"}`)
	if !rd.Ready() {
		t.Fatal("a device holding this account's namespace reported no replica")
	}
	circle, err := rd.Circle()
	if err != nil {
		t.Fatalf("circle: %v", err)
	}
	if len(circle) != 0 {
		t.Fatalf("circle = %v, want empty", circle)
	}
}

func TestTheReplicaServesOnlyTheKindAsked(t *testing.T) {
	k, rd := openReplica(t)
	seed(t, k, db.UserNS(user), db.PrefsKey, `{"_kind":"prefs","language":"cs"}`)
	seed(t, k, db.UserNS(user), "circle/user:aaa", `{"_kind":"circle","memberKey":"user:aaa"}`)
	seed(t, k, db.UserNS(user), "circle/user:bbb", `{"_kind":"circle","memberKey":"user:bbb"}`)
	seed(t, k, db.UserNS(user), "device/d1", `{"_kind":"device","id":"d1"}`)
	seed(t, k, db.UserNS(user), "scoring/s1", `{"_kind":"scoring","rounds":[]}`)

	circle, err := rd.Circle()
	if err != nil {
		t.Fatalf("circle: %v", err)
	}
	if len(circle) != 2 {
		t.Fatalf("circle = %d entries, want 2 (the device and the scorepad are not friends)", len(circle))
	}
	devices, err := rd.Devices()
	if err != nil || len(devices) != 1 {
		t.Fatalf("devices = %v (%v), want one", devices, err)
	}
	pads, err := rd.Scoring()
	if err != nil || len(pads) != 1 {
		t.Fatalf("scoring = %v (%v), want one", pads, err)
	}
}

func TestMatchHistoryIsNewestFirst(t *testing.T) {
	k, rd := openReplica(t)
	seed(t, k, db.UserReadOnlyNS(user), "matches/aaa", `{"_kind":"match","matchId":"aaa","completedAt":"2026-01-01T10:00:00Z"}`)
	seed(t, k, db.UserReadOnlyNS(user), "matches/bbb", `{"_kind":"match","matchId":"bbb","completedAt":"2026-03-01T10:00:00Z"}`)
	seed(t, k, db.UserReadOnlyNS(user), "matches/ccc", `{"_kind":"match","matchId":"ccc","completedAt":"2026-02-01T10:00:00Z"}`)

	rows, err := rd.Matches()
	if err != nil {
		t.Fatalf("matches: %v", err)
	}
	var ids []string
	for _, row := range rows {
		var entry struct {
			MatchID string `json:"matchId"`
		}
		if err := json.Unmarshal(row, &entry); err != nil {
			t.Fatalf("decode: %v", err)
		}
		ids = append(ids, entry.MatchID)
	}
	want := []string{"bbb", "ccc", "aaa"}
	for i := range want {
		if i >= len(ids) || ids[i] != want[i] {
			t.Fatalf("history = %v, want %v", ids, want)
		}
	}
}

func TestTheLocalRoutesServeWhatThisDeviceHolds(t *testing.T) {
	k, rd := openReplica(t)
	seed(t, k, db.UserNS(user), db.PrefsKey, `{"_kind":"prefs","language":"cs"}`)
	seed(t, k, db.UserReadOnlyNS(user), "stats", `{"_kind":"stats","overall":{"matches":3}}`)

	r := chi.NewRouter()
	NewHandlers(func() *Reader { return rd }).RegisterRoutes(r)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	res, err := http.Get(server.URL + "/me/stats")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var stats struct {
		Overall struct {
			Matches int `json:"matches"`
		} `json:"overall"`
	}
	if err := json.NewDecoder(res.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.Overall.Matches != 3 {
		t.Fatalf("matches = %d, want the synced figure", stats.Overall.Matches)
	}

	// A device that is not signed in answers with "not synced" rather than
	// with somebody's data or with an empty document.
	empty := chi.NewRouter()
	NewHandlers(func() *Reader { return NewReader(k, "") }).RegisterRoutes(empty)
	emptyServer := httptest.NewServer(empty)
	t.Cleanup(emptyServer.Close)
	res2, err := http.Get(emptyServer.URL + "/me/prefs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res2.StatusCode)
	}
}
