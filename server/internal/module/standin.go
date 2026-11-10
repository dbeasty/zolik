package module

import "time"

// Stand-in bots: who plays a person's seat while they are away.
//
// A game that needs everyone — every game that is not DropIn — used to have one
// answer to a dropped phone: pause, and after a couple of minutes with nobody
// coming back, abandon. That is right for a table of two who both walked away,
// and wrong for the three people still sitting at it. So the table waits for a
// while, as it always has, and then a bot plays the seat until its player is
// back. The seat stays theirs throughout; the bot is a stand-in, not a
// replacement.
//
// Declared here, like OptPauseBetweenRounds and OptBotSkill, because it is
// about how a table is staffed rather than about any game's rules: one name on
// the wire, rendered off the descriptor by a client that does not know which
// game it is configuring. A drop-in game does not declare it — it already
// plays on past an absent seat (see DropIn) — and neither does a one-seat
// game, which has nobody to keep waiting.

// OptStandInAfter is how many seconds a table waits for a missing player
// before a bot plays for them. Zero is never: the table waits, as it did
// before this option existed.
const OptStandInAfter = "standInAfter"

// StandInAfterDefault is the wait when a lobby has not chosen one. Long enough
// for a page reload, a lift, a tunnel; short enough that the people still at
// the table are not left staring at a paused board.
const StandInAfterDefault = 120

// StandInOption is the ready-made spec a module drops into its descriptor.
func StandInOption() OptionSpec {
	return OptionSpec{
		Name:  OptStandInAfter,
		Type:  OptionEnumInt,
		Label: "When a player drops",
		Help:  "How long the table waits for a player whose connection dropped before a bot plays for them. They take their seat back as soon as they return.",
		Choices: []OptionChoice{
			{Value: 60, Label: "Bot after 1 minute"},
			{Value: 120, Label: "Bot after 2 minutes"},
			{Value: 240, Label: "Bot after 4 minutes"},
			{Value: 600, Label: "Bot after 10 minutes"},
			{Value: 0, Label: "Wait for them"},
		},
	}
}

// StandInAfter reads the option: how long to wait, or zero for never.
func (c MatchConfig) StandInAfter() time.Duration {
	return time.Duration(c.Opt(OptStandInAfter, StandInAfterDefault)) * time.Second
}

// DeclaresStandIn reports whether a module offers stand-in bots at all.
func DeclaresStandIn(m GameModule) bool {
	return m != nil && m.Descriptor().Option(OptStandInAfter) != nil
}

// StandInRules is the section a module appends to its written rules, so the
// rules screen says what this table does when somebody's connection drops.
func StandInRules(c MatchConfig) RuleSection {
	secs := int(c.StandInAfter() / time.Second)
	var facts []Fact
	if secs == 0 {
		facts = append(facts, Fact{LabelKey: "rules.standIn.wait"})
	} else {
		facts = append(facts,
			Fact{LabelKey: "rules.standIn.after", Params: map[string]any{"seconds": secs}},
			Fact{LabelKey: "rules.standIn.return"},
		)
	}
	if c.StepOutAllowed() {
		facts = append(facts, Fact{LabelKey: "rules.standIn.stepOut"})
	}
	if secs != 0 || c.StepOutAllowed() {
		facts = append(facts, Fact{LabelKey: "rules.standIn.record"})
	}
	return Section("rules.standIn.section", facts...)
}

// OptStepOut lets a player hand their own seat to a stand-in bot on purpose —
// to answer the door, put the kettle on — and take it back when they return.
// The same stand-in as a dropped connection gets, put in by its own player
// rather than by a clock or the host, and it plays while they watch.
//
// Off by default, which is how a table has always behaved: whether a person
// may leave a bot playing for them is the host's call, made in the lobby.
// Declared by every game that declares OptStandInAfter, since it is the same
// bot doing the same job.
const OptStepOut = "stepOut"

// StepOutOption is the ready-made spec a module drops into its descriptor.
func StepOutOption() OptionSpec {
	return OptionSpec{
		Name:  OptStepOut,
		Type:  OptionEnumInt,
		Label: "Step out",
		Help:  "Let a player hand their seat to a bot while they step away, and take it back when they return.",
		Choices: []OptionChoice{
			{Value: OptOff, Label: "Off"},
			{Value: OptOn, Label: "Allowed"},
		},
	}
}

// StepOutAllowed reads the option. A match created before it existed has no
// value for it, and does not allow it.
func (c MatchConfig) StepOutAllowed() bool {
	return c.Opt(OptStepOut, OptOff) == OptOn
}

// DeclaresStepOut reports whether a module offers the option at all.
func DeclaresStepOut(m GameModule) bool {
	return m != nil && m.Descriptor().Option(OptStepOut) != nil
}
