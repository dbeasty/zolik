package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// The scorepad belongs to the account that made it, so every call must carry
// the session; before, all four were sent bare.
func TestScorepadCallsCarryTheSession(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"p1"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	c.SetAuth("tok", "", "")
	id, err := c.CreateScoringSession([]string{"A", "B"})
	if err != nil || id != "p1" {
		t.Fatalf("create: %q %v", id, err)
	}
	if _, err := c.GetScoringSession(id); err != nil {
		t.Fatal(err)
	}
	if err := c.PatchScoringSession(id, 1, map[string]int{"A": 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExportScoringSession(id); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST Bearer tok", "GET Bearer tok", "PATCH Bearer tok", "GET Bearer tok"}
	for i := range want {
		if i >= len(seen) || seen[i] != want[i] {
			t.Fatalf("requests = %v, want %v", seen, want)
		}
	}
}
