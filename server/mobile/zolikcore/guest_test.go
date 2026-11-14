package zolikcore

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/relay"
)

// The guest's end of the tunnel, against the host's end, through the relay:
// a table across the internet answers on a loopback address like any other.
func TestAGuestTableAnswersOnLoopbackThroughTheRelay(t *testing.T) {
	srv := relayCloud(t)
	if err := SetCloudBaseURL(srv.URL); err != nil {
		t.Fatal(err)
	}
	h := startHost(t, t.TempDir())
	h.nodeCredential, h.cloudBaseURL = "node-n1", srv.URL
	code, err := h.OpenRelay("Ada's phone")
	if err != nil {
		t.Fatalf("OpenRelay: %v", err)
	}

	// A first sit-down has nothing pinned, and learns the table's key.
	g, err := JoinRelay(code, h.InstanceID(), "")
	if err != nil {
		t.Fatalf("JoinRelay: %v", err)
	}
	t.Cleanup(LeaveGuestTables)
	if g.HostKey() != h.PublicKey() {
		t.Errorf("host key %s, want %s", g.HostKey(), h.PublicKey())
	}
	if len(g.CheckCode()) != 4 {
		t.Errorf("check code %q", g.CheckCode())
	}

	// Plain HTTP, through the tunnel.
	res, err := http.Post(g.BaseURL()+"/auth/guest", "application/json", strings.NewReader(`{"guestName":"Rita"}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(body), "accessToken") {
		t.Fatalf("sign-in through the gateway: %d %s", res.StatusCode, body)
	}
	var who struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(body, &who)

	// A table, and a socket on it.
	req, _ := http.NewRequest("POST", g.BaseURL()+"/matches", strings.NewReader(`{"moduleId":"prsi"}`))
	req.Header.Set("Authorization", "Bearer "+who.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	var made struct {
		MatchID string `json:"matchId"`
	}
	_ = json.Unmarshal(body, &made)
	if res.StatusCode != 200 && res.StatusCode != 201 || made.MatchID == "" {
		t.Fatalf("creating a table through the gateway: %d %s", res.StatusCode, body)
	}
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(g.BaseURL(), "http")+"/ws/matches/"+made.MatchID+"?token="+who.AccessToken, nil)
	if err != nil {
		t.Fatalf("socket through the gateway: %v", err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, data, err := ws.ReadMessage(); err != nil || !strings.Contains(string(data), made.MatchID) {
		t.Fatalf("the socket sent %q, %v", data, err)
	}
	_ = ws.Close()

	// The link drops (the host's door closes and opens): the next request
	// reconnects by itself, on the same address.
	addr := g.BaseURL()
	g.tun.mu.Lock()
	live := g.tun.live
	g.tun.mu.Unlock()
	live.link.close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err = http.Post(addr+"/auth/guest", "application/json", strings.NewReader(`{"guestName":"Rita"}`))
		if err == nil && res.StatusCode == 200 {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no reconnect: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// A table that answers with another key than the pinned one is refused.
	if _, err := JoinRelay(code, h.InstanceID(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != ErrHostKeyChanged {
		t.Errorf("joining with the wrong pin: %v", err)
	}

	// Closing the host's door tells the guest why the table went.
	h.CloseRelay()
	deadline = time.Now().Add(5 * time.Second)
	for g.HostAway() != "ended" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if g.HostAway() != "ended" {
		t.Errorf("after the host closed the table, HostAway = %q (relay close code %d)", g.HostAway(), relay.CloseHostEnded)
	}
}

// fakeRadio is the native side of Bluetooth: each link is a host Tunnel whose
// messages come back through BleReceive, as the Swift and Kotlin code does.
type fakeRadio struct {
	h      *Host
	mu     sync.Mutex
	next   int
	links  map[string]*Tunnel
	opened int
}

type radioSink struct {
	id string
}

func (s radioSink) Send(msg []byte) { go BleReceive(s.id, msg) }

func (r *fakeRadio) open() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	r.opened++
	id := "link-" + strconv.Itoa(r.next) + "-" + r.h.InstanceID()
	r.links[id] = r.h.NewTunnel(radioSink{id: id}, id)
	return id
}

func (r *fakeRadio) Connect(instanceID string) (string, error) {
	if instanceID != r.h.InstanceID() {
		return "", errors.New("out of range")
	}
	return r.open(), nil
}

func (r *fakeRadio) Send(linkID string, msg []byte) error {
	r.mu.Lock()
	t := r.links[linkID]
	r.mu.Unlock()
	if t == nil {
		return errors.New("no such link")
	}
	t.Receive(append([]byte(nil), msg...))
	return nil
}

func (r *fakeRadio) Disconnect(linkID string) {
	r.mu.Lock()
	t := r.links[linkID]
	delete(r.links, linkID)
	r.mu.Unlock()
	if t != nil {
		t.Close()
	}
}

// A table in the room, over Bluetooth: the guest's end in the core, the host's
// end behind a radio that is only a function call, and the table on loopback.
func TestAGuestTableAnswersOnLoopbackOverBluetooth(t *testing.T) {
	h := startHost(t, t.TempDir())
	radio := &fakeRadio{h: h, links: map[string]*Tunnel{}}

	first := radio.open()
	g, err := JoinBle(radio, h.InstanceID(), first, "")
	if err != nil {
		t.Fatalf("JoinBle: %v", err)
	}
	t.Cleanup(LeaveGuestTables)
	if g.HostKey() != h.PublicKey() || len(g.CheckCode()) != 4 {
		t.Fatalf("host key %q, check %q", g.HostKey(), g.CheckCode())
	}
	checkBefore := g.CheckCode()

	post := func() (*http.Response, error) {
		return http.Post(g.BaseURL()+"/auth/guest", "application/json", strings.NewReader(`{"guestName":"Bea"}`))
	}
	res, err := post()
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("sign-in over the radio: %v %v", res, err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var who struct {
		AccessToken string `json:"accessToken"`
	}
	_ = json.Unmarshal(body, &who)

	// A socket through the radio.
	req, _ := http.NewRequest("POST", g.BaseURL()+"/matches", strings.NewReader(`{"moduleId":"prsi"}`))
	req.Header.Set("Authorization", "Bearer "+who.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	var made struct {
		MatchID string `json:"matchId"`
	}
	_ = json.Unmarshal(body, &made)
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(g.BaseURL(), "http")+"/ws/matches/"+made.MatchID+"?token="+who.AccessToken, nil)
	if err != nil {
		t.Fatalf("socket over the radio: %v", err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, data, err := ws.ReadMessage(); err != nil || !strings.Contains(string(data), made.MatchID) {
		t.Fatalf("the socket sent %q, %v", data, err)
	}
	_ = ws.Close()

	// The radio drops the link; the next request finds the table again, over
	// a new link with a new check code, on the same address.
	BleClosed(first)
	deadline := time.Now().Add(5 * time.Second)
	for {
		res, err = post()
		if err == nil && res.StatusCode == 200 {
			res.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no reconnect over the radio: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	radio.mu.Lock()
	opened := radio.opened
	radio.mu.Unlock()
	if opened != 2 {
		t.Errorf("links opened = %d, want 2 (the first, and one after the drop)", opened)
	}
	if g.CheckCode() == checkBefore {
		t.Errorf("the check code did not change with the new handshake")
	}

	// Another table's key is refused.
	if _, err := JoinBle(radio, h.InstanceID(), radio.open(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != ErrHostKeyChanged {
		t.Errorf("joining with the wrong pin: %v", err)
	}
}

// inboundDuringSend is a radio's link: its send does not return until the
// write completes, and the write completes on the queue that also delivers
// what the table sends, so a delivery has to be able to finish first.
type inboundDuringSend struct {
	deliver func()
	done    chan struct{}
}

func (l *inboundDuringSend) send([]byte) error {
	go func() {
		l.deliver()
		close(l.done)
	}()
	select {
	case <-l.done:
		return nil
	case <-time.After(2 * time.Second):
		return errors.New("the delivery never finished: the send is holding the lock it needs")
	}
}
func (l *inboundDuringSend) close() {}

// A message arriving while one is being written. Holding the tunnel's lock
// across the send stopped the link for good, and the next status check froze
// the Mac app's main thread on the same lock. Only real Bluetooth has a send
// that waits on the delivery queue, so the loopback test could not see it.
func TestAMessageArrivingMidWriteDoesNotStopTheLink(t *testing.T) {
	block, err := aes.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	tun := newGuestTunnel(nil, nil)
	sess := &guestSession{g2h: aead, h2g: aead, welcome: make(chan []byte, 1)}
	link := &inboundDuringSend{done: make(chan struct{})}
	link.deliver = func() {
		tun.receive(sess, []byte{0})
		_ = tun.CheckCodeForTest()
	}
	sess.link = link
	tun.live = sess
	if err := tun.post(tunnelMsg{T: "wsm", ID: 1, D: "x"}); err != nil {
		t.Fatal(err)
	}
}

// CheckCodeForTest reads what GuestTable.CheckCode reads, under the same lock.
func (t *guestTunnel) CheckCodeForTest() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.check
}

// stallingRadio is a radio whose native side can lose its callbacks: a send
// or a connect that never returns, or a send that fails without the link
// ever being reported closed. That is what NearbyBle did when a peripheral
// dropped mid-write, and the tunnel held its locks across the wait for good.
type stallingRadio struct {
	*fakeRadio
	mu          sync.Mutex
	stallSend   bool
	failSend    bool
	stallDial   bool
	release     chan struct{}
	disconnects []string
}

func (r *stallingRadio) Send(linkID string, msg []byte) error {
	r.mu.Lock()
	stall, fail := r.stallSend, r.failSend
	r.failSend = false
	r.mu.Unlock()
	if stall {
		<-r.release
		return errors.New("released")
	}
	if fail {
		return errors.New("the write failed")
	}
	return r.fakeRadio.Send(linkID, msg)
}

func (r *stallingRadio) Connect(instanceID string) (string, error) {
	r.mu.Lock()
	stall := r.stallDial
	r.mu.Unlock()
	if stall {
		<-r.release
	}
	return r.fakeRadio.Connect(instanceID)
}

func (r *stallingRadio) Disconnect(linkID string) {
	r.mu.Lock()
	r.disconnects = append(r.disconnects, linkID)
	r.mu.Unlock()
	r.fakeRadio.Disconnect(linkID)
}

func (r *stallingRadio) set(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f()
}

// A radio that stops answering does not freeze the table: the request fails
// in bounded time, the locks are let go, and the next request redials.
func TestAGuestTunnelRecoversFromARadioThatStopsAnswering(t *testing.T) {
	oldDial, oldSend := linkDialTimeout, linkSendTimeout
	linkDialTimeout, linkSendTimeout = 300*time.Millisecond, 300*time.Millisecond
	t.Cleanup(func() { linkDialTimeout, linkSendTimeout = oldDial, oldSend })

	h := startHost(t, t.TempDir())
	radio := &stallingRadio{fakeRadio: &fakeRadio{h: h, links: map[string]*Tunnel{}}, release: make(chan struct{})}
	t.Cleanup(func() { close(radio.release) })

	g, err := JoinBle(radio, h.InstanceID(), radio.open(), "")
	if err != nil {
		t.Fatalf("JoinBle: %v", err)
	}
	t.Cleanup(LeaveGuestTables)

	client := &http.Client{Timeout: 10 * time.Second}
	post := func() (int, error) {
		res, err := client.Post(g.BaseURL()+"/auth/guest", "application/json", strings.NewReader(`{"guestName":"Bea"}`))
		if err != nil {
			return 0, err
		}
		res.Body.Close()
		return res.StatusCode, nil
	}
	opened := func() int {
		radio.fakeRadio.mu.Lock()
		defer radio.fakeRadio.mu.Unlock()
		return radio.fakeRadio.opened
	}
	if code, err := post(); err != nil || code != 200 {
		t.Fatalf("first request: %d %v", code, err)
	}

	// A send that never returns, and no close: the request fails instead of
	// hanging, and so does one queued behind it on the send lock.
	radio.set(func() { radio.stallSend = true })
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if code, err := post(); err != nil || code == 200 {
				t.Errorf("request over a stalled send: %d %v (want a bounded failure)", code, err)
			}
		}()
	}
	wg.Wait()
	radio.set(func() { radio.stallSend = false })
	before := opened()
	if code, err := post(); err != nil || code != 200 {
		t.Fatalf("request after the stall: %d %v", code, err)
	}
	if opened() != before+1 {
		t.Errorf("links opened %d → %d: want a redial after the stall", before, opened())
	}

	// A send that fails without the link being reported closed: the session
	// is retired and the next request redials rather than reuse it.
	radio.set(func() { radio.failSend = true })
	if code, err := post(); err != nil || code == 200 {
		t.Fatalf("request over a failing send: %d %v", code, err)
	}
	before = opened()
	if code, err := post(); err != nil || code != 200 {
		t.Fatalf("request after the failed send: %d %v", code, err)
	}
	if opened() != before+1 {
		t.Errorf("links opened %d → %d: want a redial after the failed send", before, opened())
	}

	// A connect that never returns: the dial gives up and lets the next
	// request try again.
	radio.set(func() { radio.failSend = true })
	_, _ = post() // drops the live link, so the next request dials
	radio.set(func() { radio.stallDial = true })
	if code, err := post(); err != nil || code == 200 {
		t.Fatalf("request over a stalled connect: %d %v", code, err)
	}
	radio.set(func() { radio.stallDial = false })
	if code, err := post(); err != nil || code != 200 {
		t.Fatalf("request after the stalled connect: %d %v", code, err)
	}
}
