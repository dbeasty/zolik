package admin

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"zolik/server/internal/geo"
	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/stats"
)

// The Accounts card: how many accounts there are, how they are used, and who
// uses them most. The report card above it answers "how much was played";
// this one answers "by whom".
//
// Everything here is derived on read from three sources that already exist —
// the accounts themselves, their lifetime statistics, and the daily player
// sets the metrics recorder keeps — so it needs no counters of its own and
// cannot drift from them. Region is the one new fact, and it is derived too:
// from the time zone a device reports on refresh (see geo), never from an IP.

// ActiveWindowDays is how far back "active" looks.
const ActiveWindowDays = 30

// AccountsData is what the app hands over: every account, the lifetime record
// of each that has one (keyed by stats subject key), and the daily player
// sets for the window.
type AccountsData struct {
	Users []models.User
	Stats map[string]stats.PlayerStats
	Days  []metrics.Day
}

// AccountsQuery is what the console asked to see in the table.
type AccountsQuery struct {
	// Sort is matches | active | recent | new.
	Sort  string
	Limit int
	// Search narrows the table to usernames or emails containing it. It never
	// narrows the totals: those describe the whole population.
	Search string
}

type Count struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type GameUse struct {
	ModuleID string `json:"moduleId"`
	// Accounts is how many accounts have finished at least one match of it.
	Accounts int `json:"accounts"`
	Matches  int `json:"matches"`
}

type AccountTotals struct {
	Accounts      int `json:"accounts"`
	VerifiedEmail int `json:"verifiedEmail"`
	Played        int `json:"played"`
	NeverPlayed   int `json:"neverPlayed"`
	New7          int `json:"new7"`
	New30         int `json:"new30"`
	// Active* are accounts that played on that many of the last UTC days
	// (today included), from the daily player sets. Guests* are the same for
	// guest devices, which have no account row to count any other way.
	Active1  int `json:"active1"`
	Active7  int `json:"active7"`
	Active30 int `json:"active30"`
	Guests1  int `json:"guests1"`
	Guests7  int `json:"guests7"`
	Guests30 int `json:"guests30"`
	// Returning30 is active accounts created before the window: the people
	// who came back, as opposed to the ones who just arrived.
	Returning30 int `json:"returning30"`
	// RegionKnown is how many accounts have reported a time zone that names
	// a country. Accounts sign in before they refresh, so this lags.
	RegionKnown int `json:"regionKnown"`
}

type AccountRow struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email,omitempty"`
	EmailVerified bool       `json:"emailVerified"`
	Provider      string     `json:"provider"`
	Region        string     `json:"region,omitempty"`
	TimeZone      string     `json:"timeZone,omitempty"`
	Language      string     `json:"language,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastSeenAt    *time.Time `json:"lastSeenAt,omitempty"`
	LastMatchAt   *time.Time `json:"lastMatchAt,omitempty"`
	Matches       int        `json:"matches"`
	Wins          int        `json:"wins"`
	WinRate       float64    `json:"winRate"`
	VsHumans      int        `json:"vsHumans"`
	VsAI          int        `json:"vsAI"`
	// Games is how many different games the account has finished a match of;
	// FavouriteGame the one with the most matches.
	Games         int    `json:"games"`
	FavouriteGame string `json:"favouriteGame,omitempty"`
	// ActiveDays is days played in the window, out of ActiveWindowDays.
	ActiveDays int `json:"activeDays"`
}

type AccountsReport struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	WindowDays  int           `json:"windowDays"`
	Totals      AccountTotals `json:"totals"`
	ByProvider  []Count       `json:"byProvider"`
	ByRegion    []Count       `json:"byRegion"`
	ByLanguage  []Count       `json:"byLanguage"`
	ByGame      []GameUse     `json:"byGame"`
	// Engagement buckets accounts by lifetime matches finished.
	Engagement []Count `json:"engagement"`
	// Signups is accounts created per month, oldest first, for the last
	// twelve months.
	Signups []Count `json:"signups"`

	Sort    string       `json:"sort"`
	Matched int          `json:"matched"`
	Users   []AccountRow `json:"users"`
}

var engagementBands = []struct {
	label string
	min   int
}{
	{"0", 0}, {"1–4", 1}, {"5–19", 5}, {"20–99", 20}, {"100+", 100},
}

// BuildAccounts turns the raw data into the card. Pure, so it is tested
// without a database or a clock.
func BuildAccounts(d AccountsData, q AccountsQuery, now time.Time) AccountsReport {
	now = now.UTC()
	today := now.Format(metrics.DayFormat)
	cut7 := now.AddDate(0, 0, -6).Format(metrics.DayFormat)
	cut30 := now.AddDate(0, 0, -(ActiveWindowDays - 1)).Format(metrics.DayFormat)
	windowStart, _ := time.Parse(metrics.DayFormat, cut30)

	// Days played per subject, and the three distinct-sets, in one pass.
	activeDays := map[string]int{}
	seen := [3]map[string]struct{}{{}, {}, {}}
	for _, day := range d.Days {
		if day.Date < cut30 || day.Date > today {
			continue
		}
		for _, key := range day.Players {
			activeDays[key]++
			seen[2][key] = struct{}{}
			if day.Date >= cut7 {
				seen[1][key] = struct{}{}
			}
			if day.Date == today {
				seen[0][key] = struct{}{}
			}
		}
	}

	rep := AccountsReport{GeneratedAt: now, WindowDays: ActiveWindowDays, Sort: normSort(q.Sort)}
	t := &rep.Totals
	active := [3]*int{&t.Active1, &t.Active7, &t.Active30}
	guests := [3]*int{&t.Guests1, &t.Guests7, &t.Guests30}
	for i, set := range seen {
		for key := range set {
			switch {
			case strings.HasPrefix(key, "user:"):
				*active[i]++
			case strings.HasPrefix(key, "guest:"):
				*guests[i]++
			}
		}
	}

	providers, regions, languages := map[string]int{}, map[string]int{}, map[string]int{}
	games := map[string]*GameUse{}
	bands := make([]int, len(engagementBands))
	signups := map[string]int{}
	firstMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -11, 0)

	needle := strings.ToLower(strings.TrimSpace(q.Search))
	rows := make([]AccountRow, 0, len(d.Users))
	for _, u := range d.Users {
		id := u.ID.Hex()
		key := stats.Subject{Kind: stats.SubjectUser, ID: id}.Key()
		ps, played := d.Stats[key]

		t.Accounts++
		if u.EmailVerified {
			t.VerifiedEmail++
		}
		created := u.CreatedAt.UTC()
		if now.Sub(created) < 7*24*time.Hour {
			t.New7++
		}
		if now.Sub(created) < ActiveWindowDays*24*time.Hour {
			t.New30++
		}
		if _, ok := seen[2][key]; ok && created.Before(windowStart) {
			t.Returning30++
		}
		if !created.Before(firstMonth) {
			signups[created.Format("2006-01")]++
		}

		region := geo.CountryForTimeZone(u.TimeZone)
		if region != "" {
			t.RegionKnown++
		}
		regions[region]++
		providers[orUnknown(u.AuthProvider)]++
		languages[orUnknown(u.Preferences.Language)]++

		row := AccountRow{
			ID:            id,
			Username:      u.Username,
			Email:         u.Email,
			EmailVerified: u.EmailVerified,
			Provider:      u.AuthProvider,
			Region:        region,
			TimeZone:      u.TimeZone,
			Language:      u.Preferences.Language,
			CreatedAt:     created,
			LastSeenAt:    timePtr(u.LastSeenAt),
			ActiveDays:    activeDays[key],
		}
		matches := 0
		if played {
			matches = ps.Overall.Matches
			row.Matches = matches
			row.Wins = ps.Overall.Wins
			if matches > 0 {
				row.WinRate = float64(ps.Overall.Wins) / float64(matches)
			}
			row.VsHumans = ps.VsHumans.Matches
			row.VsAI = ps.VsAI.Matches
			row.LastMatchAt = timePtr(ps.LastMatchAt)
			best := 0
			for mod, tally := range ps.ByModule {
				if tally.Matches == 0 {
					continue
				}
				row.Games++
				g := games[mod]
				if g == nil {
					g = &GameUse{ModuleID: mod}
					games[mod] = g
				}
				g.Accounts++
				g.Matches += tally.Matches
				// Ties go to the alphabetically first game, so the column does
				// not change between two reloads of the same data.
				if tally.Matches > best || (tally.Matches == best && mod < row.FavouriteGame) {
					best, row.FavouriteGame = tally.Matches, mod
				}
			}
		}
		if matches > 0 {
			t.Played++
		} else {
			t.NeverPlayed++
		}
		for i := len(engagementBands) - 1; i >= 0; i-- {
			if matches >= engagementBands[i].min {
				bands[i]++
				break
			}
		}

		if needle == "" || strings.Contains(strings.ToLower(u.Username), needle) ||
			strings.Contains(strings.ToLower(u.Email), needle) {
			rows = append(rows, row)
		}
	}

	rep.ByProvider = sortedCounts(providers)
	rep.ByRegion = sortedCounts(regions)
	// Unknown is the remainder, not a region: last, however large.
	for i, c := range rep.ByRegion {
		if c.Key == "" {
			rep.ByRegion = append(append(rep.ByRegion[:i:i], rep.ByRegion[i+1:]...), c)
			break
		}
	}
	rep.ByLanguage = sortedCounts(languages)
	for _, g := range games {
		rep.ByGame = append(rep.ByGame, *g)
	}
	sort.Slice(rep.ByGame, func(i, j int) bool {
		a, b := rep.ByGame[i], rep.ByGame[j]
		if a.Matches != b.Matches {
			return a.Matches > b.Matches
		}
		return a.ModuleID < b.ModuleID
	})
	if rep.ByGame == nil {
		rep.ByGame = []GameUse{}
	}
	for i, b := range engagementBands {
		rep.Engagement = append(rep.Engagement, Count{Key: b.label, Count: bands[i]})
	}
	for m := firstMonth; !m.After(now); m = m.AddDate(0, 1, 0) {
		label := m.Format("2006-01")
		rep.Signups = append(rep.Signups, Count{Key: label, Count: signups[label]})
	}

	sortRows(rows, rep.Sort)
	rep.Matched = len(rows)
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	rep.Users = rows
	return rep
}

func normSort(s string) string {
	switch s {
	case "active", "recent", "new":
		return s
	default:
		return "matches"
	}
}

func sortRows(rows []AccountRow, by string) {
	seenAt := func(r AccountRow) time.Time {
		if r.LastSeenAt == nil {
			return time.Time{}
		}
		return *r.LastSeenAt
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch by {
		case "active":
			if a.ActiveDays != b.ActiveDays {
				return a.ActiveDays > b.ActiveDays
			}
		case "recent":
			if !seenAt(a).Equal(seenAt(b)) {
				return seenAt(a).After(seenAt(b))
			}
		case "new":
			if !a.CreatedAt.Equal(b.CreatedAt) {
				return a.CreatedAt.After(b.CreatedAt)
			}
		}
		if a.Matches != b.Matches {
			return a.Matches > b.Matches
		}
		if a.ActiveDays != b.ActiveDays {
			return a.ActiveDays > b.ActiveDays
		}
		return a.Username < b.Username
	})
}

func sortedCounts(m map[string]int) []Count {
	out := make([]Count, 0, len(m))
	for k, v := range m {
		out = append(out, Count{Key: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// accounts serves the card. Read-only, but it lists who plays and their
// addresses, so it is audited like a change: who looked, and when.
func (h *Handlers) accounts(w http.ResponseWriter, r *http.Request) {
	if h.deps.Accounts == nil {
		http.Error(w, "account statistics are not configured", http.StatusServiceUnavailable)
		return
	}
	now := time.Now().UTC()
	data, err := h.deps.Accounts(r.Context(), now.AddDate(0, 0, -(ActiveWindowDays-1)), now)
	if err != nil {
		slog.Warn("building account statistics failed", "error", err)
		http.Error(w, "could not build the account statistics", http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	rep := BuildAccounts(data, AccountsQuery{
		Sort:   query.Get("sort"),
		Limit:  limit,
		Search: query.Get("q"),
	}, now)

	caller, _ := Caller(r)
	slog.Info("admin read account statistics",
		"user", caller.Username, "email", caller.Email, "remote", r.RemoteAddr)
	writeJSON(w, rep)
}

// AccountsSource is Deps.Accounts' type, named so app can build one without
// spelling out the signature.
type AccountsSource = func(ctx context.Context, from, to time.Time) (AccountsData, error)
