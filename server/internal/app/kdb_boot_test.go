package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/db"
)

// TestNewBootsOnKDBWithoutMongoOrRedis is the single-binary claim, tested:
// with FEATURE_FLAG_DB_ENGINE=kdb the app must come up and serve real
// traffic with no MongoDB and no Redis anywhere — the whole point of the KDB
// deployment shape. It signs in a guest and takes a match from creation to
// readback, which exercises sessions, matches and the auth middleware
// against the embedded engine end to end.
func TestNewBootsOnKDBWithoutMongoOrRedis(t *testing.T) {
	a, err := New(Config{
		DBEngine: db.EngineKDB,
		KDBPath:  t.TempDir(),
		// No MongoURI, no RedisURL: nothing to dial is the test.
	})
	if err != nil {
		t.Fatalf("app.New on kdb: %v", err)
	}
	t.Cleanup(func() { _ = a.Close(context.Background()) })

	r := chi.NewRouter()
	a.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	post := func(path, token string, body map[string]any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer res.Body.Close()
		var out map[string]any
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatalf("POST %s: decoding (status %d): %v", path, res.StatusCode, err)
		}
		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
			t.Fatalf("POST %s: status %d body %v", path, res.StatusCode, out)
		}
		return out
	}

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v (status %v)", err, res)
	}
	res.Body.Close()

	guest := post("/auth/guest", "", map[string]any{"guestName": "KdbSmoke"})
	token, _ := guest["accessToken"].(string)
	if token == "" {
		t.Fatalf("guest sign-in returned no access token: %v", guest)
	}

	created := post("/matches", token, map[string]any{"moduleId": "prsi"})
	matchID, _ := created["matchId"].(string)
	if matchID == "" {
		t.Fatalf("create match returned no id: %v", created)
	}

	got, err := http.Get(srv.URL + "/matches/" + matchID)
	if err != nil {
		t.Fatalf("GET match: %v", err)
	}
	defer got.Body.Close()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("GET match: status %d", got.StatusCode)
	}
	var m map[string]any
	if err := json.NewDecoder(got.Body).Decode(&m); err != nil {
		t.Fatalf("GET match: decode: %v", err)
	}
	if m["moduleId"] != "prsi" {
		t.Fatalf("match read back moduleId = %v, want prsi", m["moduleId"])
	}
}
