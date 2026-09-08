// Package metrics records how much the server did, one document per UTC day.
//
// It is deliberately not a metrics pipeline. The deployment is one container,
// one Go binary, one embedded database and 1 GiB (see docs/docker-deploy-plan.md),
// so a scrape endpoint and a time-series database would be a second process
// competing with the game for the same gigabyte in order to serve one operator.
// Everything the operator actually asked for — games played, people playing,
// new accounts, crashes, connections refused under load — is a count, and a
// count fits in a document per day.
//
// The shape follows from three rules, and each is here to stop a specific
// mistake:
//
//  1. **No event rows.** One row per refusal or per connection would put
//     unbounded write volume in front of an embedded database sized for
//     gameplay, to answer questions that are all sums.
//  2. **No weekly or monthly rollups.** A week is seven of these documents
//     added up on read. Storing the week as well would be a second source of
//     truth, and the two would eventually disagree.
//  3. **Counters are folded in memory and flushed on a timer.** A refusal
//     happens exactly when the box is least able to afford another write, so
//     the refusal path must not write.
package metrics

import "time"

// DayFormat is how a day is named, in UTC, everywhere: it is the document's
// own key, the bucketing the Mongo aggregation uses, and the string the
// console renders. One constant so those three cannot drift into disagreeing
// about what a day is called.
const DayFormat = "2006-01-02"

// DayKey is the document key for the UTC day a time falls in.
func DayKey(t time.Time) string { return t.UTC().Format(DayFormat) }

// Day is every counter for one UTC calendar day, keyed by the day itself so
// reading a date range is N direct lookups and never a scan.
type Day struct {
	Date     string           `bson:"_id" json:"date"`
	Counters map[string]int64 `bson:"counters" json:"counters"`

	// Players is the distinct set of subject keys seen playing that day.
	//
	// A set rather than a counter, because "how many people played" is the
	// one question in the ask that a counter cannot answer. Incrementing on
	// connect double-counts anyone who reconnects — which, on a server whose
	// whole disconnect design is about letting people come back, is most of
	// them. models.User.LastSeenAt cannot answer it either: it is overwritten
	// by the next visit, so it knows who is here now and can never say who
	// was here on the 3rd.
	//
	// Keeping the set also makes a week's distinct players the union of seven
	// days rather than an invention. That is the property a counter could
	// never have, and it is why a few kilobytes a day is worth spending.
	Players []string `bson:"players,omitempty" json:"-"`

	// PlayersTruncated records that the set hit PlayerSetCap and stopped
	// admitting new keys. The console then renders the day as a floor
	// ("≥ 5,000") instead of a number that is quietly wrong. A metric that
	// silently saturates is worse than no metric: it reads as a plateau in
	// play rather than as the ceiling it is.
	PlayersTruncated bool `bson:"playersTruncated,omitempty" json:"playersTruncated,omitempty"`
}

// PlayerSetCap bounds the per-day player set. High enough that current volume
// will not come near it, low enough that a runaway — a bot storm, a bug
// minting a guest id per request — cannot grow one document without limit.
const PlayerSetCap = 5000

// Counter returns a counter's value, and zero for one never incremented. A
// missing counter and a zero counter are the same fact here: nothing happened.
func (d Day) Counter(name string) int64 {
	if d.Counters == nil {
		return 0
	}
	return d.Counters[name]
}

// Boot is one process lifetime.
//
// It exists because a process that is OOM-killed does not get to write "I
// died". The only honest detection is on the *next* start: this row is written
// with StoppedAt nil, the shutdown path fills it in, and a boot that finds the
// previous row still open knows the previous process did not exit on purpose.
//
// That distinction is the whole point. "It went down" covers a deploy (clean
// exit, immediately followed by a start at a new version), a crash or an OOM
// kill (unclean), and a host reboot (unclean, and every service at once); an
// operator can act on exactly one of those three, and only if they are told
// apart.
type Boot struct {
	ID        string     `bson:"_id" json:"id"`
	Version   string     `bson:"version" json:"version"`
	StartedAt time.Time  `bson:"startedAt" json:"startedAt"`
	StoppedAt *time.Time `bson:"stoppedAt,omitempty" json:"stoppedAt,omitempty"`

	// Reason is why the process stopped: "signal" for SIGINT/SIGTERM, the only
	// stop this process can observe about itself. Empty alongside a nil
	// StoppedAt is what "crashed" is made of.
	Reason string `bson:"reason,omitempty" json:"reason,omitempty"`

	// Counted marks a row already folded into boot.unclean, so that a server
	// restarted repeatedly does not count one dead predecessor once per
	// restart. Without it, a crash-looping container would report a rising
	// crash count from a single crash.
	Counted bool `bson:"counted,omitempty" json:"counted,omitempty"`
}

// Uptime is how long the process ran, and zero for one still running.
func (b Boot) Uptime() time.Duration {
	if b.StoppedAt == nil {
		return 0
	}
	return b.StoppedAt.Sub(b.StartedAt)
}

// Clean reports whether the process got to run its shutdown path.
func (b Boot) Clean() bool { return b.StoppedAt != nil }
