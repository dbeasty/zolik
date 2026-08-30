// Package admission decides whether this server still has room for another
// player, and turns new arrivals away before they cost anything when it does
// not.
//
// The motivating measurement: on a 512 MiB / 1 vCPU instance running the
// embedded KDB engine, the server holds roughly 65 KiB per connected player
// (WebSocket buffers, session, match projection) on top of a ~45 MiB idle
// baseline. Ramping connections against that box, throughput and CPU stayed
// flat and healthy right up until memory hit the cgroup limit, at which point
// the kernel OOM-killed the process — every player on the box lost their game
// at once, mid-hand.
//
// That failure mode is the thing worth designing against. An OOM kill is
// indiscriminate: it does not shed the newest, cheapest-to-lose connection, it
// takes the whole table down. Refusing the 5001st arrival costs one player a
// retry; letting them in costs everyone the process. So the rule here is to
// stop admitting *before* the wall, and to shed in an order that matches what
// players actually lose:
//
//  1. A reconnect to a match already under way is never refused. It displaces
//     the player's own dead socket, so it is net-neutral on the count, and
//     refusing it would strand someone mid-hand — the exact position the rest
//     of this server works to avoid.
//  2. Waiting-room sockets are refused first. A player idling in the lobby
//     looking for a game has nothing invested; "try again in a moment" is a
//     mild disappointment, not a lost match.
//  3. New gameplay sockets are refused last, and only at the hard ceiling.
//
// Nothing here evicts an established connection. Backpressure means declining
// to grow, not killing players to make room — a server that drops live games
// to admit new ones would just redistribute the same pain.
package admission

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Class is what a would-be connection is for. It sets the order things are
// shed in when the server runs short of room: see the package comment.
type Class int

const (
	// ClassGameplay is a socket for a match — someone playing, or returning
	// to, a game. Refused only at the hard ceiling.
	ClassGameplay Class = iota
	// ClassWaiting is a waiting-room socket: a player available to be picked
	// up, with no game of their own yet. The first thing refused.
	ClassWaiting
)

func (c Class) String() string {
	if c == ClassWaiting {
		return "waiting"
	}
	return "gameplay"
}

// Reason is why an admission was refused, for logs and metrics. It is
// deliberately not what the player is shown — clients render the refusal from
// the message key the handler sends, not from this.
type Reason string

const (
	// ReasonAtCapacity means the connection ceiling is reached.
	ReasonAtCapacity Reason = "at_capacity"
	// ReasonMemoryPressure means measured memory crossed the high-water mark.
	// This is the gate that actually prevents the OOM; the count ceiling is a
	// predictable approximation of the same limit.
	ReasonMemoryPressure Reason = "memory_pressure"
	// ReasonWaitingRoomClosed means the server is busy enough that it has
	// stopped taking waiting-room arrivals, but is still serving matches.
	ReasonWaitingRoomClosed Reason = "waiting_room_closed"
	// ReasonCPUPressure means runnable work is measurably stalling on CPU.
	// Unlike memory pressure it refuses only growth — the waiting room and
	// new matches — never a gameplay socket: an overloaded CPU degrades, it
	// does not kill, and shedding players mid-hand to shave latency would
	// cost more than it saves.
	ReasonCPUPressure Reason = "cpu_pressure"
)

// Rejection is returned by Admit when there is no room. RetryAfter is a hint
// for the client and for the Retry-After header, not a promise.
type Rejection struct {
	Reason     Reason
	RetryAfter time.Duration
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("admission refused: %s", r.Reason)
}

// Limits configures a Controller. The zero value is a Controller that admits
// everything, which is what a deployment with no memory limit and no
// configured ceiling gets.
type Limits struct {
	// MaxConnections is the hard ceiling on concurrently held sockets. Zero
	// means no count-based ceiling — the memory gate is then the only limit.
	MaxConnections int

	// WaitingRoomRatio is the fraction of MaxConnections above which
	// waiting-room sockets are refused while gameplay sockets are still
	// admitted. 0.8 keeps the last fifth of capacity for people with a game
	// to get back to. Zero disables the reservation, making both classes
	// share one ceiling.
	WaitingRoomRatio float64

	// MemoryHighWatermark is the fraction of the process's memory limit at
	// which new connections stop being admitted. Zero disables the memory
	// gate, which is what happens where no limit can be read (an unconstrained
	// host, or a dev machine).
	//
	// This wants to sit far enough below 1.0 to cover the lag between the
	// decision and the memory actually landing: a connection admitted now
	// allocates its buffers and its match projection over the next few
	// hundred milliseconds, and a burst of arrivals can all pass the gate
	// before any of their cost shows up in the reading.
	MemoryHighWatermark float64

	// CPUHighWatermark is the CPU stall fraction above which the server stops
	// growing: no new waiting-room sockets, no new matches. The reading is
	// PSI "some avg10" — the fraction of the last ten seconds in which
	// runnable work sat waiting for a CPU — expressed 0..1 like the memory
	// watermark. 0.25 means "a quarter of the last ten seconds was spent
	// queueing", which on a small box is well past comfortable and still
	// short of seized. Zero disables the gate, and hosts where PSI cannot be
	// read (macOS, older kernels) behave as if it were zero.
	CPUHighWatermark float64

	// RetryAfter is the hint handed to a refused client.
	RetryAfter time.Duration
}

// Controller tracks how much of the server's capacity is currently spoken for
// and answers whether there is room for more.
//
// It is safe for concurrent use, and is deliberately cheap on the happy path:
// an atomic increment plus, at most once per sampling interval, a couple of
// small file reads. Nothing here holds a lock across a syscall.
type Controller struct {
	limits Limits
	live   atomic.Int64
	mem    *memoryGauge
	cpu    *cpuGauge

	// waitingCeiling is MaxConnections*WaitingRoomRatio, precomputed.
	waitingCeiling int64

	// refused counts rejections by reason, for observability.
	mu      sync.Mutex
	refused map[Reason]int64
}

// New builds a Controller. A nil gauge (or one that cannot read a limit)
// leaves the corresponding gate off.
func New(limits Limits) *Controller {
	return newWithGauges(limits, newMemoryGauge(), newCPUGauge())
}

func newWithGauge(limits Limits, gauge *memoryGauge) *Controller {
	return newWithGauges(limits, gauge, nil)
}

func newWithGauges(limits Limits, mem *memoryGauge, cpu *cpuGauge) *Controller {
	if limits.RetryAfter <= 0 {
		limits.RetryAfter = 5 * time.Second
	}
	c := &Controller{
		limits:  limits,
		mem:     mem,
		cpu:     cpu,
		refused: map[Reason]int64{},
	}
	if limits.MaxConnections > 0 && limits.WaitingRoomRatio > 0 && limits.WaitingRoomRatio < 1 {
		c.waitingCeiling = int64(float64(limits.MaxConnections) * limits.WaitingRoomRatio)
	}
	return c
}

// Release gives a slot back. It is safe to call more than once — handlers
// release from a defer that can run on paths where the slot was never taken,
// and a double decrement would slowly convince the server it has room it does
// not have.
type Release struct {
	once sync.Once
	c    *Controller
}

func (r *Release) Release() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		if r.c != nil {
			r.c.live.Add(-1)
		}
	})
}

// Admit takes a slot for a new connection of the given class, or explains why
// it will not.
//
// The caller must call Release on the returned handle when the connection
// ends. On refusal the handle is still non-nil and releasing it is a no-op, so
// a caller can defer the release before checking the error without having to
// reason about which path it is on.
// A nil Controller admits everything, so a handler can hold one
// unconditionally and a deployment (or a test) that has not configured
// admission control behaves exactly as it did before this existed.
func (c *Controller) Admit(class Class) (*Release, error) {
	if c == nil {
		return &Release{}, nil
	}
	// Reserve first, then validate, so that concurrent arrivals cannot both
	// read "one slot left" and both take it. An over-reservation is undone
	// below before returning the refusal.
	n := c.live.Add(1)
	rel := &Release{c: c}

	if max := int64(c.limits.MaxConnections); max > 0 && n > max {
		rel.Release()
		return rel, c.reject(ReasonAtCapacity)
	}
	if class == ClassWaiting && c.waitingCeiling > 0 && n > c.waitingCeiling {
		rel.Release()
		return rel, c.reject(ReasonWaitingRoomClosed)
	}
	if c.underMemoryPressure() {
		rel.Release()
		return rel, c.reject(ReasonMemoryPressure)
	}
	// CPU pressure sheds only growth: the waiting room here, new matches in
	// AllowMatchStart. A gameplay socket is someone joining or returning to a
	// table that already exists, and turning them away saves too little CPU
	// to justify what it costs them.
	if class == ClassWaiting && c.underCPUPressure() {
		rel.Release()
		return rel, c.reject(ReasonCPUPressure)
	}
	return rel, nil
}

// AllowMatchStart reports whether the server has the resources to take on a
// whole new match, or explains why it does not. Unlike Admit it reserves
// nothing — the HTTP request that creates or starts a match holds no slot;
// the sockets it leads to are admitted (and counted) individually.
//
// The bar is deliberately higher than for a single gameplay socket: a new
// match is a promise of several sockets, module state, and bot drivers for
// the next half hour. So it is refused at the same early threshold as the
// waiting room (the reserved tail of capacity is for finishing games, not
// starting them), and it is the one place CPU pressure refuses outright.
//
// A nil Controller allows everything, same as Admit.
func (c *Controller) AllowMatchStart() error {
	if c == nil {
		return nil
	}
	if reason := c.matchStartRefusal(); reason != "" {
		return c.reject(reason)
	}
	return nil
}

// matchStartRefusal is AllowMatchStart without the refusal bookkeeping, so
// Snapshot can ask the question without a health probe counting as a turned-
// away player. Empty means "go ahead".
func (c *Controller) matchStartRefusal() Reason {
	if ceiling := c.startCeiling(); ceiling > 0 && c.live.Load() >= ceiling {
		return ReasonAtCapacity
	}
	if c.underMemoryPressure() {
		return ReasonMemoryPressure
	}
	if c.underCPUPressure() {
		return ReasonCPUPressure
	}
	return ""
}

// startCeiling is where new matches stop being taken on: the waiting-room
// ceiling when one is configured, else the hard connection ceiling.
func (c *Controller) startCeiling() int64 {
	if c.waitingCeiling > 0 {
		return c.waitingCeiling
	}
	return int64(c.limits.MaxConnections)
}

// AdmitReconnect takes a slot without asking. It exists for the one arrival
// that is never refused — a player already registered in a room, whose new
// socket displaces their old one. The count still moves: the displaced
// handler's Release fires when it unwinds, so without this the ledger would
// drift downward on every reconnect until it swore an occupied server was
// empty. The overlap is at most one slot per reconnect, for as long as the
// two handlers coexist.
func (c *Controller) AdmitReconnect() *Release {
	if c == nil {
		return &Release{}
	}
	c.live.Add(1)
	return &Release{c: c}
}

func (c *Controller) reject(reason Reason) *Rejection {
	c.mu.Lock()
	c.refused[reason]++
	c.mu.Unlock()
	return &Rejection{Reason: reason, RetryAfter: c.limits.RetryAfter}
}

// underMemoryPressure reports whether measured memory has crossed the
// high-water mark. False whenever the gate is off or no reading is available:
// an unreadable gauge must not be able to wedge the server shut.
func (c *Controller) underMemoryPressure() bool {
	if c.limits.MemoryHighWatermark <= 0 || c.mem == nil {
		return false
	}
	used, limit, ok := c.mem.read()
	if !ok || limit == 0 {
		return false
	}
	return float64(used)/float64(limit) >= c.limits.MemoryHighWatermark
}

// underCPUPressure reports whether the CPU stall fraction has crossed the
// high-water mark. False whenever the gate is off or PSI cannot be read, for
// the same reason as the memory gauge: an unreadable gauge must not be able
// to wedge the server shut.
func (c *Controller) underCPUPressure() bool {
	if c.limits.CPUHighWatermark <= 0 || c.cpu == nil {
		return false
	}
	stall, ok := c.cpu.read()
	if !ok {
		return false
	}
	return stall >= c.limits.CPUHighWatermark
}

// Snapshot is the controller's state, for /healthz and for logs.
type Snapshot struct {
	Live           int64            `json:"live"`
	MaxConnections int              `json:"maxConnections,omitempty"`
	MemoryUsed     uint64           `json:"memoryUsedBytes,omitempty"`
	MemoryLimit    uint64           `json:"memoryLimitBytes,omitempty"`
	MemoryFraction float64          `json:"memoryFraction,omitempty"`
	CPUStall       float64          `json:"cpuStallFraction,omitempty"`
	Refused        map[Reason]int64 `json:"refused,omitempty"`
	// Accepting is whether ClassGameplay sockets would be admitted right now
	// (joining or returning to an existing match). CPU pressure does not
	// close this gate — an overloaded CPU degrades, it does not kill, and
	// refusing a gameplay socket saves too little to justify stranding someone
	// mid-hand.
	Accepting bool `json:"accepting"`
	// WaitingRoomOpen is whether ClassWaiting sockets would be admitted.
	// This is the first thing to close under pressure: a player idling in
	// the lobby has nothing invested yet.
	WaitingRoomOpen bool `json:"waitingRoomOpen"`
	// StartingMatches is whether AllowMatchStart would say yes right now.
	// Clients probe this to word a refusal they could not read off a failed
	// WebSocket handshake.
	StartingMatches bool `json:"startingMatches"`
}

func (c *Controller) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{Accepting: true, WaitingRoomOpen: true, StartingMatches: true}
	}
	s := Snapshot{
		Live:           c.live.Load(),
		MaxConnections: c.limits.MaxConnections,
		Refused:        map[Reason]int64{},
	}
	c.mu.Lock()
	for k, v := range c.refused {
		s.Refused[k] = v
	}
	c.mu.Unlock()

	if c.mem != nil {
		if used, limit, ok := c.mem.read(); ok && limit > 0 {
			s.MemoryUsed, s.MemoryLimit = used, limit
			s.MemoryFraction = float64(used) / float64(limit)
		}
	}
	if c.cpu != nil {
		if stall, ok := c.cpu.read(); ok {
			s.CPUStall = stall
		}
	}
	s.Accepting = c.gameplayRefusal() == ""
	s.WaitingRoomOpen = c.waitingRoomRefusal() == ""
	s.StartingMatches = c.matchStartRefusal() == ""
	return s
}

// gameplayRefusal is the reason a ClassGameplay socket would be turned away.
// Empty means "go ahead". CPU pressure is deliberately excluded — see Admit.
func (c *Controller) gameplayRefusal() Reason {
	if max := int64(c.limits.MaxConnections); max > 0 && c.live.Load() >= max {
		return ReasonAtCapacity
	}
	if c.underMemoryPressure() {
		return ReasonMemoryPressure
	}
	return ""
}

// waitingRoomRefusal is the reason a ClassWaiting socket would be turned away.
func (c *Controller) waitingRoomRefusal() Reason {
	if max := int64(c.limits.MaxConnections); max > 0 && c.live.Load() >= max {
		return ReasonAtCapacity
	}
	if c.waitingCeiling > 0 && c.live.Load() > c.waitingCeiling {
		return ReasonWaitingRoomClosed
	}
	if c.underMemoryPressure() {
		return ReasonMemoryPressure
	}
	if c.underCPUPressure() {
		return ReasonCPUPressure
	}
	return ""
}

// DeriveMaxConnections estimates how many connections fit in the memory the
// process is allowed, leaving headroom for the baseline and for the slack the
// high-water mark already reserves.
//
// It exists so a deployment does not have to hand-tune a number that follows
// mechanically from its instance size: on the 512 MiB box this was measured
// on it lands near 5,000, comfortably under the ~7,000 where the kernel
// stepped in. It is an estimate from one workload's average, not a promise —
// the memory gate is what actually holds the line, and this only keeps the
// count ceiling in the right neighbourhood.
//
// Returns 0 when no limit can be read, meaning "no count ceiling".
func DeriveMaxConnections(limitBytes uint64, baselineBytes uint64, perConnBytes uint64, watermark float64) int {
	if limitBytes == 0 || perConnBytes == 0 {
		return 0
	}
	if watermark <= 0 || watermark > 1 {
		watermark = 1
	}
	usable := float64(limitBytes)*watermark - float64(baselineBytes)
	if usable <= 0 {
		return 0
	}
	return int(usable / float64(perConnBytes))
}
