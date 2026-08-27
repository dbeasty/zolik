package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The committed manifest must still say what the server actually emits.
//
// This is the server half of a two-sided lock. The client half asserts every
// key in the file has wording in every locale; without this half the file
// could simply go stale, and a fully-worded manifest describing a server two
// releases old would pass both tests while a player read SCREAMING_SNAKE.
func TestManifestIsCurrent(t *testing.T) {
	root := serverRoot(t)
	golden := filepath.Join(root, "..", "client-react-native", "src", "lib", "serverKeys.json")

	committed, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v", golden, err)
	}

	cmd := exec.Command("go", "run", "./cmd/dump-keys")
	cmd.Dir = root
	fresh, err := cmd.Output()
	if err != nil {
		t.Fatalf("go run ./cmd/dump-keys: %v", err)
	}

	var a, b any
	if err := json.Unmarshal(committed, &a); err != nil {
		t.Fatalf("committed manifest is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(fresh, &b); err != nil {
		t.Fatalf("generated manifest is not valid JSON: %v", err)
	}
	if !jsonEqual(a, b) {
		t.Errorf("serverKeys.json is out of date. Regenerate it:\n\n" +
			"    cd server && go run ./cmd/dump-keys > ../client-react-native/src/lib/serverKeys.json\n\n" +
			"then word any new key in client-react-native/src/lib/i18n.ts.")
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func serverRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not find the server module root")
	return ""
}
