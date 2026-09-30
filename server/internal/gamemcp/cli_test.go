package gamemcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zolik/server/internal/gamemcp/tsbundle"
)

// A script of JSON lines is answered line for line, tables persist across
// lines, and a bad line is an error reply rather than the end of the run.
func TestCLIScript(t *testing.T) {
	script := strings.Join([]string{
		`# a comment, and a blank line`,
		``,
		`{"id":1,"tool":"list_games"}`,
		`{"id":2,"tool":"new_table","args":{"game":"zolik","seats":2,"claude_seats":[0],"opponents":["hard"],"seed":3}}`,
		`{"id":3,"tool":"observe","args":{"table":"t1","seat":0}}`,
		`{"id":4,"tool":"play","args":{"table":"t1","seat":0,"move":0}}`,
		`{"id":5,"tool":"play","args":{"table":"t1","seat":0,"move":999}}`,
		`not json`,
		`{"id":6,"tool":"rules","args":{"game":"zolik"}}`,
		`{"id":7,"tool":"replay_log","args":{"table":"t1"}}`,
		`{"id":8,"tool":"close_table","args":{"table":"t1"}}`,
		`{"id":9,"tool":"observe","args":{"table":"t1","seat":0}}`,
	}, "\n")
	var out bytes.Buffer
	if err := RunCLI(NewService(), strings.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	var replies []Reply
	sc := bufio.NewScanner(&out)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r Reply
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("reply is not one JSON line: %v\n%s", err, sc.Text())
		}
		replies = append(replies, r)
	}
	if len(replies) != 10 {
		t.Fatalf("%d replies for 10 commands", len(replies))
	}
	wantOK := []bool{true, true, true, true, false, false, true, true, true, false}
	for i, r := range replies {
		if r.OK != wantOK[i] {
			t.Fatalf("reply %d (%s): ok=%v error=%q", i, r.Tool, r.OK, r.Error)
		}
	}
	if !strings.Contains(replies[2].Text, "Your hand") || !strings.Contains(replies[6].Text, "## ") {
		t.Fatalf("texts:\n%s\n%s", replies[2].Text, replies[6].Text)
	}
	if replies[1].ID != float64(2) {
		t.Fatalf("id not echoed: %v", replies[1].ID)
	}
}

// `game-mcp call` keeps its tables in a file, so separate processes carry
// one game: here, separate CallOnce calls, each with a fresh Service.
func TestCallOncePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	r, err := CallOnce(path, "new_table", json.RawMessage(`{"game":"zolik","seats":2,"claude_seats":[0],"opponents":["hard"],"seed":3}`))
	if err != nil || !r.OK {
		t.Fatalf("new_table: %v %s", err, r.Error)
	}
	first := r.Result.(NewTableOut).Observation
	r, err = CallOnce(path, "observe", json.RawMessage(`{"table":"t1","seat":0}`))
	if err != nil || !r.OK {
		t.Fatalf("observe: %v %s", err, r.Error)
	}
	if r.Text != first.Text {
		t.Fatalf("the table came back different:\n%s\n---\n%s", first.Text, r.Text)
	}
	for i := 0; i < 5; i++ {
		r, err = CallOnce(path, "play", json.RawMessage(`{"table":"t1","seat":0,"move":0}`))
		if err != nil || !r.OK {
			t.Fatalf("play %d: %v %s", i, err, r.Error)
		}
	}
	r, _ = CallOnce(path, "replay_log", json.RawMessage(`{"table":"t1"}`))
	if n := len(r.Result.(ReplayOut).Entries); n < 10 {
		t.Fatalf("only %d actions after five plays", n)
	}
	r, _ = CallOnce(path, "new_table", json.RawMessage(`{"game":"holdem","seats":2,"claude_seats":[1],"seed":1}`))
	if r.Result.(NewTableOut).Table != "t2" {
		t.Fatalf("table ids restarted: %s", r.Result.(NewTableOut).Table)
	}
}

// The embedded English copy is the client's current bundle.
func TestMessagesMatchClientBundle(t *testing.T) {
	src, err := os.ReadFile("../../../client-react-native/src/lib/locales/en.ts")
	if err != nil {
		t.Skip("no client checkout beside the server:", err)
	}
	want, err := tsbundle.Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != len(messages) {
		t.Fatalf("en.ts has %d keys, messages_en.json %d: run go generate ./internal/gamemcp", len(want), len(messages))
	}
	for k, v := range want {
		if messages[k] != v {
			t.Fatalf("%s differs: run go generate ./internal/gamemcp", k)
		}
	}
}

func TestRender(t *testing.T) {
	n := namer{"p0": "claude"}
	for in, want := range map[string]string{"TD": "10♦", "JOKER1": "JK", "7S": "7♠", "p0": "claude", "zone.drawPile": label("zone.drawPile", nil)} {
		if got := n.token(in); got != want {
			t.Errorf("token(%q) = %q, want %q", in, got, want)
		}
	}
	if got := bySuit([]string{"7S", "6S", "TD", "JOKER2", "AH", "2H"}); got != "♠ 6 7 | ♥ 2 A | ♦ 10 | JK" {
		t.Errorf("bySuit = %q", got)
	}
	if got := humanise("holdem.seat.stack"); got != "Stack" {
		t.Errorf("humanise = %q", got)
	}
}
