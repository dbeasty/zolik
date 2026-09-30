package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The binary itself, over real stdin and stdout: the test re-runs its own
// executable as main() (TestMain below), which is what .mcp.json starts.
func TestMain(m *testing.M) {
	if os.Getenv("GAME_MCP_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func self(t *testing.T, args ...string) *exec.Cmd {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "GAME_MCP_RUN_MAIN=1")
	return cmd
}

func TestStdioServer(t *testing.T) {
	cmd := self(t)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); _ = cmd.Wait() }()
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	id := 0
	call := func(method string, params any) map[string]any {
		id++
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if _, err := stdin.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
		for sc.Scan() {
			var msg map[string]any
			if err := json.Unmarshal(sc.Bytes(), &msg); err != nil {
				t.Fatalf("stdout carried a non-JSON line: %s", sc.Text())
			}
			if msg["id"] == nil {
				continue
			}
			if msg["error"] != nil {
				t.Fatalf("%s: %v", method, msg["error"])
			}
			return msg["result"].(map[string]any)
		}
		t.Fatalf("%s: no reply", method)
		return nil
	}
	init := call("initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "0"}})
	if init["serverInfo"] == nil {
		t.Fatalf("initialize: %v", init)
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}})
	stdin.Write(append(b, '\n'))
	if tools := call("tools/list", map[string]any{})["tools"].([]any); len(tools) != 8 {
		t.Fatalf("%d tools", len(tools))
	}
	res := call("tools/call", map[string]any{"name": "rules", "arguments": map[string]any{"game": "canasta"}})
	if text := res["content"].([]any)[0].(map[string]any)["text"].(string); !strings.Contains(text, "Canasta") {
		t.Fatalf("rules: %s", text)
	}
}

// Separate processes carry one game through the state file.
func TestCallSubcommand(t *testing.T) {
	state := filepath.Join(t.TempDir(), "s.json")
	run := func(args ...string) string {
		out, err := self(t, append([]string{"call", "-state", state}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("call %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	if out := run("new_table", `{"game":"zolik","seats":2,"claude_seats":[0],"seed":3}`); !strings.Contains(out, "Legal moves") {
		t.Fatalf("new_table:\n%s", out)
	}
	if out := run("play", `{"table":"t1","seat":0,"move":0}`); !strings.Contains(out, "Played:") {
		t.Fatalf("play:\n%s", out)
	}
	out := run("-json", "observe", `{"table":"t1","seat":0}`)
	var r struct{ OK bool }
	if err := json.Unmarshal([]byte(out), &r); err != nil || !r.OK {
		t.Fatalf("observe -json: %v\n%s", err, out)
	}
	bad, err := self(t, "call", "-state", state, "play", `{"table":"t1","seat":0,"move":99}`).CombinedOutput()
	if err == nil || !strings.Contains(string(bad), "error:") {
		t.Fatalf("an illegal move exited cleanly: %s", bad)
	}
}

func TestCLISubcommand(t *testing.T) {
	cmd := self(t, "cli")
	cmd.Stdin = strings.NewReader(`{"tool":"list_games"}` + "\n" + `{"tool":"nope"}` + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"ok":true`) || !strings.Contains(lines[1], `"ok":false`) {
		t.Fatalf("cli:\n%s", out)
	}
}
