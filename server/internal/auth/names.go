package auth

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// How long a name a person may be known by. The client's own inputs stop at 24,
// so this is a ceiling for a caller that is not the client, not a limit anyone
// meets by typing.
const (
	MaxGuestNameRunes = 24
	MaxUsernameRunes  = 24
)

// CleanDisplayName makes a name safe to put on other people's screens.
//
// A name is not just a label: it is signed into the token, listed in the
// lobby and drawn at every table the person sits down at. Taken as sent it let
// through a name of ten thousand characters (the token and every table's seat
// list carry it), a name of only spaces or zero-width characters (a seat with
// nothing written on it), a NUL, and a right-to-left override — U+202E reverses
// what follows it, so one account could be shown to everyone else as another.
//
// So: invalid UTF-8 is dropped; control, line and paragraph separators and
// invisible formatting characters are dropped (the zero-width joiner and
// non-joiner are kept, because emoji sequences and Persian and Indic scripts
// are spelled with them); every other kind of whitespace becomes one space;
// runs of spaces collapse; the ends are trimmed; and the result is cut to max
// runes. The empty string is a possible answer, and the caller decides what a
// nameless person is called.
func CleanDisplayName(s string, max int) string {
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	b.Grow(len(s))
	runes := 0
	space := false
	for _, r := range s {
		switch {
		case r == '‌' || r == '‍':
			// Part of how some names are written; invisible only on its own,
			// and a name of nothing else is caught by the visibility check.
		case unicode.IsSpace(r) || unicode.Is(unicode.Zs, r):
			space = true
			continue
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), unicode.Is(unicode.Zl, r), unicode.Is(unicode.Zp, r):
			continue
		}
		if space && b.Len() > 0 {
			if runes >= max {
				break
			}
			b.WriteByte(' ')
			runes++
		}
		space = false
		if runes >= max {
			break
		}
		b.WriteRune(r)
		runes++
	}
	out := strings.TrimSpace(b.String())
	if !hasVisibleRune(out) {
		return ""
	}
	return out
}

// hasVisibleRune is whether anything in s would show: a name made only of
// joiners, combining marks or spaces is a blank seat.
func hasVisibleRune(s string) bool {
	for _, r := range s {
		if r == '‌' || r == '‍' || unicode.IsSpace(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
			continue
		}
		if r != utf8.RuneError {
			return true
		}
	}
	return false
}
