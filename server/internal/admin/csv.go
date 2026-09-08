package admin

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"zolik/server/internal/metrics"
)

// writeReportCSV renders a report as one row per bucket.
//
// It exists because the operator's actual next step is a spreadsheet. The
// console draws the shape; the question "is this month up on last" gets
// answered by pasting two of these side by side, and asking somebody to
// transcribe numbers off a web page is asking them to make a transcription
// error.
//
// The per-module and per-reason splits are deliberately not here. They are
// variable-width — the columns would change between two exports of the same
// deployment as a game is added — and a CSV whose header depends on the data
// is one that breaks whatever the operator built on top of it. Those splits
// are on the screen and in the JSON.
func writeReportCSV(w http.ResponseWriter, rep metrics.Report) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	// Named for the range it covers, so two exports do not land in the
	// downloads folder as report.csv and report (1).csv.
	w.Header().Set("Content-Disposition",
		`attachment; filename="zolik-`+string(rep.Bucket)+`-`+rep.From+`-to-`+rep.To+`.csv"`)

	c := csv.NewWriter(w)
	defer c.Flush()

	_ = c.Write([]string{
		"label", "start", "end", "partial",
		"matches_created", "matches_started", "matches_completed",
		"matches_abandoned", "matches_never_started", "completion_rate",
		"players_distinct", "players_is_floor",
		"users_registered", "guests_new",
		"connections", "refused", "match_starts_refused",
		"boots", "unclean_boots",
	})

	for _, p := range rep.Buckets {
		_ = c.Write([]string{
			p.Label, p.Start, p.End, strconv.FormatBool(p.Partial),
			itoa(p.Matches.Created), itoa(p.Matches.Started), itoa(p.Matches.Completed),
			itoa(p.Matches.Abandoned), itoa(p.Matches.NeverStarted),
			strconv.FormatFloat(p.Matches.CompletionRate, 'f', 4, 64),
			strconv.Itoa(p.Players.Distinct), strconv.FormatBool(p.Players.IsFloor),
			itoa(p.Users.Registered), itoa(p.Users.Guests),
			itoa(p.Admission.Connections), itoa(p.Admission.Total), itoa(p.Admission.MatchStartDenied),
			itoa(p.Ops.Boots), itoa(p.Ops.UncleanBoots),
		})
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
