package db

import (
	"strings"
	"testing"
)

// Plain JSON is what an unset KDB_DOC_WRITE_FORMAT means; extjson is there for
// a node that cannot read it, and anything else stops the server rather than
// quietly writing one shape or the other.
func TestDocWriteFormatSpellings(t *testing.T) {
	for in, want := range map[string]DocWriteFormat{"": DocWriteJSON, "json": DocWriteJSON, " JSON ": DocWriteJSON, "extjson": DocWriteExtJSON} {
		got, err := ParseDocWriteFormat(in)
		if err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseDocWriteFormat("bson"); err == nil {
		t.Error("an unknown format should be refused")
	}
	t.Setenv("KDB_DOC_WRITE_FORMAT", "jsno")
	if _, err := KDBStorageFromEnv(); err == nil || !strings.Contains(err.Error(), "KDB_DOC_WRITE_FORMAT") {
		t.Errorf("a typo should refuse to start, got %v", err)
	}
}

// Both writers produce rows the one reader understands.
func TestBothWriteFormatsReadBack(t *testing.T) {
	defer SetDocWriteFormat(CurrentDocWriteFormat())
	type row struct {
		Name string `bson:"name"`
		N    int64  `bson:"n"`
	}
	for _, f := range []DocWriteFormat{DocWriteJSON, DocWriteExtJSON} {
		SetDocWriteFormat(f)
		b, err := MarshalDoc(row{Name: "ada", N: 1 << 40})
		if err != nil {
			t.Fatal(err)
		}
		if f == DocWriteJSON && strings.Contains(string(b), "$") {
			t.Errorf("plain JSON carries a wrapper: %s", b)
		}
		var got row
		if err := UnmarshalDoc(b, &got); err != nil || got.Name != "ada" || got.N != 1<<40 {
			t.Errorf("%v: read back %+v, %v from %s", f, got, err, b)
		}
	}
}
