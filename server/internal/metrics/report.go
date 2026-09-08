package metrics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Bucket is how a report groups days.
type Bucket string

const (
	BucketDay   Bucket = "day"
	BucketWeek  Bucket = "week"
	BucketMonth Bucket = "month"
)

// ParseBucket maps a query parameter onto a bucket, defaulting to day.
func ParseBucket(s string) Bucket {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "week":
		return BucketWeek
	case "month":
		return BucketMonth
	default:
		return BucketDay
	}
}

// Report is the operator's answer, and the shape the console renders.
//
// Timezone is on the response rather than assumed, because every number in it
// is bucketed by UTC day and an operator reading it at 22:00 in California is
// looking at a "today" that ends in two hours. A report that does not say
// which day it means is a report that will be misread.
type Report struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Bucket   Bucket   `json:"bucket"`
	Timezone string   `json:"timezone"`
	Buckets  []Period `json:"buckets"`
	Totals   Totals   `json:"totals"`
}

// Period is one bucket of the report.
type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
	// Label is what the console prints: the day, the ISO week, or the month.
	Label string `json:"label"`
	// Partial marks a bucket the range only partly covers — the first and last
	// week of a month-long range, or today. It is surfaced rather than
	// smoothed away because a half-week that looks like a collapse in play is
	// the single easiest way to misread this screen.
	Partial bool `json:"partial"`

	Matches   MatchCounts   `json:"matches"`
	Players   PlayerCounts  `json:"players"`
	Users     UserCounts    `json:"users"`
	Admission RefusalCounts `json:"admission"`
	Ops       OpsCounts     `json:"ops"`
}

// MatchCounts splits games four ways, because "unfinished" has three meanings
// and mixing them gives a number nobody can act on.
type MatchCounts struct {
	Created   int64 `json:"created"`
	Started   int64 `json:"started"`
	Completed int64 `json:"completed"`
	Abandoned int64 `json:"abandoned"`
	// NeverStarted is a lobby that never filled: created, and never became a
	// game at all. Not a game anybody failed to finish.
	NeverStarted int64 `json:"neverStarted"`
	// ByModule is completions per game, highest first.
	ByModule []ModuleCount `json:"byModule"`
	// CompletionRate is completed / (completed + abandoned) — deliberately
	// not over Created, which would count an empty lobby as a game somebody
	// walked away from. Nil when nothing resolved in the bucket, because
	// zero-over-zero is not "0% completed".
	CompletionRate *float64 `json:"completionRate"`
}

type ModuleCount struct {
	ModuleID string `json:"moduleId"`
	Matches  int64  `json:"matches"`
}

type PlayerCounts struct {
	// Distinct is people, not sessions: the union of the daily player sets.
	Distinct int `json:"distinct"`
	// IsFloor says the real figure is at least Distinct — some day in this
	// bucket saturated its set.
	IsFloor bool `json:"isFloor"`
}

type UserCounts struct {
	Registered int64 `json:"registered"`
	Guests     int64 `json:"guests"`
}

type RefusalCounts struct {
	// Total is every refusal in the bucket, and ByReason splits it. Memory
	// pressure and a closed waiting room mean very different things — one is
	// the server protecting itself from an OOM kill, the other is it declining
	// to grow while still serving every game in progress.
	Total            int64         `json:"total"`
	ByReason         []ReasonCount `json:"byReason"`
	MatchStartDenied int64         `json:"matchStartDenied"`
	Connections      int64         `json:"connections"`
}

type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

type OpsCounts struct {
	Boots        int64 `json:"boots"`
	UncleanBoots int64 `json:"uncleanBoots"`
}

// Totals is the whole range added up, so the console's headline tiles do not
// have to re-sum the buckets and risk defining a total differently from the
// rows underneath it.
type Totals struct {
	Matches   MatchCounts   `json:"matches"`
	Players   PlayerCounts  `json:"players"`
	Users     UserCounts    `json:"users"`
	Admission RefusalCounts `json:"admission"`
	Ops       OpsCounts     `json:"ops"`
}

// Reporter builds reports from the store.
type Reporter struct{ store Store }

func NewReporter(store Store) *Reporter { return &Reporter{store: store} }

// MaxRangeDays is the longest range one report may cover.
const MaxRangeDays = 400

// Build assembles the report for a closed range of UTC days.
//
// Gap-filling is not optional and not a display concern: every bucket in the
// range is present, including the ones nothing happened in. A chart drawn from
// a sparse series closes the gap and implies play that never happened, which
// is a graph that lies about the quiet week it was drawn to show.
func (r *Reporter) Build(ctx context.Context, from, to time.Time, bucket Bucket) (Report, error) {
	from, to = clampRange(from, to)
	fromKey, toKey := DayKey(from), DayKey(to)

	days, err := r.store.Days(ctx, fromKey, toKey)
	if err != nil {
		return Report{}, err
	}
	byDay := make(map[string]Day, len(days))
	for _, d := range days {
		byDay[d.Date] = d
	}

	boots, err := r.store.Boots(ctx, from, to.Add(24*time.Hour-time.Nanosecond))
	if err != nil {
		return Report{}, err
	}

	out := Report{
		From:     fromKey,
		To:       toKey,
		Bucket:   bucket,
		Timezone: "UTC",
		Buckets:  []Period{},
	}

	// The union of every day's player set, for the range as a whole. Held
	// separately from the per-bucket sets because a person who played on
	// Monday and Thursday is one player in the total and one player in each of
	// two weeks — summing the buckets would count them twice.
	rangePlayers := map[string]struct{}{}
	rangeFloor := false

	for _, span := range spansFor(from, to, bucket) {
		p := Period{
			Start:   DayKey(span.start),
			End:     DayKey(span.end),
			Label:   span.label,
			Partial: span.partial,
		}
		players := map[string]struct{}{}

		for d := span.start; !d.After(span.end); d = d.AddDate(0, 0, 1) {
			day, ok := byDay[DayKey(d)]
			if !ok {
				continue
			}
			foldDay(&p, day)
			for _, k := range day.Players {
				players[k] = struct{}{}
				rangePlayers[k] = struct{}{}
			}
			if day.PlayersTruncated {
				p.Players.IsFloor = true
				rangeFloor = true
			}
		}

		p.Players.Distinct = len(players)
		p.Matches.NeverStarted = max64(p.Matches.Created-p.Matches.Started, 0)
		p.Matches.CompletionRate = completionRate(p.Matches)
		p.Matches.ByModule = sortModules(p.Matches.ByModule)
		p.Admission.ByReason = sortReasons(p.Admission.ByReason)

		out.Buckets = append(out.Buckets, p)
	}

	out.Totals = totalsFrom(out.Buckets, len(rangePlayers), rangeFloor)
	// Boots are read from their own rows rather than from the counters,
	// because the rows carry the truth the counters only summarise — and
	// because a crash that happened before the counter flushed is in the rows
	// and not in the counters.
	out.Totals.Ops = opsFromBoots(boots)
	return out, nil
}

// foldDay adds one day's counters into a bucket.
func foldDay(p *Period, d Day) {
	p.Matches.Created += d.Counter(MatchesCreated)
	p.Matches.Started += d.Counter(MatchesStarted)
	p.Matches.Completed += d.Counter(MatchesCompleted)
	p.Matches.Abandoned += d.Counter(MatchesAbandoned)
	p.Users.Registered += d.Counter(UsersRegistered)
	p.Users.Guests += d.Counter(SessionsGuest)
	p.Admission.Connections += d.Counter(WSConnected)
	p.Admission.MatchStartDenied += d.Counter(AdmissionRefusedMatchStart)
	p.Ops.Boots += d.Counter(BootsTotal)
	p.Ops.UncleanBoots += d.Counter(BootsUnclean)

	for name, n := range d.Counters {
		switch {
		case strings.HasPrefix(name, MatchesCompletedPrefix):
			p.Matches.ByModule = addModule(p.Matches.ByModule, strings.TrimPrefix(name, MatchesCompletedPrefix), n)
		case name == AdmissionRefusedMatchStart:
			// Counted above, and deliberately not folded into Total: it is a
			// refusal to start a match, not a refused connection, and adding
			// the two would double-count a player who was told no once.
		case strings.HasPrefix(name, AdmissionRefusedPrefix):
			p.Admission.ByReason = addReason(p.Admission.ByReason, strings.TrimPrefix(name, AdmissionRefusedPrefix), n)
			p.Admission.Total += n
		}
	}
}

func addModule(rows []ModuleCount, id string, n int64) []ModuleCount {
	for i := range rows {
		if rows[i].ModuleID == id {
			rows[i].Matches += n
			return rows
		}
	}
	return append(rows, ModuleCount{ModuleID: id, Matches: n})
}

func addReason(rows []ReasonCount, reason string, n int64) []ReasonCount {
	for i := range rows {
		if rows[i].Reason == reason {
			rows[i].Count += n
			return rows
		}
	}
	return append(rows, ReasonCount{Reason: reason, Count: n})
}

func sortModules(rows []ModuleCount) []ModuleCount {
	if rows == nil {
		return []ModuleCount{}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Matches != rows[j].Matches {
			return rows[i].Matches > rows[j].Matches
		}
		return rows[i].ModuleID < rows[j].ModuleID
	})
	return rows
}

func sortReasons(rows []ReasonCount) []ReasonCount {
	if rows == nil {
		return []ReasonCount{}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Reason < rows[j].Reason
	})
	return rows
}

// completionRate is nil rather than zero when nothing resolved, because "no
// games finished or were abandoned" is not the same statement as "0% of games
// were finished", and a tile showing 0% on a quiet day is alarming for no
// reason.
func completionRate(m MatchCounts) *float64 {
	resolved := m.Completed + m.Abandoned
	if resolved == 0 {
		return nil
	}
	rate := float64(m.Completed) / float64(resolved)
	return &rate
}

func totalsFrom(buckets []Period, distinctPlayers int, floor bool) Totals {
	var t Totals
	for _, p := range buckets {
		t.Matches.Created += p.Matches.Created
		t.Matches.Started += p.Matches.Started
		t.Matches.Completed += p.Matches.Completed
		t.Matches.Abandoned += p.Matches.Abandoned
		t.Users.Registered += p.Users.Registered
		t.Users.Guests += p.Users.Guests
		t.Admission.Total += p.Admission.Total
		t.Admission.MatchStartDenied += p.Admission.MatchStartDenied
		t.Admission.Connections += p.Admission.Connections
		t.Ops.Boots += p.Ops.Boots
		t.Ops.UncleanBoots += p.Ops.UncleanBoots
		for _, m := range p.Matches.ByModule {
			t.Matches.ByModule = addModule(t.Matches.ByModule, m.ModuleID, m.Matches)
		}
		for _, rr := range p.Admission.ByReason {
			t.Admission.ByReason = addReason(t.Admission.ByReason, rr.Reason, rr.Count)
		}
	}
	t.Matches.NeverStarted = max64(t.Matches.Created-t.Matches.Started, 0)
	t.Matches.CompletionRate = completionRate(t.Matches)
	t.Matches.ByModule = sortModules(t.Matches.ByModule)
	t.Admission.ByReason = sortReasons(t.Admission.ByReason)
	// Not a sum: a player who appeared in three weeks is one person.
	t.Players = PlayerCounts{Distinct: distinctPlayers, IsFloor: floor}
	return t
}

func opsFromBoots(boots []Boot) OpsCounts {
	var o OpsCounts
	for _, b := range boots {
		o.Boots++
		// A row still open belongs to a process nothing has judged yet —
		// usually the one serving this request. Only a row a successor
		// explicitly marked unclean counts as a crash.
		if b.Crashed() {
			o.UncleanBoots++
		}
	}
	return o
}

type span struct {
	start, end time.Time
	label      string
	partial    bool
}

// spansFor divides a range into buckets, clipped to the range at both ends.
//
// A week or month that the range only partly covers is kept and marked
// partial, rather than dropped or silently shortened. Dropping it loses real
// play; keeping it unmarked draws a half-week next to full ones and invites
// the reader to see a decline.
func spansFor(from, to time.Time, bucket Bucket) []span {
	var out []span
	switch bucket {
	case BucketDay:
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			out = append(out, span{start: d, end: d, label: DayKey(d)})
		}
	case BucketWeek:
		for d := startOfISOWeek(from); !d.After(to); d = d.AddDate(0, 0, 7) {
			start, end := d, d.AddDate(0, 0, 6)
			partial := start.Before(from) || end.After(to)
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			year, week := d.ISOWeek()
			out = append(out, span{start: start, end: end, label: fmt.Sprintf("%d-W%02d", year, week), partial: partial})
		}
	case BucketMonth:
		for d := startOfMonth(from); !d.After(to); d = d.AddDate(0, 1, 0) {
			start, end := d, d.AddDate(0, 1, 0).AddDate(0, 0, -1)
			partial := start.Before(from) || end.After(to)
			if start.Before(from) {
				start = from
			}
			if end.After(to) {
				end = to
			}
			out = append(out, span{start: start, end: end, label: d.Format("2006-01"), partial: partial})
		}
	}
	return out
}

// startOfISOWeek returns the Monday of t's ISO week, in UTC.
func startOfISOWeek(t time.Time) time.Time {
	t = t.UTC().Truncate(24 * time.Hour)
	// Go's Weekday puts Sunday at 0; ISO weeks start on Monday, so Sunday is
	// six days into its week rather than the start of a new one.
	offset := (int(t.Weekday()) + 6) % 7
	return t.AddDate(0, 0, -offset)
}

func startOfMonth(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// clampRange normalises a requested range to whole UTC days, in order, and no
// longer than MaxRangeDays.
func clampRange(from, to time.Time) (time.Time, time.Time) {
	from = from.UTC().Truncate(24 * time.Hour)
	to = to.UTC().Truncate(24 * time.Hour)
	if to.Before(from) {
		from, to = to, from
	}
	if to.Sub(from) > MaxRangeDays*24*time.Hour {
		from = to.AddDate(0, 0, -(MaxRangeDays - 1))
	}
	return from, to
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
