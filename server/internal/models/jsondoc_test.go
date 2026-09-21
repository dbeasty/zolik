package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func withTextWrites(t *testing.T, on bool) {
	t.Helper()
	was := jsonDocWritesText
	jsonDocWritesText = on
	t.Cleanup(func() { jsonDocWritesText = was })
}

// Until readers everywhere understand text, a JSONDoc must be stored exactly
// as the json.RawMessage it replaces was — a rollback reads it unchanged.
func TestJSONDocWritesTodaysBinaryByDefault(t *testing.T) {
	withTextWrites(t, false)
	raw := json.RawMessage(`{"seed":-4611686018427387904,"hand":["QC","QS"]}`)

	type before struct {
		S json.RawMessage `bson:"s,omitempty"`
		A json.RawMessage `bson:"a"`
	}
	type after struct {
		S JSONDoc `bson:"s,omitempty"`
		A JSONDoc `bson:"a"`
	}
	for _, c := range []struct {
		name string
		s, a json.RawMessage
	}{{"set", raw, raw}, {"empty", nil, nil}} {
		want, err := bson.MarshalExtJSON(before{c.s, c.a}, false, false)
		if err != nil {
			t.Fatal(err)
		}
		got, err := bson.MarshalExtJSON(after{JSONDoc(c.s), JSONDoc(c.a)}, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

// Text is the point: no base64, no wrapper, and the seeds six modules keep in
// their state — near 2^62 and negative — come back digit for digit, because
// nothing ever parses the numbers.
func TestJSONDocTextRoundTrip(t *testing.T) {
	withTextWrites(t, true)
	type doc struct {
		S JSONDoc `bson:"s,omitempty"`
	}
	for _, s := range []string{
		`{"seed":4611686018427387904,"neg":-4611686018427387904}`,
		`{"$date":"not a wrapper","k":"\"quoted\" ünïcode \u2603"}`,
		`{}`,
		`{"deep":[[[{"a":[1,2.5,-0,1e3,null,true]}]]]}`,
	} {
		out, err := bson.MarshalExtJSON(doc{JSONDoc(s)}, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "$binary") {
			t.Errorf("stored as binary: %s", out)
		}
		var back doc
		if err := bson.UnmarshalExtJSON(out, false, &back); err != nil {
			t.Fatalf("decoding %s: %v", out, err)
		}
		if string(back.S) != s {
			t.Errorf("round trip:\n got %s\nwant %s", back.S, s)
		}
	}
}

// Every stored form a document can be found in: today's binary (and every
// historical version BoardAfter reads), text, and null.
func TestJSONDocReadsEveryStoredForm(t *testing.T) {
	type doc struct {
		S JSONDoc `bson:"s"`
	}
	for name, stored := range map[string]string{
		"binary": `{"s":{"$binary":{"base64":"eyJhIjoxfQ==","subType":"00"}}}`,
		"text":   `{"s":"{\"a\":1}"}`,
	} {
		var d doc
		if err := bson.UnmarshalExtJSON([]byte(stored), false, &d); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(d.S) != `{"a":1}` {
			t.Errorf("%s: got %s", name, d.S)
		}
	}
	d := doc{S: JSONDoc(`{"stale":true}`)}
	if err := bson.UnmarshalExtJSON([]byte(`{"s":null}`), false, &d); err != nil {
		t.Fatal(err)
	}
	if d.S != nil {
		t.Errorf("null decoded as %q, want nil", d.S)
	}
}

// MatchAction's JSON form is what it was when Action was a json.RawMessage:
// the action inline, not base64.
func TestMatchActionJSONIsUnchanged(t *testing.T) {
	at := time.Date(2026, 9, 21, 5, 53, 24, 0, time.UTC)
	got, err := json.Marshal(MatchAction{Seq: 1, PlayerID: "p1", Action: JSONDoc(`{"verb":"draw"}`), At: at})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"seq":1,"playerId":"p1","action":{"verb":"draw"},"at":"2026-09-21T05:53:24Z"}`
	if string(got) != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
	var back MatchAction
	if err := json.Unmarshal(got, &back); err != nil || string(back.Action) != `{"verb":"draw"}` {
		t.Errorf("decoding: %v, action %s", err, back.Action)
	}
}
