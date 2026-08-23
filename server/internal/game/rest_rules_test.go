package game

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"zolik/server/internal/rules"
)

// getRules serves one pair of defaults, so it has to be told which variation
// it is describing. Answering with Continental's floor for every profile is
// how a lobby prefilled from this endpoint ends up creating a Žolík Classic
// table with a 35-point initial-meld minimum it never defaults to.
func TestGetRules_DefaultsFollowTheRequestedProfile(t *testing.T) {
	type body struct {
		DefaultInitialMeldMinimum  int `json:"defaultInitialMeldMinimum"`
		DefaultDiscardDrawMinRound int `json:"defaultDiscardDrawMinRound"`
	}
	get := func(query string) body {
		t.Helper()
		rec := httptest.NewRecorder()
		(&GameRestHandlers{}).getRules(rec, httptest.NewRequest("GET", "/rules"+query, nil))
		if rec.Code != 200 {
			t.Fatalf("GET /rules%s: status %d", query, rec.Code)
		}
		var out body
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	cases := []struct {
		query string
		want  rules.RulesConfig
	}{
		// Unnamed and unknown both keep the long-standing answer, so an
		// older client that never sends the parameter sees no change.
		{"", rules.ProfileContinental},
		{"?profile=nonesuch", rules.ProfileContinental},
		{"?profile=continental", rules.ProfileContinental},
		{"?profile=zolik_classic", rules.ProfileZolikClassic},
	}
	for _, c := range cases {
		got := get(c.query)
		if got.DefaultInitialMeldMinimum != c.want.InitialMeldMinimum {
			t.Errorf("GET /rules%s: defaultInitialMeldMinimum = %d, want %d",
				c.query, got.DefaultInitialMeldMinimum, c.want.InitialMeldMinimum)
		}
		if got.DefaultDiscardDrawMinRound != c.want.DiscardDrawMinRound {
			t.Errorf("GET /rules%s: defaultDiscardDrawMinRound = %d, want %d",
				c.query, got.DefaultDiscardDrawMinRound, c.want.DiscardDrawMinRound)
		}
	}

	// The point of the whole exercise: the two variations must not report the
	// same floor, or the parameter is decorative.
	if rules.ProfileZolikClassic.InitialMeldMinimum == rules.ProfileContinental.InitialMeldMinimum {
		t.Fatal("the shipped profiles now share an initial-meld minimum — this test no longer proves anything")
	}
}
