package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// A guest key is what a device holds to prove it *is* a guest, as opposed to
// having merely seen one.
//
// The guest id cannot be that proof. It is the seat subject, so every table
// the guest sits at broadcasts it in players[].id, and anyone who has played
// against them knows it. Resuming a guest on the strength of the id alone let
// any opponent mint a session as them.
//
// A refresh token cannot be it either: it rotates on every refresh and is
// deleted on sign-out, and a guest identity is meant to outlive both — that
// is the whole of "play for a week, then sign in and keep your games".
//
// So the key is the id plus a MAC over it under the server's access secret:
// stateless, never rotated, and small enough to be carried in a link. Whoever
// holds it is the guest, on any device they open it on. It grants nothing a
// guest session does not, and becomes worthless once the guest is claimed —
// the history has moved to an account by then. Rotating the access secret
// invalidates every key, which turns returning guests into new ones; that is
// the price of keeping nothing on the server.
const guestKeySep = "."

// GuestKeyFor is the key that proves possession of guestID.
func GuestKeyFor(guestID string) string {
	return guestID + guestKeySep + guestKeyMAC(guestID)
}

// GuestIDFromKey returns the guest id a key proves, or "" for anything that is
// not a key this server issued.
func GuestIDFromKey(key string) string {
	id, mac, ok := strings.Cut(strings.TrimSpace(key), guestKeySep)
	if !ok || sanitizeGuestID(id) == "" {
		return ""
	}
	if !hmac.Equal([]byte(mac), []byte(guestKeyMAC(id))) {
		return ""
	}
	return id
}

func guestKeyMAC(guestID string) string {
	// Domain-separated, so no other use of the access secret can be made to
	// produce a guest key.
	m := hmac.New(sha256.New, []byte("zolik guest key v1\x00"+accessSecret()))
	m.Write([]byte(guestID))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
