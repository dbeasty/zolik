package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// AnonymousGuestID is the guest id a seat goes under once nobody may be named
// in it: a player who did not agree to a local match going to the cloud, or
// an account that was deleted. It is derived rather than random, so saying it
// twice says the same thing, and it has the shape of a guest id so it is
// stored like any other. But no device holds a receipt for it, so no account
// can ever claim it.
func AnonymousGuestID(matchHex, seat string) string {
	sum := sha256.Sum256([]byte("zolik:anonymous-seat:" + matchHex + "/" + seat))
	return hex.EncodeToString(sum[:16])
}

// AnonymousSeatName is what an anonymous seat is called, from its index in
// turn order: "Player 1" for the first seat. The match record and the result
// derived from it both use this, so a replay and a history row agree.
func AnonymousSeatName(seat int) string {
	return "Player " + strconv.Itoa(seat+1)
}
