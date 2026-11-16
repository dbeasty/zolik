package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	charmssh "github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// fakeAuth counts what the SSH door asked the account store for.
type fakeAuth struct {
	mu      sync.Mutex
	guests  int
	logins  int
	letIn   string // the one password that works
	guestOK bool
}

func (f *fakeAuth) Guest(_ context.Context, name string) (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.guests++
	return Session{AccessToken: fmt.Sprintf("guest-%d", f.guests), UserID: fmt.Sprintf("g%d", f.guests), Username: "Guest", IsGuest: true}, nil
}

func (f *fakeAuth) Login(_ context.Context, user, password string) (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logins++
	if password != f.letIn {
		return Session{}, errors.New("bad password")
	}
	return Session{AccessToken: "user", UserID: user, Username: user}, nil
}

func (f *fakeAuth) counts() (guests, logins int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.guests, f.logins
}

// serve runs the real SSH server on a loopback port and returns its address.
func serve(t *testing.T, auth Authenticator, g *gate, allowAllKeys bool) string {
	t.Helper()
	srv, err := newServer(Config{
		HostKeyPath:  filepath.Join(t.TempDir(), "host_key"),
		AllowAllKeys: allowAllKeys,
	}, Deps{Auth: auth}, g)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// connect opens a session and runs it to the end. With no pty the UI declines
// to start, which is fine: the guest is settled before it is reached. It
// returns whatever the server wrote, and the dial error if authentication
// was refused.
func connect(t *testing.T, addr, user string, auth gossh.AuthMethod) (string, error) {
	t.Helper()
	c, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{auth},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		return "", err
	}
	defer c.Close()
	s, err := c.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer s.Close()
	out, _ := s.CombinedOutput("")
	return string(out), nil
}

func newKey(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A player who comes back with the same key is the same guest. Before, every
// connection wrote a new guest account, so reconnecting to the terminal
// client grew the store by a user each time.
func TestReconnectingWithTheSameKeyReusesTheGuest(t *testing.T) {
	fa := &fakeAuth{}
	addr := serve(t, fa, newGate(nil), true)

	key := newKey(t)
	for i := 0; i < 3; i++ {
		if _, err := connect(t, addr, "guest", gossh.PublicKeys(key)); err != nil {
			t.Fatalf("connect %d: %v", i, err)
		}
	}
	if g, _ := fa.counts(); g != 1 {
		t.Fatalf("three connections with one key made %d guests, want 1", g)
	}

	if _, err := connect(t, addr, "guest", gossh.PublicKeys(newKey(t))); err != nil {
		t.Fatalf("connect with a second key: %v", err)
	}
	if g, _ := fa.counts(); g != 2 {
		t.Fatalf("a second key made %d guests in all, want 2", g)
	}
}

// A reused guest is only reused while its token is certainly still good.
func TestAReusedGuestExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	fa := &fakeAuth{}
	g := newGate(clock)
	key, err := charmssh.ParsePublicKey(newKey(t).PublicKey().Marshal())
	if err != nil {
		t.Fatal(err)
	}
	addr := &net.TCPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 1}

	if _, err := g.guest(context.Background(), fa, "guest", key, addr); err != nil {
		t.Fatal(err)
	}
	now = now.Add(guestReuseFor - time.Minute)
	if _, err := g.guest(context.Background(), fa, "guest", key, addr); err != nil {
		t.Fatal(err)
	}
	if n, _ := fa.counts(); n != 1 {
		t.Fatalf("within the reuse window: %d guests, want 1", n)
	}
	now = now.Add(2 * time.Minute)
	if _, err := g.guest(context.Background(), fa, "guest", key, addr); err != nil {
		t.Fatal(err)
	}
	if n, _ := fa.counts(); n != 2 {
		t.Fatalf("after the reuse window: %d guests, want 2", n)
	}
}

// Connections without a key cannot be told apart, so each is a new guest —
// but only so many from one address, and the one past the limit is told why
// rather than given a seatless placeholder.
func TestAnonymousGuestsAreRationedPerAddress(t *testing.T) {
	fa := &fakeAuth{}
	addr := serve(t, fa, newGate(nil), false)

	for i := 0; i < guestsPerAddr; i++ {
		if _, err := connect(t, addr, "guest", gossh.Password("")); err != nil {
			t.Fatalf("connect %d: %v", i, err)
		}
	}
	out, err := connect(t, addr, "guest", gossh.Password(""))
	if err != nil {
		t.Fatalf("connect past the limit: %v", err)
	}
	if g, _ := fa.counts(); g != guestsPerAddr {
		t.Fatalf("%d anonymous connections made %d guests, want the limit %d", guestsPerAddr+1, g, guestsPerAddr)
	}
	if want := "Too many new guest sessions"; !strings.Contains(out, want) {
		t.Fatalf("the refused connection was told %q, want it to say %q", out, want)
	}
}

func TestGuestsAreRationedPerServer(t *testing.T) {
	fa := &fakeAuth{}
	g := newGate(nil)
	made := 0
	for i := 0; i < guestsPerServer+20; i++ {
		addr := &net.TCPAddr{IP: net.IPv4(10, 1, byte(i/250), byte(i%250)), Port: 1}
		if _, err := g.guest(context.Background(), fa, "guest", nil, addr); err == nil {
			made++
		}
	}
	if made != guestsPerServer {
		t.Fatalf("made %d guests from many addresses, want the server limit %d", made, guestsPerServer)
	}
}

// A password guesser gets a handful of tries, and then the door stops
// asking the store at all — even for the right password, which would
// otherwise tell the guesser it had found it.
func TestPasswordGuessingIsLimited(t *testing.T) {
	fa := &fakeAuth{letIn: "right"}
	addr := serve(t, fa, newGate(nil), false)

	for i := 0; i < 20; i++ {
		if _, err := connect(t, addr, "alice", gossh.Password(fmt.Sprintf("wrong-%d", i))); err == nil {
			t.Fatalf("a wrong password was let in")
		}
	}
	if _, logins := fa.counts(); logins != loginFailuresAllowed {
		t.Fatalf("20 wrong passwords reached the store %d times, want %d", logins, loginFailuresAllowed)
	}
	if _, err := connect(t, addr, "alice", gossh.Password("right")); err == nil {
		t.Fatalf("the right password was let in while the address was locked out")
	}
}

func TestPasswordLimitIsPerAccountAcrossAddresses(t *testing.T) {
	fa := &fakeAuth{letIn: "right"}
	g := newGate(nil)
	for i := 0; i < 20; i++ {
		addr := &net.TCPAddr{IP: net.IPv4(10, 2, 0, byte(i)), Port: 1}
		g.login(context.Background(), fa, "alice", "wrong", addr)
	}
	if _, logins := fa.counts(); logins != loginFailuresAllowed {
		t.Fatalf("20 addresses guessing one account reached the store %d times, want %d", logins, loginFailuresAllowed)
	}
}

func TestAGoodPasswordStillSignsIn(t *testing.T) {
	fa := &fakeAuth{letIn: "right"}
	addr := serve(t, fa, newGate(nil), false)
	if _, err := connect(t, addr, "alice", gossh.Password("right")); err != nil {
		t.Fatalf("the right password: %v", err)
	}
	if g, _ := fa.counts(); g != 0 {
		t.Fatalf("a signed-in player was also given %d guest accounts", g)
	}
}

func TestFailuresAgeOut(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	fa := &fakeAuth{letIn: "right"}
	g := newGate(func() time.Time { return now })
	addr := &net.TCPAddr{IP: net.IPv4(10, 3, 0, 1), Port: 1}
	for i := 0; i < loginFailuresAllowed; i++ {
		g.login(context.Background(), fa, "bob", "wrong", addr)
	}
	if _, ok := g.login(context.Background(), fa, "bob", "right", addr); ok {
		t.Fatal("signed in while locked out")
	}
	now = now.Add(loginFailureWindow + time.Second)
	if _, ok := g.login(context.Background(), fa, "bob", "right", addr); !ok {
		t.Fatal("still locked out after the window passed")
	}
}
