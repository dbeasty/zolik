package zolikcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type recordingSink struct {
	fetches  chan fetchResult
	opened   chan int64
	messages chan string
	closed   chan int
}

type fetchResult struct {
	status  int
	headers map[string]string
	body    string
	err     string
}

func newSink() *recordingSink {
	return &recordingSink{
		fetches:  make(chan fetchResult, 4),
		opened:   make(chan int64, 4),
		messages: make(chan string, 16),
		closed:   make(chan int, 4),
	}
}

func (s *recordingSink) FetchDone(_ int64, status int, headersJSON string, body []byte, errMsg string) {
	h := map[string]string{}
	_ = json.Unmarshal([]byte(headersJSON), &h)
	s.fetches <- fetchResult{status, h, string(body), errMsg}
}
func (s *recordingSink) SocketOpened(id int64)              { s.opened <- id }
func (s *recordingSink) SocketMessage(_ int64, data string) { s.messages <- data }
func (s *recordingSink) SocketClosed(_ int64, code int, _ string) {
	s.closed <- code
}

func wait[T any](t *testing.T, ch chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
		var zero T
		return zero
	}
}

func TestTheDesktopCoreOnlyDialsJokerlessPlaces(t *testing.T) {
	if err := SetCloudBaseURL("https://jokerless.com"); err != nil {
		t.Fatal(err)
	}
	for raw, want := range map[string]bool{
		"https://jokerless.com/api/x":    true,
		"wss://JOKERLESS.com/ws":         true,
		"http://127.0.0.1:47800/x":       true,
		"http://localhost:8090/":         true,
		"http://192.168.1.24:47800/":     true,
		"http://10.0.0.5/":               true,
		"https://example.com/":           false,
		"https://jokerless.com.evil.io/": false,
		"http://8.8.8.8/":                false,
		"file:///etc/passwd":             false,
	} {
		u, _ := url.Parse(raw)
		if got := allowed(u) == nil; got != want {
			t.Errorf("allowed(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestNetFetchCarriesTheRequestAndTheAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "" {
			t.Errorf("the page's origin was forwarded: %q", r.Header.Get("Origin"))
		}
		w.Header().Set("X-Seen", r.Method+" "+r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("body:" + r.URL.Path))
	}))
	defer srv.Close()

	sink := newSink()
	NetFetch(sink, 1, "POST", srv.URL+"/api/x", `{"Authorization":"Bearer t","Origin":"app://jokerless"}`, []byte("{}"))
	got := wait(t, sink.fetches)
	if got.err != "" || got.status != http.StatusTeapot || got.body != "body:/api/x" || got.headers["x-seen"] != "POST Bearer t" {
		t.Fatalf("got %+v", got)
	}

	NetFetch(sink, 2, "GET", "https://example.com/", "", nil)
	if refused := wait(t, sink.fetches); refused.err == "" || refused.status != 0 {
		t.Fatalf("a request outside the allowlist was made: %+v", refused)
	}
}

func TestNetSocketRoundTripsAndClosesOnce(t *testing.T) {
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, m, err := c.ReadMessage()
			if err != nil {
				return
			}
			_ = c.WriteMessage(websocket.TextMessage, append([]byte("echo:"), m...))
		}
	}))
	defer srv.Close()

	sink := newSink()
	NetSocketOpen(sink, 7, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws")
	wait(t, sink.opened)
	if err := NetSocketSend(7, "hi"); err != nil {
		t.Fatal(err)
	}
	if m := wait(t, sink.messages); m != "echo:hi" {
		t.Fatalf("message %q", m)
	}
	NetSocketClose(7, 1000, "bye")
	if code := wait(t, sink.closed); code != 1000 {
		t.Fatalf("close code %d", code)
	}
	if err := NetSocketSend(7, "late"); err == nil {
		t.Fatal("sent on a closed socket")
	}

	NetSocketOpen(sink, 8, "wss://example.com/ws")
	if code := wait(t, sink.closed); code != 1006 {
		t.Fatalf("a refused socket closed with %d", code)
	}
}

func TestWebAssetRoutesLikeTheServer(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>home</html>"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "offline.html"), []byte("<html>offline</html>"), 0o644))
	must(os.MkdirAll(filepath.Join(dir, "_expo/static/js"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "_expo/static/js/app.js"), []byte("console.log(1)"), 0o644))
	SetWebRoot(dir)
	defer SetWebRoot("")

	for path, want := range map[string]string{
		"/":                       "home",
		"/offline":                "offline",
		"/match/abc123":           "home",
		"/_expo/static/js/app.js": "console.log(1)",
		"/../../etc/passwd":       "home",
	} {
		r := WebAsset(path)
		if r.Status != http.StatusOK || !strings.Contains(string(r.Body), want) {
			t.Errorf("%s: %d %q", path, r.Status, r.Body)
		}
	}
	if ct := WebAsset("/_expo/static/js/app.js").ContentType; !strings.Contains(ct, "javascript") {
		t.Errorf("script served as %q", ct)
	}
}
