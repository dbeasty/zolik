package match

import (
	"encoding/json"

	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Replay: a finished game, played back from what was already stored.
//
// Nothing here adds a field to a match. The runtime has kept an append-only
// ActionLog since the module protocol existed, and every module's Apply is pure
// — no clock, seeded shuffles, reshuffle seeds derived from state — so the deal
// plus the log *is* the game, and a replay is a fold rather than a recording.
//
// Which is also why this is read-side only: nothing calls it during play, and a
// bug here can lose a replay but can never lose a match.

// ReplayMsg is a played-out match, frame by frame.
//
// The sibling of MatchStateMsg, and shaped the same way: everything invariant
// across the match lives here once, everything that changes lives in a frame.
type ReplayMsg struct {
	Type      string         `json:"type"` // "match_replay"
	MatchID   string         `json:"matchId"`
	ModuleID  string         `json:"moduleId"`
	Variation string         `json:"variation,omitempty"`
	Options   map[string]int `json:"options,omitempty"`
	Players   []PlayerMsg    `json:"players"`
	// ViewerID is the seat these frames were projected for.
	ViewerID string `json:"viewerId,omitempty"`
	// Open says every hand is face up — granted only to a finished game, where
	// there is no longer anything to protect. See openable.
	Open bool `json:"open,omitempty"`
	// Total is every frame this match has: the deal, plus one per action.
	Total  int           `json:"total"`
	From   int           `json:"from"`
	Frames []ReplayFrame `json:"frames"`
	// Truncated says the fold stopped before Total: the module refused a move
	// it once accepted, because its rules have moved since this game was
	// played. Everything up to TruncatedAt is still exactly what happened.
	Truncated     bool   `json:"truncated,omitempty"`
	TruncatedAt   int    `json:"truncatedAt,omitempty"`
	TruncatedCode string `json:"truncatedCode,omitempty"`
}

// ReplayFrame is the board after one step. Frame 0 is the deal, and carries no
// action because nobody made one.
type ReplayFrame struct {
	Index    int      `json:"index"`
	Seq      int      `json:"seq,omitempty"`
	PlayerID string   `json:"playerId,omitempty"`
	Verb     string   `json:"verb,omitempty"`
	OfferID  string   `json:"offerId,omitempty"`
	Cards    []string `json:"cards,omitempty"`
	At       string   `json:"at,omitempty"`

	// Round is which round this frame sits in, 1-based, for a game that keeps
	// them; RoundEnded marks the frame that closed one. Both are derived from
	// the round log growing, so neither needs a word from any game.
	Round      int  `json:"round,omitempty"`
	RoundEnded bool `json:"roundEnded,omitempty"`

	Status    string            `json:"status"`
	Winners   []string          `json:"winners,omitempty"`
	View      module.ViewModel  `json:"view"`
	Standings []module.Standing `json:"standings,omitempty"`
	// Rounds rides only on the deal and on frames that ended a round. The log
	// is monotonic, so repeating it on every frame is quadratic bytes for
	// nothing; a client carries the last one forward.
	Rounds *module.RoundLog `json:"rounds,omitempty"`
}

// How many frames one request may fold.
//
// The cap is not a preference, it is the defense: there is no rate limiter on
// gameplay routes, and each frame costs a handful of JSON decodes of the whole
// state. Mirrors DefaultPlayerMatchLimit / MaxPlayerMatchLimit next door.
const (
	DefaultReplayFrames = 100
	MaxReplayFrames     = 300
)

// ReplayOptions bounds one fold.
type ReplayOptions struct {
	From  int
	Limit int
	// Open asks for every hand face up. A request, not a grant: BuildReplay
	// refuses it on anything but a finished match.
	Open bool
}

// BuildReplay folds a stored match's action log back into frames, projected for
// one viewer.
//
// The sibling of BuildStateMsg, and it renders every frame through the very
// same projection — so the one place that filters hidden information stays one
// place, and a replay cannot drift from a live board in what it hides.
func (m *Manager) BuildReplay(match models.Match, viewerID string, opts ReplayOptions) (ReplayMsg, error) {
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return ReplayMsg{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	// A table nobody ever dealt has no game in it to step through. Not an
	// error the caller made, which is why the handler answers 409 rather than
	// 400 — it is the same table, just never played.
	if len(match.State) == 0 || len(match.ActionLog) == 0 {
		return ReplayMsg{}, module.Error{Code: "NOTHING_TO_REPLAY"}
	}

	from := opts.From
	if from < 0 {
		from = 0
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultReplayFrames
	}
	if limit > MaxReplayFrames {
		limit = MaxReplayFrames
	}

	msg := ReplayMsg{
		Type:      "match_replay",
		MatchID:   match.ID.Hex(),
		ModuleID:  match.ModuleID,
		Variation: match.Variation,
		Options:   match.Options,
		ViewerID:  viewerID,
		Open:      openable(match.Status),
		Total:     len(match.ActionLog) + 1,
		From:      from,
		Frames:    []ReplayFrame{},
	}
	for _, p := range match.Players {
		msg.Players = append(msg.Players, PlayerMsg{ID: p.ID, Name: p.Name, IsAI: p.IsAI, Avatar: p.Avatar})
	}

	prevRounds := -1
	stopped, err := foldActions(mod, match, func(step int, entry *models.MatchAction, a module.Action, s module.State) (bool, error) {
		// Below the window, and not the step just before it: nothing to
		// compute, and the state bytes are dropped on the way out.
		if step < from-1 {
			return true, nil
		}
		rounds := module.RoundsFor(mod, s)
		count := 0
		if rounds != nil {
			count = len(rounds.Rounds)
		}
		// The step before the window is folded but not rendered, purely so the
		// first rendered frame knows whether it ended a round.
		if step < from {
			prevRounds = count
			return true, nil
		}

		f := m.replayFrame(mod, match, viewerID, msg.Open, step, entry, a, s, rounds)
		if step > 0 && prevRounds >= 0 && count > prevRounds {
			f.RoundEnded = true
		}
		if step == 0 || f.RoundEnded {
			f.Rounds = rounds
		}
		if rounds != nil {
			f.Round = count + 1
		}
		prevRounds = count

		msg.Frames = append(msg.Frames, f)
		return len(msg.Frames) < limit, nil
	})
	if err != nil {
		// A fold that dies on the very first frame has nothing to show, so it
		// is the module's error. Anything later is a real, partial replay:
		// those frames were produced by the module from states it accepted,
		// and throwing them away because step 412 no longer folds gives the
		// player nothing.
		if len(msg.Frames) == 0 {
			return ReplayMsg{}, err
		}
		msg.Truncated = true
		msg.TruncatedAt = stopped
		msg.TruncatedCode = module.CodeOf(err)
	}
	return msg, nil
}

// replayFrame renders one folded state as a frame.
//
// The envelope is a value copy of the stored match with the historical state
// swapped in, so projectStateMsg needs to know nothing about replay: it is
// handed something shaped exactly like a match, because it is one.
func (m *Manager) replayFrame(
	mod module.GameModule,
	match models.Match,
	viewerID string,
	open bool,
	step int,
	entry *models.MatchAction,
	a module.Action,
	s module.State,
	rounds *module.RoundLog,
) ReplayFrame {
	env := match // flat struct; Players and Options are shared read-only
	env.State = s
	env.ActionLog = nil
	// Suspension is a fact about the envelope now, not about this moment in
	// the game. A replay of a paused table is not itself paused.
	env.SuspendedPlayer = ""
	env.Status, env.Winners, env.WinnerID = "active", nil, ""
	// Asked per frame rather than "the last frame inherits the stored status",
	// because that is game-agnostic and it makes the frame that *ended* the
	// match identifiable even in the middle of a page.
	if done, winners, err := mod.Finished(s); err == nil && done {
		env.Status = "completed"
		env.Winners = winners
		if len(winners) > 0 {
			env.WinnerID = winners[0]
		}
	}

	msg := m.projectStateMsg(env, viewerID, stateMsgOpts{rounds: rounds, openView: open})

	f := ReplayFrame{
		Index:     step,
		Status:    msg.Status,
		Winners:   msg.Winners,
		View:      msg.View,
		Standings: msg.Standings,
	}
	if entry != nil {
		f.Seq = entry.Seq
		f.PlayerID = entry.PlayerID
		f.Verb = a.Verb
		f.OfferID = a.OfferID
		f.Cards = a.Cards
		if !entry.At.IsZero() {
			f.At = entry.At.UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}
	return f
}

// openable reports whether a match may be replayed with every hand face up.
//
// Only a finished game. A paused, set-aside or still-running table can be
// resumed by somebody, so showing its hidden cards — to anyone, including the
// player asking — would turn replay into a way to see what you are not
// entitled to. Decided here rather than in the handler so that no future
// caller of BuildReplay can route around it.
func openable(status string) bool { return status == "completed" }

// foldActions replays a match's log through its module, handing each step's
// state to visit.
//
// It stops at the first refusal, reporting which frame could not be produced,
// and stops early once visit says it has what it needs — so rendering a page
// costs O(from+limit) applies rather than the whole log every time.
//
// State bytes are never retained: visit renders a frame and the snapshot is
// dropped. Holding 800 canasta states would be megabytes for no gain.
func foldActions(
	mod module.GameModule,
	match models.Match,
	visit func(step int, entry *models.MatchAction, a module.Action, s module.State) (more bool, err error),
) (stoppedAt int, err error) {
	// The deal is reproducible because its inputs are frozen: Join and Seat
	// both refuse once the match leaves the lobby, and Seed is written once at
	// Create. So these are the same arguments Start passed, necessarily.
	s, err := mod.NewMatch(
		module.MatchConfig{Variation: match.Variation, Options: match.Options},
		playerRefs(match.Players),
		match.Seed,
	)
	if err != nil {
		return 0, err
	}
	more, err := visit(0, nil, module.Action{}, s)
	if err != nil || !more {
		return 0, err
	}

	for i := range match.ActionLog {
		entry := match.ActionLog[i]
		var a module.Action
		if err := json.Unmarshal(entry.Action, &a); err != nil {
			return i + 1, err
		}
		next, _, err := mod.Apply(s, entry.PlayerID, a)
		if err != nil {
			return i + 1, err
		}
		s = next
		more, err := visit(i+1, &entry, a, s)
		if err != nil || !more {
			return i + 1, err
		}
	}
	return len(match.ActionLog), nil
}
