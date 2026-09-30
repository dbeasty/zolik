package zolikcore

import (
	"net"
	"sync"
)

// trackingListener remembers every connection it has accepted and not yet
// seen closed, so closing the room can hang up on all of them.
//
// http.Server cannot do that on its own. A WebSocket is a hijacked
// connection, and once hijacked the server forgets it: neither Shutdown nor
// Close touches it. Without this, a guest whose host had closed the room
// would sit at the table for as long as the TCP connection lived.
type trackingListener struct {
	net.Listener
	mu    sync.Mutex
	conns map[*trackedConn]struct{}
}

func newTrackingListener(ln net.Listener) *trackingListener {
	return &trackingListener{Listener: ln, conns: map[*trackedConn]struct{}{}}
}

func (l *trackingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tc := &trackedConn{Conn: c, owner: l}
	l.mu.Lock()
	l.conns[tc] = struct{}{}
	l.mu.Unlock()
	return tc, nil
}

// closeAll hangs up on every connection still open.
func (l *trackingListener) closeAll() {
	l.mu.Lock()
	open := make([]*trackedConn, 0, len(l.conns))
	for c := range l.conns {
		open = append(open, c)
	}
	l.mu.Unlock()
	for _, c := range open {
		_ = c.Close()
	}
}

type trackedConn struct {
	net.Conn
	owner *trackingListener
	once  sync.Once
}

func (c *trackedConn) Close() error {
	c.once.Do(func() {
		c.owner.mu.Lock()
		delete(c.owner.conns, c)
		c.owner.mu.Unlock()
	})
	return c.Conn.Close()
}
