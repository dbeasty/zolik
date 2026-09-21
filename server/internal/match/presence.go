package match

import (
	"context"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
	"zolik/server/internal/ws"
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
	if !awaits(module.AwaitedSeats(mod, module.State(match.State), viewerFor(match), refsOf(match)), playerID) {
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

// resumableBy is ResumeAbandoned's own eligibility rule, pulled out so a
// listing can offer the same button it would actually honour rather than
// re-deriving the rule and risking the two disagreeing: a seated human, and
// every other human seat back at the table right now.
//
// The rule used to be "every other seat a bot", which was a proxy for the
// thing actually being protected — nobody should be able to restart a game
// on their own that the others have counted as over. As a proxy it was both
// too strong and, for the people it was protecting, useless: a game between
// two friends could never be picked up again by anybody, including the player
// who had never left and had sat watching it be swept up. The position was
// intact, both of them were looking at it, and the only offer either screen
// made was "Back to games".
//
// Presence is the honest version of the same protection. A player who is at
// the table has not counted the game as over — they are here, now, with it
// open in front of them — so there is nobody left to surprise. A seat that is
// away still blocks the resume, which is the case the original rule was
// written for and the one that is genuinely not one player's to decide.
//
// Under Redis fan-out the registry is still this instance's own, so a player
// held by a peer reads as away and their table stays unresumable until they
// are both on one instance. That is exactly today's behaviour for every human
// table, so it degrades to what shipped rather than to something new.
func (m *Manager) resumableBy(match models.Match, playerID string) bool {
	if p := playerByID(match.Players, playerID); p == nil || p.IsAI {
		return false
	}
	room := match.ID.Hex()
	for _, p := range match.Players {
		if p.ID == playerID || p.IsAI {
			continue
		}
		if !m.hub.Registry().Has(room, p.ID) {
			return false
		}
	}
	return true
}

// playersAway names the human seats resumableBy is still waiting for, so a
// refusal — and the banner a client draws before anyone presses anything —
// can say who rather than only no.
func (m *Manager) playersAway(match models.Match, playerID string) []string {
	room := match.ID.Hex()
	var away []string
	for _, p := range match.Players {
		if p.ID == playerID || p.IsAI {
			continue
		}
		if !m.hub.Registry().Has(room, p.ID) {
			away = append(away, p.ID)
		}
	}
	return away
}

// attended reports whether anybody is still sitting at this table.
//
// The reaper's question, not resumableBy's: one live human socket is enough,
// where resuming needs all of them. See ReapAbandoned for why a table with
// somebody still at it is not a stranded one.
func (m *Manager) attended(match models.Match) bool {
	room := match.ID.Hex()
	for _, p := range match.Players {
		if p.IsAI {
			continue
		}
		if m.hub.Registry().Has(room, p.ID) {
			return true
		}
	}
	return false
}

// AnnouncePresence re-sends a swept-up table's state to everyone at it, so
// that the resume offer follows who is in the room.
//
// Needed because CanResume is a fact about the *room*, not about the match:
// on an abandoned table it turns on whether every other human seat currently
// holds a socket, and a socket opening or closing writes nothing to the match
// for a broadcast to ride on. Without this the rule worked and nobody could
// use it — the first player to open the game saw "waiting for Bo", Bo then
// opened it and saw the resume, and the first player's screen went on saying
// "waiting for Bo" until they reloaded the page. Two people looking at the
// same table, one of them holding the only working button, is a worse failure
// than the dead end it replaced.
//
// Only for abandoned tables: every other status answers CanResume false no
// matter who is watching, so a broadcast per connection would be pure cost on
// the path every ordinary player takes.
func (m *Manager) AnnouncePresence(ctx context.Context, matchID string) {
	oid, err := bson.ObjectIDFromHex(matchID)
	if err != nil {
		return
	}
	match, err := m.repo.FindByID(ctx, oid)
	if err != nil || match.Status != string(rules.StatusAbandoned) {
		return
	}
	m.Broadcast(match)
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
//   - Only when every other human seat is back at the table. One player
//     deciding on their own that a game is live again would resurrect it for
//     people who have long since counted it as over — possibly hours later,
//     possibly on a phone in a pocket. Somebody looking at the board right now
//     has counted it as no such thing, so once everyone is here there is
//     nobody left for the resume to surprise. See resumableBy, which is also
//     what the client's own offer is drawn from.
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
	if !m.resumableBy(match, playerID) {
		if p := playerByID(match.Players, playerID); p == nil || p.IsAI {
			return module.Error{Code: "NOT_AT_THIS_TABLE", Message: playerID}
		}
		// Named rather than counted: "waiting for Anna" is something the
		// player can act on — send her the link — where "this table has other
		// players" is a fact they already knew.
		return module.Error{
			Code:    "TABLE_HAS_PLAYERS_AWAY",
			Message: strings.Join(m.playersAway(match, playerID), ","),
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

// DeleteAsHost ends a table at its host's request, for every seat.
//
// The rule is deliberately blunt: the host may always delete, in any status,
// and nobody else may ever. A narrower rule — solo-vs-bots only, mirroring
// ResumeAbandoned's own restriction — would leave the ordinary case (a host
// clearing an abandoned or finished table off their own list) needing every
// other seat to be a bot, which most hosted tables already satisfy but not
// all. The cost accepted here is real: a host can end a game other people are
// still playing. There is no vote and no "only if everyone left" grace period.
//
// Deleting a completed match is allowed and is the safe end of the range: its
// permanent record went to match_results the moment it finished
// (Manager.recorder is handed the match by value, so it never re-reads this
// row), and retention would delete the row itself once its window passes.
// What the host discards here is the board, not the history.
func (m *Manager) DeleteAsHost(ctx context.Context, idOrCode, playerID string) error {
	match, err := m.repo.Resolve(ctx, idOrCode)
	if err != nil {
		return module.Error{Code: "MATCH_NOT_FOUND", Message: idOrCode}
	}
	if playerByID(match.Players, playerID) == nil {
		return module.Error{Code: "NOT_AT_THIS_TABLE", Message: playerID}
	}
	if match.HostID != playerID {
		return module.Error{Code: "NOT_THE_HOST", Message: playerID}
	}

	// Captured before the write: once the row is gone there is nothing left
	// to read the seat list from, and the announcement below still needs it.
	id := match.ID
	status := match.Status

	// The same version guard retention uses, for the same reason: a delete
	// that ignored it could land between a player's action being applied and
	// being persisted, and that player would be told their move succeeded at
	// a table that no longer exists.
	if err := m.repo.DeleteIfUnchanged(ctx, id, match.Version); err != nil {
		return module.Error{Code: "MATCH_MOVED_ON", Message: err.Error()}
	}

	m.metrics.Add(metrics.MatchesDeletedByHost, 1)
	log.Printf("match=%s host=%s deleted a %s table", id.Hex(), playerID, status)
	m.announceDeleted(id.Hex(), match.Players)
	return nil
}

// errMatchDeleted is what a seated player's socket receives once their table
// has been deleted from under them. Built as a module.Error, like every other
// refusal this package emits, rather than a bare map literal: dump-keys only
// recognises a code at a construction site — a Code: field, a call argument,
// a bare return — and a string sitting in a map literal's value position is
// invisible to it, which would leave MATCH_DELETED reaching a player as
// SCREAMING_SNAKE with nothing to catch it.
var errMatchDeleted = module.Error{Code: "MATCH_DELETED"}

// announceDeleted tells every human seat the table is gone.
//
// Sent after the delete succeeds, never before: an announcement ahead of a
// version conflict would tell people a live game had ended when it had not.
// Publish, not WriteDirect — a seated player may be connected to a different
// server instance, and "your table is gone" is not a message worth losing to
// save a Redis round trip. Bots hold no socket and are skipped.
func (m *Manager) announceDeleted(matchID string, players []models.Player) {
	var msgs []ws.PlayerMessage
	for _, p := range players {
		if p.IsAI {
			continue
		}
		msgs = append(msgs, ws.PlayerMessage{PlayerID: p.ID, Payload: map[string]any{
			"type": "error", "code": module.CodeOf(errMatchDeleted),
		}})
	}
	if len(msgs) == 0 {
		return
	}
	m.hub.Publish(matchID, msgs)
}
