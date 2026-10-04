// Command botsoak loads a running server with bot tables and measures what a
// person at one of them would feel.
//
// It is the soak of docs/bot-compute-gating-plan.md §5.7: the capacity formula
// the bot governor leases by (§3.2) is arithmetic on measured costs, and this
// checks it against a real process under a real CPU limit. Each table is one
// guest host and bots at a chosen skill. The host plays over the WebSocket like
// a person — it waits a moment, then sends the first legal move its offers
// describe — and every move it makes is timed until the next state frame
// arrives. That round trip (validate, apply, persist, broadcast) is what a
// player's tap waits on, and it is the first thing to suffer when bots take
// more CPU than the box has.
//
// Every interval it prints one line: tables playing, human moves, their
// round-trip p50/p95/p99, and what the server says about itself — the bot
// decision p95, the monitor's level, how many bot seats the governor has
// moved to a cheaper class. The server needs ENABLE_DEBUG_ENDPOINTS for the
// last three.
//
//	go run ./cmd/botsoak -base http://127.0.0.1:8090 -tables 20 -game marias -skill hard -for 3m
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"zolik/server/internal/module"
)

func main() {
	base := flag.String("base", "http://127.0.0.1:8090", "server base URL")
	tables := flag.Int("tables", 10, "concurrent tables")
	game := flag.String("game", "marias", "game id")
	skill := flag.String("skill", "hard", "bot skill")
	seats := flag.Int("seats", 3, "seats per table, the host included")
	think := flag.Duration("think", 600*time.Millisecond, "how long the host waits before moving")
	dur := flag.Duration("for", 2*time.Minute, "how long to run")
	every := flag.Duration("every", 15*time.Second, "report interval")
	ramp := flag.Duration("ramp", 200*time.Millisecond, "pause between opening tables")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()
	s := &soak{base: strings.TrimRight(*base, "/"), game: *game, skill: *skill, seats: *seats, think: *think}

	var wg sync.WaitGroup
	for i := 0; i < *tables; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.host(ctx, i)
		}(i)
		time.Sleep(*ramp)
	}

	t := time.NewTicker(*every)
	defer t.Stop()
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			s.report(time.Since(start), true)
			return
		case <-t.C:
			s.report(time.Since(start), false)
		}
	}
}

type soak struct {
	base, game, skill string
	seats             int
	think             time.Duration

	mu      sync.Mutex
	rtts    []time.Duration // since the last report
	all     []time.Duration
	playing atomic.Int64
	matches atomic.Int64
	errs    atomic.Int64
}

func (s *soak) post(path, token string, body any) (map[string]any, int, error) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(http.MethodPost, s.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out, res.StatusCode, nil
}

// host keeps one table playing until ctx ends, opening a new one whenever a
// match finishes or wedges.
func (s *soak) host(ctx context.Context, n int) {
	auth, code, err := s.post("/auth/guest", "", map[string]any{"name": fmt.Sprintf("Soak %d", n)})
	if err != nil || code != http.StatusOK {
		s.errs.Add(1)
		fmt.Fprintf(os.Stderr, "table %d: guest: %v %d\n", n, err, code)
		return
	}
	token, _ := auth["accessToken"].(string)
	me, _ := auth["userId"].(string)
	for ctx.Err() == nil {
		if err := s.playOne(ctx, token, me); err != nil && ctx.Err() == nil {
			s.errs.Add(1)
			fmt.Fprintf(os.Stderr, "table %d: %v\n", n, err)
			time.Sleep(time.Second)
		}
	}
}

func (s *soak) playOne(ctx context.Context, token, me string) error {
	created, code, err := s.post("/matches", token, map[string]any{"moduleId": s.game})
	if err != nil || code != http.StatusOK {
		return fmt.Errorf("create: %v %d %v", err, code, created)
	}
	id, _ := created["matchId"].(string)
	for i := 1; i < s.seats; i++ {
		if _, code, err := s.post("/matches/"+id+"/add-bot", token, map[string]any{"skill": s.skill}); err != nil || code != http.StatusOK {
			return fmt.Errorf("add-bot: %v %d", err, code)
		}
	}
	if _, code, err := s.post("/matches/"+id+"/start", token, nil); err != nil || code != http.StatusOK {
		return fmt.Errorf("start: %v %d", err, code)
	}
	s.matches.Add(1)

	wsURL := strings.Replace(s.base, "http", "ws", 1) + "/ws/matches/" + id + "?token=" + token
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("socket: %w", err)
	}
	defer conn.Close()
	go func() { <-ctx.Done(); _ = conn.Close() }()
	s.playing.Add(1)
	defer s.playing.Add(-1)

	var sentAt time.Time
	var tried []module.Action
	lastProgress := time.Now()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}
		var msg struct {
			Type         string               `json:"type"`
			Status       string               `json:"status"`
			LegalActions []module.ActionOffer `json:"legalActions"`
			Code         string               `json:"code"`
		}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if msg.Type == "error" {
			// Refused: try the next legal move this position offered.
			sentAt = time.Time{}
			if len(tried) > 0 {
				tried = tried[1:]
			}
			if len(tried) > 0 {
				s.send(conn, tried[0], &sentAt)
			}
			continue
		}
		if msg.Type != "match_state" {
			continue
		}
		if !sentAt.IsZero() {
			s.record(time.Since(sentAt))
			sentAt = time.Time{}
		}
		if msg.Status != "active" {
			return nil
		}
		live := false
		for _, o := range msg.LegalActions {
			if o.Enabled {
				live = true
				break
			}
		}
		if !live {
			if time.Since(lastProgress) > 2*time.Minute {
				return fmt.Errorf("no move for 2 minutes")
			}
			continue
		}
		lastProgress = time.Now()
		tried = module.ChooseActions(msg.LegalActions, nil)
		if len(tried) == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(s.think):
		}
		s.send(conn, tried[0], &sentAt)
	}
}

func (s *soak) send(conn *websocket.Conn, a module.Action, sentAt *time.Time) {
	*sentAt = time.Now()
	if err := conn.WriteJSON(a); err != nil {
		*sentAt = time.Time{}
	}
}

func (s *soak) record(d time.Duration) {
	s.mu.Lock()
	s.rtts = append(s.rtts, d)
	s.all = append(s.all, d)
	s.mu.Unlock()
}

func pct(xs []time.Duration, q float64) time.Duration {
	if len(xs) == 0 {
		return 0
	}
	i := int(q * float64(len(xs)-1))
	return xs[i]
}

func (s *soak) report(elapsed time.Duration, final bool) {
	s.mu.Lock()
	window := s.rtts
	if final {
		window = s.all
	}
	xs := append([]time.Duration(nil), window...)
	s.rtts = nil
	s.mu.Unlock()
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })

	srv := s.serverState()
	label := "t=" + elapsed.Round(time.Second).String()
	if final {
		label = "TOTAL"
	}
	fmt.Printf("%-8s tables=%-3d matches=%-4d moves=%-5d rtt p50=%-7s p95=%-7s p99=%-7s errs=%d | %s\n",
		label, s.playing.Load(), s.matches.Load(), len(xs),
		pct(xs, .5).Round(time.Millisecond), pct(xs, .95).Round(time.Millisecond), pct(xs, .99).Round(time.Millisecond),
		s.errs.Load(), srv)
}

// serverState is the server's own account: bot p95 and how much CPU went on
// bots, the monitor's level and readings, and the governor's reductions.
func (s *soak) serverState() string {
	var bots struct {
		BotCPUShare float64 `json:"botCpuShare"`
		Orphans     int64   `json:"orphans"`
		Timeouts    int64   `json:"timeoutsTotal"`
	}
	var capRep struct {
		Monitor struct {
			Level string `json:"level"`
			Last  struct {
				CPUStall float64       `json:"cpuStallFraction"`
				MemFrac  float64       `json:"memoryFraction"`
				ActP95   time.Duration `json:"actP95"`
			} `json:"last"`
			GOMAXPROCS int     `json:"gomaxprocs"`
			CPUQuota   float64 `json:"cpuQuota"`
		} `json:"monitor"`
		Governor struct {
			Mode         string  `json:"mode"`
			LeasedCores  float64 `json:"leasedCores"`
			Capacity     float64 `json:"capacityCores"`
			Leases       int     `json:"leases"`
			ReducedSeats int     `json:"reducedSeats"`
		} `json:"governor"`
	}
	get := func(path string, into any) bool {
		res, err := http.Get(s.base + path)
		if err != nil {
			return false
		}
		defer res.Body.Close()
		return json.NewDecoder(res.Body).Decode(into) == nil
	}
	if !get("/debug/capacity", &capRep) {
		return "(no /debug/capacity)"
	}
	get("/debug/bots", &bots)
	m, g := capRep.Monitor, capRep.Governor
	return fmt.Sprintf("level=%s psi=%.2f mem=%.2f botP95=%s botCPU=%.2f quota=%.1f gov=%s leases=%d (%.2f/%.2f cores) reduced=%d timeouts=%d",
		m.Level, m.Last.CPUStall, m.Last.MemFrac, m.Last.ActP95.Round(time.Millisecond), bots.BotCPUShare, m.CPUQuota,
		g.Mode, g.Leases, g.LeasedCores, g.Capacity, g.ReducedSeats, bots.Timeouts)
}
