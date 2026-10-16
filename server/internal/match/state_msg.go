package match

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"time"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// MatchStateMsg is what a client renders a hosted match from.
//
// Compare GameStateMsg: 24 rummy-named fields (drawPile, melds, meldMeta,
// roundReqMet, discardDrawnCardPendingMeld…) plus a growing set of ad-hoc
// booleans. This has none, because it has no idea what game it is describing —
// a board is zones, and what a player may do is offers.
type MatchStateMsg struct {
	Type      string `json:"type"`
	MatchID   string `json:"matchId"`
	ModuleID  string `json:"moduleId"`
	Variation string `json:"variation,omitempty"`
	// Deck is the module's pack, when it is not the French one — see
	// module.ModuleDescriptor.Deck. On every frame rather than only in
	// /modules, so a match screen, a replay and a reconnect all draw the
	// cards the same way without a second request.
	Deck     string `json:"deck,omitempty"`
	Status   string `json:"status"`
	JoinCode string `json:"joinCode,omitempty"`
	// InviteURL is the join code as something a host can send to somebody:
	// a link that opens the client and seats whoever follows it.
	//
	// Minted here rather than assembled by each client because the two clients
	// would have to agree on the route byte-for-byte, and because only the
	// server knows how the outside world reaches this deployment — a phone on
	// a LAN address would otherwise share a link that works nowhere else.
	// Omitted when no public base is configured; a client then falls back to
	// showing the code, which is all it ever had.
	InviteURL string `json:"inviteUrl,omitempty"`
	HostID    string `json:"hostId,omitempty"`
	// Options is what the lobby chose, echoed back so a client can offer the
	// same table again without remembering how this one was set up — the
	// numbers are already the module's own vocabulary, so passing them on
	// teaches the client nothing about any game.
	Options map[string]int `json:"options,omitempty"`
	// WinnerID is the first winner; Winners is all of them. Both ship, because
	// a partnership and a split pot each have more than one and a client
	// written before that was true should not break.
	WinnerID string   `json:"winnerId,omitempty"`
	Winners  []string `json:"winners,omitempty"`

	Players []PlayerMsg `json:"players"`
	// Sides is who is playing with whom, in seat order, for a table that has
	// not been dealt yet — the lobby's answer to "who is my partner if we start
	// now". Absent once the match is active, where the board carries the
	// partnerships itself, and absent entirely for a game with no sides.
	Sides [][]string `json:"sides,omitempty"`
	// View is the board as this viewer may see it — the only place hidden
	// information is filtered, decided by the module.
	View module.ViewModel `json:"view"`
	// LegalActions is what this viewer may do right now.
	LegalActions []module.ActionOffer `json:"legalActions"`
	// Standings is the scoreboard, in a shape no game owns — so one screen can
	// show who is ahead at rummy, canasta and poker without knowing what any of
	// those measure.
	Standings []module.Standing `json:"standings,omitempty"`
	// Rounds is the round-by-round history and, between rounds, the fact that
	// the table is waiting on its seats.
	//
	// Absent for a game with no rounds, which is not the same as a game with
	// none played yet — Prší sends nothing, a fresh Canasta match sends an
	// empty list. It rides on the state message rather than on an event so that
	// it survives a reconnection and a page reload, which the deal-ended events
	// a client might otherwise have accumulated do not.
	Rounds *module.RoundLog `json:"rounds,omitempty"`
	// SuspendedPlayer names the seat a paused match is waiting for.
	SuspendedPlayer string `json:"suspendedPlayer,omitempty"`
	// CanResume says whether this viewer may bring a swept-up table back,
	// and AwayPlayers names who is missing when they may not.
	//
	// Both are answered by the server rather than worked out on the board,
	// because the answer depends on who currently holds a socket in this
	// match's room — which no client can see. Deriving it was how a screen
	// came to offer a resume the server would refuse, and, worse, how a
	// resumable table came to offer nothing at all: the client's stand-in
	// rule was "every other seat is a bot", so two friends looking at their
	// own intact game were shown a banner with one button on it, "Back to
	// games".
	//
	// Only meaningful on an abandoned table; false everywhere else, where the
	// question does not arise.
	CanResume bool `json:"canResume,omitempty"`
	// AwayPlayers is who still has to come back, in seat order. Ids, not
	// names: the client already has the names, in Players.
	AwayPlayers []string `json:"awayPlayers,omitempty"`
	// StoodIn is every seat a stand-in bot has played for at this table, so
	// a results screen can say whose record the match does not count on.
	StoodIn []string `json:"stoodIn,omitempty"`
	// Rematch is the table this finished one is being played again at, and
	// who asked — what turns "Play again" into "Join Bob's rematch" for
	// everybody else who is still looking at the result.
	Rematch *models.RematchRef `json:"rematch,omitempty"`
	// CanDealAgain says this finished game may be dealt again, card for card,
	// at a new table — see Manager.DealAgain. The server decides, so a client
	// never has to know which games are played alone.
	CanDealAgain bool `json:"canDealAgain,omitempty"`
	// RepeatDeal says the player at this table had seen this deal before, so
	// the finished banner can say a score was not a first attempt. A flag and
	// not the match it came from: see models.Match.DealFrom.
	RepeatDeal bool `json:"repeatDeal,omitempty"`
	// Reserved is who a rematch lobby is still holding seats for.
	Reserved []models.Reservation `json:"reserved,omitempty"`
	// RecentMoves is what the last few moves at the table were, as this viewer
	// may read them, oldest first. It rides on the state message, not on the
	// events, so it survives a reconnection. Absent for a game whose module
	// does not narrate its moves.
	RecentMoves []module.Move `json:"recentMoves,omitempty"`
}

type PlayerMsg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IsAI bool   `json:"isAI"`
	// IsAgent is an AI client playing over MCP; AgentLabel is its client or
	// model. SatOut is a seat the table is playing on without — its player
	// has been away past the grace period at a drop-in game.
	IsAgent    bool   `json:"isAgent,omitempty"`
	AgentLabel string `json:"agentLabel,omitempty"`
	SatOut     bool   `json:"satOut,omitempty"`
	// Simplified is a bot seat the server is playing with a cheaper engine
	// than it was seated with, because CPU is short (internal/botgov). It
	// goes back to full strength by itself, at a round boundary.
	Simplified bool `json:"simplified,omitempty"`
	// Avatar is the face this seat wears — cosmetic, opaque, and omitted when
	// the seat never named one, which every client reads as "derive it".
	//
	// It rides on the state message rather than being fetched per player
	// because it is part of who is at the table, and who is at the table is
	// already here. Deliberately absent from module.PlayerRef: a module
	// decides rules, and a face is not one.
	Avatar string `json:"avatar,omitempty"`
	// Skill is how well a bot seat plays (module.Skill), so the host's
	// strength picker can show the current choice. Omitted for people.
	Skill string `json:"skill,omitempty"`
	// StandIn is a bot playing this person's seat while they are away (see
	// standin.go); the seat stays theirs. StandInAt is when one will, for a
	// seat that is away now: what a "a bot plays for them in 0:42" counts
	// down to. Both are facts about the room, so a replay frame has neither.
	StandIn   *StandInMsg `json:"standIn,omitempty"`
	StandInAt *time.Time  `json:"standInAt,omitempty"`
}

// StandInMsg is a stand-in as the table sees it.
type StandInMsg struct {
	Skill string    `json:"skill"`
	Since time.Time `json:"since"`
	// By is "timeout" when the wait ran out, "host" when the host put it in.
	By string `json:"by"`
}

// BuildStateMsg renders one viewer's state.
//
// A lobby has no state yet, so the module is not asked for a view — which is
// itself a small demonstration that the runtime can describe a match before
// the game owning it has done anything.
func (m *Manager) BuildStateMsg(match models.Match, viewerID string) MatchStateMsg {
	msg := m.buildStateMsg(match, viewerID, module.RoundsFor(m.registry.Get(match.ModuleID), module.State(match.State)), nil)
	return m.withSatOut(match, msg)
}

// withSatOut marks who the table is playing past. A fact about the room right
// now, so only a live state carries it — a replay frame is a past position and
// must not ask.
func (m *Manager) withSatOut(match models.Match, msg MatchStateMsg) MatchStateMsg {
	return withSeatFacts(msg, m.seatFacts(match))
}

// seatFact is what withSatOut adds to one seat. None of it depends on who is
// looking, so a broadcast works it out once for every recipient rather than
// once per recipient per seat — which, through standInDue, used to mean
// building the module's descriptor twice for each.
type seatFact struct {
	satOut, simplified bool
	standIn            *StandInMsg
	standInAt          *time.Time
}

func (m *Manager) seatFacts(match models.Match) []seatFact {
	out := make([]seatFact, len(match.Players))
	for i, p := range match.Players {
		out[i].satOut = m.satOut(match, p)
		if p.StandIn != nil {
			out[i].standIn = &StandInMsg{Skill: p.StandIn.Skill, Since: p.StandIn.Since, By: p.StandIn.By}
		} else if due, ok := m.standInDue(match, p); ok {
			out[i].standInAt = &due
		}
		if p.IsAI {
			out[i].simplified = m.governor.Reduced(match.ID.Hex(), p.ID)
		}
	}
	return out
}

// withSeatFacts writes the facts into one recipient's message. Each message
// gets its own copies of the pointed-to values, so no two recipients' frames
// share memory.
func withSeatFacts(msg MatchStateMsg, facts []seatFact) MatchStateMsg {
	for i, f := range facts {
		if i >= len(msg.Players) {
			break
		}
		msg.Players[i].SatOut = f.satOut
		if f.standIn != nil {
			in := *f.standIn
			msg.Players[i].StandIn = &in
		} else if f.standInAt != nil {
			at := *f.standInAt
			msg.Players[i].StandInAt = &at
		}
		if f.simplified {
			msg.Players[i].Simplified = true
		}
	}
	return msg
}

// buildStateMsg is BuildStateMsg with the round log handed in.
//
// It exists because a broadcast renders one message per viewer, and the round
// log is the one part of a state message that is the same for all of them — it
// takes no viewer, by construction. Rebuilding it per recipient meant decoding
// the module's whole state an extra time for every seat at the table, on every
// single action.
func (m *Manager) buildStateMsg(match models.Match, viewerID string, rounds *module.RoundLog, proj module.Projection) MatchStateMsg {
	return m.buildStateMsgWith(match, viewerID, stateMsgOpts{rounds: rounds, proj: proj, withOffers: true})
}

// buildStateMsgWith is buildStateMsg with every shared part in hand.
func (m *Manager) buildStateMsgWith(match models.Match, viewerID string, o stateMsgOpts) MatchStateMsg {
	msg := m.projectStateMsg(match, viewerID, o)
	if e := m.live.peek(match.ID.Hex()); e != nil {
		msg.RecentMoves = e.recentFor(viewerID)
	}
	return msg
}

// stateMsgOpts is what varies between the two things a projection is wanted
// for: a live board, and one frame of a replay.
type stateMsgOpts struct {
	rounds *module.RoundLog
	// proj is the state already decoded, shared by every viewer of one
	// broadcast; nil means ask the module, which decodes for itself.
	proj module.Projection
	// withOffers asks the module what this viewer may do. A replay frame says
	// no: nobody is playing it, and offer enumeration is both the most
	// expensive and by far the most verbose thing this function does.
	withOffers bool
	// desc is the module's descriptor, when the caller already has it: a
	// broadcast builds it once for every recipient, where each message used
	// to build the whole thing again for one field of it.
	desc *module.ModuleDescriptor
	// openView asks for the board with every hand face up, for replaying a
	// finished game. Whether that is allowed is decided long before here, by
	// BuildReplay; a module that cannot do it is projected per viewer as usual.
	openView bool
}

// projectStateMsg renders one viewer's state.
//
// The only place hidden information is filtered — by mod.View, below — which is
// why a replay frame is built through here rather than through a second builder
// that would have to be kept honest separately.
func (m *Manager) projectStateMsg(match models.Match, viewerID string, o stateMsgOpts) MatchStateMsg {
	msg := MatchStateMsg{
		Type:            "match_state",
		MatchID:         match.ID.Hex(),
		ModuleID:        match.ModuleID,
		Variation:       match.Variation,
		Options:         match.Options,
		Status:          match.Status,
		JoinCode:        match.JoinCode,
		InviteURL:       m.InviteURL(match.JoinCode),
		HostID:          match.HostID,
		WinnerID:        match.WinnerID,
		Winners:         match.Winners,
		SuspendedPlayer: match.SuspendedPlayer,
		Rematch:         match.Rematch,
		CanDealAgain:    match.Status == "completed" && m.savedGame(match),
		RepeatDeal:      match.DealRepeat,
		Reserved:        match.Reserved,
		// Never nil: these round-trip to JSON, and a nil slice serialises to
		// `null`, which every client then has to guard before indexing.
		LegalActions: []module.ActionOffer{},
	}
	for _, p := range match.Players {
		msg.Players = append(msg.Players, playerMsg(p))
	}
	// Asked only where it can be true. A spectator (no viewer id) gets the
	// same false every other status gets, since bringing a table back is not
	// something a passer-by does.
	msg.StoodIn = match.StoodIn
	if match.Status == string(rules.StatusAbandoned) && viewerID != "" {
		msg.CanResume = m.resumableBy(match, viewerID)
		if !msg.CanResume {
			msg.AwayPlayers = m.playersAway(match, viewerID)
		}
	}

	mod := m.registry.Get(match.ModuleID)
	if o.desc != nil {
		msg.Deck = o.desc.Deck
	} else if mod != nil {
		msg.Deck = mod.Descriptor().Deck
	}
	// Sides are a lobby fact. Once the match is dealt the board carries the
	// partnerships itself — per seat, in the ViewModel — and repeating them here
	// would be a second copy to keep honest.
	if mod != nil && match.Status == "lobby" {
		msg.Sides = module.SidesOf(mod,
			module.MatchConfig{Variation: match.Variation, Options: match.Options},
			playerRefs(match.Players))
	}
	if mod == nil || len(match.State) == 0 {
		return msg
	}
	// A shared projection answers for any viewer from one decode; without one
	// the module is asked directly, and decodes the state for each question.
	view := func() (module.ViewModel, error) { return mod.View(module.State(match.State), viewerID) }
	offersFor := func() ([]module.ActionOffer, error) { return mod.LegalActions(module.State(match.State), viewerID) }
	if o.proj != nil {
		view = func() (module.ViewModel, error) { return o.proj.View(viewerID) }
		offersFor = func() ([]module.ActionOffer, error) { return o.proj.LegalActions(viewerID) }
	}
	if o.openView {
		if vm, ok := module.OpenViewFor(mod, module.State(match.State)); ok {
			msg.View = vm
		} else if vm, err := view(); err == nil {
			msg.View = vm
		}
	} else if vm, err := view(); err == nil {
		msg.View = vm
	}
	if o.withOffers {
		if offers, err := offersFor(); err == nil && offers != nil {
			msg.LegalActions = holdOffers(match.Status, offers)
		}
	}
	if o.proj != nil {
		msg.Standings = o.proj.Standings()
	} else {
		msg.Standings = module.StandingsFor(mod, module.State(match.State))
	}
	msg.Rounds = o.rounds
	return msg
}

// newSeed is a fresh shuffle seed from the system's secure source.
//
// It used to be the creation time in nanoseconds, which is guessable to within
// a window small enough to search: the creation time is roughly known and the
// first face-up cards are a checksum. Solitaire, played against the deck alone,
// is what made that matter (docs/solitaire-plan.md §4a); no game's shuffle has
// any business being predictable.
func newSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:]) &^ (1 << 63))
}

func randInt(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}

// holdOffers greys out every live offer while the table is not running. A
// suspended or set-aside match refuses every action with MATCH_NOT_ACTIVE, so
// an enabled "start the next round" beside "pick up where you left off" was a
// button that did nothing until the table had been resumed first.
func holdOffers(status string, offers []module.ActionOffer) []module.ActionOffer {
	if status != "suspended" && status != "abandoned" {
		return offers
	}
	held := make([]module.ActionOffer, len(offers))
	for i, o := range offers {
		if o.Enabled {
			o.Enabled = false
			o.WhyNot = "MATCH_NOT_ACTIVE"
		}
		held[i] = o
	}
	return held
}
