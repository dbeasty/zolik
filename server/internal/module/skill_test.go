package module

import "testing"

// The option is int-keyed on the wire and string-keyed everywhere it is
// stored, and the two have to agree or a lobby's choice silently becomes a
// different opponent.
func TestSkillOptionRoundTrips(t *testing.T) {
	for _, s := range Skills {
		got, auto := ParseSkillOpt(SkillOpt(s))
		if auto || got != s {
			t.Errorf("%s round-tripped to (%q, auto=%v)", s, got, auto)
		}
	}
}

// Zero is Mixed, not Easy. That distinction is the whole reason the values
// start at one: a stored match that never set the option, and a client too old
// to send it, both read as "no preference" — and silently seating the weakest
// opponent instead would be a change nobody asked for.
func TestUnsetOptionMeansMixedNotEasy(t *testing.T) {
	if _, auto := ParseSkillOpt(BotSkillAuto); !auto {
		t.Error("value 0 is not Mixed")
	}
	if _, auto := ParseSkillOpt(99); !auto {
		t.Error("an out-of-range value must fall back to Mixed rather than a skill")
	}
	if s, auto := ParseSkill(""); auto == false || s != "" {
		t.Errorf(`ParseSkill("") = (%q, %v), want a Mixed answer`, s, auto)
	}
}

// Mixed is answered per seat, which is the only way two bots at one table can
// differ — and the answer has to be stable, or a bot loop restarted after a
// reconnect would seat a different opponent mid-deal.
func TestMixedDrawsPerSeatAndRepeats(t *testing.T) {
	seen := map[Skill]bool{}
	for seat := 0; seat < 40; seat++ {
		seed := SeatSeed(12345, string(rune('a'+seat)), "seat")
		got := ResolveSkill("", true, seed)
		if !got.Valid() {
			t.Fatalf("Mixed produced %q, which is not a skill", got)
		}
		if again := ResolveSkill("", true, seed); again != got {
			t.Fatalf("the same seed drew %q then %q", got, again)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Errorf("forty seats drew only %v — that is not a mixed table", seen)
	}
}

// A named skill is never overridden by Mixed, however the option was set.
func TestAnExplicitSkillWins(t *testing.T) {
	if got := ResolveSkill(SkillHard, false, 1); got != SkillHard {
		t.Errorf("asked for hard, got %q", got)
	}
}

// Two Master Miroslavs at one table would be two seats sharing one name and
// one lifetime record.
func TestPersonasAreNotSeatedTwice(t *testing.T) {
	taken := map[string]bool{}
	roster := PersonasFor(SkillHard)
	for i := 0; i < len(roster); i++ {
		p := PickPersona(SkillHard, taken, int64(i*7919))
		if taken[p.Key()] {
			t.Fatalf("%s was seated twice with %d names still free", p.Name, len(roster)-i)
		}
		if p.Skill != SkillHard {
			t.Fatalf("%s is a %s persona, seated as hard", p.Name, p.Skill)
		}
		taken[p.Key()] = true
	}
	// More bots than names is a cosmetic problem; refusing to seat one is a
	// real one, so the roster is reused rather than exhausted.
	if p := PickPersona(SkillHard, taken, 1); p.Name == "" {
		t.Error("an exhausted roster produced no persona at all")
	}
}

// The persona key is the stats subject id, and the skill has to be readable
// back off it — that is what keeps "how do I do against hard?" working when
// the record is keyed by "hard:miroslav".
func TestPersonaKeyLeadsWithItsSkill(t *testing.T) {
	for _, s := range Skills {
		for _, p := range PersonasFor(s) {
			if got, ok := PersonaByKey(p.Key()); !ok || got.Slug != p.Slug {
				t.Errorf("%s did not resolve back from its own key %q", p.Name, p.Key())
			}
			if want := string(s) + ":"; p.Key()[:len(want)] != want {
				t.Errorf("key %q does not lead with %q", p.Key(), want)
			}
		}
	}
}

// Every persona slug must be unique across the whole roster, because the slug
// plus the skill is a durable record that outlives any one match.
func TestPersonaSlugsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range personas {
		if seen[p.Key()] {
			t.Errorf("duplicate persona key %q", p.Key())
		}
		seen[p.Key()] = true
	}
}
