package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

// The SSH terminal client is an optional door onto the game, and the server
// that hosts it keeps running when the door will not open (see
// server/cmd/server/main.go). That only holds if a host key it cannot write
// comes back as an *error* rather than taking the process with it — which is
// what this pins, because the failure it guards against was found in
// production: a container whose unprivileged user could not create the
// default key directory crash-looped instead of serving the web players who
// were never using SSH in the first place.
func TestEnsureHostKeyReportsAnUnwritableDirectory(t *testing.T) {
	base := t.TempDir()
	locked := filepath.Join(base, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatalf("preparing an unwritable directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	if os.Geteuid() == 0 {
		t.Skip("running as root, which can write anywhere")
	}

	if err := ensureHostKey(filepath.Join(locked, "sub", "host_key")); err == nil {
		t.Error("ensureHostKey reported success for a directory it cannot create")
	}
}

func TestEnsureHostKeyRefusesAnEmptyPath(t *testing.T) {
	if err := ensureHostKey(""); err == nil {
		t.Error("ensureHostKey accepted an empty path")
	}
}

func TestEnsureHostKeyCreatesTheDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "host_key")
	if err := ensureHostKey(path); err != nil {
		t.Fatalf("ensureHostKey: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("the key's directory was not created: %v", err)
	}
}
