package match

import (
	"testing"
	"time"

	"zolik/server/internal/botstats"
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
	_, ok, timedOut := botAct(bot, module.State(`{}`), module.BotSeat{PlayerID: "p1"}, nil, budget, nil, botstats.Key{})
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
	a, ok, timedOut := botAct(quickBot{}, module.State(`{}`), module.BotSeat{PlayerID: "p1"}, nil, time.Second, nil, botstats.Key{})
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
	botAct(scribbler, state, module.BotSeat{}, nil, time.Second, nil, botstats.Key{})
	if string(state) != `{"x":1}` {
		t.Errorf("the bot wrote through to the caller's state: %s", state)
	}
}

type botFunc func(module.State, module.BotSeat, []module.ActionOffer) (module.Action, bool)

func (f botFunc) Act(s module.State, seat module.BotSeat, o []module.ActionOffer) (module.Action, bool) {
	return f(s, seat, o)
}

// An abandoned decision is still costing a core, so it is counted as an orphan
// until it returns — and then recorded at the time it really took, not at the
// budget the runtime waited.
func TestBotActCountsAnAbandonedDecisionUntilItReturns(t *testing.T) {
	rec := botstats.New()
	key := botstats.Key{Module: "holdem", Skill: "hard", Source: botstats.SourceLoop}
	bot := newHungBot()
	const budget = 20 * time.Millisecond

	if _, _, timedOut := botAct(bot, module.State(`{}`), module.BotSeat{PlayerID: "p1"}, nil, budget, rec, key); !timedOut {
		t.Fatal("expected a timeout")
	}
	if s := rec.Snapshot(); s.Orphans != 1 || s.InFlight != 1 || s.Timeouts != 1 || len(s.Total) != 0 {
		t.Fatalf("while abandoned: %+v", s)
	}

	time.Sleep(3 * budget)
	close(bot.release)
	<-bot.finished
	deadline := time.Now().Add(time.Second)
	for rec.Snapshot().Orphans != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	s := rec.Snapshot()
	if s.Orphans != 0 || s.InFlight != 0 || s.Timeouts != 1 {
		t.Fatalf("after return: %+v", s)
	}
	if len(s.Total) != 1 || s.Total[0].Count != 1 || s.Total[0].MaxM < float64(4*budget/time.Millisecond) {
		t.Fatalf("the decision should be recorded at its real length (≥ %s): %+v", 4*budget, s.Total)
	}
}

func TestBotActRecordsAPromptDecision(t *testing.T) {
	rec := botstats.New()
	key := botstats.Key{Module: "prsi", Skill: "easy", Source: botstats.SourceLoop}
	if _, ok, _ := botAct(quickBot{}, module.State(`{}`), module.BotSeat{PlayerID: "p1"}, nil, time.Second, rec, key); !ok {
		t.Fatal("no answer")
	}
	deadline := time.Now().Add(time.Second)
	for rec.Snapshot().InFlight != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	s := rec.Snapshot()
	if s.Timeouts != 0 || len(s.Total) != 1 || s.Total[0].Key != key {
		t.Fatalf("%+v", s)
	}
}

// layeredBot stands in for learn.HardModel's switched-on bot: a model for Hard
// seats over the heuristic for the rest.
type layeredBot struct{ quickBot }

func (layeredBot) Heuristic() module.Bot { return quickBot{} }

// The cost series name the engine that decided, so a game whose Hard seats
// were switched onto a model does not blend model and heuristic timings into
// one "hard" line.
func TestEngineOfNamesTheModelOnlyForHardSeats(t *testing.T) {
	for _, tc := range []struct {
		bot   module.Bot
		skill module.Skill
		want  string
	}{
		{quickBot{}, module.SkillHard, botstats.EngineRule},
		{layeredBot{}, module.SkillHard, botstats.EngineNet},
		{layeredBot{}, module.SkillMedium, botstats.EngineRule},
		{layeredBot{}, "", botstats.EngineRule},
	} {
		if got := engineOf(tc.bot, module.BotSeat{Skill: tc.skill}); got != tc.want {
			t.Errorf("engineOf(%T, %q) = %s, want %s", tc.bot, tc.skill, got, tc.want)
		}
	}
}
