package admin

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/metrics"
	"zolik/server/internal/models"
	"zolik/server/internal/stats"
)

func TestBuildAccounts(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, -3, 0)

	ann := models.User{ID: bson.NewObjectID(), Username: "ann", Email: "ann@example.com", EmailVerified: true,
		AuthProvider: "google", CreatedAt: old, LastSeenAt: now.Add(-time.Hour), TimeZone: "Europe/Prague",
		Preferences: models.UserPreferences{Language: "cs"}}
	bob := models.User{ID: bson.NewObjectID(), Username: "bob", AuthProvider: "email",
		CreatedAt: now.AddDate(0, 0, -2), TimeZone: "Europe/Kiev", Preferences: models.UserPreferences{Language: "en"}}
	cid := models.User{ID: bson.NewObjectID(), Username: "cid", AuthProvider: "email",
		CreatedAt: old, Preferences: models.UserPreferences{Language: "en"}}

	key := func(u models.User) string { return stats.Subject{Kind: stats.SubjectUser, ID: u.ID.Hex()}.Key() }
	data := AccountsData{
		Users: []models.User{ann, bob, cid},
		Stats: map[string]stats.PlayerStats{
			key(ann): {Overall: stats.Tally{Matches: 30, Wins: 12}, VsAI: stats.Tally{Matches: 30},
				ByModule: map[string]stats.Tally{"canasta": {Matches: 20}, "zolik": {Matches: 10}}},
			key(bob): {Overall: stats.Tally{Matches: 3, Wins: 3}, VsHumans: stats.Tally{Matches: 3},
				ByModule: map[string]stats.Tally{"zolik": {Matches: 3}}},
		},
		Days: []metrics.Day{
			{Date: "2026-08-01", Players: []string{key(cid)}}, // before the window
			{Date: "2026-09-20", Players: []string{key(ann), "guest:g1"}},
			{Date: "2026-10-05", Players: []string{key(ann), key(bob)}},
			{Date: "2026-10-08", Players: []string{key(ann), "guest:g2", "ai:hard"}},
		},
	}

	rep := BuildAccounts(data, AccountsQuery{}, now)
	tt := rep.Totals
	want := AccountTotals{Accounts: 3, VerifiedEmail: 1, Played: 2, NeverPlayed: 1, New7: 1, New30: 1,
		Active1: 1, Active7: 2, Active30: 2, Guests1: 1, Guests7: 1, Guests30: 2, Returning30: 1, RegionKnown: 2}
	if tt != want {
		t.Errorf("totals = %+v\nwant     %+v", tt, want)
	}

	if len(rep.ByRegion) != 3 || rep.ByRegion[0].Count != 1 {
		t.Errorf("ByRegion = %+v, want CZ, UA and unknown once each", rep.ByRegion)
	}
	if rep.ByGame[0].ModuleID != "canasta" || rep.ByGame[1].ModuleID != "zolik" || rep.ByGame[1].Accounts != 2 || rep.ByGame[1].Matches != 13 {
		t.Errorf("ByGame = %+v", rep.ByGame)
	}
	if got := rep.Engagement; got[0].Count != 1 || got[1].Count != 1 || got[3].Count != 1 {
		t.Errorf("Engagement = %+v", got)
	}
	if len(rep.Signups) != 12 || rep.Signups[11].Key != "2026-10" || rep.Signups[11].Count != 1 {
		t.Errorf("Signups = %+v", rep.Signups)
	}

	if rep.Users[0].Username != "ann" || rep.Users[0].FavouriteGame != "canasta" || rep.Users[0].Games != 2 ||
		rep.Users[0].ActiveDays != 3 || rep.Users[0].Region != "CZ" {
		t.Errorf("top row = %+v", rep.Users[0])
	}

	byNew := BuildAccounts(data, AccountsQuery{Sort: "new", Limit: 1}, now)
	if byNew.Matched != 3 || len(byNew.Users) != 1 || byNew.Users[0].Username != "bob" {
		t.Errorf("newest first, limit 1: matched %d, rows %+v", byNew.Matched, byNew.Users)
	}

	// A search narrows the table, never the totals.
	found := BuildAccounts(data, AccountsQuery{Search: "ANN@"}, now)
	if found.Matched != 1 || found.Totals.Accounts != 3 {
		t.Errorf("search: matched %d, accounts %d", found.Matched, found.Totals.Accounts)
	}
}
