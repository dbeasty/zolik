package metrics

import "strings"

// Every counter this server records. They live in one file, as constants, and
// TestCounterNamesAreLiterals fails the build if an Add call anywhere passes
// something that is not one of them.
//
// The house learned this with server message keys: a key passed through a
// variable dropped silently out of the generated manifest, because the tool
// that builds it reads the source rather than running it. A metric name
// assembled at run time is the same bug with a quieter symptom — not a failing
// CI check, just a column that never appears on the operator's screen, for a
// number nobody realises is missing.
const (
	// Match lifecycle. Created and Started are separate because a lobby that
	// never fills is not a game anybody failed to finish, and lumping the two
	// together makes the completion rate meaningless.
	MatchesCreated   = "matches.created"
	MatchesStarted   = "matches.started"
	MatchesCompleted = "matches.completed"
	MatchesAbandoned = "matches.abandoned"
	// MatchesResumed counts abandoned tables a player brought back. It is a
	// correction rather than a lifecycle event of its own: the table really
	// was abandoned when the sweeper said so, and that line stays in the log,
	// but a game that got played out afterwards is not one anybody failed to
	// finish. completionRate subtracts these from the abandoned side for
	// exactly that reason.
	MatchesResumed = "matches.resumed"

	// UsersRegistered counts accounts, not people arriving: a guest who plays
	// for a month and never signs up is in SessionsGuest and in the day's
	// player set, and is deliberately not here.
	UsersRegistered = "users.registered"
	SessionsGuest   = "sessions.guest"

	WSConnected = "ws.connected"

	// AdmissionRefusedMatchStart is a player told "not now" before a socket
	// was ever opened, which no WebSocket-level counter would see.
	AdmissionRefusedMatchStart = "admission.refused.matchstart"

	// BootsTotal counts process starts; BootsUnclean counts the ones that
	// found the previous process's row still open. Named with the plural so
	// neither collides with the Boot row they are derived from — a counter
	// and the record it counts are different things and should not share a
	// name.
	BootsTotal   = "boot"
	BootsUnclean = "boot.unclean"

	// The console's own door. A failed sign-in against a console that is
	// supposed to be unreachable from the internet is the single most
	// interesting line this server can log, so it is counted as well.
	AdminLoginOK     = "admin.login.ok"
	AdminLoginDenied = "admin.login.denied"
)

// Prefixes for the two families whose last segment is not known at compile
// time: the module a match belongs to, and the reason an admission was
// refused. Both are closed sets in practice — the module registry and
// admission.Reason — but neither is a constant here, so the linter test
// admits a name built from one of these prefixes and nothing else.
const (
	MatchesCompletedPrefix = "matches.completed."
	AdmissionRefusedPrefix = "admission.refused."
	MatchesAbandonedPrefix = "matches.abandoned."
	MatchesResumedPrefix   = "matches.resumed."
)

// MatchesCompletedFor names the per-module completion counter.
func MatchesCompletedFor(moduleID string) string {
	return MatchesCompletedPrefix + sanitise(moduleID)
}

// MatchesAbandonedFor names the per-module abandonment counter.
func MatchesAbandonedFor(moduleID string) string {
	return MatchesAbandonedPrefix + sanitise(moduleID)
}

// MatchesResumedFor names the per-module resumption counter.
func MatchesResumedFor(moduleID string) string {
	return MatchesResumedPrefix + sanitise(moduleID)
}

// AdmissionRefusedFor names the per-reason refusal counter.
func AdmissionRefusedFor(reason string) string {
	return AdmissionRefusedPrefix + sanitise(reason)
}

// sanitise keeps a name from a run-time value safe to use as a map key and to
// render. A dot would fake a level of hierarchy that is not there, and an
// empty segment would produce a trailing dot that reads as truncation, so both
// are replaced rather than left to look like structure.
func sanitise(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, s)
}
