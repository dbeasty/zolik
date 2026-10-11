// Package lessontest walks a module's guided lessons through the real engine.
//
// A lesson is only worth showing if a person can actually follow it: every
// step that waits for a move must be reachable from the lesson's seed, at
// every variation and option the module ships. That is a property of the
// engine, not of the lesson's text, so it is checked by playing — the lesson's
// own player takes the moves each step asks for, the bots take theirs, and the
// run fails the moment a step can never be satisfied.
//
// Used by every module's tests through Check, and by the cross-module sweep in
// internal/app, so a module that ships a variation or option its lessons were
// not written for fails CI rather than a player's screen.
package lessontest

import (
	"fmt"
	"sort"
	"testing"

	"zolik/server/internal/module"
)

const (
	// maxMoves bounds one walk-through. A lesson is a few turns long; a run
	// that needs thousands of moves is a lesson that cannot finish.
	maxMoves = 3000
	human    = "you"
)

// Configs is every table configuration worth teaching on: each variation at
// its defaults, then each variation with each option set to each of its
// choices in turn. One option at a time, because a lesson's conditions are
// per option; the full product would be a sweep nobody could wait for.
func Configs(m module.GameModule) []module.MatchConfig {
	d := m.Descriptor()
	variations := []string{""}
	for _, v := range d.Variations {
		variations = append(variations, v.ID)
	}
	if len(d.Variations) > 0 {
		variations = variations[1:]
	}
	var out []module.MatchConfig
	for _, v := range variations {
		out = append(out, module.MatchConfig{Variation: v, Options: module.Options{}})
		for _, o := range d.Options {
			for _, c := range o.Choices {
				out = append(out, module.MatchConfig{Variation: v, Options: module.Options{o.Name: c.Value}})
			}
		}
	}
	return out
}

// Check plays every lesson the module ships on every configuration, and
// fails with the configuration named. A module with no lessons passes: the
// coverage rule is the caller's, not this walk's.
func Check(t *testing.T, m module.GameModule) {
	t.Helper()
	id := m.Descriptor().ID
	for _, cfg := range Configs(m) {
		cfg := cfg
		lessons, ok, err := module.LessonsFor(m, cfg)
		if err != nil {
			t.Errorf("%s %s: Lessons: %v", id, Describe(cfg), err)
			continue
		}
		if !ok {
			return
		}
		if len(lessons) == 0 {
			t.Errorf("%s %s: a module that implements LessonProvider must ship at least one lesson", id, Describe(cfg))
		}
		for _, l := range lessons {
			if err := l.Validate(); err != nil {
				t.Errorf("%s %s: %v", id, Describe(cfg), err)
				continue
			}
			if err := Walk(m, cfg, l); err != nil {
				t.Errorf("%s %s lesson %s: %v", id, Describe(cfg), l.ID, err)
			}
			// A rule a step points at must exist in the rules this very
			// table prints, or "Read the rule" opens a screen with nothing
			// highlighted.
			ids := map[string]bool{}
			for _, sec := range module.RulesFor(m, cfg) {
				for _, it := range sec.Items {
					ids[it.ID] = true
				}
			}
			for _, s := range l.Steps {
				for _, rid := range s.RuleIDs {
					if !ids[rid] {
						t.Errorf("%s %s lesson %s step %s: rule %q is not in this table's rules", id, Describe(cfg), l.ID, s.ID, rid)
					}
				}
			}
		}
	}
}

// Describe names a configuration for a failure message.
func Describe(cfg module.MatchConfig) string {
	keys := make([]string, 0, len(cfg.Options))
	for k := range cfg.Options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := "variation=" + cfg.Variation
	for _, k := range keys {
		s += fmt.Sprintf(" %s=%d", k, cfg.Options[k])
	}
	return "[" + s + "]"
}

// Walk plays one lesson to its last step.
func Walk(m module.GameModule, cfg module.MatchConfig, l module.Lesson) error {
	refs := []module.PlayerRef{{ID: human, Name: "You"}}
	for i := 0; i < l.Bots; i++ {
		refs = append(refs, module.PlayerRef{ID: fmt.Sprintf("bot%d", i+1), Name: fmt.Sprintf("Bot %d", i+1), IsAI: true})
	}
	d := m.Descriptor()
	min, max := d.SeatRange(cfg.Variation)
	if len(refs) < min || len(refs) > max {
		return fmt.Errorf("the lesson seats %d, the table takes %d to %d", len(refs), min, max)
	}
	state, err := m.NewMatch(cfg, refs, l.Seed)
	if err != nil {
		return fmt.Errorf("dealing: %w", err)
	}

	bot := module.BotFor(m)
	idx, done := 0, 0
	for moves := 0; moves < maxMoves; moves++ {
		for idx < len(l.Steps) && l.Steps[idx].Kind == module.StepSay {
			idx++
		}
		if idx >= len(l.Steps) {
			return nil
		}
		step := l.Steps[idx]

		over, _, err := m.Finished(state)
		if err != nil {
			return fmt.Errorf("finished: %w", err)
		}
		if over {
			return fmt.Errorf("the match ended before step %q could be done", step.ID)
		}

		awaited := module.AwaitedSeats(m, state, human, refs)
		if len(awaited) == 0 {
			return fmt.Errorf("step %q: nobody is on turn", step.ID)
		}
		seat := awaited[0]
		for _, a := range awaited {
			if a == human {
				seat = human
			}
		}

		offers, err := m.LegalActions(state, seat)
		if err != nil {
			return fmt.Errorf("legal actions for %s: %w", seat, err)
		}

		if seat != human {
			next, err := botMove(m, bot, state, seat, offers, l.Seed)
			if err != nil {
				return fmt.Errorf("step %q: %w", step.ID, err)
			}
			state = next
			continue
		}

		// The lesson's player: the moves the step asks for if there are any,
		// and on a loose step whatever gets the turn moving otherwise.
		wanted := wantedActions(offers, step)
		if len(wanted) == 0 {
			if step.Strict {
				return fmt.Errorf("step %q is strict and waits for %v, but the player was offered %s",
					step.ID, step.Verbs, offerVerbs(offers))
			}
			next, err := botMove(m, bot, state, human, offers, l.Seed)
			if err != nil {
				return fmt.Errorf("step %q (finding any move): %w", step.ID, err)
			}
			state = next
			continue
		}
		var played bool
		for _, a := range wanted {
			next, _, err := m.Apply(state, human, a)
			if err != nil {
				continue
			}
			state, played = next, true
			done++
			break
		}
		if !played {
			return fmt.Errorf("step %q: every move the offers described was refused", step.ID)
		}
		if done >= step.Need() {
			idx++
			done = 0
		}
	}
	return fmt.Errorf("gave up after %d moves, still on step %d", maxMoves, idx)
}

func wantedActions(offers []module.ActionOffer, step module.LessonStep) []module.Action {
	var out []module.Action
	for _, a := range module.ChooseActions(offers, step.Verbs) {
		if step.Takes(a.Verb) {
			out = append(out, a)
		}
	}
	return out
}

func offerVerbs(offers []module.ActionOffer) string {
	var vs []string
	for _, o := range offers {
		if o.Live() {
			vs = append(vs, o.Verb)
		}
	}
	return fmt.Sprint(vs)
}

// botMove plays one move for a seat: the module's bot first, then anything
// the offers describe, as the runtime's own bot loop does.
func botMove(m module.GameModule, bot module.Bot, state module.State, seat string, offers []module.ActionOffer, seed int64) (module.State, error) {
	var tries []module.Action
	if a, ok := bot.Act(state, module.BotSeat{PlayerID: seat, Skill: module.SkillEasy, Seed: module.SeatSeed(seed, seat, "lesson")}, offers); ok {
		tries = append(tries, a)
	}
	tries = append(tries, module.ChooseActions(offers, nil)...)
	for _, a := range tries {
		if next, _, err := m.Apply(state, seat, a); err == nil {
			return next, nil
		}
	}
	return nil, fmt.Errorf("%s has no move the engine accepts (offered %s)", seat, offerVerbs(offers))
}

// Keys is every locale key a module's lessons can show, across every
// configuration worth teaching on, sorted and without repeats: titles,
// summaries, step texts and the words for a held-back move. The client checks
// that each has wording in every language, because a lesson card that falls
// back to a bare key is a guide that cannot be followed.
func Keys(m module.GameModule) ([]string, error) {
	seen := map[string]bool{}
	for _, cfg := range Configs(m) {
		lessons, ok, err := module.LessonsFor(m, cfg)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", m.Descriptor().ID, Describe(cfg), err)
		}
		if !ok {
			return nil, nil
		}
		for _, l := range lessons {
			for _, k := range []string{l.TitleKey, l.SummaryKey} {
				if k != "" {
					seen[k] = true
				}
			}
			for _, s := range l.Steps {
				for _, k := range []string{s.TextKey, s.WrongKey} {
					if k != "" {
						seen[k] = true
					}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
