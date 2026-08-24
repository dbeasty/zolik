// Package lobby is the waiting room: the place a human player sits, visible
// to everyone else, until someone picks them up into a game. It is
// deliberately separate from a specific match's lobby ("players sitting at
// one table before it starts") — this is the pool a host draws *from*, one
// level up.
//
// Presence here rides the same WebSocket transport internal/ws already
// provides (Hub, ConnRegistry, the ping/pong keepalive) rather than a new one:
// a waiting player is simply a connection registered under one reserved
// room id, so the existing local-write-plus-Redis-fanout broadcast path
// works for the waiting room for free, on the same multi-instance
// deployment the game rooms already run on.
package lobby

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RoomID is the reserved Hub/ConnRegistry room this package's connections
// register under. It can never collide with a real game id — those are
// 24-character hex ObjectIds, and this is neither that shape nor a valid
// one, by construction.
const RoomID = "__lobby__"

// staleAfter bounds how long a Redis-mirrored presence entry survives
// without a heartbeat before List stops reporting it. It exists so a
// player whose instance crashed without a clean disconnect — the one case
// an explicit Leave call can't cover — eventually disappears from every
// other instance's view without requiring per-field TTL support from
// whatever Redis version is deployed (HEXPIRE is a 7.4+ feature; nothing
// here assumes it). heartbeatEvery is how often a connected client
// refreshes its own entry — comfortably inside staleAfter so a couple of
// missed beats under load don't flicker a still-present player out of the
// list.
const (
	staleAfter     = 45 * time.Second
	heartbeatEvery = 15 * time.Second
)

// Entry is one player currently waiting to be picked up.
type Entry struct {
	// PlayerID is the JWT subject: an account's ObjectID hex, or a guest's
	// durable device id. It is what a host's invite names as the target,
	// and what a game's Player.ID becomes if they're picked up.
	PlayerID string `json:"playerId"`
	Username string `json:"username"`
	IsGuest  bool   `json:"isGuest"`
	// JoinedAt is when this player most recently started waiting — reset on
	// reconnect, not carried across a disconnect, so the list reads as "who
	// has been here how long" rather than accumulating a lifetime figure.
	JoinedAt time.Time `json:"joinedAt"`
	// lastSeen drives Redis-backed staleness filtering; never serialized to
	// a client, which only ever needs to know someone is still here, not
	// the mechanism that decided so.
	lastSeen time.Time
}

// Store tracks who is currently waiting.
//
// Every instance keeps its own connections' entries locally (authoritative
// for who *this* instance can reach directly). When Redis is configured,
// entries are also mirrored there so List reflects players waiting on any
// instance, not just this one — the same trade the game Hub already makes
// for broadcast, applied to presence. Without Redis (local dev, the
// documented "local-only" mode), List simply reports what this instance
// knows, which is everyone, since there is only one instance.
type Store struct {
	mu    sync.RWMutex
	local map[string]Entry // playerID -> entry, this instance's connections

	redis *redis.Client
}

// NewStore builds a Store. redisURL empty means local-only, matching
// game.NewHub's own fallback — this package never requires more
// infrastructure than the game hub it rides on.
func NewStore(redisURL string) (*Store, error) {
	s := &Store{local: map[string]Entry{}}
	if redisURL == "" {
		return s, nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	s.redis = redis.NewClient(opt)
	if err := s.redis.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.redis != nil {
		return s.redis.Close()
	}
	return nil
}

const redisKey = "zolik:lobby:waiting"

// redisRecord is what's stored per waiting player in the Redis hash — Entry
// plus the heartbeat timestamp List filters on.
type redisRecord struct {
	Entry
	LastSeen time.Time `json:"lastSeen"`
}

// Join registers a player as waiting, replacing any prior entry for them
// (a reconnect resets JoinedAt rather than resuming the old one).
func (s *Store) Join(ctx context.Context, e Entry) {
	e.JoinedAt = time.Now().UTC()
	e.lastSeen = e.JoinedAt

	s.mu.Lock()
	s.local[e.PlayerID] = e
	s.mu.Unlock()

	s.mirror(ctx, e)
}

// Heartbeat refreshes a waiting player's staleness clock. A no-op if they
// are not currently registered locally (they disconnected on this instance
// already, and heartbeat loses the race — Leave is what actually matters).
func (s *Store) Heartbeat(ctx context.Context, playerID string) {
	s.mu.Lock()
	e, ok := s.local[playerID]
	if ok {
		e.lastSeen = time.Now().UTC()
		s.local[playerID] = e
	}
	s.mu.Unlock()
	if ok {
		s.mirror(ctx, e)
	}
}

// Leave removes a player from the waiting pool.
func (s *Store) Leave(ctx context.Context, playerID string) {
	s.mu.Lock()
	delete(s.local, playerID)
	s.mu.Unlock()

	if s.redis != nil {
		if err := s.redis.HDel(ctx, redisKey, playerID).Err(); err != nil {
			log.Printf("lobby: redis leave for %s: %v", playerID, err)
		}
	}
}

// Pickup removes a player exactly as Leave does, but reports whether they
// were actually present — the distinction an invite needs, since inviting
// someone who has already left (or was never here) must fail rather than
// silently seat a name nobody chose.
func (s *Store) Pickup(ctx context.Context, playerID string) bool {
	s.mu.Lock()
	_, wasLocal := s.local[playerID]
	delete(s.local, playerID)
	s.mu.Unlock()

	if s.redis == nil {
		return wasLocal
	}
	// HDel reports how many fields were actually removed — the authoritative
	// answer in the multi-instance case, where the player might be
	// connected to a peer instance and therefore absent from s.local here.
	n, err := s.redis.HDel(ctx, redisKey, playerID).Result()
	if err != nil {
		log.Printf("lobby: redis pickup for %s: %v", playerID, err)
		return wasLocal
	}
	return wasLocal || n > 0
}

// IsWaiting reports whether a player is currently in the pool, and if so,
// the display details a game seat is built from.
func (s *Store) IsWaiting(ctx context.Context, playerID string) (name string, isGuest bool, ok bool) {
	// When Redis is configured it is the cross-instance source of truth, and
	// it must be consulted rather than this instance's own local map:
	// Pickup or Leave called against a *different* instance (the common case
	// for an invite — the host's request lands wherever load-balancing sent
	// it, not necessarily where the waiting player's socket is) removes the
	// Redis entry but has no way to reach into this instance's local map.
	// Trusting local first here would let a player already picked up
	// elsewhere still read back as waiting on this instance.
	if s.redis != nil {
		return s.isWaitingRedis(ctx, playerID)
	}
	s.mu.RLock()
	e, found := s.local[playerID]
	s.mu.RUnlock()
	if !found {
		return "", false, false
	}
	return e.Username, e.IsGuest, true
}

func (s *Store) isWaitingRedis(ctx context.Context, playerID string) (name string, isGuest bool, ok bool) {
	raw, err := s.redis.HGet(ctx, redisKey, playerID).Result()
	if err != nil {
		return "", false, false
	}
	var rec redisRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return "", false, false
	}
	if time.Since(rec.LastSeen) > staleAfter {
		return "", false, false
	}
	return rec.Username, rec.IsGuest, true
}

// List returns everyone currently waiting, oldest first — the order a
// "who's been here longest" queue reads naturally in.
func (s *Store) List(ctx context.Context) []Entry {
	if s.redis == nil {
		return s.localSnapshot()
	}

	raw, err := s.redis.HGetAll(ctx, redisKey).Result()
	if err != nil {
		log.Printf("lobby: redis list: %v", err)
		// Degrade to this instance's own connections rather than reporting
		// an empty room while the cross-instance merge is briefly
		// unavailable.
		return s.localSnapshot()
	}
	now := time.Now().UTC()
	out := make([]Entry, 0, len(raw))
	for _, v := range raw {
		var rec redisRecord
		if err := json.Unmarshal([]byte(v), &rec); err != nil {
			continue
		}
		if now.Sub(rec.LastSeen) > staleAfter {
			continue
		}
		out = append(out, rec.Entry)
	}
	sortByJoinedAt(out)
	return out
}

func (s *Store) localSnapshot() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, 0, len(s.local))
	for _, e := range s.local {
		out = append(out, e)
	}
	sortByJoinedAt(out)
	return out
}

func sortByJoinedAt(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].JoinedAt.Before(entries[j].JoinedAt) })
}

func (s *Store) mirror(ctx context.Context, e Entry) {
	if s.redis == nil {
		return
	}
	raw, err := json.Marshal(redisRecord{Entry: e, LastSeen: e.lastSeen})
	if err != nil {
		return
	}
	if err := s.redis.HSet(ctx, redisKey, e.PlayerID, raw).Err(); err != nil {
		log.Printf("lobby: redis mirror for %s: %v", e.PlayerID, err)
	}
}
