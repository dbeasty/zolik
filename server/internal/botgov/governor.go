// Package botgov decides which engine plays each bot seat, so that the bots
// this server runs never cost more CPU than it can spare.
//
// It is Phase 2 of docs/bot-compute-gating-plan.md. Every bot decision falls
// in a class — a game, a skill and an engine (the hand-written rule bot, or a
// trained model) — and §5.4 measured what each costs. Most classes are cheap
// enough to ignore. The few that are not (Mariáš Hard, Žolíky's model) are
// played only by seats that hold a lease, and the leases add up to a share of
// the cores this process may use, scaled down as the resource monitor reports
// amber or red. A seat without a lease plays the next class down — the rule
// bot instead of the model, Medium instead of Hard — which is always cheap.
//
// The decision is made where it is safe to make it. A seat downgrades at the
// start of its next turn, never in the middle of one: a multi-action turn
// (draw, meld, meld, discard) is never finished by a bot that did not start
// it. It upgrades only at a round boundary, so an opponent never gets stronger
// in the middle of a hand.
package botgov

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"zolik/server/internal/capacity"
)

// Mode is how much the governor is allowed to do.
type Mode int

const (
	// Off decides nothing: every seat plays what it asked for.
	Off Mode = iota
	// Observe decides, counts and logs, and changes no move.
	Observe
	// Enforce plays the decision.
	Enforce
)

// ParseMode reads BOT_GOVERNOR's value. Anything unrecognised is Observe, the
// mode that cannot change a move.
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "false", "0":
		return Off
	case "enforce", "on", "true", "1":
		return Enforce
	}
	return Observe
}

func (m Mode) String() string {
	switch m {
	case Off:
		return "off"
	case Enforce:
		return "enforce"
	}
	return "observe"
}

func (m Mode) MarshalText() ([]byte, error) { return []byte(m.String()), nil }

// Engines, as botstats names them.
const (
	EngineRule = "rule"
	EngineNet  = "net"
)

// Class is one kind of bot decision.
type Class struct {
	Module string `json:"module"`
	Skill  string `json:"skill"`
	Engine string `json:"engine"`
}

func (c Class) String() string { return c.Module + "/" + c.Skill + "/" + c.Engine }

// Fallback is the class a seat plays when it may not play this one: the rule
// bot (Hard) for a model, one skill down for a rule bot. The
// second is false when there is nowhere lower to go.
func (c Class) Fallback() (Class, bool) {
	if c.Engine == EngineNet {
		// The network is the AI seat; the rule bot that stands in for it is
		// the strongest hand-written one, Hard.
		return Class{c.Module, "hard", EngineRule}, true
	}
	switch c.Skill {
	case "hard":
		return Class{c.Module, "medium", EngineRule}, true
	case "medium":
		return Class{c.Module, "easy", EngineRule}, true
	}
	return c, false
}

// Config tunes a Governor. The zero value of any field takes its default.
type Config struct {
	Mode Mode
	// Cores is how many cores this process may use — runtime.GOMAXPROCS(0),
	// which Go sizes from the container's CPU limit.
	Cores float64
	// Share is the fraction of Cores bots may spend: the rest is the
	// storage engine's, the sockets' and HTTP's. Default 0.5.
	Share float64
	// Comfort keeps headroom for bursts — every leased seat hitting its
	// expensive position at once. Default 0.7.
	Comfort float64
	// Think is the average pause between one seat's decisions, which turns a
	// cost per decision into a cost per second. Default 1.35 s.
	Think time.Duration
	// Floor is the mean cost below which a class needs no lease. Default 2 ms.
	Floor time.Duration
	// Costs are the mean CPU per decision of each class, before Speed.
	// Default DefaultCosts.
	Costs map[Class]time.Duration
	// Speed scales every cost: 2 means this machine takes twice as long as
	// the one the costs were measured on. Default 1. See Calibrate.
	Speed float64
	// Idle is how long a seat may go unseen before its lease is let go of —
	// the table finished, was abandoned, or moved to another node. Default
	// 2 minutes.
	Idle time.Duration
}

func (c Config) withDefaults() Config {
	if c.Cores <= 0 {
		c.Cores = 1
	}
	if c.Share <= 0 {
		c.Share = 0.5
	}
	if c.Comfort <= 0 {
		c.Comfort = 0.7
	}
	if c.Think <= 0 {
		c.Think = 1350 * time.Millisecond
	}
	if c.Floor <= 0 {
		c.Floor = 2 * time.Millisecond
	}
	if c.Costs == nil {
		c.Costs = DefaultCosts()
	}
	if c.Speed <= 0 {
		c.Speed = 1
	}
	if c.Idle <= 0 {
		c.Idle = 2 * time.Minute
	}
	return c
}

// Decision is what a seat plays this turn.
type Decision struct {
	// Class is what plays. Equal to the class asked for unless Reduced.
	Class Class
	// Reduced is a seat playing below what it asked for.
	Reduced bool
}

type seatKey struct{ match, seat string }

type seat struct {
	want    Class
	plays   Class
	leased  bool
	revoked bool
	weight  float64
	humans  bool
	round   int
	decided bool
	// grantedAt orders revocation: the most recent lease goes first.
	grantedAt time.Time
	lastSeen  time.Time
	// wouldReduce is what Observe mode would have played, for the count.
	wouldReduce bool
}

// Governor hands out leases. Safe for concurrent use.
type Governor struct {
	cfg Config

	mu    sync.Mutex
	level capacity.Level
	used  float64
	seats map[seatKey]*seat
	// moved is who made each match's last move, so a seat can tell the
	// start of its turn from the middle of one across bot-loop restarts.
	moved map[string]string

	reduced     int64
	wouldReduce int64
	revocations int64
	grants      int64
}

// New builds a governor.
func New(cfg Config) *Governor {
	return &Governor{cfg: cfg.withDefaults(), seats: map[seatKey]*seat{}, moved: map[string]string{}}
}

// Mode is the governor's mode; a nil governor is Off.
func (g *Governor) Mode() Mode {
	if g == nil {
		return Off
	}
	return g.cfg.Mode
}

// cost is a class's mean CPU per decision on this machine, and whether the
// table knows it. An unknown class is cheap: every class the server can seat
// that measured above the floor is in DefaultCosts.
func (g *Governor) cost(c Class) (time.Duration, bool) {
	d, ok := g.cfg.Costs[c]
	return time.Duration(float64(d) * g.cfg.Speed), ok
}

// weight is the share of one core a seat of this class keeps busy, or zero
// for a class cheap enough to need no lease.
func (g *Governor) weight(c Class) float64 {
	d, ok := g.cost(c)
	if !ok || d < g.cfg.Floor {
		return 0
	}
	return float64(d) / float64(g.cfg.Think)
}

// capacityLocked is how many cores' worth of leases the current level allows.
func (g *Governor) capacityLocked() float64 {
	full := g.cfg.Cores * g.cfg.Share * g.cfg.Comfort
	switch g.level {
	case capacity.Amber:
		return full / 2
	case capacity.Red:
		return 0
	}
	return full
}

// cheapest walks down from c to the first class that needs no lease.
func (g *Governor) cheapest(c Class) Class {
	for g.weight(c) > 0 {
		next, ok := c.Fallback()
		if !ok {
			break
		}
		c = next
	}
	return c
}

// Moved notes who made a match's latest move. The runtime calls it for every
// accepted action, a person's or a bot's.
func (g *Governor) Moved(matchID, playerID string) {
	if g == nil || g.cfg.Mode == Off {
		return
	}
	g.mu.Lock()
	g.moved[matchID] = playerID
	g.mu.Unlock()
}

// Decide is what a bot seat plays for its next move.
//
// want is what the seat asked for; round is how many rounds the match has
// completed (zero for a game without rounds); humans is whether a person sits
// at the table, which spares it from revocation until the bot-only tables
// have given theirs up.
func (g *Governor) Decide(matchID, seatID string, want Class, round int, humans bool, now time.Time) Decision {
	if g == nil || g.cfg.Mode == Off {
		return Decision{Class: want}
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	k := seatKey{matchID, seatID}
	s := g.seats[k]
	if s == nil || s.want != want {
		// A new seat, or one that now asks for something else — the
		// operator switched a game's model on or off between turns. Either
		// way, a fresh decision; a lease held for the old class is returned.
		if s != nil {
			g.releaseLocked(s)
		}
		s = &seat{want: want}
		g.seats[k] = s
	}
	s.lastSeen, s.humans = now, humans

	// Mid-turn: the seat made the last move, so this is the same turn and
	// it keeps whatever it started with.
	midTurn := s.decided && g.moved[matchID] == seatID
	if !midTurn {
		g.turnStartLocked(s, round, now)
	}

	if g.cfg.Mode == Observe {
		return Decision{Class: want}
	}
	return Decision{Class: s.plays, Reduced: s.plays != want}
}

// turnStartLocked is where a seat's engine may change.
func (g *Governor) turnStartLocked(s *seat, round int, now time.Time) {
	w := g.weight(s.want)
	switch {
	case w == 0:
		s.plays = s.want
	case s.leased && s.revoked:
		// Revoked while it played: give it up now, at the turn boundary.
		g.releaseLocked(s)
		s.plays = g.cheapest(s.want)
	case s.leased:
		// Keeps its lease.
	case !s.decided || round > s.round:
		// A seat's first turn, or the first since a round ended: the two
		// moments a seat may move up.
		if g.used+w <= g.capacityLocked()+1e-9 {
			s.leased, s.revoked, s.weight, s.grantedAt = true, false, w, now
			g.used += w
			g.grants++
			s.plays = s.want
		} else {
			s.plays = g.cheapest(s.want)
		}
	default:
		// No lease, mid-round: stays where it is.
	}
	reduced := s.plays != s.want
	if reduced {
		g.reduced++
		if g.cfg.Mode == Observe {
			g.wouldReduce++
		}
	}
	s.wouldReduce = reduced
	s.round, s.decided = round, true
}

func (g *Governor) releaseLocked(s *seat) {
	if s.leased {
		g.used -= s.weight
		if g.used < 1e-9 {
			g.used = 0
		}
	}
	s.leased, s.revoked, s.weight = false, false, 0
}

// SetLevel applies the resource monitor's level. A level that leaves fewer
// cores than are leased marks the newest leases revoked, bot-only tables
// before any table with a person at it; each revoked seat gives its lease up
// at the start of its next turn.
func (g *Governor) SetLevel(l capacity.Level) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.level = l
	g.revokeOverLocked()
}

func (g *Governor) revokeOverLocked() {
	capCores := g.capacityLocked()
	// What the leases still standing would add up to once the revoked ones
	// are given back.
	standing := 0.0
	var live []*seat
	for _, s := range g.seats {
		if s.leased && !s.revoked {
			standing += s.weight
			live = append(live, s)
		}
	}
	if standing <= capCores+1e-9 {
		return
	}
	sort.Slice(live, func(i, j int) bool {
		if live[i].humans != live[j].humans {
			return !live[i].humans // bot-only tables first
		}
		return live[i].grantedAt.After(live[j].grantedAt) // newest first
	})
	for _, s := range live {
		if standing <= capCores+1e-9 {
			break
		}
		s.revoked = true
		standing -= s.weight
		g.revocations++
	}
}

// ForHint is the class a hint is worked out with. A hint holds no seat and no
// lease — it is one decision, asked by a person — so it plays what it asks for
// while the server is green, and the cheapest class at or below it otherwise:
// a person asking for advice on a busy server gets a quick answer, not the
// dearest one. Unchanged outside Enforce.
func (g *Governor) ForHint(want Class) Class {
	if g == nil || g.cfg.Mode != Enforce {
		return want
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.level == capacity.Green {
		return want
	}
	return g.cheapest(want)
}

// Reduced reports whether a seat is currently playing below what it asked
// for. Always false outside Enforce, since nothing was changed.
func (g *Governor) Reduced(matchID, seatID string) bool {
	if g == nil || g.cfg.Mode != Enforce {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	s := g.seats[seatKey{matchID, seatID}]
	return s != nil && s.decided && s.plays != s.want
}

// ReleaseMatch lets go of every seat of a match that has stopped.
func (g *Governor) ReleaseMatch(matchID string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, s := range g.seats {
		if k.match == matchID {
			g.releaseLocked(s)
			delete(g.seats, k)
		}
	}
	delete(g.moved, matchID)
}

// Sweep releases seats not seen for Config.Idle — a match that ended or moved
// away without the runtime saying so. Returns how many it let go of.
func (g *Governor) Sweep(now time.Time) int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for k, s := range g.seats {
		if now.Sub(s.lastSeen) >= g.cfg.Idle {
			g.releaseLocked(s)
			delete(g.seats, k)
			n++
		}
	}
	live := map[string]bool{}
	for k := range g.seats {
		live[k.match] = true
	}
	for m := range g.moved {
		if !live[m] {
			delete(g.moved, m)
		}
	}
	return n
}

// Status is the governor at one moment, for /debug/capacity and /bots/capacity.
type Status struct {
	Mode          Mode           `json:"mode"`
	Level         capacity.Level `json:"level"`
	CapacityCores float64        `json:"capacityCores"`
	LeasedCores   float64        `json:"leasedCores"`
	Leases        int            `json:"leases"`
	Seats         int            `json:"seats"`
	// ReducedSeats are seats playing below what they asked for now (in
	// Observe: would be).
	ReducedSeats int     `json:"reducedSeats"`
	Speed        float64 `json:"speed"`
	// Classes is every class that needs a lease, with the seats one core of
	// this machine could carry at full capacity.
	Classes      []ClassStatus `json:"classes"`
	Grants       int64         `json:"grantsTotal"`
	Revocations  int64         `json:"revocationsTotal"`
	ReducedTurns int64         `json:"reducedTurnsTotal"`
}

// ClassStatus is one leased class.
type ClassStatus struct {
	Class
	MeanCPU      string  `json:"meanCpu"`
	SeatsPerCore float64 `json:"seatsPerCore"`
	Plays        Class   `json:"fallback"`
}

// Status reports the governor.
func (g *Governor) Status() Status {
	if g == nil {
		return Status{}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	st := Status{
		Mode: g.cfg.Mode, Level: g.level, CapacityCores: g.capacityLocked(), LeasedCores: g.used,
		Seats: len(g.seats), Speed: g.cfg.Speed,
		Grants: g.grants, Revocations: g.revocations, ReducedTurns: g.reduced,
	}
	for _, s := range g.seats {
		if s.leased {
			st.Leases++
		}
		if s.decided && s.wouldReduce {
			st.ReducedSeats++
		}
	}
	for c := range g.cfg.Costs {
		if w := g.weight(c); w > 0 {
			d, _ := g.cost(c)
			st.Classes = append(st.Classes, ClassStatus{
				Class: c, MeanCPU: d.String(),
				SeatsPerCore: g.cfg.Share * g.cfg.Comfort / w,
				Plays:        g.cheapest(c),
			})
		}
	}
	sort.Slice(st.Classes, func(i, j int) bool { return st.Classes[i].Class.String() < st.Classes[j].Class.String() })
	return st
}

// String renders a decision for a log line.
func (d Decision) String() string {
	if !d.Reduced {
		return d.Class.String()
	}
	return fmt.Sprintf("%s (reduced)", d.Class)
}
