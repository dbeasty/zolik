package mcp

import (
	"testing"
	"time"
)

// Two clients registering the same client_name are two agents unless their
// codes go to the same host: the name is the client's own choice, the
// redirect host is not. A reconnect of one client (same name, same host, a
// fresh client_id) keeps its identity.
func TestAgentIDIsNotJustTheClientName(t *testing.T) {
	real := agentIDFor("u1", "Claude", "https://claude.ai/api/mcp/auth_callback")
	again := agentIDFor("u1", "Claude", "https://CLAUDE.ai/api/mcp/other_path")
	other := agentIDFor("u1", "Claude", "https://evil.example/cb")
	if real != again {
		t.Errorf("the same client on the same host must keep its id: %s vs %s", real, again)
	}
	if real == other {
		t.Errorf("a client borrowing the name %q got the real one's agent id %s", "Claude", real)
	}
	if agentIDFor("u2", "Claude", "https://claude.ai/cb") == agentIDFor("u1", "Claude", "https://claude.ai/cb") {
		t.Error("two people share an agent id")
	}
}

// An agent unheard from for EvictAfter is dropped the next time anyone
// registers; one heard from recently stays.
func TestRegistryEvictsStaleAgents(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	r := NewRegistry()
	r.SetClock(func() time.Time { return now })
	r.Register("old", "Old", "", false, false)
	now = now.Add(EvictAfter - time.Hour)
	r.Register("recent", "Recent", "", false, false)
	now = now.Add(2 * time.Hour)
	r.Register("new", "New", "", false, false)
	if _, ok := r.Get("old"); ok {
		t.Error("an agent unheard from past EvictAfter is still registered")
	}
	for _, id := range []string{"recent", "new"} {
		if _, ok := r.Get(id); !ok {
			t.Errorf("%s was evicted", id)
		}
	}
	if r.Len() != 2 {
		t.Errorf("registry holds %d agents, want 2", r.Len())
	}
}
