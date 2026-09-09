package match

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/metrics"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
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

// ResumeAbandoned brings a swept-up table back to life.
//
// The sweeper resolves a table nobody came back to (see reaper.go), and that
// is right for the statistics — but it left the *player* with nowhere to go.
// A game abandoned while they were away came back as a board with a green dot
// reading "in progress", every control refused, and no way out but the browser
// back button. Nothing was lost when it was swept: the reaper writes a status
// and an end time and touches none of the module's state, so the hands, the
// melds, the draw pile and the score are all exactly where they were. Bringing
// it back is therefore a matter of undoing the envelope, and there is no
// restore step for the same reason SuspendOnDisconnect has none.
//
// Two conditions, and both are about other people rather than about the rules:
//
//   - Only a seated human may do it. A table is not a public resource, and
//     "abandoned" is not an invitation.
//   - Only when every other seat is a bot. A table with other people in it was
//     abandoned for all of them, and one player deciding on their own that it
//     is live again would resurrect a game the others have long since counted
//     as over — possibly hours later, possibly on a phone in a pocket. Where
//     other humans are involved the honest answer is a new table, which is
//     what the client offers beside this.
//
// The abandonment counter is deliberately left standing: the table really was
// abandoned, and the log line saying so is a fact about what this server did.
// MatchesResumed corrects the arithmetic instead, which is what completionRate
// subtracts — see the comment there for why a correction beats a retraction
// when the two events can land in different daily buckets.
func (m *Manager) ResumeAbandoned(ctx context.Context, matchID, playerID string) error {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return module.Error{Code: "INVALID_MATCH_ID", Message: matchID}
	}
	match, err := m.repo.FindByID(ctx, oid)
	if err != nil {
		return module.Error{Code: "MATCH_NOT_FOUND", Message: matchID}
	}
	if match.Status != string(rules.StatusAbandoned) {
		return module.Error{Code: "MATCH_NOT_ABANDONED", Message: "status is " + match.Status}
	}
	if p := playerByID(match.Players, playerID); p == nil || p.IsAI {
		return module.Error{Code: "NOT_AT_THIS_TABLE", Message: playerID}
	}
	for _, p := range match.Players {
		if p.ID != playerID && !p.IsAI {
			return module.Error{Code: "TABLE_HAS_OTHER_PLAYERS", Message: p.ID}
		}
	}

	expected := match.Version
	match.Status = "active"
	match.EndedAt = nil
	match.SuspendedAt = nil
	match.AbandonAt = nil
	match.SuspendedPlayer = ""

	// The version check is what makes a double press safe: two sockets asking
	// at once, or a player pressing twice on a slow connection, and only the
	// first write lands. The loser is told the table moved rather than being
	// allowed to write a second "active" over the first.
	if err := m.repo.UpdateWithVersion(ctx, oid, expected, match); err != nil {
		return module.Error{Code: "MATCH_MOVED_ON", Message: err.Error()}
	}
	match.Version = expected + 1

	m.metrics.Add(metrics.MatchesResumed, 1)
	m.metrics.Add(metrics.MatchesResumedFor(match.ModuleID), 1)
	log.Printf("match=%s player=%s resumed from abandoned", matchID, playerID)

	m.Broadcast(match)
	// The table was very likely swept mid-turn with bots waiting on a seat that
	// never came back, so the loop has to be restarted rather than left for the
	// player's next action — which, if the table is waiting on a bot, they have
	// no way to make.
	m.RunBotsIfNeeded(context.WithoutCancel(ctx), matchID)
	return nil
}
