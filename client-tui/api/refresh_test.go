package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

// tokenServer accepts one access token at a time. A refresh trades the
// current refresh token for the next pair and retires the old one, the way
// the real server rotates them.
type tokenServer struct {
	*httptest.Server
	mu        sync.Mutex
	access    string
	refresh   string
	gen       int
	refreshes atomic.Int32
}

func newTokenServer(t *testing.T) *tokenServer {
	t.Helper()
	ts := &tokenServer{access: "access-1", refresh: "refresh-1", gen: 1}
	up := websocket.Upgrader{}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.mu.Lock()
		access, refresh := ts.access, ts.refresh
		ts.mu.Unlock()
		switch r.URL.Path {
		case "/auth/refresh":
			var body struct {
				RefreshToken string `json:"refreshToken"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.RefreshToken != refresh {
				http.Error(w, "refresh token refused", http.StatusUnauthorized)
				return
			}
			ts.refreshes.Add(1)
			ts.mu.Lock()
			ts.gen++
			ts.access = "access-" + string(rune('0'+ts.gen))
			ts.refresh = "refresh-" + string(rune('0'+ts.gen))
			out := map[string]string{"accessToken": ts.access, "refreshToken": ts.refresh}
			ts.mu.Unlock()
			_ = json.NewEncoder(w).Encode(out)
		case "/users/me/stats":
			if r.Header.Get("Authorization") != "Bearer "+access {
				http.Error(w, "token expired", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"matches":3,"wins":1}`))
		case "/ws/matches/m1":
			if r.URL.Query().Get("token") != access {
				http.Error(w, "token expired", http.StatusUnauthorized)
				return
			}
			c, err := up.Upgrade(w, r, nil)
			if err == nil {
				_ = c.Close()
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// expire makes the server stop accepting the access token it last issued,
// as fifteen minutes passing would.
func (ts *tokenServer) expire() {
	ts.mu.Lock()
	ts.access = "expired-elsewhere"
	ts.mu.Unlock()
}

// An expired access token is renewed from the refresh token and the request
// goes through. The client used to keep the access token it signed in with
// for the life of the session, so fifteen minutes into an account's SSH
// session every authenticated request failed.
func TestAnExpiredTokenIsRefreshed(t *testing.T) {
	ts := newTokenServer(t)
	c := New(ts.URL)
	c.SetAuth("access-1", "refresh-1", "u1")
	ts.expire()

	stats, err := c.GetStats()
	if err != nil {
		t.Fatalf("GetStats after the access token expired: %v", err)
	}
	if stats["matches"] != float64(3) {
		t.Errorf("stats = %v", stats)
	}
	if access, refresh := c.Tokens(); access != "access-2" || refresh != "refresh-2" {
		t.Errorf("tokens = %q, %q; want the rotated pair", access, refresh)
	}
}

// Two requests refused at once spend the refresh token once. Rotating it
// twice would leave the second trade holding a retired token.
func TestConcurrentRefusalsRefreshOnce(t *testing.T) {
	ts := newTokenServer(t)
	c := New(ts.URL)
	c.SetAuth("access-1", "refresh-1", "u1")
	ts.expire()

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.GetStats()
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("GetStats: %v", err)
		}
	}
	if n := ts.refreshes.Load(); n != 1 {
		t.Errorf("refreshed %d times, want 1", n)
	}
}

// The match socket's handshake is refused the same way, and dials again with
// the new token.
func TestDialWSRefreshesAnExpiredToken(t *testing.T) {
	ts := newTokenServer(t)
	c := New(ts.URL)
	c.SetAuth("access-1", "refresh-1", "u1")
	ts.expire()

	conn, err := c.DialWS("m1")
	if err != nil {
		t.Fatalf("DialWS after the access token expired: %v", err)
	}
	_ = conn.Close()
}

// Without a refresh token there is nothing to trade, and the refusal is
// reported rather than looped on.
func TestNoRefreshTokenReportsTheRefusal(t *testing.T) {
	ts := newTokenServer(t)
	c := New(ts.URL)
	c.SetAuth("access-1", "", "u1")
	ts.expire()

	_, err := c.GetStats()
	if err == nil || !strings.Contains(err.Error(), "token expired") {
		t.Fatalf("err = %v, want the server's refusal", err)
	}
	if n := ts.refreshes.Load(); n != 0 {
		t.Errorf("refreshed %d times with no refresh token", n)
	}
}
