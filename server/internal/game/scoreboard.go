package game

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"zolik/server/internal/stats"
)

// Scoreboard returns the standings of one match — the deal-by-deal table for
// the players currently at this table.
//
// It works on a game in any state. During play it is the running scoreboard
// with a provisional leader; once the match is complete it is the final
// result, computed by the same function that produced the permanent record,
// so the two can never disagree.
//
// It reads the live game rather than the recorded match on purpose: an
// in-progress match has no record yet, and a just-finished one may still be
// mid-record, so going to the source means the endpoint never shows a gap.
func (h *GameRestHandlers) scoreboard(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	idOrJoin := chi.URLParam(req, "id")

	g, _, err := h.repo.ParseGameIDOrJoin(ctx, idOrJoin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	sb := stats.BuildScoreboard(g, GameRules(g))

	out := map[string]any{"scoreboard": sb}

	// ?lifetime=1 attaches each player's career record to their row, which is
	// what turns a scoreboard into "who am I actually up against" — including
	// for the bots, whose record is kept on the same footing as a person's.
	// It costs one extra query, so it is opt-in rather than always on.
	if h.stats != nil && req.URL.Query().Get("lifetime") != "" {
		keys := make([]string, 0, len(sb.Standings))
		for _, s := range sb.Standings {
			if k := s.Subject.Key(); k != "" {
				keys = append(keys, k)
			}
		}
		if records, err := h.stats.FindManyPlayerStats(ctx, keys); err == nil {
			lifetime := map[string]any{}
			for _, s := range sb.Standings {
				k := s.Subject.Key()
				if k == "" {
					// A guest keeps no career record — see stats.SubjectGuest.
					continue
				}
				ps, ok := records[k]
				if !ok {
					ps = stats.ZeroStats(s.Subject)
				}
				lifetime[s.PlayerID] = map[string]any{
					"subject":          ps.Subject,
					"overall":          ps.Overall.View(),
					"currentStreak":    ps.CurrentStreak,
					"longestWinStreak": ps.LongestWinStreak,
				}
			}
			out["lifetime"] = lifetime
		}
	}

	_ = json.NewEncoder(w).Encode(out)
}
