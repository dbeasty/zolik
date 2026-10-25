package module

import "fmt"

// Lessons: the interactive guide that teaches a game, one hands-on step at a
// time, on a real table with real opponents.
//
// A lesson is *data the module owns*, built from the same resolved config its
// Rules() and NewMatch use. That is the whole variation story: a Samba lesson
// differs from a Classic one because Lessons(cfg) leaves a step in or out, or
// fills a number from the config, not because a client knows which game it is
// teaching. Like every other Fact, a step carries a key and params — never
// prose — so the client keeps every word.
//
// What a lesson does NOT do is play the game. The table is an ordinary match
// dealt from the lesson's seed, the opponents are ordinary bots, and the
// engine decides every legality question. A lesson only says which of the
// player's own verbs a step is waiting for, so the guide can point at it and,
// on a strict step, hold back the ones that would derail the walk-through. A
// replay test (internal/lessontest) drives every lesson through the real
// engine for every shipped variation and option, so a lesson cannot quietly
// drift from the rules.

// Step kinds.
const (
	// StepSay is a card to read; the player presses Next.
	StepSay = "say"
	// StepAct waits for the player to make Count moves whose verb is in Verbs.
	StepAct = "act"
)

// Anchors name the parts of the match screen a step can point at. The client
// maps each to whatever it draws there; an anchor a screen does not have is
// ignored, so a lesson never fails because a layout moved.
const (
	AnchorHand      = "hand"
	AnchorDrawPile  = "drawPile"
	AnchorDiscard   = "discard"
	AnchorTable     = "table"
	AnchorActionBar = "actionBar"
	AnchorStatus    = "status"
	AnchorSeats     = "seats"
	AnchorScore     = "score"
	AnchorHint      = "hint"
	AnchorLog       = "log"
	AnchorRules     = "rules"
)

// Lesson is one guided walk-through of a game at a table configured by cfg.
type Lesson struct {
	ID string `json:"id"`
	// TitleKey and SummaryKey name the lesson in the list that offers it.
	TitleKey   string `json:"titleKey"`
	SummaryKey string `json:"summaryKey,omitempty"`
	// Seed deals the table. Chosen so the deal shows what the lesson teaches,
	// and checked by the replay test, which fails if the steps cannot be
	// reached from it.
	Seed int64 `json:"seed"`
	// Bots is how many opponents sit down with the player.
	Bots  int          `json:"bots"`
	Steps []LessonStep `json:"steps"`
}

// LessonStep is one card in the guide.
type LessonStep struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// TextKey is the sentence shown, with Params resolved against the config
	// in force.
	TextKey string         `json:"textKey"`
	Params  map[string]any `json:"params,omitempty"`
	// Anchors are what to point at, most important first.
	Anchors []string `json:"anchors,omitempty"`
	// RuleIDs are the written rules this step is about, for "read the rule".
	RuleIDs []string `json:"ruleIds,omitempty"`

	// Verbs, Count and Strict only mean something for StepAct. The step is
	// done once Count (default 1) moves with one of these verbs have been
	// made. Strict holds back every other move meanwhile and answers a
	// refused one with WrongKey; a loose step lets the player do anything
	// and simply waits.
	Verbs    []string `json:"verbs,omitempty"`
	Count    int      `json:"count,omitempty"`
	Strict   bool     `json:"strict,omitempty"`
	WrongKey string   `json:"wrongKey,omitempty"`
}

// LessonProvider is implemented by a module that ships guided lessons.
type LessonProvider interface {
	// Lessons returns the lessons for the table cfg describes, in the order
	// they should be offered. cfg is already resolved the way Rules(cfg) and
	// NewMatch(cfg) resolve it.
	Lessons(cfg MatchConfig) ([]Lesson, error)
}

// LessonsFor asks a module for its lessons. The second result is false for a
// module that ships none, which is a normal answer rather than an error.
func LessonsFor(m GameModule, cfg MatchConfig) ([]Lesson, bool, error) {
	lp, ok := m.(LessonProvider)
	if !ok {
		return nil, false, nil
	}
	ls, err := lp.Lessons(cfg)
	return ls, true, err
}

// FindLesson returns the lesson with this id for cfg.
func FindLesson(m GameModule, cfg MatchConfig, id string) (Lesson, error) {
	ls, ok, err := LessonsFor(m, cfg)
	if err != nil {
		return Lesson{}, err
	}
	if !ok {
		return Lesson{}, Error{Code: "NO_LESSONS", Message: m.Descriptor().ID}
	}
	for _, l := range ls {
		if l.ID == id {
			return l, nil
		}
	}
	return Lesson{}, Error{Code: "UNKNOWN_LESSON", Message: id}
}

// Validate reports a lesson that cannot be played, before any table is dealt:
// the mistakes an author can make without running the engine.
func (l Lesson) Validate() error {
	if l.ID == "" || l.TitleKey == "" {
		return fmt.Errorf("lesson needs an id and a title key")
	}
	if len(l.Steps) == 0 {
		return fmt.Errorf("lesson %s has no steps", l.ID)
	}
	seen := map[string]bool{}
	for i, s := range l.Steps {
		if s.ID == "" || seen[s.ID] {
			return fmt.Errorf("lesson %s step %d: id %q is empty or repeated", l.ID, i, s.ID)
		}
		seen[s.ID] = true
		if s.TextKey == "" {
			return fmt.Errorf("lesson %s step %s: no text key", l.ID, s.ID)
		}
		switch s.Kind {
		case StepSay:
			if len(s.Verbs) > 0 || s.Strict {
				return fmt.Errorf("lesson %s step %s: a say step waits for nothing", l.ID, s.ID)
			}
		case StepAct:
			if len(s.Verbs) == 0 {
				return fmt.Errorf("lesson %s step %s: an act step names no verbs", l.ID, s.ID)
			}
			if s.Strict && s.WrongKey == "" {
				return fmt.Errorf("lesson %s step %s: a strict step needs a wrong-move key", l.ID, s.ID)
			}
		default:
			return fmt.Errorf("lesson %s step %s: unknown kind %q", l.ID, s.ID, s.Kind)
		}
	}
	return nil
}

// Need is how many moves an act step waits for.
func (s LessonStep) Need() int {
	if s.Count > 0 {
		return s.Count
	}
	return 1
}

// Takes reports whether a move with this verb counts towards the step.
func (s LessonStep) Takes(verb string) bool {
	for _, v := range s.Verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// Say builds a read-and-continue step.
func Say(id, textKey string, params map[string]any, anchors ...string) LessonStep {
	return LessonStep{ID: id, Kind: StepSay, TextKey: textKey, Params: params, Anchors: anchors}
}

// Do builds a step that waits for the player to make a move with one of verbs.
func Do(id, textKey string, params map[string]any, verbs []string, anchors ...string) LessonStep {
	return LessonStep{ID: id, Kind: StepAct, TextKey: textKey, Params: params, Verbs: verbs, Anchors: anchors}
}

// Held makes an act step strict: other moves are held back and answered with
// wrongKey.
func (s LessonStep) Held(wrongKey string) LessonStep {
	s.Strict, s.WrongKey = true, wrongKey
	return s
}

// Times makes an act step wait for n moves.
func (s LessonStep) Times(n int) LessonStep { s.Count = n; return s }

// About attaches the written rules a step is about.
func (s LessonStep) About(ruleIDs ...string) LessonStep { s.RuleIDs = ruleIDs; return s }
