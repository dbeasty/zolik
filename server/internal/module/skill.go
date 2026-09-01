package module

import (
	"hash/fnv"
	"math/rand"
)

// How good the opponents are, and who they are.
//
// Two things live here that look like one. The first is a *skill*: how well a
// seat nobody is sitting at plays, chosen by the host when the table is made.
// The second is a *persona*: which named opponent is sitting in that seat.
//
// They are separate because they answer different questions. Skill is what the
// host picks and what the module's bot reads. Persona is what the player sees
// and — because a persona is stable across every match ever played, unlike a
// bot instance id, which is fresh every lobby — it is what a lifetime record
// can be hung on. "How do I do against hard?" is a skill question. "Has
// Master Miroslav ever lost to me?" is a persona one, and until personas
// existed there was no way to ask it.

// Skill is how strongly a bot plays.
//
// A string rather than an int because it is what lands in
// models.Player.AIDifficulty and in the stats subject, both of which existed
// with these exact spellings before this type did — easy, medium and hard are
// the ids already in the database and they keep their meaning.
type Skill string

const (
	SkillEasy   Skill = "easy"
	SkillMedium Skill = "medium"
	SkillHard   Skill = "hard"
)

// There is no Expert here, and there was.
//
// A fourth level was built — perfect recall of the deal, outs counted against
// every unfinished pair and part-run, an endgame it reacted to a turn earlier
// than Hard does. It was then measured, which is the entire reason
// internal/ai/sim exists, and across three rulesets and two table sizes it
// could not be shown to beat Hard on either wins or penalty points. Shipping a
// level a player chooses *because* it says it is harder, which is not harder,
// is worse than shipping three levels that are honestly ordered — so the
// capabilities moved into Hard, where they measure neutral-to-positive, and
// the label did not ship.
//
// Adding a fourth later is one row in ai.profiles and one entry here. What it
// needs first is an idea that measures: on this evidence, more *perception*
// has run out of road, and a real Expert would have to search.

// Skills is every skill, weakest first. The order is meaningful: it is the
// order a picker renders, the order aiStats sorts, and the order the
// monotonicity gate in internal/ai asserts win rates increase along.
var Skills = []Skill{SkillEasy, SkillMedium, SkillHard}

// Rank orders a skill, weakest first, for sorting and for the strength gate.
// An unknown skill ranks past every known one rather than aliasing to easy.
func (s Skill) Rank() int {
	for i, k := range Skills {
		if k == s {
			return i
		}
	}
	return len(Skills)
}

// Valid reports whether s is a skill this build knows how to play at.
func (s Skill) Valid() bool { return s.Rank() < len(Skills) }

// OptBotSkill is how good the opponents at this table are.
//
// Declared here rather than in any game's own option list for the same reason
// OptPauseBetweenRounds is: how strong a bot plays is a property of how a match
// is *staffed*, which the runtime owns, and every module's engine option list
// stays about rules. A client renders the control off the descriptor without
// knowing which game it is configuring, exactly as it does for every other
// option.
const OptBotSkill = "botSkill"

// The option's values on the wire. Options are int-keyed (see Options), so the
// skills are numbered — and zero is Auto rather than a skill, so that a client
// or a stored match that never set the option reads as "no preference" instead
// of silently meaning "easy".
const (
	// BotSkillAuto asks for a mixed table: every seat gets its own skill,
	// drawn at seating. See ResolveSkill.
	BotSkillAuto = 0
	BotSkillEasy = 1
	// ...one past the last, in Skills order.
)

// SkillOpt encodes a skill as its option value.
func SkillOpt(s Skill) int {
	if !s.Valid() {
		return BotSkillAuto
	}
	return BotSkillEasy + s.Rank()
}

// ParseSkillOpt decodes an option value. The second result reports Auto, which
// is not a skill and cannot be one: it is a request to pick a different skill
// per seat, which only the seating code can honour.
func ParseSkillOpt(v int) (skill Skill, auto bool) {
	i := v - BotSkillEasy
	if i < 0 || i >= len(Skills) {
		return "", true
	}
	return Skills[i], false
}

// ParseSkill reads a skill written as a string — an API request, or the
// aiDifficulty already stored on a seat. Anything unrecognised is Auto, so a
// stale client cannot seat a bot that plays at a strength this build has never
// heard of.
func ParseSkill(s string) (Skill, bool) {
	for _, k := range Skills {
		if string(k) == s {
			return k, false
		}
	}
	return "", true
}

// BotSkillOption is the ready-made spec a module drops into its descriptor.
func BotSkillOption() OptionSpec {
	spec := OptionSpec{
		Name:  OptBotSkill,
		Type:  OptionEnumInt,
		Label: "Opponents",
		Help: "How well the players nobody is sitting at play. " +
			"Mixed deals each seat its own strength.",
		Choices: []OptionChoice{{Value: BotSkillAuto, Label: "Mixed"}},
	}
	for _, s := range Skills {
		spec.Choices = append(spec.Choices, OptionChoice{Value: SkillOpt(s), Label: skillLabels[s]})
	}
	return spec
}

var skillLabels = map[Skill]string{
	SkillEasy:   "Easy",
	SkillMedium: "Medium",
	SkillHard:   "Hard",
}

// BotSkill reads the lobby's choice, against the module's own default. The
// second result is Auto — see ParseSkillOpt.
func (c MatchConfig) BotSkill(dflt Skill) (Skill, bool) {
	return ParseSkillOpt(c.Opt(OptBotSkill, SkillOpt(dflt)))
}

// ResolveSkill decides what one seat actually plays at.
//
// The whole point of Auto is that it is answered *per seat* rather than once
// per table: a mixed table is the interesting one to play against, and it is
// the only setting under which two bots at the same table can differ. So the
// choice is made here, at seating, from a seed the caller derives from the
// match — which keeps a table reproducible (the same match seeded the same way
// seats the same opponents) without making every table identical.
func ResolveSkill(want Skill, auto bool, seed int64) Skill {
	if !auto && want.Valid() {
		return want
	}
	return Skills[rand.New(rand.NewSource(seed)).Intn(len(Skills))]
}

// Persona is a named opponent.
//
// Stable across matches, which is the entire reason it exists. A bot's
// per-lobby id is minted fresh every time one is seated, so a record keyed on
// it would be a new one-match player every game and never a usable number. A
// persona is the same identity every time it sits down, so it can carry a
// lifetime record the way an account does — and it gives a player someone to
// beat rather than a difficulty to beat.
//
// A persona's skill is fixed. Rookie Rita is not sometimes an expert: her
// record means something precisely because the strength behind it never moved.
type Persona struct {
	// Slug is the stable identity, unique across every skill. It is the half
	// of the stats subject id that survives a rename, so it must never change
	// for a persona that has already played — see stats.SubjectForPlayer.
	Slug string
	// Name is what a player sees at the table.
	Name string
	// Skill is how this persona plays, always.
	Skill Skill
}

// Key is the persona's durable identity, skill included.
//
// Skill-first so the key sorts into strength order and so the skill can be
// read back off a stats subject that carries nothing else — see
// stats.SkillOfSubjectID. Colon-separated because the stats subject key is
// already colon-separated and cuts on the *first* colon, which leaves this
// intact as the id.
func (p Persona) Key() string { return string(p.Skill) + ":" + p.Slug }

// personas is the whole roster, four to a skill.
//
// The names came from internal/ai, where they had sat unused since they were
// written: nothing ever read them, so every bot was called "Bot 4F". They are
// here rather than there because naming a seat nobody is sitting at is a
// runtime job — internal/ai is one game's brain, and Prší's bots want names
// too.
var personas = []Persona{
	{Slug: "rita", Name: "Rookie Rita", Skill: SkillEasy},
	{Slug: "lukas", Name: "Lucky Lukáš", Skill: SkillEasy},
	{Slug: "wanda", Name: "Wobbly Wanda", Skill: SkillEasy},
	{Slug: "stefan", Name: "Slow Štefan", Skill: SkillEasy},

	{Slug: "karel", Name: "Clever Karel", Skill: SkillMedium},
	{Slug: "sarka", Name: "Sharp Šárka", Skill: SkillMedium},
	{Slug: "stanislav", Name: "Steady Stanislav", Skill: SkillMedium},
	{Slug: "klara", Name: "Crafty Klára", Skill: SkillMedium},

	{Slug: "miroslav", Name: "Master Miroslav", Skill: SkillHard},
	{Slug: "sona", Name: "Shark Soňa", Skill: SkillHard},
	{Slug: "ivan", Name: "Iron Ivan", Skill: SkillHard},
	{Slug: "radka", Name: "Relentless Radka", Skill: SkillHard},
}

// PersonasFor lists the roster for one skill, in a fixed order.
func PersonasFor(s Skill) []Persona {
	var out []Persona
	for _, p := range personas {
		if p.Skill == s {
			out = append(out, p)
		}
	}
	return out
}

// PersonaByKey looks a persona up by the key stored on a seat.
func PersonaByKey(key string) (Persona, bool) {
	for _, p := range personas {
		if p.Key() == key {
			return p, true
		}
	}
	return Persona{}, false
}

// PickPersona chooses who sits down.
//
// taken is the keys already at this table: two Master Miroslavs would be two
// seats sharing one lifetime record and one name, so the roster is drawn from
// without replacement. When a skill's roster is exhausted — more bots than
// names, which needs a five-seat table of one strength — it falls back to
// reusing one, because a repeated name is a cosmetic problem and refusing to
// seat the bot is a real one.
func PickPersona(s Skill, taken map[string]bool, seed int64) Persona {
	roster := PersonasFor(s)
	if len(roster) == 0 {
		return Persona{Slug: "bot", Name: "Bot", Skill: s}
	}
	var free []Persona
	for _, p := range roster {
		if !taken[p.Key()] {
			free = append(free, p)
		}
	}
	if len(free) == 0 {
		free = roster
	}
	return free[rand.New(rand.NewSource(seed)).Intn(len(free))]
}

// TakenPersonas is the set of persona keys already seated, for PickPersona.
func TakenPersonas(keys []string) map[string]bool {
	out := map[string]bool{}
	for _, k := range keys {
		if k != "" {
			out[k] = true
		}
	}
	return out
}

// SeatSeed derives a stable per-seat seed from the match's own.
//
// Stable is the requirement: the same seat of the same match must draw the
// same skill, the same persona and the same tie-breaks every time it is
// evaluated, or a bot loop restarted after a reconnect would play a different
// opponent than the one before it. Hashing the id into the match seed gives
// that without any of it being stored.
func SeatSeed(matchSeed int64, id string, salt string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(salt))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(id))
	return matchSeed ^ int64(h.Sum64())
}
