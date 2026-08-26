package match

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/module"
)

// Reconnection, for every game.
//
// The rummy runtime had this and nothing else did, which meant a Prší player
// whose train went into a tunnel simply left their table stuck. It is a
// *runtime* property — a seat went quiet, and the table is waiting on it — so
// it belongs here, where every module gets it.
//
// It is also markedly simpler than the version it replaces, and the reason is
// the module seam. The rummy version had to stash the phase it interrupted
// (`PreSuspendPhase`) and put the game into a `suspended` *phase*, because
// suspension was mixed into the same state machine as drawing and melding.
// Here the module's state is untouched: the game is exactly where it was, and
// only the envelope knows anything happened. There is nothing to restore.

// AbandonWindow is how long a table waits for a missing player before the
// match may be abandoned.
const AbandonWindow = 2 * time.Minute

// SuspendOnDisconnect pauses a match when the player it is waiting on drops.
//
// Only when it is waiting on them: a spectator or an idle opponent losing a
// socket is not a reason to stop the game, and treating it as one would let any
// player pause a match they were losing by pulling their network cable.
func (m *Manager) SuspendOnDisconnect(ctx context.Context, matchID, playerID, reason string) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return
	}
	match, err := m.repo.FindByID(ctx, oid)
	if err != nil || match.Status != "active" {
		return
	}
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return
	}
	// A bot's socket cannot drop, and a bot seat should never pause a table.
	if p := playerByID(match.Players, playerID); p == nil || p.IsAI {
		return
	}
	// Suspended when the table is waiting on this player — which between rounds
	// can be several people at once. Comparing against a single "active" seat
	// meant that a player who dropped while the table waited on them, but who
	// was not the first such seat, left it waiting on a socket that was never
	// coming back: still active, never abandoned, and this is edge-triggered on
	// the disconnect so it would not fire again.
	if !awaits(module.AwaitedSeats(mod, match.State, viewerFor(match), refsOf(match)), playerID) {
		return
	}

	now := time.Now().UTC()
	abandon := now.Add(AbandonWindow)
	expected := match.Version
	match.Status = "suspended"
	match.SuspendedAt = &now
	match.AbandonAt = &abandon
	match.SuspendedPlayer = playerID

	if err := m.repo.UpdateWithVersion(ctx, oid, expected, match); err != nil {
		log.Printf("match=%s player=%s suspend failed: %v", matchID, playerID, err)
		return
	}
	match.Version = expected + 1
	log.Printf("match=%s player=%s suspended (%s)", matchID, playerID, reason)
	m.Broadcast(match)
}

// ResumeIfReturning un-suspends a match when the player it was waiting on
// comes back.
//
// Only that player: a match suspended for one seat is not resumed by a
// different one reconnecting, or by a spectator arriving.
func (m *Manager) ResumeIfReturning(ctx context.Context, matchID, playerID string) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return
	}
	match, err := m.repo.FindByID(ctx, oid)
	if err != nil || match.Status != "suspended" {
		return
	}
	if match.SuspendedPlayer != playerID {
		return
	}

	expected := match.Version
	match.Status = "active"
	match.SuspendedAt = nil
	match.AbandonAt = nil
	match.SuspendedPlayer = ""

	if err := m.repo.UpdateWithVersion(ctx, oid, expected, match); err != nil {
		log.Printf("match=%s player=%s resume failed: %v", matchID, playerID, err)
		return
	}
	match.Version = expected + 1
	log.Printf("match=%s player=%s resumed", matchID, playerID)
	m.Broadcast(match)

	// The returning player may not be the only one who was waiting: a bot
	// behind them in the order has been idle too.
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), matchID)
}

func awaits(seats []string, playerID string) bool {
	for _, id := range seats {
		if id == playerID {
			return true
		}
	}
	return false
}
