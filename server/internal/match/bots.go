package match

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"zolik/server/internal/botstats"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
)

// Driving the seats nobody is sitting at.
//
// This is the half of Phase 6 that makes a new module playable the day it is
// registered. The rummy runtime had an AI loop of its own, four hundred lines
// of it, that knew about melds and discards and had a rummy-specific recovery
// path when its agent proposed something illegal. None of that is a property of
// rummy: *when a seat is a bot's*, *when it is that seat's turn* and *what to do
// when a bot proposes an illegal move* are runtime questions with the same
// answer in every game.
//
// So the loop lives here and knows nothing. It asks the module who is on turn,
// asks the module's own bot for a move, and applies it through exactly the same
// path a human's action takes — no privileged entry point, no skipped
// validation.

const (
	// botMaxSteps bounds a whole chain of bot moves. It is a runaway guard,
	// not a turn limiter: one bot turn can legitimately be several actions
	// (draw, meld, meld, discard) and several bots can act back to back.
	botMaxSteps = 400
	// botMaxStall is how many consecutive actions one seat may take without
	// the turn moving on before the loop gives up on it.
	botMaxStall = 30
	// The pause a bot takes before answering, when nothing has said otherwise
	// (see Manager.SetBotPace). It has to outlast the client's narration of
	// the move before it, or a bot's next state lands on top of an animation
	// that is still playing.
	botThinkMinDefault = 900 * time.Millisecond
	botThinkMaxDefault = 1800 * time.Millisecond
	// botActBudgetDefault bounds one call to a module's Bot.Act (see
	// Manager.SetBotActBudget). Act takes no context and some bots run a real
	// search — Žolíky's initial-meld planner is combinatorial — so a slow or
	// wedged decision used to hold this loop forever: no stall log, no
	// give-up, and a table nobody could move again, because the seat it waits
	// on is the one no human may act for. Generous next to any sane decision,
	// short next to a person staring at a frozen board.
	botActBudgetDefault = 5 * time.Second
)

// thinkFor is how long a bot pauses before answering. Purely cosmetic, and the
// one thing in this file that is about people rather than rules.
func (m *Manager) thinkFor(rnd *rand.Rand) time.Duration {
	lo, hi := m.botThinkMin, m.botThinkMax
	if lo <= 0 || hi <= lo {
		lo, hi = botThinkMinDefault, botThinkMaxDefault
	}
	return lo + time.Duration(rnd.Int63n(int64(hi-lo)))
}

func (m *Manager) actBudget() time.Duration {
	if m.botActBudget > 0 {
		return m.botActBudget
	}
	return botActBudgetDefault
}

// botAct asks a module's bot for a move, but stops waiting after budget.
//
// A timeout is reported as "no answer" plus timedOut, and the caller carries on
// down the offer list exactly as it does when a bot declines — the path every
// module is guaranteed to support. Act cannot be cancelled, so the call is
// abandoned rather than stopped: its goroutine runs until Act returns (forever,
// for a bot that truly hangs), and its result goes into a buffered channel
// nobody reads, so a late answer can neither block that goroutine nor reach
// the loop.
//
// What the abandoned call holds must not be anything the loop goes on using.
// The state it gets is a private copy, since match.State may be the live board
// the Manager keeps in memory. The offers it gets are the caller's; on a
// timeout the caller must stop reading them and fetch its own again, rather
// than race a call that may still be reading — or, if it misbehaves, writing —
// the nested slices inside them.
func botAct(bot module.Bot, state module.State, seat module.BotSeat, offers []module.ActionOffer, budget time.Duration, rec *botstats.Recorder, key botstats.Key) (action module.Action, ok, timedOut bool) {
	type answer struct {
		action module.Action
		ok     bool
	}
	ch := make(chan answer, 1)
	// Closed when Act returns, however long after the timeout that is, so
	// the time it really took is what gets recorded — and an abandoned call
	// is counted as burning a core for as long as it actually does.
	done := make(chan struct{})
	state = bytes.Clone(state)
	end := rec.Begin(key)
	go func() {
		defer close(done)
		defer end()
		a, ok := bot.Act(state, seat, offers)
		ch <- answer{a, ok}
	}()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.action, r.ok, false
	case <-timer.C:
		finished := rec.Abandoned()
		go func() { <-done; finished() }()
		return module.Action{}, false, true
	}
}

// RunBotsIfNeeded starts the bot loop for a match, unless it is already
// running.
func (m *Manager) RunBotsIfNeeded(ctx context.Context, matchID string) {
	m.botMu.Lock()
	if m.botRunning == nil {
		m.botRunning = map[string]bool{}
	}
	if m.botRunning[matchID] {
		m.botMu.Unlock()
		return
	}
	m.botRunning[matchID] = true
	m.botMu.Unlock()

	m.botStats.LoopStarted()
	go m.botLoop(ctx, matchID)
}

func (m *Manager) botLoop(ctx context.Context, matchID string) {
	defer func() {
		m.botStats.LoopEnded()
		m.botMu.Lock()
		delete(m.botRunning, matchID)
		m.botMu.Unlock()
	}()

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	lastActor, stall := "", 0
	var turn botTurn

	for step := 0; step < botMaxSteps; step++ {
		// The turn boundary is where this match can change hands: nothing is
		// half-applied here, and the node taking it over rebuilds from what is
		// stored. See Manager.Release.
		if m.botsStopping(matchID) {
			return
		}
		match, err := m.current(ctx, matchID)
		if err != nil || match.Status != "active" {
			if err == nil {
				m.matchStopped(matchID)
			}
			return
		}
		mod := m.registry.Get(match.ModuleID)
		if mod == nil {
			return
		}

		// The first bot among the seats being waited on, rather than whichever
		// seat happens to be first.
		//
		// For almost all of a match those are the same thing, because there is
		// only ever one seat to wait on. Between rounds there are several, and
		// taking only the first would mean a bot behind a human in the seat
		// order never readied until that human had — leaving the table on a
		// human's click to do work that has nothing to do with them, and
		// leaving a window where this loop unwinds just as that click lands and
		// nothing restarts it.
		actor, passive := m.drivenSeat(match, mod, module.AwaitedSeats(mod, module.State(match.State), viewerFor(match), refsOf(match)))
		if actor == "" {
			return // nobody awaited, or nobody awaited is a bot
		}

		if actor == lastActor {
			stall++
			if stall > botMaxStall {
				log.Printf("bot loop: match=%s seat=%s made no progress in %d actions, giving up",
					matchID, actor, stall)
				return
			}
		} else {
			lastActor, stall = actor, 0
			turn = botTurn{}
		}

		time.Sleep(m.thinkFor(rnd))

		offers, err := mod.LegalActions(module.State(match.State), actor)
		if err != nil {
			return
		}
		// The module's own bot, bounded — see botAct. Once it has run out of
		// time in a turn it is not asked again until the turn moves on: a bot
		// that hung once on a position will likely hang on the next, and
		// asking would cost the budget and one more stranded goroutine per
		// action for nothing.
		var botPick module.Action
		botOK := false
		if !turn.botHung {
			var timedOut bool
			var bot module.Bot
			var seat module.BotSeat
			var label, engine string
			if passive {
				// A sat-out seat is not played to win. See module.DropIn.
				// Recorded apart: it costs an offer scan, and counting it as
				// the seat's skill would make that skill look cheaper.
				verbs, _ := module.SitOutVerbs(mod)
				bot, seat = module.OfferBot(verbs...), botSeatFor(match, actor)
				label, engine = "sitout", botstats.EngineRule
			} else {
				// Chosen at the start of the seat's turn and held for the
				// rest of it — see governor.go.
				tb := m.seatBot(match, mod, actor)
				bot, seat, engine = tb.bot, tb.seat, tb.engine
				label = skillLabel(seat.Skill)
			}
			botPick, botOK, timedOut = botAct(bot, module.State(match.State),
				seat, offers, m.actBudget(), m.botStats,
				botstats.Key{Module: match.ModuleID, Skill: label, Source: botstats.SourceLoop, Engine: engine})
			if timedOut {
				log.Printf("bot loop: match=%s seat=%s bot gave no move within %s; playing from the offer list",
					matchID, actor, m.actBudget())
				turn.botHung = true
				// The abandoned call still holds those offers. Take a fresh
				// set so nothing below shares memory with it.
				if offers, err = mod.LegalActions(module.State(match.State), actor); err != nil {
					return
				}
			}
		}
		// Everything this seat could send, best first: the module's own bot if it
		// has an answer, then every submission the offer list describes.
		//
		// A refusal is not the end of the turn. An offer is built by probing the
		// validator, so a refused one means the state moved under the probe or
		// the module's translation is wrong about that verb — neither of which
		// says anything about the *next* offer. Trying them in turn is what a
		// person does when a control they expected to work does not, and it is
		// the difference between a seat that loses one move and a deal that
		// stops.
		candidates := botCandidates(offers)
		if botOK {
			candidates = append([]botMove{{action: botPick, undo: isUndoIn(offers, botPick)}}, candidates...)
		}
		if len(candidates) == 0 {
			logOfferState(matchID, actor, offers, "has no legal move; stopping")
			return
		}

		played, skipped := false, 0
		for i, c := range candidates {
			// Only the prepended bot pick is deduplicated, and only against the
			// list's first choice, which is the one case where the two sources
			// routinely agree. Nothing further down is compared: sameAction
			// reads verb, target and cards but not OfferID, and the four undo
			// offers are identical under it — skipping "duplicates" would drop
			// undo:lay_meld and undo:turn, which are the candidates most likely
			// to get a wedged turn moving again.
			if i == 1 && sameAction(c.action, candidates[0].action) {
				continue
			}
			if turn.skips(c) {
				skipped++
				continue
			}
			err := m.HandleAction(ctx, matchID, actor, c.action)
			if err == nil {
				if i > 0 {
					log.Printf("bot loop: match=%s seat=%s recovered on candidate %d/%d (%s)",
						matchID, actor, i+1, len(candidates), c.action.Verb)
				}
				turn.note(c)
				played = true
				break
			}
			log.Printf("bot loop: match=%s seat=%s candidate %d/%d (%s) refused: %v",
				matchID, actor, i+1, len(candidates), c.action.Verb, err)
		}
		if !played {
			// Skipped and refused counted apart, because they mean opposite
			// things to whoever reads this. Refused is the engine saying no;
			// skipped is this loop declining to make a move it has already
			// taken back, and a line that is mostly skips is a turn that
			// unwound and found nothing else to do rather than one the rules
			// shut out.
			logOfferState(matchID, actor, offers,
				fmt.Sprintf("all %d candidate moves exhausted (%d refused, %d already tried this turn); stopping",
					len(candidates), len(candidates)-skipped, skipped))
			return
		}
	}
}

// logOfferState logs the state of all available offers, which ones are enabled,
// and which error codes disabled the rest. This helps diagnose game state wedges.
func logOfferState(matchID, actor string, offers []module.ActionOffer, reason string) {
	var enabled, disabled int
	disabledByCode := make(map[string][]string)

	for _, o := range offers {
		if o.Enabled {
			enabled++
		} else {
			disabled++
			disabledByCode[o.WhyNot] = append(disabledByCode[o.WhyNot], o.ID)
		}
	}

	log.Printf("bot loop: match=%s seat=%s %s [enabled=%d disabled=%d]",
		matchID, actor, reason, enabled, disabled)

	for code, verbs := range disabledByCode {
		log.Printf("  disabled by %q: %v", code, verbs)
	}
}

// botMove is one thing this seat could send, and whether sending it would take
// a move back rather than make one. The offer list already says which — see
// module.ActionOffer.Undo — and the loop's replay guard needs to know.
type botMove struct {
	action module.Action
	undo   bool
}

// botTurn is what the loop remembers inside one seat's turn: the moves that
// seat has made, and whether it has started taking any of them back. Cleared
// the moment the turn moves on, because none of it means anything about the
// next one.
//
// It exists for one shape of stuck turn. An undo is the only candidate that
// can succeed and leave the seat facing exactly the decision it just made, and
// a module's bot is a pure function of the position — TestBotNeverTakesAMoveBack
// is what stops it oscillating on its own account — so handed back a position
// it has already answered, it answers the same way. The loop then played
// take_pile, undo_take_pile, take_pile for thirty actions and gave up, on a
// table that was frozen for everybody from then on. That is what happened to
// game 6aaa157d0079d0b3a6624b3a; canasta's own fix for the position that led
// there is in meld.go, and this is the guard that means the next such position
// costs a seat one bad move rather than the whole table its deal.
//
// So unwinding is one-way: once a move has been taken back, what this turn has
// already tried is off the list and the loop works down to something it has not
// done — drawing, in the case above, which is exactly what a person does after
// putting the pile back. It arms only once an undo has actually been played, so
// ordinary play, where the same submission twice is a coincidence of card codes
// rather than a loop, is untouched.
type botTurn struct {
	played    []module.Action
	unwinding bool
	// botHung is set once the module's bot has overrun its budget this turn;
	// the rest of the turn plays from the offer list. See botAct.
	botHung bool
}

// skips is whether this candidate is one the turn has already made and taken
// back. Undos are never skipped: they are how the unwinding gets anywhere, and
// there are only ever as many of them as there are moves to take back.
func (t *botTurn) skips(c botMove) bool {
	if !t.unwinding || c.undo {
		return false
	}
	for _, p := range t.played {
		if sameAction(p, c.action) {
			return true
		}
	}
	return false
}

// note records a candidate the loop has just played.
func (t *botTurn) note(c botMove) {
	if c.undo {
		t.unwinding = true
		return
	}
	t.played = append(t.played, c.action)
}

// botCandidates is every submission the offer list describes, in the order it
// describes them. module.ChooseActions with no preference, carrying the one
// property of the offer the loop cannot re-derive from the submission alone.
func botCandidates(offers []module.ActionOffer) []botMove {
	out := make([]botMove, 0, len(offers))
	for i := range offers {
		if a, ok := module.SubmissionFor(offers[i]); ok {
			out = append(out, botMove{action: a, undo: offers[i].Undo})
		}
	}
	return out
}

// isUndoIn is whether the module's own bot has picked an undo, read off the
// offer it must have come from. The shipped bots decline to, but the loop is
// not the place to assume that of a module it has never seen.
func isUndoIn(offers []module.ActionOffer, a module.Action) bool {
	for i := range offers {
		if !offers[i].Undo {
			continue
		}
		if b, ok := module.SubmissionFor(offers[i]); ok && sameAction(a, b) {
			return true
		}
	}
	return false
}

// botSeatFor is who this seat is and how well it plays.
//
// The skill comes off the seat rather than off the match, which is what makes
// a mixed table possible: under the lobby's Mixed setting every bot drew its
// own strength when it sat down, and this is where that shows up as different
// play rather than as a different label. A seat with nothing recorded — every
// bot seated before any of this existed — resolves to the module's own
// default, which for Žolíky is the agent those matches have been playing all
// along.
//
// The seed is derived, never stored: the same seat of the same match always
// gets the same one, so a bot loop that restarts after a reconnect carries on
// making the same decisions instead of becoming a subtly different opponent
// halfway through a deal.
func botSeatFor(match models.Match, actor string) module.BotSeat {
	seat := module.BotSeat{
		PlayerID: actor,
		Seed:     module.SeatSeed(match.Seed, actor, "bot"),
	}
	if p := playerByID(match.Players, actor); p != nil {
		if skill, auto := module.ParseSkill(p.AIDifficulty); !auto {
			seat.Skill = skill
		}
	}
	return seat
}

// skillLabel names a seat's skill for the cost series. A seat seated before
// skills existed plays its module's own default, which is not any one level.
func skillLabel(s module.Skill) string {
	if s == "" {
		return "default"
	}
	return string(s)
}

// engineOf names what will decide this seat's move. A module whose Hard seats
// have been switched onto its trained model hands out a bot that layers the
// model over the heuristic (learn.HardModel); only a Hard seat reaches the
// model, and everything else is the hand-written bot.
func engineOf(bot module.Bot, seat module.BotSeat) string {
	if _, layered := bot.(interface{ Heuristic() module.Bot }); layered && seat.Skill == module.SkillHard {
		return botstats.EngineNet
	}
	return botstats.EngineRule
}

// firstBot picks the first awaited seat that nobody is sitting at.
func firstBot(awaited []string, players []models.Player) string {
	for _, id := range awaited {
		if p := playerByID(players, id); p != nil && p.IsAI {
			return id
		}
	}
	return ""
}

func sameAction(a, b module.Action) bool {
	if a.Verb != b.Verb || a.Target != b.Target || len(a.Cards) != len(b.Cards) {
		return false
	}
	for i := range a.Cards {
		if a.Cards[i] != b.Cards[i] {
			return false
		}
	}
	return true
}

func playerByID(players []models.Player, id string) *models.Player {
	for i := range players {
		if players[i].ID == id {
			return &players[i]
		}
	}
	return nil
}

func refsOf(match models.Match) []module.PlayerRef {
	out := make([]module.PlayerRef, 0, len(match.Players))
	for _, p := range match.Players {
		out = append(out, module.PlayerRef{ID: p.ID, Name: p.Name, IsAI: p.IsAI})
	}
	return out
}

// viewerFor picks whose eyes to read the seat list through.
//
// Any seat will do: which seats exist and which one is active are public in
// every game — it is the *cards* a view hides, not the table.
func viewerFor(match models.Match) string {
	if len(match.Players) > 0 {
		return match.Players[0].ID
	}
	return ""
}
