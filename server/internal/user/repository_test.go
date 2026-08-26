package user

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestEmptySearchMatchesEveryone(t *testing.T) {
	if got := (Query{Search: "   "}).filter(); len(got) != 0 {
		t.Errorf("a blank search built a filter %v, want an empty one", got)
	}
}

// The search term reaches Mongo inside a $regex, so it has to arrive meaning
// literally itself. Unescaped, "." would match any character — widening a
// search into a scan — and a nested quantifier could pin a core backtracking.
func TestSearchTermIsEscaped(t *testing.T) {
	cases := []struct {
		term string
		want string
	}{
		{"o.brien", `o\.brien`},
		{"(a+)+", `\(a\+\)\+`},
		{".*", `\.\*`},
		{"plain", "plain"},
	}

	for _, tc := range cases {
		clauses, ok := (Query{Search: tc.term}).filter()["$or"].([]bson.M)
		if !ok || len(clauses) != 2 {
			t.Fatalf("%q: filter was not the expected two-clause $or", tc.term)
		}
		for _, clause := range clauses {
			for field, cond := range clause {
				got := cond.(bson.M)["$regex"]
				if got != tc.want {
					t.Errorf("%q on %s: got %q, want %q", tc.term, field, got, tc.want)
				}
			}
		}
	}
}

func TestSearchCoversNameAndEmail(t *testing.T) {
	clauses := (Query{Search: "ann"}).filter()["$or"].([]bson.M)

	fields := map[string]bool{}
	for _, clause := range clauses {
		for field := range clause {
			fields[field] = true
		}
	}
	for _, want := range []string{"username", "email"} {
		if !fields[want] {
			t.Errorf("search does not cover %s (covers %v)", want, fields)
		}
	}
}
