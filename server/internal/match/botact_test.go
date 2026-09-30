package match

import (
	"testing"
	"time"

	"zolik/server/internal/module"
)

// hungBot is a module bot whose decision never comes back until the test lets
// it. finished closes once Act has actually returned, so a test can see that
// the abandoned call was not left blocked on a send nobody reads.
type hungBot struct {
	release  chan struct{}
	finished chan struct{}
}

func newHungBot() *hungBot {
	return &hungBot{release: make(chan struct{}), finished: make(chan struct{})}
}

func (b *hungBot) Act(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool) {
	defer close(b.finished)
	<-b.release
	return module.Action{Verb: "too_late"}, true
}

type quickBot struct{}

func (quickBot) Act(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool) {
	return module.Action{Verb: "draw"}, true
}

// A bot that never decides costs the budget and no more, and reads as "no
// answer" so the loop falls through to the offer list.
func TestBotActGivesUpOnAHungBotWithinTheBudget(t *testing.T) {
	bot := newHungBot()
	const budget = 50 * time.Millisecond

	start := time.Now()
	_, ok, timedOut := botAct(bot, module.State(`{}`), module.BotSeat{PlayerID: "p1"}, nil, budget)
	took := time.Since(start)

	if !timedOut || ok {
		t.Fatalf("botAct = ok %v, timedOut %v; want no answer and a timeout", ok, timedOut)
	}
	if took < budget || took > budget+time.Second {
		t.Errorf("botAct returned after %s, want about %s", took, budget)
	}

	// The late answer must go nowhere and must not strand its goroutine.
	close(bot.release)
	select {
	case <-bot.finished:
	case <-time.After(time.Second):
		t.Fatal("the abandoned Act never returned: its result send blocked")
	}
}

// A bot that answers in time is not second-guessed.
func TestBotActPassesThroughAPromptAnswer(t *testing.T) {
	a, ok, timedOut := botAct(quickBot{}, module.State(`{}`), module.BotSeat{PlayerID: "p1"}, nil, time.Second)
	if timedOut || !ok || a.Verb != "draw" {
		t.Fatalf("botAct = %+v, ok %v, timedOut %v; want the bot's draw", a, ok, timedOut)
	}
}

// The abandoned call works on its own copy of the board, so a bot that writes
// to it after the loop has moved on cannot touch the live match.
func TestBotActHandsTheBotItsOwnCopyOfTheState(t *testing.T) {
	state := module.State(`{"x":1}`)
	scribbler := botFunc(func(s module.State, _ module.BotSeat, _ []module.ActionOffer) (module.Action, bool) {
		s[0] = 'X'
		return module.Action{}, false
	})
	botAct(scribbler, state, module.BotSeat{}, nil, time.Second)
	if string(state) != `{"x":1}` {
		t.Errorf("the bot wrote through to the caller's state: %s", state)
	}
}

type botFunc func(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool)

func (f botFunc) Act(s module.State, seat module.BotSeat, o []module.ActionOffer) (module.Action, bool) {
	return f(s, seat, o)
}
