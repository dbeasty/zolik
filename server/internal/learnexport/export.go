// Package learnexport turns stored matches into training records: every
// decision a person made, as the numbers a learning bot is trained on.
//
// The human training data is the match history the server already keeps — the
// move log behind /users/me/history and /matches/{id}/replay. Nothing is
// recorded for training's sake. A stored match is its deal and its moves, every
// module's Apply is deterministic, so folding the moves back through the module
// reproduces every position a player stood in, and at each one the learning
// adapter (learn.Game) can say what the seat could see (Encode), what it could
// have done (Candidates), and what came of it (Reward).
//
// It is a package of its own, not a file in internal/learn, because it has to
// read the runtime's stored form and fold it with the runtime's own fold
// (match.FoldMoves), and internal/match's tests already import the game
// modules — which import learn. learn importing match would be a cycle the
// first time those tests were built.
//
// What goes out is chosen as carefully as what the encoder reads. Every
// observation is encoded for the seat that acted, so a record holds exactly
// what that person could see and nothing they could not. No user id, name,
// email or chat leaves: a match and a seat are salted hashes, the date is a
// day, and the action itself is described only by which candidate it was.
package learnexport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"zolik/server/internal/learn"
	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Record is one decision, as the trainer reads it. One JSON line each.
type Record struct {
	Game      string         `json:"game"`
	Variation string         `json:"variation,omitempty"`
	Options   map[string]int `json:"options,omitempty"`
	// Seats is how many sat at the table.
	Seats int `json:"seats"`
	// MatchHash and SeatHash are the match id and the seat's player id,
	// salted and hashed (see Hash). Stable within one salt, so every decision
	// of one match — or of one seat — can be grouped; meaningless without it.
	MatchHash string `json:"matchHash"`
	SeatHash  string `json:"seatHash"`
	// SeatIndex is the seat's place in the match's player list — its position
	// at the table, which the game already treats as public.
	SeatIndex int `json:"seatIndex"`
	// Bot marks a decision made by a bot seat, present only when bots were
	// asked for (Options.IncludeBots).
	Bot bool `json:"bot,omitempty"`
	// Date is the day the match ended, the only time that leaves. A trainer
	// wants to weight recent play; nobody needs the minute.
	Date string `json:"date,omitempty"`
	// Seq is the move's place in the match's log, 1-based.
	Seq int `json:"seq"`
	// Episode is how many episodes — hands, deals — this seat had finished
	// before this decision, so a trainer can group a decision with the
	// reward that closed it.
	Episode int `json:"episode"`
	// Verb is what kind of move the seat made. Only the verb: which move it
	// was is Chosen.
	Verb string `json:"verb"`

	// Obs is learn.Game.Encode of the position before the move, for this
	// seat: len(Obs) == StateDim.
	Obs []float32 `json:"obs"`
	// Cands are the candidates' feature vectors, each CandDim wide, in the
	// order Candidates returned them.
	Cands [][]float32 `json:"cands"`
	// Chosen indexes Cands, or is -1 when the move matched none of them; then
	// Unmatched says why. Never a guess.
	Chosen    int    `json:"chosen"`
	Unmatched string `json:"unmatched,omitempty"`
	// Approx is set when the move was matched to the nearest candidate rather
	// than an identical one — a Hold'em raise to a size the candidates do not
	// list — and carries the exact figure the seat chose.
	Approx *Approx `json:"approx,omitempty"`
	// PlanSteps is how many actions the chosen candidate makes (learn.
	// Candidate.Then), and PlanFollowed how many of them the seat went on to
	// make, in order. Both are 1 for a single-action move.
	PlanSteps    int `json:"planSteps,omitempty"`
	PlanFollowed int `json:"planFollowed,omitempty"`
	// Reordered says the seat made every step of the chosen plan, but in an
	// order of its own (see Choose).
	Reordered bool `json:"reordered,omitempty"`
	// WithinPlan marks a move that continues the multi-step candidate chosen
	// at this seat's previous decision — the second meld of a Canasta opening.
	// The environment never asks a network about these positions, because it
	// plays a plan whole; a NetBot does, one step at a time.
	WithinPlan bool `json:"withinPlan,omitempty"`

	Outcome Outcome `json:"outcome"`
}

// Approx is how far a nearest match was from the move actually made.
type Approx struct {
	Param  string `json:"param"`
	Actual string `json:"actual"`
	Chosen string `json:"chosen"`
}

// Outcome is what the decision led to.
type Outcome struct {
	// RewardToEpisodeEnd is the sum of learn.Game.Reward for this seat from
	// this move (inclusive) to the end of the episode it was made in —
	// undiscounted; a trainer discounts as it likes.
	RewardToEpisodeEnd float32 `json:"reward_to_episode_end"`
	// EpisodeDone is false when the log ran out first: a match abandoned
	// mid-hand, or a fold that stopped on a move the module now refuses.
	EpisodeDone bool `json:"episode_done"`
	// MatchOutcome is learn.Benchable.Outcome for this seat at the last
	// position the fold reached, in the game's own unit.
	MatchOutcome float64 `json:"match_outcome"`
	// MatchFinished says that position was the end of the match.
	MatchFinished bool `json:"match_finished"`
}

// Options are what one export is asked for.
type Options struct {
	// Salt keys every hash. Required: an unsalted SHA-256 of a Mongo ObjectID
	// can be reversed by anyone holding the ids, which is anyone with the
	// database.
	Salt string
	// IncludeBots exports bot decisions as well — for checking that a network
	// trained to clone the heuristic does — marked Bot.
	IncludeBots bool
}

// Stats is what an export saw, for the summary a run prints.
type Stats struct {
	Matches int `json:"matches"`
	// Truncated are matches whose fold stopped before the end of the log: the
	// module refuses a move it once accepted. Their decisions up to that move
	// are still exported.
	Truncated int `json:"truncated"`
	// Human and Bot count the decisions looked at; Records the ones written.
	Human   int `json:"humanDecisions"`
	Bot     int `json:"botDecisions"`
	Records int `json:"records"`
	// Matched are decisions with Chosen >= 0; Approximate the subset matched
	// to the nearest candidate; Forced the subset with only one candidate.
	Matched     int `json:"matched"`
	Approximate int `json:"approximate"`
	Forced      int `json:"forced"`
	// PlanStarts are decisions matched to a multi-step candidate; PlansWhole
	// the ones whose every step the seat then made (in any order); WithinPlan
	// the moves that were such a step.
	PlanStarts int `json:"planStarts"`
	PlansWhole int `json:"plansWhole"`
	WithinPlan int `json:"withinPlan"`
	// Unmatched counts Chosen == -1 by reason.
	Unmatched map[string]int `json:"unmatched,omitempty"`
	// Dropped counts decisions that produced no record at all — the position
	// could not be encoded — by reason. Should be empty.
	Dropped map[string]int `json:"dropped,omitempty"`
	// ByVerb is decisions and matches per verb, which is where a match rate
	// that is not 100% says what it is made of.
	ByVerb map[string]*VerbStats `json:"byVerb,omitempty"`
}

// VerbStats is the match rate of one verb.
type VerbStats struct {
	Decisions int `json:"decisions"`
	Matched   int `json:"matched"`
}

// Add folds another export's counts into s.
func (s *Stats) Add(o Stats) {
	s.Matches += o.Matches
	s.Truncated += o.Truncated
	s.Human += o.Human
	s.Bot += o.Bot
	s.Records += o.Records
	s.Matched += o.Matched
	s.Approximate += o.Approximate
	s.Forced += o.Forced
	s.PlanStarts += o.PlanStarts
	s.PlansWhole += o.PlansWhole
	s.WithinPlan += o.WithinPlan
	for k, v := range o.Unmatched {
		s.unmatched(k, v)
	}
	for k, v := range o.Dropped {
		s.dropped(k, v)
	}
	for k, v := range o.ByVerb {
		s.verb(k).Decisions += v.Decisions
		s.verb(k).Matched += v.Matched
	}
}

func (s *Stats) unmatched(reason string, n int) {
	if s.Unmatched == nil {
		s.Unmatched = map[string]int{}
	}
	s.Unmatched[reason] += n
}

func (s *Stats) dropped(reason string, n int) {
	if s.Dropped == nil {
		s.Dropped = map[string]int{}
	}
	s.Dropped[reason] += n
}

func (s *Stats) verb(v string) *VerbStats {
	if s.ByVerb == nil {
		s.ByVerb = map[string]*VerbStats{}
	}
	if s.ByVerb[v] == nil {
		s.ByVerb[v] = &VerbStats{}
	}
	return s.ByVerb[v]
}

// Hash is a salted SHA-256 of an id, as the 32 hex digits the records carry.
// The kind is mixed in so that a match and a seat that happened to share an
// id would not share a hash.
func Hash(salt, kind, id string) string {
	h := sha256.New()
	for _, part := range []string{salt, kind, id} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// maxLookahead bounds how far past a move the log is read to tell apart
// multi-step candidates that begin the same way. Canasta's openings are at
// most four actions (maxOpeningSteps), and a capture before one makes five.
const maxLookahead = 8

// Export folds one stored match and returns a record for every decision it
// is asked for: every human seat's, and the bots' too under IncludeBots.
// Its errors do not name the match — the caller knows which it asked about,
// and can log it by its hash.
//
// m, moves and snapshot are what match.BuildReplay reads — the envelope, the
// whole move log, and the stored boards (nil to rebuild the deal from the
// seed). A fold that stops part-way is not an error: the decisions before the
// refused move are still exactly what happened, and are returned with
// Stats.Truncated set.
func Export(g learn.Game, m models.Match, moves []models.MatchAction,
	snapshot func(seq int) (models.JSONDoc, error), opt Options) ([]Record, Stats, error) {
	var st Stats
	if opt.Salt == "" {
		return nil, st, fmt.Errorf("learnexport: a salt is required")
	}
	if m.ModuleID != g.Name() {
		return nil, st, fmt.Errorf("learnexport: the match is %q, not %q", m.ModuleID, g.Name())
	}
	mod := g.Module()
	st.Matches = 1

	seatIndex := make(map[string]int, len(m.Players))
	exported := map[string]bool{}
	for i, p := range m.Players {
		seatIndex[p.ID] = i
		if !p.IsAI || opt.IncludeBots {
			exported[p.ID] = true
		}
	}
	// The moves decoded once, for the lookahead that tells apart candidates
	// sharing a first step.
	actions := make([]module.Action, len(moves))
	decoded := make([]bool, len(moves))
	for i, mv := range moves {
		decoded[i] = json.Unmarshal(mv.Action, &actions[i]) == nil
	}

	matchHash := Hash(opt.Salt, "match", m.ID.Hex())
	date := ""
	switch {
	case m.EndedAt != nil:
		date = m.EndedAt.UTC().Format(time.DateOnly)
	case !m.CreatedAt.IsZero():
		date = m.CreatedAt.UTC().Format(time.DateOnly)
	}

	var (
		recs []Record
		prev module.State
		// prevPos is prev decoded once (learn.Positional), for the decision
		// made from it and for every seat's reward out of it.
		prevPos  learn.Position
		view     = learn.Positions(g)
		open     = map[string][]int{} // seat -> records whose episode is still running
		episodes = map[string]int{}
		plan     struct {
			seat string
			rest []module.Action
		}
	)
	_, foldErr := match.FoldMoves(mod, m, moves, snapshot,
		func(step int, entry *models.MatchAction, a module.Action, s module.State) (bool, error) {
			pos, err := view.Position(s)
			if err != nil {
				return false, fmt.Errorf("decode seq %d: %w", step, err)
			}
			if step == 0 || entry == nil {
				prev, prevPos = s, pos
				return true, nil
			}
			actor := entry.PlayerID
			if plan.seat != actor {
				plan.seat, plan.rest = "", nil
			}
			if exported[actor] {
				p := m.Players[seatIndex[actor]]
				if p.IsAI {
					st.Bot++
				} else {
					st.Human++
				}
				rec := Record{
					Game: g.Name(), Variation: m.Variation, Options: m.Options,
					Seats: len(m.Players), MatchHash: matchHash,
					SeatHash: Hash(opt.Salt, "seat", actor), SeatIndex: seatIndex[actor],
					Bot: p.IsAI, Date: date, Seq: entry.Seq, Episode: episodes[actor], Verb: a.Verb,
				}
				// Within a plan: the move is one of the steps still to make of
				// the multi-step candidate this seat chose last. Checked before
				// the plan is replaced by whatever this decision matches, and
				// kept if this one matches nothing.
				if rest := remove(plan.rest, a); len(rest) < len(plan.rest) {
					rec.WithinPlan = true
					st.WithinPlan++
					plan.rest = rest
				} else {
					plan.rest = nil
				}
				plan.seat = actor
				next := lookahead(moves, actions, decoded, step-1, actor)
				rest, reason := decide(g, view, mod, prev, prevPos, actor, a, next, &rec)
				switch {
				case reason != "":
					st.dropped(reason, 1)
				default:
					v := st.verb(a.Verb)
					v.Decisions++
					if rec.Chosen >= 0 {
						v.Matched++
						st.Matched++
						if rec.Approx != nil {
							st.Approximate++
						}
						if len(rec.Cands) == 1 {
							st.Forced++
						}
						if rec.PlanSteps > 1 && !rec.WithinPlan {
							st.PlanStarts++
							if rec.PlanFollowed == rec.PlanSteps {
								st.PlansWhole++
							}
						}
						plan.seat, plan.rest = actor, rest
					} else {
						st.unmatched(rec.Unmatched, 1)
					}
					open[actor] = append(open[actor], len(recs))
					recs = append(recs, rec)
				}
			}
			// The reward this move earned every exported seat, credited to
			// each of that seat's decisions whose episode is still running.
			// Asked of every exported seat, not only the ones with a decision
			// open, so the episode count stays right for a seat that sat a
			// hand out.
			for seat := range exported {
				r, done, err := view.RewardFor(prevPos, pos, seat)
				if err != nil {
					return false, fmt.Errorf("reward for seq %d: %w", entry.Seq, err)
				}
				for _, i := range open[seat] {
					recs[i].Outcome.RewardToEpisodeEnd += r
				}
				if done {
					for _, i := range open[seat] {
						recs[i].Outcome.EpisodeDone = true
					}
					open[seat] = nil
					episodes[seat]++
				}
			}
			prev, prevPos = s, pos
			return true, nil
		})
	if foldErr != nil {
		if prev == nil {
			// Not even the deal: there is no game here to learn from.
			return nil, st, fmt.Errorf("learnexport: no deal: %w", foldErr)
		}
		st.Truncated = 1
	}

	// The match's result, from the last position reached.
	finished := false
	if done, _, err := mod.Finished(prev); err == nil && done {
		finished = true
	}
	outcome := map[string]float64{}
	for i := range recs {
		seat := m.Players[recs[i].SeatIndex].ID
		o, ok := outcome[seat]
		if !ok {
			v, err := g.Outcome(prev, seat)
			if err != nil {
				return nil, st, fmt.Errorf("learnexport: outcome: %w", err)
			}
			o, outcome[seat] = v, v
		}
		recs[i].Outcome.MatchOutcome = o
		recs[i].Outcome.MatchFinished = finished
	}
	st.Records = len(recs)
	return recs, st, nil
}

// lookahead is the move at index from and the moves the same seat went on to
// make straight after it, without anybody else moving in between: the rest of
// its turn, which is where the rest of a multi-step candidate would be.
func lookahead(moves []models.MatchAction, actions []module.Action, decoded []bool, from int, seat string) []module.Action {
	var out []module.Action
	for i := from; i < len(moves) && len(out) < maxLookahead; i++ {
		if moves[i].PlayerID != seat || !decoded[i] {
			break
		}
		out = append(out, actions[i])
	}
	return out
}

// decide fills in the record for one decision: the observation, the
// candidates, and which of them the seat chose. It returns the rest of the
// chosen candidate's plan, and a reason when no record can be made at all.
func decide(g learn.Game, view learn.Positional, mod module.GameModule, s module.State, pos learn.Position,
	seat string, a module.Action, next []module.Action, rec *Record) (rest []module.Action, drop string) {
	obs, err := view.EncodeFor(pos, seat)
	if err != nil {
		return nil, "encode_error"
	}
	if len(obs) != g.StateDim() {
		return nil, "encode_width"
	}
	rec.Obs = obs
	rec.Cands = [][]float32{}
	rec.Chosen = -1

	offers, err := mod.LegalActions(s, seat)
	if err != nil {
		rec.Unmatched = "offers_error"
		return nil, ""
	}
	cands, err := view.CandidatesFor(pos, seat, offers)
	if err != nil {
		rec.Unmatched = "candidates_error"
		return nil, ""
	}
	for _, c := range cands {
		rec.Cands = append(rec.Cands, c.Features)
	}
	c := Choose(g, s, seat, cands, a, next)
	if c.Index < 0 {
		rec.Unmatched = c.Reason
		return nil, ""
	}
	rec.Chosen, rec.Approx, rec.Reordered = c.Index, c.Approx, c.Reordered
	steps := cands[c.Index].Steps()
	rec.PlanSteps, rec.PlanFollowed = len(steps), c.Followed
	return remove(steps, a), ""
}

// Choice is which candidate a played action was.
type Choice struct {
	// Index into the candidates, or -1 with Reason saying why none.
	Index  int
	Reason string
	// Approx is set for a nearest match (see Choose).
	Approx *Approx
	// Followed is how many of the candidate's steps the seat made, and
	// Reordered that it made all of them, but in another order.
	Followed  int
	Reordered bool
}

// Choose is which candidate a played action was, or -1 and why not.
//
// A candidate is the played move when:
//
//  1. its first action is the played one — verb, target, cards as a
//     multiset, params. The offer id is not compared: two builders can spell
//     the same move's offer differently, and the move is what was made; or
//  2. it is a plan of several actions and the seat made exactly those, in any
//     order, as the moves of its turn starting with this one. A Canasta
//     opening is a set of melds, and the adapter's search finds each set
//     once, in its own order — laying the sevens and then the fours is the
//     plan it offers as fours-then-sevens. The order a person lays them in
//     is not a different opening; or
//  3. the game says its first action is the same move (learn.Equivalence);
//     or
//  4. failing all of those, it differs from the played move only in numeric
//     params and is the nearest such — a raise to a size the candidates do
//     not list — reported with the exact figure (Approx).
//
// Several candidates can qualify when they are plans that begin alike and go
// on differently. next — the move being matched and those the seat made
// straight after it — decides: a plan the seat made whole beats one it made
// part of, and one it followed further beats one it left sooner. Beyond that a
// tie goes to the matching offer id, then to the first — identical moves, so
// not a guess.
func Choose(g learn.Game, s module.State, seat string, cands []learn.Candidate, a module.Action,
	next []module.Action) Choice {
	if len(cands) == 0 {
		return Choice{Index: -1, Reason: "no_candidates"}
	}
	if len(next) == 0 || !sameAction(next[0], a) {
		next = append([]module.Action{a}, next...)
	}
	eq, _ := g.(learn.Equivalence)
	best, bestScore := Choice{Index: -1}, -1
	for i, c := range cands {
		steps := c.Steps()
		inOrder := followed(steps, next)
		whole := inOrder == len(steps) || (len(steps) > 1 && madeAll(steps, next))
		first := sameAction(c.Action, a)
		if !first && !whole && !(eq != nil && eq.SameMove(s, seat, a, c.Action)) {
			continue
		}
		score := inOrder * 4
		if whole {
			score += 1000
		}
		if first {
			score += 2
		}
		if c.Action.OfferID == a.OfferID {
			score++
		}
		if score > bestScore {
			bestScore = score
			best = Choice{Index: i, Followed: inOrder}
			if whole {
				best.Followed = len(steps)
				best.Reordered = inOrder < len(steps)
			}
		}
	}
	if best.Index >= 0 {
		return best
	}
	if i, approx := nearest(cands, a); i >= 0 {
		return Choice{Index: i, Approx: approx, Followed: 1}
	}
	for _, c := range cands {
		for _, st := range c.Steps() {
			if st.Verb == a.Verb {
				return Choice{Index: -1, Reason: "not_a_candidate:" + a.Verb}
			}
		}
	}
	return Choice{Index: -1, Reason: "verb_not_offered:" + a.Verb}
}

// madeAll reports that the first len(steps) moves of next are steps, as a
// multiset of actions.
func madeAll(steps, next []module.Action) bool {
	if len(next) < len(steps) {
		return false
	}
	left := append([]module.Action(nil), steps...)
	for _, mv := range next[:len(steps)] {
		rest := remove(left, mv)
		if len(rest) == len(left) {
			return false
		}
		left = rest
	}
	return len(left) == 0
}

// remove is steps without the first action the same as a, or steps unchanged
// — as a copy — if none is.
func remove(steps []module.Action, a module.Action) []module.Action {
	out := make([]module.Action, 0, len(steps))
	done := false
	for _, st := range steps {
		if !done && sameAction(st, a) {
			done = true
			continue
		}
		out = append(out, st)
	}
	return out
}

// followed is how many of a candidate's steps the seat made, in order. next
// starts with the move being matched.
func followed(steps, next []module.Action) int {
	n := 0
	for i, st := range steps {
		if i >= len(next) || !sameAction(st, next[i]) {
			break
		}
		n++
	}
	return n
}

// sameAction compares what two actions do: verb, target, cards as a multiset
// (a hand holds two of a card under two decks, and the order a client lists
// them in is not part of the move), and params.
func sameAction(a, b module.Action) bool {
	if a.Verb != b.Verb || a.Target != b.Target || !sameCards(a.Cards, b.Cards) {
		return false
	}
	if len(a.Params) != len(b.Params) {
		return false
	}
	for k, v := range a.Params {
		if w, ok := b.Params[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func sameCards(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := make(map[string]int, len(a))
	for _, c := range a {
		count[c]++
	}
	for _, c := range b {
		count[c]--
		if count[c] < 0 {
			return false
		}
	}
	return true
}

// nearest is the candidate that makes the played move but for the value of
// its numeric params, closest by the sum of the differences.
func nearest(cands []learn.Candidate, a module.Action) (int, *Approx) {
	best, bestDist := -1, math.Inf(1)
	var bestApprox *Approx
	for i, c := range cands {
		ca := c.Action
		if ca.Verb != a.Verb || ca.Target != a.Target || !sameCards(ca.Cards, a.Cards) || len(ca.Params) != len(a.Params) {
			continue
		}
		dist, ok := 0.0, true
		var diff []string
		for k, v := range a.Params {
			w, has := ca.Params[k]
			if !has {
				ok = false
				break
			}
			if v == w {
				continue
			}
			x, err1 := strconv.ParseFloat(v, 64)
			y, err2 := strconv.ParseFloat(w, 64)
			if err1 != nil || err2 != nil {
				ok = false
				break
			}
			dist += math.Abs(x - y)
			diff = append(diff, k)
		}
		if !ok || len(diff) == 0 || dist >= bestDist {
			continue
		}
		sort.Strings(diff)
		best, bestDist = i, dist
		bestApprox = &Approx{Param: diff[0], Actual: a.Params[diff[0]], Chosen: ca.Params[diff[0]]}
	}
	return best, bestApprox
}
