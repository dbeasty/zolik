// Package mcp lets AI clients play at a table over the Model Context Protocol.
//
// An agent connects to POST /mcp with an ordinary bearer token (a guest token
// from /auth/guest is enough), registers itself, and then plays through tools:
// join a table, read its state, wait for its turn, act. It is seated as a
// human-like player flagged IsAgent, so every rule about presence applies to it
// — and a game where nobody has to stay connected (poker, blackjack) plays on
// without it when it goes quiet.
package mcp

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// PresenceTTL is how long after its last call an agent still counts as here.
// An LLM can think for a while between calls, so this is generous; a table
// that cannot wait that long is the drop-in kind, which has its own grace
// period on top.
const PresenceTTL = 90 * time.Second

// Agent is one registered AI client.
type Agent struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	// Available means the agent will take a seat a host offers it, rather than
	// only the tables it joins itself.
	Available bool `json:"available"`
	// Guest is whether the identity behind it is a guest, which decides how
	// its seat records who played.
	Guest    bool      `json:"-"`
	LastSeen time.Time `json:"lastSeen"`
}

// Registry is the in-memory set of agents. It is deliberately not persisted:
// an agent re-registers on every connection, and a stale record of a client
// that is no longer there would only offer hosts a seat nobody answers.
type Registry struct {
	mu     sync.Mutex
	agents map[string]*Agent
	now    func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{agents: map[string]*Agent{}, now: time.Now}
}

// SetClock replaces the clock. For tests.
func (r *Registry) SetClock(now func() time.Time) { r.now = now }

func clean(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		s = s[:max]
	}
	return s
}

// Register records an agent, or refreshes it, and returns it.
func (r *Registry) Register(id, name, label string, available, guest bool) Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.agents[id]
	if a == nil {
		a = &Agent{ID: id}
		r.agents[id] = a
	}
	if n := clean(name, 24); n != "" {
		a.Name = n
	}
	if a.Name == "" {
		a.Name = "Agent"
	}
	a.Label = clean(label, 32)
	a.Available = available
	a.Guest = guest
	a.LastSeen = r.now()
	return *a
}

// Touch notes that id was just heard from. Reports whether it is registered.
func (r *Registry) Touch(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.agents[id]
	if a == nil {
		return false
	}
	a.LastSeen = r.now()
	return true
}

// Get returns a registered agent.
func (r *Registry) Get(id string) (Agent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a := r.agents[id]; a != nil {
		return *a, true
	}
	return Agent{}, false
}

// SetAvailable flips whether hosts may seat the agent.
func (r *Registry) SetAvailable(id string, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a := r.agents[id]; a != nil {
		a.Available = on
	}
}

// Present implements match.AgentPresence.
func (r *Registry) Present(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.agents[id]
	return a != nil && r.now().Sub(a.LastSeen) < PresenceTTL
}

// Available lists the agents a host may seat: registered as available and
// heard from recently, by name.
func (r *Registry) Available() []Agent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Agent{}
	for _, a := range r.agents {
		if a.Available && r.now().Sub(a.LastSeen) < PresenceTTL {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
