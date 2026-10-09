package zolikcore

// What a desktop app needs from the core besides a table to host.
//
// The Mac app (client-macos) shows the web client in a system web view that
// is not allowed to touch the network. It loads its own pages from WebAsset,
// in process, and hands every request the page makes — to the cloud, to a
// table on this machine, to a table across the room — to NetFetch and
// NetSocketOpen here. So the web view never opens a socket, the core is the
// one place that does, and the core only dials where a Jokerless client has
// business: the cloud it was pointed at, this machine, and the local network.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/webui"
)

// WebResponse is one answer from the app's own page bundle.
type WebResponse struct {
	Status      int
	ContentType string
	Body        []byte
}

// WebAsset answers a request for the desktop app's own page, from the web
// export compiled into the core (or the folder SetWebRoot names). It resolves
// paths exactly as the server does for a browser — the file, then its
// prerendered `.html`, then `index.html` for a route only the client knows —
// so the app and jokerless.com can never route differently.
func WebAsset(path string) *WebResponse {
	h := webui.NewHandler(webFiles())
	if !h.Available() {
		return &WebResponse{Status: http.StatusNotFound, ContentType: "text/plain; charset=utf-8",
			Body: []byte("this build has no web client compiled in")}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if name, ok := h.Lookup(path); ok {
		h.ServeFile(rec, req, name)
	} else {
		h.ServeIndex(rec, req)
	}
	return &WebResponse{Status: rec.Code, ContentType: rec.Header().Get("Content-Type"), Body: rec.Body.Bytes()}
}

func webFiles() fs.FS {
	if dir := webRootDir(); dir != "" {
		return os.DirFS(dir)
	}
	return webui.Embedded()
}

// NetSink is how the core answers the app's requests. Every call arrives on a
// goroutine of the core's, never the caller's thread.
type NetSink interface {
	// FetchDone answers NetFetch. errMsg is set, and the rest empty, when no
	// response arrived at all; an HTTP error status is a response.
	FetchDone(id int64, status int, headersJSON string, body []byte, errMsg string)
	SocketOpened(id int64)
	SocketMessage(id int64, data string)
	// SocketClosed is the last call for a socket, whichever side ended it.
	SocketClosed(id int64, code int, reason string)
}

var (
	netMu      sync.Mutex
	cloudHosts = map[string]bool{}
	// cloudBase is the server's address as given, for what the core itself
	// dials for a guest (the relay).
	cloudBase string
)

// SetCloudBaseURL names the server the app plays online against. Its host is
// the one address outside this machine and the local network the core will
// dial for the app. Calling it again replaces the previous one.
func SetCloudBaseURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("not a server address: %q", raw)
	}
	netMu.Lock()
	defer netMu.Unlock()
	cloudHosts = map[string]bool{strings.ToLower(u.Hostname()): true}
	cloudBase = strings.TrimRight(strings.TrimSpace(raw), "/")
	return nil
}

// allowed reports whether the app may reach u: the cloud it was given, this
// machine, or an address on the local network. Anything else is refused
// before a connection is made, whatever the page asked for.
func allowed(u *url.URL) error {
	switch u.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("scheme %q is not allowed", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	netMu.Lock()
	cloud := cloudHosts[host]
	netMu.Unlock()
	if cloud || host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return nil
	}
	return fmt.Errorf("%s is not a Jokerless server", host)
}

var netClient = &http.Client{
	Timeout: 60 * time.Second,
	// A redirect must stay where a request could have gone in the first place.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		return allowed(req.URL)
	},
}

// NetFetch makes one HTTP request for the app and answers through sink. It
// returns at once; the answer comes later, on another goroutine.
//
// headersJSON is a JSON object of header names to values. The page's Origin
// is never forwarded: the request comes from the app, not from a website, and
// the server's CORS rules are for browsers.
func NetFetch(sink NetSink, id int64, method, rawURL, headersJSON string, body []byte) {
	// gobind lends body only for the length of this call: the caller's
	// buffer may be freed the moment it returns, so it is copied first.
	body = bytes.Clone(body)
	go func() {
		status, headers, respBody, err := doFetch(method, rawURL, headersJSON, body)
		if err != nil {
			sink.FetchDone(id, 0, "{}", nil, err.Error())
			return
		}
		sink.FetchDone(id, status, headers, respBody, "")
	}()
}

func doFetch(method, rawURL, headersJSON string, body []byte) (int, string, []byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, "", nil, err
	}
	if err := allowed(u); err != nil {
		return 0, "", nil, err
	}
	if method == "" {
		method = http.MethodGet
	}
	var rd io.Reader
	if len(body) > 0 {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u.String(), rd)
	if err != nil {
		return 0, "", nil, err
	}
	for k, v := range parseHeaders(headersJSON) {
		req.Header.Set(k, v)
	}
	resp, err := netClient.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return 0, "", nil, err
	}
	out := map[string]string{}
	for k, v := range resp.Header {
		out[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	raw, _ := json.Marshal(out)
	return resp.StatusCode, string(raw), respBody, nil
}

func parseHeaders(raw string) map[string]string {
	in := map[string]string{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			log.Printf("zolikcore: ignoring unreadable request headers: %v", err)
		}
	}
	for k := range in {
		switch strings.ToLower(k) {
		case "origin", "referer", "host", "cookie", "content-length", "connection":
			delete(in, k)
		}
	}
	return in
}

type netSocket struct {
	mu     sync.Mutex
	conn   *websocket.Conn
	closed bool
	cancel context.CancelFunc
}

var (
	socketsMu sync.Mutex
	sockets   = map[int64]*netSocket{}
)

// NetSocketOpen dials a WebSocket for the app. sink hears SocketOpened, then
// any messages, then exactly one SocketClosed — including when the dial fails.
func NetSocketOpen(sink NetSink, id int64, rawURL string) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &netSocket{cancel: cancel}
	socketsMu.Lock()
	sockets[id] = s
	socketsMu.Unlock()

	go func() {
		defer func() {
			socketsMu.Lock()
			delete(sockets, id)
			socketsMu.Unlock()
		}()
		u, err := url.Parse(rawURL)
		if err == nil {
			err = allowed(u)
		}
		if err != nil {
			sink.SocketClosed(id, 1006, err.Error())
			return
		}
		dialer := websocket.Dialer{HandshakeTimeout: 20 * time.Second, Proxy: http.ProxyFromEnvironment}
		conn, _, err := dialer.DialContext(ctx, u.String(), nil)
		if err != nil {
			sink.SocketClosed(id, 1006, err.Error())
			return
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			conn.Close()
			sink.SocketClosed(id, 1000, "")
			return
		}
		s.conn = conn
		s.mu.Unlock()
		sink.SocketOpened(id)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				code := 1006
				reason := ""
				var ce *websocket.CloseError
				if errors.As(err, &ce) {
					code, reason = ce.Code, ce.Text
				} else if s.isClosed() {
					code = 1000
				}
				conn.Close()
				sink.SocketClosed(id, code, reason)
				return
			}
			sink.SocketMessage(id, string(data))
		}
	}()
}

func (s *netSocket) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func lookupSocket(id int64) *netSocket {
	socketsMu.Lock()
	defer socketsMu.Unlock()
	return sockets[id]
}

// NetSocketSend sends one text message on an open socket.
func NetSocketSend(id int64, data string) error {
	s := lookupSocket(id)
	if s == nil {
		return errors.New("the socket is closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil || s.closed {
		return errors.New("the socket is not open")
	}
	_ = s.conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	return s.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

// NetSocketClose closes a socket from the app's side. SocketClosed still
// follows, from the reading goroutine.
func NetSocketClose(id int64, code int, reason string) {
	s := lookupSocket(id)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.conn == nil {
		s.cancel()
		return
	}
	if code == 0 {
		code = websocket.CloseNormalClosure
	}
	_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason),
		time.Now().Add(2*time.Second))
	// The reader returns when the server echoes the close, or when this
	// deadline passes and the connection is torn down.
	_ = s.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
}
