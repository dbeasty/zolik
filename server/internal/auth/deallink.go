package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// A deal link is how a finished game of solitaire is sent to somebody else to
// play: the same cards, at a table of their own (docs/solitaire-plan.md §4b).
//
// What it carries is the match the deal was played at, and that is exactly
// what must not be readable. A finished match's board is visible to anybody
// who has its id, and that board shows every card its player turned up — so a
// link that named the match would let the person it was sent to look at the
// deal before playing it. The id is therefore sealed, not merely signed:
// encrypted and authenticated under a key derived from the server's access
// secret. The link reveals nothing, cannot be forged, and needs no storage.
//
// Like a guest key, it is worthless once the access secret rotates. A deal
// link is an invitation, not an archive, so that is an acceptable cost.

func dealLinkAEAD() (cipher.AEAD, error) {
	// Domain-separated, so no other use of the access secret yields this key.
	key := sha256.Sum256([]byte("zolik deal link v1\x00" + accessSecret()))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// SealDealLink is the token for a deal played at matchHex, or "" when matchHex
// is not a match id. A fresh nonce each time, so two links to one deal differ.
func SealDealLink(matchHex string) string {
	id, err := hex.DecodeString(matchHex)
	if err != nil || len(id) != 12 {
		return ""
	}
	aead, err := dealLinkAEAD()
	if err != nil {
		return ""
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, id, nil))
}

// OpenDealLink returns the match a token was sealed for, or "" for anything
// this server did not issue.
func OpenDealLink(token string) string {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	aead, err := dealLinkAEAD()
	if err != nil || len(raw) < aead.NonceSize() {
		return ""
	}
	id, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil || len(id) != 12 {
		return ""
	}
	return hex.EncodeToString(id)
}
