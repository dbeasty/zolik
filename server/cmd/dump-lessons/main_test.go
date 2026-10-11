package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The committed key list must still say what the lessons actually show. The
// client half asserts each listed key has wording in every locale; without
// this half the file could go stale and a fully-worded list describing last
// month's lessons would pass while a card read as a bare key.
func TestLessonKeysAreCurrent(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "..", "..")
	golden := filepath.Join(root, "..", "client-react-native", "src", "guide", "lessonKeys.json")

	committed, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v", golden, err)
	}
	cmd := exec.Command("go", "run", "./cmd/dump-lessons")
	cmd.Dir = root
	fresh, err := cmd.Output()
	if err != nil {
		t.Fatalf("go run ./cmd/dump-lessons: %v", err)
	}
	var a, b []string
	if err := json.Unmarshal(committed, &a); err != nil {
		t.Fatalf("committed list is not valid JSON: %v", err)
	}
	if err := json.Unmarshal(fresh, &b); err != nil {
		t.Fatalf("generated list is not valid JSON: %v", err)
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if !bytes.Equal(x, y) {
		t.Errorf("lessonKeys.json is out of date. Regenerate it:\n\n" +
			"    cd server && go run ./cmd/dump-lessons > ../client-react-native/src/guide/lessonKeys.json\n\n" +
			"then word any new key in client-react-native/src/lib/locales/guide.")
	}
}
