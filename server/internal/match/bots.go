package match

import (
	"context"
	"log"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

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
	// botActTimeout bounds how long a module's Bot.Act may take to decide a
	// single move. Reproduced live: a zolikmod match sat with a bot on turn
	// and zero bot actions recorded for the rest of the session, with no
	// error and none of botMaxStall's own stall log — heuristicBot.Act runs
	// a real combinatorial search (internal/ai's initial-meld planner) with
	// no way to be cancelled, so a slow or wedged search previously blocked
	// this seat's turn forever rather than merely stalling it.
	botActTimeout = 5 * time.Second
)

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

	go m.botLoop(ctx, matchID)
}

func (m *Manager) botLoop(ctx context.Context, matchID string) {
	defer func() {
		m.botMu.Lock()
		delete(m.botRunning, matchID)
		m.botMu.Unlock()
	}()

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	lastActor, stall := "", 0

	for step := 0; step < botMaxSteps; step++ {
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

		actor := module.ActiveSeat(mod, match.State, viewerFor(match), refsOf(match))
		if actor == "" {
			return
		}
		seat := playerByID(match.Players, actor)
		if seat == nil || !seat.IsAI {
			return // a human is on turn; nothing for this loop to do
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
		}

		// A pause, so a bot does not answer instantly. Purely cosmetic, and
		// the one thing in this file that is about people rather than rules.
		time.Sleep(time.Duration(400+rnd.Intn(900)) * time.Millisecond)

		offers, err := mod.LegalActions(match.State, actor)
		if err != nil {
			return
		}
		action, ok := actWithTimeout(mod, match.State, actor, offers, botActTimeout)
		if !ok {
			// The module's own bot had no answer. Fall back to the offer list,
			// which is the one thing every module is guaranteed to produce.
			action, ok = module.ChooseAction(offers, nil)
			if !ok {
				log.Printf("bot loop: match=%s seat=%s has no legal move; stopping", matchID, actor)
				return
			}
		}

		if err := m.HandleAction(ctx, matchID, actor, action); err != nil {
			// A bot proposing something illegal must not be retried verbatim —
			// it would choose the same losing move again and burn the whole
			// step budget without ever ending its turn. Recovering through the
			// offer list is the generic version of the rummy runtime's
			// "fall back to discarding the worst card", and it works for a game
			// with no discards.
			fallback, ok := module.ChooseAction(offers, nil)
			if !ok || sameAction(fallback, action) {
				log.Printf("bot loop: match=%s seat=%s move refused (%v) and no fallback; stopping",
					matchID, actor, err)
				return
			}
			if err := m.HandleAction(ctx, matchID, actor, fallback); err != nil {
				log.Printf("bot loop: match=%s seat=%s fallback refused too: %v", matchID, actor, err)
				return
			}
		}
	}
}

// actWithTimeout bounds a module's Bot.Act to a wall-clock budget instead of
// trusting it to return promptly. Act takes no context and offers no
// cooperative way to cancel it, so a call that blows the budget is treated
// exactly like "the bot had no answer" — the one outcome every caller here
// already knows how to recover from via the illegal-move fallback below —
// rather than left to hang the turn. The call itself is abandoned, not
// killed: Go cannot force a goroutine to stop, so a bot that truly never
// returns leaks one goroutine per occurrence instead of wedging the match.
func actWithTimeout(
	mod module.GameModule,
	state module.State,
	actor string,
	offers []module.ActionOffer,
	timeout time.Duration,
) (module.Action, bool) {
	type result struct {
		action module.Action
		ok     bool
	}
	ch := make(chan result, 1)
	go func() {
		action, ok := module.BotFor(mod).Act(state, actor, offers)
		ch <- result{action, ok}
	}()
	select {
	case r := <-ch:
		return r.action, r.ok
	case <-time.After(timeout):
		log.Printf("bot loop: seat=%s Bot.Act exceeded %s, falling back to offer list", actor, timeout)
		return module.Action{}, false
	}
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
