package models

// Cosmetic identity, on the way in.
//
// It lives beside the fields it describes — Player.Avatar and
// UserPreferences.Avatar — rather than in whichever package first needed it,
// because two doors lead to the same seat: taking one directly, and being
// picked up out of the waiting room. Both accept a slug from a client, and
// they have to agree about what one is.
//
// The face a player wears is chosen on the client and broadcast to everyone
// else at the table, which makes it the one piece of a seat that a client
// authors and other clients render. So it is checked here — not against a list
// of faces, which the server has no business knowing, but against a shape.
//
// Refusing is deliberately not an error. A slug this server does not like
// becomes no slug at all, and every client then derives a face from the player
// id in exactly the same way. The seat is never blank, the request is never
// rejected over decoration, and the roster stays a client concern that can
// grow without a server release.

// avatarMaxLen bounds a slug at rather more than any name the client ships,
// which is the point: it is a ceiling, not a fit.
const avatarMaxLen = 24

// SanitizeAvatar keeps a cosmetic slug to the shape every client agrees on —
// lowercase letters, digits and hyphens — and returns "" for anything else.
func SanitizeAvatar(s string) string {
	if s == "" || len(s) > avatarMaxLen {
		return ""
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return ""
		}
	}
	return s
}
