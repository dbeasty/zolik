package stats

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Usage is the operator's-eye view of the match record: how much has been
// played, at what, and when.
//
// It is computed from match_results rather than from PlayerStats because the
// aggregates are per-subject — summing them would count a four-player match
// four times, and would miss matches played entirely by guests, who keep no
// lifetime record at all.
type Usage struct {
	TotalMatches int64 `json:"totalMatches"`
	// RecentMatches counts only the window ByDay covers.
	RecentMatches int64         `json:"recentMatches"`
	ByModule      []ModuleUsage `json:"byModule"`
	// ByDay is oldest-first and gap-filled: a day nothing was played on is
	// present with a zero count, so a chart drawn from it cannot silently
	// close the gap and imply play that never happened.
	ByDay []DayUsage `json:"byDay"`
}

type ModuleUsage struct {
	ModuleID           string  `json:"moduleId"`
	Matches            int64   `json:"matches"`
	AvgDurationSeconds float64 `json:"avgDurationSeconds"`
}

type DayUsage struct {
	// Date is an ISO calendar day in UTC, matching how the records are
	// bucketed.
	Date    string `json:"date"`
	Matches int64  `json:"matches"`
}

// usageDayFormat is the ISO day both the Mongo bucketing and the gap-filling
// below format with, so the two agree on what a day is called.
const usageDayFormat = "2006-01-02"

// UsageSummary aggregates the match record over the last `days` calendar days.
func (r *mongoRepository) UsageSummary(ctx context.Context, days int) (Usage, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	// Midnight UTC `days-1` days back, so the window is whole calendar days
	// and its last bucket is today rather than a part-day that looks like a
	// collapse in play.
	now := time.Now().UTC()
	start := now.Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))

	out := Usage{ByModule: []ModuleUsage{}, ByDay: []DayUsage{}}

	total, err := r.matches.CountDocuments(ctx, bson.M{})
	if err != nil {
		return Usage{}, err
	}
	out.TotalMatches = total

	recent, err := r.matches.CountDocuments(ctx, bson.M{"completedAt": bson.M{"$gte": start}})
	if err != nil {
		return Usage{}, err
	}
	out.RecentMatches = recent

	byModule, err := r.matches.Aggregate(ctx, []bson.M{
		{"$group": bson.M{
			"_id":         "$moduleId",
			"matches":     bson.M{"$sum": 1},
			"avgDuration": bson.M{"$avg": "$durationSeconds"},
		}},
		{"$sort": bson.M{"matches": -1}},
	})
	if err != nil {
		return Usage{}, err
	}
	var moduleRows []struct {
		ID          string  `bson:"_id"`
		Matches     int64   `bson:"matches"`
		AvgDuration float64 `bson:"avgDuration"`
	}
	if err := byModule.All(ctx, &moduleRows); err != nil {
		return Usage{}, err
	}
	for _, row := range moduleRows {
		out.ByModule = append(out.ByModule, ModuleUsage{
			ModuleID:           row.ID,
			Matches:            row.Matches,
			AvgDurationSeconds: row.AvgDuration,
		})
	}

	byDay, err := r.matches.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"completedAt": bson.M{"$gte": start}}},
		{"$group": bson.M{
			"_id":     bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$completedAt"}},
			"matches": bson.M{"$sum": 1},
		}},
	})
	if err != nil {
		return Usage{}, err
	}
	var dayRows []struct {
		ID      string `bson:"_id"`
		Matches int64  `bson:"matches"`
	}
	if err := byDay.All(ctx, &dayRows); err != nil {
		return Usage{}, err
	}
	counts := make(map[string]int64, len(dayRows))
	for _, row := range dayRows {
		counts[row.ID] = row.Matches
	}
	for d := 0; d < days; d++ {
		key := start.AddDate(0, 0, d).Format(usageDayFormat)
		out.ByDay = append(out.ByDay, DayUsage{Date: key, Matches: counts[key]})
	}

	return out, nil
}
