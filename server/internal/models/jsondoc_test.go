package models

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Text, not base64 — and the seeds modules keep in their state, near 2^62 and
// negative, come back digit for digit, because nothing parses the numbers.
func TestJSONDocRoundTripsAsText(t *testing.T) {
	type doc struct {
		S JSONDoc `bson:"s,omitempty"`
	}
	for _, s := range []string{
		`{"seed":4611686018427387904,"neg":-4611686018427387904}`,
		`{"$date":"not a wrapper","k":"\"quoted\" ünïcode ☃"}`,
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

func TestJSONDocNullIsNil(t *testing.T) {
	type doc struct {
		S JSONDoc `bson:"s"`
	}
	out, err := bson.MarshalExtJSON(doc{}, false, false)
	if err != nil || string(out) != `{"s":null}` {
		t.Fatalf("empty wrote %s, %v", out, err)
	}
	d := doc{S: JSONDoc(`{"stale":true}`)}
	if err := bson.UnmarshalExtJSON(out, false, &d); err != nil || d.S != nil {
		t.Errorf("null decoded as %q, %v", d.S, err)
	}
}

// On the JSON wire a JSONDoc is the document itself, not base64.
func TestJSONDocOnTheWire(t *testing.T) {
	got, err := json.Marshal(struct {
		A JSONDoc `json:"a"`
	}{JSONDoc(`{"verb":"draw"}`)})
	if err != nil || string(got) != `{"a":{"verb":"draw"}}` {
		t.Errorf("got %s, %v", got, err)
	}
}
