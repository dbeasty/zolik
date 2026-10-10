package auth

import (
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"
)

// MaxPasswordBytes is the longest password accepted. A cap, not a policy: it
// keeps a request from making the server hash a megabyte.
const MaxPasswordBytes = 1024

// bcryptLimit is how much of a password bcrypt reads. Go's bcrypt refuses
// anything longer rather than silently ignoring the tail, which is the right
// behaviour for the library and was a 500 for a person with a long passphrase.
const bcryptLimit = 72

// bcryptInput is what is actually handed to bcrypt.
//
// A password of up to 72 bytes goes in as it is, so every hash made before this
// existed still verifies. A longer one — which could never have been registered
// — is reduced to a fixed-length digest first, so the whole passphrase counts
// and not its first 72 bytes.
func bcryptInput(password string) []byte {
	if len(password) <= bcryptLimit {
		return []byte(password)
	}
	sum := sha256.Sum256([]byte(password))
	return []byte(base64.StdEncoding.EncodeToString(sum[:]))
}

// hashPassword is the one place a password becomes a stored hash.
func hashPassword(password string, cost int) (string, error) {
	h, err := bcrypt.GenerateFromPassword(bcryptInput(password), cost)
	return string(h), err
}

// passwordMatches is the one place a password is checked against a stored hash.
func passwordMatches(hash, password string) bool {
	if len(password) > MaxPasswordBytes {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), bcryptInput(password)) == nil
}
