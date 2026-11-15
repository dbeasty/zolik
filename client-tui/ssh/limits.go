package ssh

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// The SSH door hands out a guest account to anyone who connects, and tries a
// password against any account name it is given. Both were unbounded: every
// connection minted a new guest row in the store, so a loop of `ssh guest@…`
// grew the users collection without limit, and nothing slowed a password
// guesser down. These are the limits.
//
// They live here rather than in server/internal/ratelimit because this module
// cannot import the server's internal packages, and they are per process for
// the same reason the server's own HTTP limiters are.
const (
	// A player who reconnects with the same key gets the same guest back, for
	// this long. Shorter than the guest access token's lifetime (seven days),
	// so a reused session is never one whose token has already run out.
	guestReuseFor = 24 * time.Hour
	// At most this many reusable guests are remembered; past it, new ones are
	// still issued but not remembered until old ones expire.
	guestCacheMax = 4096

	// New guest accounts per remote address, and for the whole server.
	guestsPerAddr   = 10
	guestsPerServer = 200
	guestWindow     = time.Hour

	// Failed password attempts per remote address, and per account name,
	// before further attempts are refused without being tried.
	loginFailuresAllowed = 5
	loginFailureWindow   = 15 * time.Minute
)

var errTooManyGuests = errors.New("too many new guest sessions; try again later")

// window counts events per key over a sliding window.
type window struct {
	mu    sync.Mutex
	limit int
	per   time.Duration
	now   func() time.Time
	hits  map[string][]time.Time
}

func newWindow(limit int, per time.Duration, now func() time.Time) *window {
	return &window{limit: limit, per: per, now: now, hits: map[string][]time.Time{}}
}

// recent returns key's events still inside the window, pruning the rest.
// Callers hold w.mu.
func (w *window) recent(key string) []time.Time {
	cut := w.now().Add(-w.per)
	hs := w.hits[key]
	i := 0
	for i < len(hs) && !hs[i].After(cut) {
		i++
	}
	hs = hs[i:]
	if len(hs) == 0 {
		delete(w.hits, key)
		return nil
	}
	w.hits[key] = hs
	return hs
}

// full reports whether key has used its allowance.
func (w *window) full(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.recent(key)) >= w.limit
}

// add records one event for key.
func (w *window) add(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hits[key] = append(w.recent(key), w.now())
	if len(w.hits) > 10000 {
		for k := range w.hits {
			w.recent(k)
		}
	}
}

type cachedGuest struct {
	sess Session
	at   time.Time
}

// gate holds the SSH door's limits and its memory of guests it has issued.
type gate struct {
	now func() time.Time

	mu     sync.Mutex
	guests map[string]cachedGuest

	guestsByAddr *window
	guestsTotal  *window
	failsByAddr  *window
	failsByUser  *window
}

func newGate(now func() time.Time) *gate {
	if now == nil {
		now = time.Now
	}
	return &gate{
		now:          now,
		guests:       map[string]cachedGuest{},
		guestsByAddr: newWindow(guestsPerAddr, guestWindow, now),
		guestsTotal:  newWindow(guestsPerServer, guestWindow, now),
		failsByAddr:  newWindow(loginFailuresAllowed, loginFailureWindow, now),
		failsByUser:  newWindow(loginFailuresAllowed, loginFailureWindow, now),
	}
}

func remoteHost(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

// login tries a password, unless this address or this account has already
// failed too often recently. A refused attempt is not tried at all, so a
// guesser learns nothing from it, and a correct password does not reset the
// count — the window does.
func (g *gate) login(ctx context.Context, auth Authenticator, user, password string, addr net.Addr) (Session, bool) {
	host := remoteHost(addr)
	if g.failsByAddr.full(host) || g.failsByUser.full(user) {
		return Session{}, false
	}
	sess, err := auth.Login(ctx, user, password)
	if err != nil {
		g.failsByAddr.add(host)
		g.failsByUser.add(user)
		return Session{}, false
	}
	return sess, true
}

// guest returns a guest session for a connection that did not sign in. A
// connection that presented a public key gets the guest that key was given
// last time, if it was recent; anything else is a new guest account, and
// those are rationed per address and per server.
func (g *gate) guest(ctx context.Context, auth Authenticator, user string, key ssh.PublicKey, addr net.Addr) (Session, error) {
	name := user
	// An anonymous connection sends no name rather than "Guest", which every
	// anonymous connection shared — three of them at a table were three seats
	// reading Guest. The server invents one instead, and it comes back on the
	// session; see auth.GuestNameFor.
	if user == "guest" || user == "" {
		name = ""
	}

	cacheKey := ""
	if key != nil {
		cacheKey = gossh.FingerprintSHA256(key) + "\x00" + name
		g.mu.Lock()
		c, ok := g.guests[cacheKey]
		g.mu.Unlock()
		if ok && g.now().Sub(c.at) < guestReuseFor {
			return c.sess, nil
		}
	}

	host := remoteHost(addr)
	if g.guestsByAddr.full(host) || g.guestsTotal.full("") {
		return Session{}, errTooManyGuests
	}
	g.guestsByAddr.add(host)
	g.guestsTotal.add("")

	sess, err := auth.Guest(ctx, name)
	if err != nil {
		return Session{}, err
	}
	if cacheKey != "" {
		g.remember(cacheKey, sess)
	}
	return sess, nil
}

func (g *gate) remember(key string, sess Session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if len(g.guests) >= guestCacheMax {
		for k, c := range g.guests {
			if now.Sub(c.at) >= guestReuseFor {
				delete(g.guests, k)
			}
		}
		if len(g.guests) >= guestCacheMax {
			return
		}
	}
	g.guests[key] = cachedGuest{sess: sess, at: now}
}
