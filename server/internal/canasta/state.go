// Package canasta implements Canasta — Classic, Modern American and Samba — as
// a game module.
//
// It is the third module behind the runtime, and the first one that is a
// *rummy* and still not the rummy engine. `architecture.md` §1 predicted this
// game would be a configuration problem — a profile of `internal/rules`. It is
// not, and `docs/canasta-plan.md` §1 records why: partnership-owned melds,
// capture of the whole discard pile, a pile that can be frozen, cards that
// leave your hand on sight, wild limits inside a meld and multi-deal scoring
// are rules with no knob behind them. Expressing them as `RulesConfig` values
// would put a second game's clauses inside the engine Žolíky depends on, which
// is the coupling the module seam exists to prevent.
//
// Rules implemented (Hoyle / pagat.com classic canasta):
//   - 108 cards: two decks plus four jokers. Two or four players; at four,
//     seats 0+2 play seats 1+3.
//   - Melds are sets of equal rank, 3–7 cards, at most three wilds and at
//     least two naturals, owned by the partnership. Seven cards is a canasta:
//     natural (no wilds) 500, mixed 300.
//   - Jokers and 2s are wild. Red threes lay themselves down for 100 each
//     (800 for all four) and count against a partnership with no canasta.
//     Black threes block the pile when discarded, score 100 melded and cost
//     100 stranded, and may only be melded on the way out — and in Modern
//     American, not at all.
//   - A turn is draw (or take the whole discard pile) → meld → discard.
//   - The pile may be taken only by using its top card at once, and is frozen
//     against everyone by a buried wild and against a partnership that has
//     not yet made its initial meld.
//   - The initial meld minimum rises with the partnership's own score:
//     15 / 50 / 90 / 120.
//   - Going out needs the partnership's canasta quota. 100, or 200 concealed.
//   - Deals repeat until a partnership passes the target score.
//
// Samba differs in the deck, the draw, the wild limits, the meld kinds, the pile
// and every bonus (docs/samba-plan.md §2), and none of that is written twice:
// ruleset.go holds one struct per variation and the engine reads it. What a
// variation cannot change is the shape of a turn or who owns a meld — those are
// this package, not a knob in it.
package canasta

import (
	"encoding/json"
	"fmt"

	"zolik/server/internal/module"
)

// Verbs this module accepts.
const (
	VerbDraw     = "draw"
	VerbTakePile = "take_pile"
	// VerbTakeTop is Samba's: the top card onto a sequence, instead of drawing.
	// A separate verb rather than a flavour of take_pile because it does
	// something else — one card comes off and the pile stays standing.
	VerbTakeTop      = "take_top"
	VerbLayMeld      = "lay_meld"
	VerbLayOff       = "lay_off"
	VerbDiscard      = "discard"
	VerbUndoTakePile = "undo_take_pile"
	// VerbUndoLayOff takes back a lay-off made this turn — see LaidOff. The
	// same word Žolíky's own undo uses (rules.ActionUndoLayOff), because it is
	// the same move and nobody should have to learn two names for it.
	VerbUndoLayOff = "undo_lay_off"
)

// Turn phases. Two, not three: melding and discarding are the same phase,
// because in Canasta a discard is simply the move that ends it.
const (
	phaseDraw = "draw"
	phaseMeld = "meld"
)

// Error codes. Stable keys a client renders from its locale bundle, never
// sentences — the same contract the other two modules keep.
const (
	ErrNotYourTurn       = "NOT_YOUR_TURN"
	ErrGameNotActive     = "GAME_NOT_ACTIVE"
	ErrWrongPhase        = "WRONG_PHASE"
	ErrCardNotInHand     = "CARD_NOT_IN_HAND"
	ErrUnknownAction     = "UNKNOWN_ACTION"
	ErrNothingToDraw     = "NOTHING_TO_DRAW"
	ErrPileEmpty         = "PILE_EMPTY"
	ErrPileBlocked       = "PILE_BLOCKED"
	ErrPileFrozen        = "PILE_FROZEN"
	ErrTopCardUnusable   = "TOP_CARD_UNUSABLE"
	ErrMeldTooSmall      = "MELD_TOO_SMALL"
	ErrMeldTooLarge      = "MELD_TOO_LARGE"
	ErrMeldMixedRanks    = "MELD_MIXED_RANKS"
	ErrTooManyWilds      = "TOO_MANY_WILDS"
	ErrNotEnoughNaturals = "NOT_ENOUGH_NATURALS"
	ErrRankAlreadyMelded = "RANK_ALREADY_MELDED"
	ErrCannotMeldThree   = "CANNOT_MELD_THREE"
	// ErrBlackThreeGoOutOnly is narrower than ErrCannotMeldThree and is the
	// reason a variation that *allows* the black-three meld gives for refusing
	// one. "Threes are never melded" would be a lie at a Classic table that is
	// offering the move three lines further down the same screen.
	ErrBlackThreeGoOutOnly = "BLACK_THREE_GO_OUT_ONLY"
	ErrNoSuchMeld          = "NO_SUCH_MELD"
	ErrNotYourMeld         = "NOT_YOUR_MELD"
	ErrMeldClosed          = "MELD_CLOSED"
	ErrWrongRank           = "WRONG_RANK"
	ErrInitialMeldNotMet   = "INITIAL_MELD_NOT_MET"
	ErrMustMeldFirst       = "MUST_MELD_FIRST"
	ErrCannotDiscardThree  = "CANNOT_DISCARD_RED_THREE"
	ErrCannotGoOutYet      = "CANNOT_GO_OUT_YET"
	ErrMustKeepACard       = "MUST_KEEP_A_CARD"
	// ErrNothingToUndo is shared with Žolíky's own undo (internal/rules):
	// same fact, same word, no reason for a client to carry two keys for it.
	ErrNothingToUndo = "NOTHING_TO_UNDO"

	// Sequences, in the variations that have them. A closed samba reports the
	// existing MELD_CLOSED and a permanently frozen pile the existing
	// PILE_FROZEN: both say exactly what happened, and a second code per
	// variation would be a second sentence to translate for no new meaning.
	ErrSequenceNoWilds      = "SEQUENCE_NO_WILDS"
	ErrSequenceNeedsOneSuit = "SEQUENCE_NEEDS_ONE_SUIT"
	ErrRunNotConsecutive    = "RUN_NOT_CONSECUTIVE"
)

// Meld is one partnership's set of a single rank.
//
// Owned by the team rather than the player: either partner may extend it, and
// that single fact is most of why Canasta could not be a `RulesConfig` profile.
type Meld struct {
	ID     string `json:"id"`
	TeamID int    `json:"teamId"`
	// Kind is "set" — n cards of one rank — or "run", a sequence in one suit.
	// Empty means "set", so a meld written before sequences existed reads back
	// as what it was.
	Kind string `json:"kind,omitempty"`
	// Rank is a set's rank; Suit is a run's suit. Each is empty for the other.
	Rank  string   `json:"rank"`
	Suit  string   `json:"suit,omitempty"`
	Cards []string `json:"cards"`
}

// The two kinds of meld. A zero Kind is a set, so nothing has to be migrated.
const (
	meldSet = "set"
	meldRun = "run"
)

// kind is Kind with the empty-means-set default applied.
func (m Meld) kind() string {
	if m.Kind == "" {
		return meldSet
	}
	return m.Kind
}

func meldID(teamID int, rank string) string {
	return fmt.Sprintf("t%d-%s", teamID, rank)
}

// PileTaken snapshots one capture of the discard pile so it can be undone
// before anything else touches the table.
//
// Taking the pile is not a plain draw: it commits to a meld and, before a
// partnership has opened, to reaching the whole turn's minimum, in the same
// motion that draws the cards. reachableValue's own reachability check is
// only a lower bound (meld.go), so it can refuse a take that would in fact
// have worked but cannot always tell a genuinely doomed one apart — a
// partnership can take the pile believing the minimum is still reachable and
// then find the concrete hand it holds will not cooperate. Without a way back
// that partnership is stuck for the rest of the deal, so there is one, scoped
// as narrowly as the problem: it undoes exactly this capture, and only for as
// long as none of its cards have gone anywhere else. LaidOff below is the only
// other move that takes a card back off the table, and it is there for a
// different reason — a mistake rather than a dead end.
type PileTaken struct {
	// Pile is the discard pile exactly as it stood before the capture, top
	// card last — put back verbatim rather than reconstructed from parts.
	Pile []string `json:"pile"`
	// PriorHand is the player's own hand exactly as it stood before the
	// capture. Restored verbatim rather than by reversing the individual
	// removals and additions, which would put every card back but not
	// necessarily in the order the player last arranged them in.
	PriorHand []string `json:"priorHand"`
	// MeldID is which meld received the top card. MeldWasNew says whether to
	// delete it outright on undo or truncate it back to PriorCards.
	MeldID     string   `json:"meldId"`
	MeldWasNew bool     `json:"meldWasNew"`
	PriorCards []string `json:"priorCards,omitempty"`
	// RedThreesGained are the buried red threes this capture handed straight
	// to the team's row, named individually so undo removes exactly those —
	// never "however many are there now", which a partner's move in between
	// could have changed.
	RedThreesGained   []string `json:"redThreesGained,omitempty"`
	PriorHasMelded    bool     `json:"priorHasMelded"`
	PriorLaidThisTurn int      `json:"priorLaidThisTurn,omitempty"`
	PriorFrozen       bool     `json:"priorFrozen,omitempty"`
}

// LaidOff snapshots one lay-off so it can be taken back.
//
// The reason is smaller than PileTaken's and far more ordinary. A lay-off is a
// single tap onto one of several melds sitting side by side, and the card is
// gone the moment it lands: putting the wrong card on the wrong meld is the
// easiest mistake this game lets a player make, and the one it gave them no way
// to correct.
//
// Not the dead end PileTaken exists for: checkInitialMeld already refuses a
// lay-off that would put the opening minimum out of reach, and an undo only
// ever hands cards back, so nothing here is about rescuing a stuck turn. It is
// about the mistakes the rules happily allow and never let you take back — a
// wild spent on a meld you were saving it for, or a seventh card that closes a
// canasta you wanted to keep open, which in Classic shuts that rank for the
// rest of the deal.
//
// A stack, rather than PileTaken's single snapshot, because a turn holds one
// capture and any number of lay-offs: a player who puts three cards down before
// seeing the mistake would otherwise get one of them back and no more. Unwound
// last-first, and the whole stack is dropped the instant anything else reaches
// the table, so an undo only ever reverses a move nothing was built on top of.
type LaidOff struct {
	// MeldID is the meld that received the cards; PriorCards is exactly what
	// it held beforehand, restored verbatim rather than by removing what was
	// added — a sequence is stored sorted, so the cards do not come off the
	// end they went on.
	MeldID     string   `json:"meldId"`
	PriorCards []string `json:"priorCards"`
	// Cards are the cards that left the hand, named so the event that reverses
	// this can say which ones came back.
	Cards []string `json:"cards"`
	// PriorHand is the hand exactly as it stood before, restored whole rather
	// than by appending the cards again — for the reason PileTaken keeps one:
	// somebody arranged that hand.
	PriorHand         []string `json:"priorHand"`
	PriorLaidThisTurn int      `json:"priorLaidThisTurn,omitempty"`
	PriorHasMelded    bool     `json:"priorHasMelded,omitempty"`
}

func (m Meld) naturals() int {
	n := 0
	for _, c := range m.Cards {
		if !isWild(c) {
			n++
		}
	}
	return n
}

func (m Meld) wilds() int { return len(m.Cards) - m.naturals() }

// isCanasta reports whether this meld has reached seven cards, which is both
// the unit of progress and the licence to go out.
func (m Meld) isCanasta() bool { return len(m.Cards) >= canastaSize }

// isNatural reports a canasta with no wilds in it — worth 500 rather than 300.
func (m Meld) isNatural() bool { return m.wilds() == 0 }

// closed reports a meld that can take no more cards.
//
// In Canasta a canasta is complete at seven and an eighth card is a rule nobody
// plays. Samba disagrees for groups — a canasta there keeps taking cards, bounded
// by the wild limits instead — so the answer belongs to the variation. A sequence
// always closes at seven, because a seven-card sequence is a samba and that is
// the whole of what a samba is.
func (m Meld) closed(r ruleset) bool {
	if len(m.Cards) < canastaSize {
		return false
	}
	return m.Kind == meldRun || r.GroupCanastaCloses
}

// room is how many more cards this meld will take. Unbounded melds report a
// number large enough to mean "as many as you have", which is true: a hand is
// the real limit.
func (m Meld) room(r ruleset) int {
	if m.closed(r) {
		return 0
	}
	if m.Kind == meldRun || r.GroupCanastaCloses {
		return canastaSize - len(m.Cards)
	}
	return canastaSize
}

// Team is a partnership: the scoring unit, and the owner of melds.
type Team struct {
	ID      int      `json:"id"`
	Players []string `json:"players"`
	// Score is carried across deals and is what the initial meld minimum and
	// the target are measured against.
	Score int    `json:"score"`
	Melds []Meld `json:"melds,omitempty"`
	// RedThrees are this deal's, laid face up the moment they appear.
	RedThrees []string `json:"redThrees,omitempty"`
	// HasMelded is whether the initial meld minimum has been satisfied this
	// deal. It gates lay-offs, unfreezes the pile for this team, and is reset
	// every deal.
	HasMelded bool `json:"hasMelded"`
	// MeldSeq numbers the sequences this side has laid, so their ids stay
	// stable while the runs themselves grow at both ends.
	MeldSeq int `json:"meldSeq,omitempty"`
}

// groupsOfRank is every group this side has of one rank, open or closed.
//
// Plural because Samba allows more than one and keeps them separate. Canasta
// allows one, which the ruleset says with GroupsPerRank rather than by this
// function pretending there can only ever be a single answer.
func (t *Team) groupsOfRank(rank string) []*Meld {
	var out []*Meld
	for i := range t.Melds {
		if t.Melds[i].kind() == meldSet && t.Melds[i].Rank == rank {
			out = append(out, &t.Melds[i])
		}
	}
	return out
}

// openGroup is the group of this rank that can still take cards, or nil.
func (t *Team) openGroup(r ruleset, rank string) *Meld {
	for _, m := range t.groupsOfRank(rank) {
		if !m.closed(r) {
			return m
		}
	}
	return nil
}

// rankIsFull reports that this side may not start another group of this rank.
//
// One question, asked by the three places that used to ask "is there a meld of
// this rank": laying a new meld, enumerating candidates, and capturing the pile.
// Canasta's cap of one is what made "a meld of this rank exists" and "you may not
// start another" the same sentence; Samba's absence of a cap is what separates
// them.
func (t *Team) rankIsFull(r ruleset, rank string) bool {
	if r.GroupsPerRank <= 0 {
		return false
	}
	return len(t.groupsOfRank(rank)) >= r.GroupsPerRank
}

// newMeldID names a meld about to go on the table.
//
// A side's first group of a rank keeps the id it has always had, so nothing that
// already refers to `t0-K` has to learn anything. The cases that could not arise
// before get suffixes of their own: a second group of a rank is `t0-K-2`, and a
// sequence is `t0-seq1` from a counter, because a run grows at both ends and an
// id derived from its low card would not survive the growth.
func (t *Team) newMeldID(kind, rank string) string {
	if kind == meldRun {
		t.MeldSeq++
		return fmt.Sprintf("t%d-seq%d", t.ID, t.MeldSeq)
	}
	base := meldID(t.ID, rank)
	if t.meldByID(base) == nil {
		return base
	}
	for n := 2; ; n++ {
		id := fmt.Sprintf("%s-%d", base, n)
		if t.meldByID(id) == nil {
			return id
		}
	}
}

func (t *Team) meldByID(id string) *Meld {
	for i := range t.Melds {
		if t.Melds[i].ID == id {
			return &t.Melds[i]
		}
	}
	return nil
}

func (t *Team) canastas() int {
	n := 0
	for _, m := range t.Melds {
		if m.isCanasta() {
			n++
		}
	}
	return n
}

// GameState is the whole match — every deal of it. Opaque to the runtime.
type GameState struct {
	Status    string   `json:"status"` // "active" | "completed"
	Variation string   `json:"variation,omitempty"`
	Players   []string `json:"players"`
	TurnOrder []string `json:"turnOrder"`
	Current   string   `json:"current"`
	Phase     string   `json:"phase"`

	Teams  []Team         `json:"teams"`
	TeamOf map[string]int `json:"teamOf"`

	DrawPile    []string            `json:"drawPile"`
	DiscardPile []string            `json:"discardPile"` // top = last
	Hands       map[string][]string `json:"hands"`

	// Frozen is the pile-wide freeze: a wild buried in the pile, or a red
	// three turned up to start the deal. It outlives turns and is cleared
	// only when the pile is taken.
	Frozen bool `json:"frozen,omitempty"`

	// LaidThisTurn is the card value melded so far this turn, which is what
	// the initial meld minimum is measured against — the minimum is a
	// property of a *turn*, not of a single meld.
	LaidThisTurn int `json:"laidThisTurn,omitempty"`
	// MeldsAtTurnStart records whether the partnership had anything on the
	// table when this turn began. It is what makes a concealed go-out
	// detectable after the fact.
	MeldsAtTurnStart bool `json:"meldsAtTurnStart,omitempty"`
	TookPileThisTurn bool `json:"tookPileThisTurn,omitempty"`
	// PileTaken is this turn's pile capture, if it can still be undone. Set by
	// applyTakePile and cleared the instant any other action reaches the
	// table, so an undo can only ever unwind exactly what it captured.
	PileTaken *PileTaken `json:"pileTaken,omitempty"`
	// LaidOff is this turn's lay-offs that can still be taken back, oldest
	// first — see LaidOff. Appended to by applyLayOff, and emptied by anything
	// else that reaches the table.
	LaidOff []LaidOff `json:"laidOff,omitempty"`

	// Rules resolved at deal time, so a match cannot change shape underneath
	// a deal in progress.
	HandSize        int `json:"handSize"`
	TargetScore     int `json:"targetScore"`
	CanastasToGoOut int `json:"canastasToGoOut"`
	// Rules is the whole resolved ruleset, of which the three scalars above are
	// the part that shipped first. Nil for a match dealt before this field
	// existed — see rules(), which reconstructs one rather than making every
	// reader check.
	Rules *ruleset `json:"rules,omitempty"`

	DealNumber int `json:"dealNumber"`
	// Dealer is the seat index that dealt, rotating each deal so the
	// first-player advantage moves around.
	Dealer int `json:"dealer"`

	// LastDeal is how the previous deal scored, for a scoreboard.
	LastDeal *DealResult `json:"lastDeal,omitempty"`
	// Deals is every deal that has been scored, oldest first.
	//
	// LastDeal is kept beside it rather than replaced by it: the board shows
	// one deal's settlement and the history shows all of them, and a reader of
	// either should not have to know about the other. Empty for a match that
	// was already in flight when this field arrived, which is why the round
	// numbers here are the deal's own and not a renumbering from one — a gap is
	// better read as a gap than papered over.
	Deals []DealResult `json:"deals,omitempty"`
	// Break is the pause between deals, and Pause whether this table takes one.
	//
	// Pause is resolved once, at NewMatch, and never re-read from the lobby's
	// options afterwards — which is what lets a match already in flight when
	// this shipped play out under the rules it was dealt under, with no
	// migration at all.
	Pause bool                `json:"pause,omitempty"`
	Break module.Intermission `json:"break,omitempty"`

	// OpenDiscard is whether this table publishes the whole discard pile
	// rather than its top card (view.go's shownPile). Resolved once at
	// NewMatch like Pause, and false — the folded pile this game has always
	// shown — for a match that was dealt before the option existed.
	OpenDiscard bool `json:"openDiscard,omitempty"`

	WinnerTeam int    `json:"winnerTeam"`
	WinnerID   string `json:"winnerId,omitempty"`
	Seed       int64  `json:"seed"`
}

// DealResult is one deal's arithmetic, kept so a client can show why the
// score moved without recomputing anything.
type DealResult struct {
	DealNumber int          `json:"dealNumber"`
	WentOut    string       `json:"wentOut,omitempty"`
	Concealed  bool         `json:"concealed,omitempty"`
	Exhausted  bool         `json:"exhausted,omitempty"`
	Teams      []TeamResult `json:"teams"`
}

// TeamResult breaks a deal's score into the parts a player argues about.
type TeamResult struct {
	TeamID    int `json:"teamId"`
	MeldCards int `json:"meldCards"`
	Canastas  int `json:"canastas"`
	RedThrees int `json:"redThrees"`
	GoingOut  int `json:"goingOut"`
	InHand    int `json:"inHand"`
	Total     int `json:"total"`
	Running   int `json:"running"`
}

// rules is the ruleset this match is being played under.
//
// A match dealt before the ruleset existed has only the three scalars, so one is
// reconstructed from its variation and those values are laid back over the top.
// That keeps the migration in one function instead of a nil check at every call
// site — and a match in flight when this shipped plays on under the rules it was
// dealt under, which is the same guarantee `Pause` makes.
func (s *GameState) rules() ruleset {
	if s.Rules != nil {
		return *s.Rules
	}
	r := resolveVariation(s.Variation)
	r.HandSize = s.HandSize
	r.TargetScore = s.TargetScore
	r.CanastasToGoOut = s.CanastasToGoOut
	return r
}

func (s *GameState) team(playerID string) *Team {
	id, ok := s.TeamOf[playerID]
	if !ok {
		return nil
	}
	for i := range s.Teams {
		if s.Teams[i].ID == id {
			return &s.Teams[i]
		}
	}
	return nil
}

func (s *GameState) top() string {
	if len(s.DiscardPile) == 0 {
		return ""
	}
	return s.DiscardPile[len(s.DiscardPile)-1]
}

func (s *GameState) nextPlayer(from string) string {
	for i, p := range s.TurnOrder {
		if p == from {
			return s.TurnOrder[(i+1)%len(s.TurnOrder)]
		}
	}
	if len(s.TurnOrder) > 0 {
		return s.TurnOrder[0]
	}
	return ""
}

// allMelds is every meld on the table, both partnerships'. Used by the view
// and by lay-off validation, which has to be able to say "that is not yours".
func (s *GameState) allMelds() []Meld {
	var out []Meld
	for i := range s.Teams {
		out = append(out, s.Teams[i].Melds...)
	}
	return out
}

func (s *GameState) findMeld(id string) (*Team, *Meld) {
	for i := range s.Teams {
		if m := s.Teams[i].meldByID(id); m != nil {
			return &s.Teams[i], m
		}
	}
	return nil, nil
}

func decode(raw module.State) (*GameState, error) {
	var s GameState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("canasta: decode state: %w", err)
	}
	return &s, nil
}

func encode(s *GameState) (module.State, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("canasta: encode state: %w", err)
	}
	return raw, nil
}
