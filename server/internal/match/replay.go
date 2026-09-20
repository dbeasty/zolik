package match

import (
	"context"
	"encoding/json"

	"go.mongodb.org/mongo-driver/v2/bson"

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
	// Chapters are the rounds this match was played in — the landmarks a
	// player actually thinks in, and the only way a six-hundred move replay
	// is navigable by anything other than dragging a slider and hoping.
	//
	// Read straight off the stored checkpoints, so they cost no fold at all
	// and arrive complete with the first page, however deep into the match
	// that page is. Absent for a game that keeps no rounds, where a single
	// chapter spanning everything would be a label pretending to be a map.
	Chapters []ReplayChapter `json:"chapters,omitempty"`
	// Tracks are the threads through the match a reader might follow — one
	// seat's moves, or the round boundaries — as the frames on each.
	//
	// Every one is a filter on who moved, which the action log already
	// records, so these cost a walk of it and nothing else: no module asked
	// anything, no state decoded, no fold. Like Chapters they ride on every
	// page, because a reader who opens deep in a match still needs the map.
	Tracks []ReplayTrack `json:"tracks,omitempty"`
	// Truncated says the fold stopped before Total: the module refused a move
	// it once accepted, because its rules have moved since this game was
	// played. Everything up to TruncatedAt is still exactly what happened.
	Truncated     bool   `json:"truncated,omitempty"`
	TruncatedAt   int    `json:"truncatedAt,omitempty"`
	TruncatedCode string `json:"truncatedCode,omitempty"`
}

// ReplayChapter is one round of the match, as somewhere to jump to.
//
// The word for it — deal, hand, leg — is the module's own, and rides on the
// round log the frames already carry rather than being repeated here.
type ReplayChapter struct {
	Round int `json:"round"`
	// From is the first frame of this round; To the frame that closed it,
	// absent while the round is the one the match stopped in.
	From int `json:"from"`
	To   int `json:"to,omitempty"`
}

// ReplayTrack is one thread through a match, as the frames that belong to it.
//
// "Your moves" and "the bot's moves" are the two a reader actually asks for,
// and a long replay is unreadable without them: stepping one frame at a time
// through somebody else's turns to reach your own is not navigation.
type ReplayTrack struct {
	// ID is "seat:<playerId>" or "rounds".
	ID       string `json:"id"`
	PlayerID string `json:"playerId,omitempty"`
	// Frames are the frame indices on this track, ascending.
	Frames []int `json:"frames"`
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

// BoardSource is a repository that can produce a match's board as it stood,
// out of the store's own history, instead of by replaying the log to it.
//
// Optional, in the same way and for the same reason as module.Ranked: the
// engines genuinely differ, and one that cannot answer should decline rather
// than pretend. KDB keeps every version of every document it has written and
// can answer exactly; Mongo replaces the document in place and cannot, so it
// does not implement this and every fold there begins at a checkpoint or at
// the deal, as it always has.
//
// Where it does answer, a frame stops being a reconstruction and becomes the
// board as it was: independent of Apply still being pure, of the module's
// rules not having moved since, and of anything having written state outside
// the log.
type BoardSource interface {
	BoardAfter(ctx context.Context, id bson.ObjectID, actions int) (models.Match, error)
	// RetainsHistory reports whether the versions BoardAfter walks are still
	// being kept, and says why not when they are not.
	//
	// On the interface rather than beside it because implementing BoardAfter
	// is not the same as being able to answer it: KDB migrated to
	// history=none reclaims commits on a retention window, so the same code
	// path returns boards today and nothing at all next week. An engine that
	// offers the one has to be able to answer the other.
	RetainsHistory(ctx context.Context) error
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
func (m *Manager) BuildReplay(ctx context.Context, match models.Match, viewerID string, opts ReplayOptions) (ReplayMsg, error) {
	mod := m.registry.Get(match.ModuleID)
	if mod == nil {
		return ReplayMsg{}, module.Error{Code: "UNKNOWN_MODULE", Message: match.ModuleID}
	}
	// A table nobody ever dealt has no game in it to step through. Not an
	// error the caller made, which is why the handler answers 409 rather than
	// 400 — it is the same table, just never played.
	//
	// Being dealt is the whole test, rather than having moves in the log: a
	// match that was started and abandoned before anybody played still has a
	// deal to look at, and drawing the line here is what lets a stored-table
	// row advertise canReplay from StartedAt alone — the list projection
	// strips the action log, so it could not count moves if it wanted to.
	if len(match.State) == 0 {
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
		// Asking and being allowed, both: the caller says whether it wants an
		// open board, and openable says whether this match may give one.
		Open:     opts.Open && openable(match.Status),
		Total:    len(match.ActionLog) + 1,
		From:     from,
		Frames:   []ReplayFrame{},
		Chapters: chaptersOf(match),
		Tracks:   tracksOf(match),
	}
	for _, p := range match.Players {
		msg.Players = append(msg.Players, PlayerMsg{ID: p.ID, Name: p.Name, IsAI: p.IsAI, Avatar: p.Avatar})
	}

	prevRounds := -1
	// One frame earlier than the window, so the first rendered frame knows
	// whether it closed a round. Below zero there is nothing before the deal.
	startAt := from - 1
	if startAt < 0 {
		startAt = 0
	}
	stopped, err := foldActions(ctx, m.boardSource(), mod, match, startAt, func(step int, entry *models.MatchAction, a module.Action, s module.State) (bool, error) {
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
// It starts at the newest checkpoint at or before startAt rather than at the
// deal, which is what keeps jumping to the end of a long match from costing
// the whole match. A checkpoint is the stored board, so starting from one also
// means the frames after it do not depend on the actions before it still
// folding — a rules change that breaks move 12 no longer costs you deal 5.
//
// It stops at the first refusal, reporting which frame could not be produced,
// and stops early once visit says it has what it needs — so rendering a page
// costs O(limit) applies once a checkpoint is near it.
//
// State bytes are never retained: visit renders a frame and the snapshot is
// dropped. Holding 800 canasta states would be megabytes for no gain.
func foldActions(
	ctx context.Context,
	boards BoardSource,
	mod module.GameModule,
	match models.Match,
	startAt int,
	visit func(step int, entry *models.MatchAction, a module.Action, s module.State) (more bool, err error),
) (stoppedAt int, err error) {
	s, step, err := foldOrigin(ctx, boards, mod, match, startAt)
	if err != nil {
		return step, err
	}
	more, err := visit(step, entryAt(match, step), actionAt(match, step), s)
	if err != nil || !more {
		return step, err
	}

	for i := step; i < len(match.ActionLog); i++ {
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

// foldOrigin is where a fold aiming at startAt should begin: a stored
// checkpoint if one lies at or before it, otherwise the deal.
//
// The deal is reproducible because its inputs are frozen: Join and Seat both
// refuse once the match leaves the lobby, and Seed is written once at Create.
// So these are the same arguments Start passed, necessarily.
func foldOrigin(ctx context.Context, boards BoardSource, mod module.GameModule, match models.Match, startAt int) (module.State, int, error) {
	// The store's own history first, where there is one: it lands exactly on
	// the frame asked for, so the fold applies nothing at all, and the board
	// it hands back is the one that was actually there rather than one
	// computed from the log now.
	//
	// A failure here is never fatal. Every reason it can fail — no history
	// kept, a row written before any of this existed, a compacted log —
	// leaves the checkpoints and the deal exactly as able to answer as they
	// were, so it falls through rather than refusing.
	if boards != nil && startAt > 0 && !match.ID.IsZero() {
		if at, err := boards.BoardAfter(ctx, match.ID, startAt); err == nil && len(at.State) > 0 {
			return at.State, len(at.ActionLog), nil
		}
	}
	for i := len(match.Checkpoints) - 1; i >= 0; i-- {
		if c := match.Checkpoints[i]; c.Seq <= startAt && len(c.State) > 0 {
			return append(module.State(nil), c.State...), c.Seq, nil
		}
	}
	s, err := mod.NewMatch(
		module.MatchConfig{Variation: match.Variation, Options: match.Options},
		playerRefs(match.Players),
		match.Seed,
	)
	return s, 0, err
}

// entryAt and actionAt describe the move that produced frame step, for a fold
// that began part-way through the log and so did not apply it.
func entryAt(match models.Match, step int) *models.MatchAction {
	if step <= 0 || step > len(match.ActionLog) {
		return nil
	}
	return &match.ActionLog[step-1]
}

func actionAt(match models.Match, step int) module.Action {
	e := entryAt(match, step)
	if e == nil {
		return module.Action{}
	}
	var a module.Action
	_ = json.Unmarshal(e.Action, &a)
	return a
}

// chaptersOf turns the stored checkpoints into somewhere to jump to.
//
// No fold, no module, no state decoded: a checkpoint already knows which round
// it closed and at which move, which is all a chapter is. A match with no
// checkpoints gets none rather than one chapter covering everything — that
// would be a label pretending to be a map.
func chaptersOf(match models.Match) []ReplayChapter {
	if len(match.Checkpoints) == 0 {
		return nil
	}
	out := make([]ReplayChapter, 0, len(match.Checkpoints)+1)
	from := 0
	for _, c := range match.Checkpoints {
		out = append(out, ReplayChapter{Round: c.Round, From: from, To: c.Seq})
		from = c.Seq + 1
	}
	// The round the match stopped in, if it did not stop exactly on a
	// boundary. It has a start and no end, which is the truth about it.
	if from <= len(match.ActionLog) {
		out = append(out, ReplayChapter{Round: lastCheckpointRound(match) + 1, From: from})
	}
	return out
}

// boardSource is this runtime's repository, when it is one that keeps history.
//
// Asked per call rather than stored, because a Manager is built with a
// Repository and nothing decides here which engine that is — the type
// assertion is the whole of the engine-awareness in this package.
func (m *Manager) boardSource() BoardSource {
	if b, ok := m.repo.(BoardSource); ok {
		return b
	}
	return nil
}

// ReplayAvailable says whether this deployment can offer replay at all, and
// why not when it cannot.
//
// Three gates, and every one of them is about the deployment rather than
// about the match:
//
//   - the operator turned it on (FEATURE_FLAG_MATCH_REPLAY),
//   - the store keeps every version of a document, which is KDB and not Mongo,
//   - and that store is still *keeping* them, rather than running history=none
//     and reclaiming on a window.
//
// It is one function because the answer has to be the same in both places it
// is needed — the endpoint, and the canReplay a stored-table row advertises.
// A list that offers a button the endpoint then refuses is the failure mode
// this exists to prevent.
//
// Asked per call rather than resolved at boot: the second and third gates are
// engine state, and SetHistoryMode can move the third one under a running
// process. Both checks are a type assertion and a read lock, which is nothing
// beside the fold they guard.
func (m *Manager) ReplayAvailable(ctx context.Context) error {
	if !m.replayEnabled {
		return module.Error{
			Code:    "REPLAY_UNAVAILABLE",
			Message: "replay is off in this deployment (FEATURE_FLAG_MATCH_REPLAY)",
		}
	}
	src := m.boardSource()
	if src == nil {
		return module.Error{
			Code: "REPLAY_UNAVAILABLE",
			Message: "replay needs a store that keeps every version of a document, " +
				"which is FEATURE_FLAG_DB_ENGINE=kdb; this one replaces them in place",
		}
	}
	if err := src.RetainsHistory(ctx); err != nil {
		return module.Error{Code: "REPLAY_UNAVAILABLE", Message: err.Error()}
	}
	return nil
}

// tracksOf turns the action log into the threads a reader can follow.
//
// One track per seat that ever moved, plus the round boundaries where a game
// keeps them. Derived from the log alone — ActionLog[i].PlayerID is who moved,
// and frame i+1 is the board after they did — so this asks no module anything
// and decodes no state, exactly like chaptersOf.
//
// Seats come in the order they first moved rather than in seat order: a reader
// scanning the chips wants their own, and whoever opened the game moved first.
func tracksOf(match models.Match) []ReplayTrack {
	if len(match.ActionLog) == 0 {
		return nil
	}
	bySeat := map[string]*ReplayTrack{}
	order := make([]string, 0, len(match.Players))
	for i, a := range match.ActionLog {
		t, ok := bySeat[a.PlayerID]
		if !ok {
			t = &ReplayTrack{ID: "seat:" + a.PlayerID, PlayerID: a.PlayerID}
			bySeat[a.PlayerID] = t
			order = append(order, a.PlayerID)
		}
		t.Frames = append(t.Frames, i+1)
	}

	out := make([]ReplayTrack, 0, len(order)+1)
	for _, id := range order {
		out = append(out, *bySeat[id])
	}
	// The rounds, where the game keeps any: the frames the chapters close on,
	// offered as something to step through rather than to jump between.
	if len(match.Checkpoints) > 0 {
		rounds := ReplayTrack{ID: "rounds"}
		for _, c := range match.Checkpoints {
			rounds.Frames = append(rounds.Frames, c.Seq)
		}
		out = append(out, rounds)
	}
	return out
}
