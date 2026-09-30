package zolikcore

import (
	"bytes"
	"compress/flate"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var updateVectors = flag.Bool("update-vectors", false, "rewrite the cross-language BLE test vectors")

// chanSink is a TunnelSink that hands each outgoing message to the test.
type chanSink struct{ out chan []byte }

func (s *chanSink) Send(msg []byte) { s.out <- append([]byte(nil), msg...) }

// guest is the test's own implementation of a guest's half of the tunnel,
// written from the protocol comment rather than from the host's code.
type guest struct {
	t         *testing.T
	tun       *Tunnel
	sink      *chanSink
	keys      *sessionKeys
	check     string
	sendCount uint64
	recvCount uint64
	sent      [][]byte
	nextID    int64
}

func deriveGuest(guestEph *ecdh.PrivateKey, hostStatic, hostEph *ecdh.PublicKey) (*sessionKeys, string, error) {
	ee, err := guestEph.ECDH(hostEph)
	if err != nil {
		return nil, "", err
	}
	es, err := guestEph.ECDH(hostStatic)
	if err != nil {
		return nil, "", err
	}
	return schedule(ee, es, guestEph.PublicKey().Bytes(), hostStatic.Bytes(), hostEph.Bytes())
}

func connectGuest(t *testing.T, h *Host) *guest {
	t.Helper()
	sink := &chanSink{out: make(chan []byte, 64)}
	g := &guest{t: t, tun: h.NewTunnel(sink, "test-"+t.Name()), sink: sink}
	eph, _ := ecdh.X25519().GenerateKey(rand.Reader)
	g.tun.Receive(append([]byte{msgHello, tunnelVersion}, eph.PublicKey().Bytes()...))
	welcome := g.next()
	if len(welcome) != 2+64 || welcome[0] != msgWelcome {
		t.Fatalf("welcome = %x", welcome)
	}
	hostStatic, _ := ecdh.X25519().NewPublicKey(welcome[2:34])
	hostEph, _ := ecdh.X25519().NewPublicKey(welcome[34:66])
	if base64.StdEncoding.EncodeToString(hostStatic.Bytes()) != h.PublicKey() {
		t.Fatal("the welcome carried a static key that is not the host's")
	}
	keys, check, err := deriveGuest(eph, hostStatic, hostEph)
	if err != nil {
		t.Fatal(err)
	}
	g.keys, g.check = keys, check
	if g.tun.CheckCode() != check || len(check) != 4 {
		t.Fatalf("check codes differ: host %q, guest %q", g.tun.CheckCode(), check)
	}
	return g
}

func (g *guest) next() []byte {
	g.t.Helper()
	select {
	case m := <-g.sink.out:
		return m
	case <-time.After(5 * time.Second):
		g.t.Fatal("the host said nothing")
		return nil
	}
}

func (g *guest) sendMsg(m tunnelMsg) {
	raw, _ := json.Marshal(m)
	sealed := g.keys.g2h.Seal([]byte{msgSealed}, nonce(g.sendCount), append([]byte{0}, raw...), []byte{msgSealed})
	g.sendCount++
	g.sent = append(g.sent, sealed)
	g.tun.Receive(sealed)
}

// recv opens the next message from the host and reports whether it was
// compressed.
func (g *guest) recv() (tunnelMsg, bool) {
	g.t.Helper()
	sealed := g.next()
	if sealed[0] != msgSealed {
		g.t.Fatalf("expected a sealed message, got type %#x", sealed[0])
	}
	plain, err := g.keys.h2g.Open(nil, nonce(g.recvCount), sealed[1:], sealed[:1])
	if err != nil {
		g.t.Fatalf("a message from the host failed to open: %v", err)
	}
	g.recvCount++
	body, deflated := plain[1:], plain[0]&flagDeflate != 0
	if deflated {
		body, _ = io.ReadAll(flate.NewReader(bytes.NewReader(body)))
	}
	var m tunnelMsg
	if err := json.Unmarshal(body, &m); err != nil {
		g.t.Fatalf("host sent %q: %v", body, err)
	}
	return m, deflated
}

// until reads messages until one matches, returning it.
func (g *guest) until(match func(tunnelMsg) bool) tunnelMsg {
	g.t.Helper()
	for i := 0; i < 50; i++ {
		if m, _ := g.recv(); match(m) {
			return m
		}
	}
	g.t.Fatal("never got the message")
	return tunnelMsg{}
}

func (g *guest) do(method, path, token string, body any) tunnelMsg {
	g.t.Helper()
	g.nextID++
	h := map[string]string{"Content-Type": "application/json"}
	if token != "" {
		h["Authorization"] = "Bearer " + token
	}
	raw, _ := json.Marshal(body)
	id := g.nextID
	g.sendMsg(tunnelMsg{T: "req", ID: id, M: method, P: path, H: h, B: string(raw)})
	return g.until(func(m tunnelMsg) bool { return m.T == "res" && m.ID == id })
}

// A guest across the Bluetooth tunnel does what a guest over Wi-Fi does:
// sits down, opens a table, and gets the match's state on its socket. None
// of it crosses in the clear, and big answers arrive compressed.
func TestAGuestPlaysThroughTheTunnel(t *testing.T) {
	h := startHost(t, t.TempDir())
	g := connectGuest(t, h)

	g.sendMsg(tunnelMsg{T: "ping"})
	if m, _ := g.recv(); m.T != "pong" {
		t.Fatalf("ping answered %+v", m)
	}

	res := g.do("POST", "/auth/guest", "", map[string]string{"guestName": "Radio"})
	if res.S != 200 {
		t.Fatalf("guest sign-in over the tunnel: %d %s", res.S, res.B)
	}
	var s struct{ AccessToken string }
	_ = json.Unmarshal([]byte(res.B), &s)

	g.nextID++
	id := g.nextID
	g.sendMsg(tunnelMsg{T: "req", ID: id, M: "GET", P: "/modules"})
	var mods tunnelMsg
	var deflated bool
	for {
		m, d := g.recv()
		if m.T == "res" && m.ID == id {
			mods, deflated = m, d
			break
		}
	}
	if mods.S != 200 || !deflated {
		t.Fatalf("/modules: status %d, compressed %v (a %d-byte answer should be)", mods.S, deflated, len(mods.B))
	}

	created := g.do("POST", "/matches", s.AccessToken, map[string]string{"moduleId": "blackjack"})
	var c struct{ MatchID string }
	_ = json.Unmarshal([]byte(created.B), &c)
	if g.do("POST", "/matches/"+c.MatchID+"/add-bot", s.AccessToken, map[string]any{}).S != 200 ||
		g.do("POST", "/matches/"+c.MatchID+"/start", s.AccessToken, map[string]any{}).S != 200 {
		t.Fatal("could not set the table up over the tunnel")
	}

	g.sendMsg(tunnelMsg{T: "wso", ID: 99, P: "/ws/matches/" + c.MatchID + "?token=" + s.AccessToken})
	g.until(func(m tunnelMsg) bool { return m.T == "wsopen" && m.ID == 99 })
	state := g.until(func(m tunnelMsg) bool { return m.T == "wsm" && m.ID == 99 })
	if !strings.Contains(state.D, `"type":"match_state"`) {
		t.Fatalf("first socket frame: %.120s", state.D)
	}

	g.sendMsg(tunnelMsg{T: "wsc", ID: 99})
	g.tun.Close()
}

// Anything that is not the next message, sealed with the right key, ends
// the tunnel: a flipped bit, a replay, a second handshake, or a sealed
// message before any handshake at all.
func TestTheTunnelRefusesWhatItDidNotExpect(t *testing.T) {
	h := startHost(t, t.TempDir())

	cases := map[string]func(g *guest){
		"tampered": func(g *guest) {
			raw, _ := json.Marshal(tunnelMsg{T: "ping"})
			sealed := g.keys.g2h.Seal([]byte{msgSealed}, nonce(g.sendCount), append([]byte{0}, raw...), []byte{msgSealed})
			sealed[len(sealed)-1] ^= 1
			g.tun.Receive(sealed)
		},
		"replayed": func(g *guest) {
			g.sendMsg(tunnelMsg{T: "ping"})
			g.recv()
			g.tun.Receive(g.sent[0])
		},
		"second hello": func(g *guest) {
			eph, _ := ecdh.X25519().GenerateKey(rand.Reader)
			g.tun.Receive(append([]byte{msgHello, tunnelVersion}, eph.PublicKey().Bytes()...))
		},
	}
	for name, attack := range cases {
		t.Run(name, func(t *testing.T) {
			g := connectGuest(t, h)
			attack(g)
			// The tunnel is closed: an honest ping now goes unanswered.
			g.sendMsg(tunnelMsg{T: "ping"})
			select {
			case m := <-g.sink.out:
				t.Fatalf("the tunnel still answered after %s: %x", name, m)
			case <-time.After(300 * time.Millisecond):
			}
		})
	}

	t.Run("a path that does not parse", func(t *testing.T) {
		g := connectGuest(t, h)
		g.sendMsg(tunnelMsg{T: "req", ID: 1, M: "GET", P: "/has a space\x7f%zz"})
		if m, _ := g.recv(); m.T != "res" || m.S != 400 {
			t.Fatalf("a bad path answered %+v; it should be a 400, not a crash", m)
		}
		g.sendMsg(tunnelMsg{T: "req", ID: 2, M: "BAD METHOD", P: "/healthz"})
		if m, _ := g.recv(); m.T != "res" || m.S != 400 {
			t.Fatalf("a bad method answered %+v", m)
		}
	})

	t.Run("sealed before hello", func(t *testing.T) {
		sink := &chanSink{out: make(chan []byte, 4)}
		tun := h.NewTunnel(sink, "x")
		tun.Receive([]byte{msgSealed, 1, 2, 3})
		eph, _ := ecdh.X25519().GenerateKey(rand.Reader)
		tun.Receive(append([]byte{msgHello, tunnelVersion}, eph.PublicKey().Bytes()...))
		select {
		case m := <-sink.out:
			t.Fatalf("a closed tunnel shook hands: %x", m)
		case <-time.After(300 * time.Millisecond):
		}
	})
}

// The host's Bluetooth key is part of its identity: it survives a restart,
// is what /nearby/info advertises, and is what BLEInfo carries.
func TestTheHostKeepsItsBluetoothKey(t *testing.T) {
	dir := t.TempDir()
	h := startHost(t, dir)
	pk := h.PublicKey()
	var info struct {
		V  int    `json:"v"`
		ID string `json:"id"`
		N  string `json:"n"`
		PK string `json:"pk"`
	}
	if err := json.Unmarshal(h.BLEInfo("Kitchen"), &info); err != nil ||
		info.V != ProtocolVersion || info.ID != h.InstanceID() || info.N != "Kitchen" || info.PK != pk {
		t.Fatalf("BLEInfo = %+v (%v)", info, err)
	}
	h.Stop()
	if again := startHost(t, dir); again.PublicKey() != pk {
		t.Errorf("the key changed across a restart")
	}
}

// The vectors the app's own implementation (src/net/ble/crypto.ts) is
// checked against. The handshake uses fixed keys, so both sides must derive
// the same session keys and check code, and open each other's messages.
// `go test -run Vectors -update-vectors` rewrites the file.
func TestCrossLanguageVectors(t *testing.T) {
	fixed := func(b byte) *ecdh.PrivateKey {
		k, _ := ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{b}, 32))
		return k
	}
	guestEph, hostStatic, hostEph := fixed(0x11), fixed(0x22), fixed(0x33)
	hostKeys, check, err := deriveHost(hostStatic, hostEph, guestEph.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	guestKeys, guestCheck, _ := deriveGuest(guestEph, hostStatic.PublicKey(), hostEph.PublicKey())
	if check != guestCheck {
		t.Fatal("host and guest disagree on the check code")
	}

	g2hPlain := []byte("\x00" + `{"t":"ping"}`)
	h2gJSON := `{"t":"res","id":7,"s":200,"b":"` + strings.Repeat("ok ", 200) + `"}`
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestSpeed)
	_, _ = w.Write([]byte(h2gJSON))
	_ = w.Close()
	h2gPlain := append([]byte{flagDeflate}, buf.Bytes()...)

	v := map[string]string{
		"guestEphPriv":   hex.EncodeToString(guestEph.Bytes()),
		"hostStaticPub":  hex.EncodeToString(hostStatic.PublicKey().Bytes()),
		"hostEphPub":     hex.EncodeToString(hostEph.PublicKey().Bytes()),
		"checkCode":      check,
		"g2hSealed0":     hex.EncodeToString(guestKeys.g2h.Seal([]byte{msgSealed}, nonce(0), g2hPlain, []byte{msgSealed})),
		"g2hSealed1":     hex.EncodeToString(guestKeys.g2h.Seal([]byte{msgSealed}, nonce(1), g2hPlain, []byte{msgSealed})),
		"h2gSealed0":     hex.EncodeToString(hostKeys.h2g.Seal([]byte{msgSealed}, nonce(0), h2gPlain, []byte{msgSealed})),
		"g2hPlainJSON":   `{"t":"ping"}`,
		"h2gPlainJSON":   h2gJSON,
		"hostStaticPriv": hex.EncodeToString(hostStatic.Bytes()),
		"hostEphPriv":    hex.EncodeToString(hostEph.Bytes()),
	}
	path := filepath.Join("..", "..", "..", "client-react-native", "src", "net", "ble", "vectors.json")
	got, _ := json.MarshalIndent(v, "", "  ")
	got = append(got, '\n')
	if *updateVectors {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update-vectors to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the key schedule no longer matches the vectors the app is tested against; " +
			"if that is intended, rerun with -update-vectors and change src/net/ble/crypto.ts to match")
	}
}
