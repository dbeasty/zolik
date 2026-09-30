package match

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// What the distributed database needs to know before it lets a match change
// hands, and what it has to be able to do to this process when it does.
//
// A match is written by one node at a time. Moving it means this process has
// to stop being the writer completely - not "mostly", and not "after the bot
// finishes thinking" - before another node starts, because the two overlapping
// is exactly the state the whole single-writer arrangement exists to prevent.
// The fence in the store would refuse the loser's write, but a bot that
// carries on playing against a board nobody is storing is a worse thing to
// explain than a refusal.

// Seated reports whether a user is one of a match's players.
//
// Asked of the node that currently writes the match, so it reads the match it
// is holding rather than a copy that may be behind.
func (m *Manager) Seated(ctx context.Context, matchHex, userHex string) (bool, error) {
	id, err := bson.ObjectIDFromHex(matchHex)
	if err != nil {
		return false, fmt.Errorf("match: %q is not a match id: %w", matchHex, err)
	}
	env, err := m.repo.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	for _, p := range env.Players {
		if p.UserID == userHex {
			return true, nil
		}
	}
	return false, nil
}

// Idle reports whether the match can change hands right now: nothing this
// process is doing to it is half-finished.
//
// Both halves matter. An entry whose lock is held is a move being applied, and
// handing over underneath it would land that move here and the next one
// somewhere else. A bot loop is not holding the lock between turns, but it
// will take it again in a second or two, and a bot playing on a board this
// node no longer owns is a game that silently stops making sense.
func (m *Manager) Idle(_ context.Context, matchHex string) (bool, error) {
	m.botMu.Lock()
	running := m.botRunning[matchHex]
	m.botMu.Unlock()
	if running {
		return false, nil
	}
	e := m.live.peek(matchHex)
	if e == nil {
		// Not held here at all, which is as idle as a match gets.
		return true, nil
	}
	if !e.mu.TryLock() {
		return false, nil
	}
	e.mu.Unlock()
	return true, nil
}

// Release stops this process writing a match: the bots stop, and the board is
// dropped from memory so nothing here can answer for it.
//
// It waits for the bot loop rather than only asking it to stop, because the
// point of the call is that when it returns, this node is no longer playing.
func (m *Manager) Release(ctx context.Context, matchHex string) error {
	m.stopBots(matchHex)
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.botMu.Lock()
		running := m.botRunning[matchHex]
		m.botMu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("match: %s is still being played here", matchHex)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	m.live.forget(matchHex)
	return nil
}

// Adopt loads a match this node has just become the writer of, so the first
// move played here does not wait for a rebuild, and so a board that cannot be
// rebuilt is discovered now rather than mid-turn.
func (m *Manager) Adopt(ctx context.Context, matchHex string) error {
	// Forget first: anything held from before the match was somewhere else is
	// a board with somebody else's moves missing from it.
	m.live.forget(matchHex)
	e, err := m.lockMatch(ctx, matchHex)
	if err != nil {
		return err
	}
	defer e.mu.Unlock()
	if e.stateErr != nil {
		return fmt.Errorf("match: %s came across but its board could not be rebuilt: %w", matchHex, e.stateErr)
	}
	// Bots resume wherever the match is, which is here now.
	m.RunBotsIfNeeded(ctx, matchHex)
	return nil
}

// stopBots asks a match's bot loop to finish. The loop checks the flag between
// turns, which is where a handover can safely happen.
func (m *Manager) stopBots(matchID string) {
	m.botMu.Lock()
	if m.botStopping == nil {
		m.botStopping = map[string]bool{}
	}
	if m.botRunning[matchID] {
		m.botStopping[matchID] = true
	}
	m.botMu.Unlock()
}

// botsStopping reports whether a bot loop has been asked to stop, and clears
// the request when it has.
func (m *Manager) botsStopping(matchID string) bool {
	m.botMu.Lock()
	defer m.botMu.Unlock()
	if m.botStopping[matchID] {
		delete(m.botStopping, matchID)
		return true
	}
	return false
}
