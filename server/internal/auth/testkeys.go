package auth

import "crypto/ed25519"

// SnapshotKeysForTest captures this process's key state and returns a function
// that puts it back.
//
// The signing keyring and the node identity are process-wide, as a server's
// identity has to be. That makes them shared state between tests, and a test
// that installs a key and leaves it there quietly changes how every later
// test's tokens are signed - which is a failure that shows up somewhere else
// entirely, in a test that looks unrelated and was passing an hour ago.
//
// Exported because the tests that need it are in other packages: internal/app
// builds whole nodes with identities of their own.
func SnapshotKeysForTest() (restore func()) {
	keyring.Lock()
	signer, issuer, legacy := keyring.signer, keyring.issuer, keyring.legacyUntil
	verify := make(map[string][]byte, len(keyring.verify))
	for kid, pub := range keyring.verify {
		verify[kid] = append([]byte(nil), pub...)
	}
	keyring.Unlock()

	nodeIdentity.Lock()
	id, priv := nodeIdentity.id, nodeIdentity.priv
	nodeIdentity.Unlock()

	return func() {
		keyring.Lock()
		keyring.signer, keyring.issuer, keyring.legacyUntil = signer, issuer, legacy
		keyring.verify = make(map[string]ed25519.PublicKey, len(verify))
		for kid, pub := range verify {
			keyring.verify[kid] = pub
		}
		keyring.Unlock()

		nodeIdentity.Lock()
		nodeIdentity.id, nodeIdentity.priv = id, priv
		nodeIdentity.Unlock()
	}
}
