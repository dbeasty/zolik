package sync

import (
	"log"
	"sort"
	"sync"
	"time"

	"zolik/server/internal/db"
)

// A self-hosted node - a server in a club, a games room, somebody's house -
// is a spoke like a phone, but it does not belong to one person. Whoever
// walks in and signs in is who it holds data for, and that set changes by the
// evening rather than by the install.
//
// So its sync set is not configured: it follows a person's namespaces while
// they are playing there and lets go some time after they stop. Holding
// everyone who ever signed in would turn a machine in somebody's house into a
// copy of the whole database, which is both more than it needs and more than
// its owner should have.

// attendanceWindow is how long after somebody's last activity this node keeps
// holding their data.
//
// A day rather than an hour: somebody who plays on Friday and comes back on
// Saturday should find their things already here rather than waiting for a
// sync, and a club night that runs past midnight is one visit, not two.
const attendanceWindow = 24 * time.Hour

type attendance struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// Attend records that a person is using this node, and makes sure it is
// following their data. Cheap to call on every request: it only asks the
// replicator for a change when the set actually changes.
func (n *Node) Attend(userHex string) {
	if n == nil || n.cfg.Role != RoleSpoke || userHex == "" {
		return
	}
	n.attend.mu.Lock()
	if n.attend.seen == nil {
		n.attend.seen = map[string]time.Time{}
	}
	_, known := n.attend.seen[userHex]
	n.attend.seen[userHex] = time.Now()
	n.attend.mu.Unlock()
	if known {
		return
	}
	if err := n.Follow(db.UserNS(userHex), db.UserReadOnlyNS(userHex)); err != nil {
		log.Printf("sync: following %s: %v", userHex, err)
	}
}

// Attending lists the people this node is currently holding data for.
func (n *Node) Attending() []string {
	if n == nil {
		return nil
	}
	n.attend.mu.Lock()
	defer n.attend.mu.Unlock()
	out := make([]string, 0, len(n.attend.seen))
	for user := range n.attend.seen {
		out = append(out, user)
	}
	sort.Strings(out)
	return out
}

// ForgetIdle stops following the people who have not used this node for the
// attendance window, and returns how many it let go.
//
// It stops syncing them; it does not delete what is already here. Deleting is
// a different decision with a different failure mode - somebody coming back to
// find their scorepads gone - and it belongs to whoever runs the machine, not
// to a timer.
func (n *Node) ForgetIdle(now time.Time) int {
	if n == nil || n.cfg.Role != RoleSpoke {
		return 0
	}
	n.attend.mu.Lock()
	var stale []string
	for user, seen := range n.attend.seen {
		if now.Sub(seen) > attendanceWindow {
			stale = append(stale, user)
			delete(n.attend.seen, user)
		}
	}
	n.attend.mu.Unlock()

	for _, user := range stale {
		if err := n.Unfollow(db.UserNS(user), db.UserReadOnlyNS(user)); err != nil {
			log.Printf("sync: letting go of %s: %v", user, err)
		}
	}
	return len(stale)
}
