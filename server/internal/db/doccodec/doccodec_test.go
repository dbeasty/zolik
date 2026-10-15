package doccodec_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec"
	"zolik/server/internal/db/doccodec/doccodectest"
	"zolik/server/internal/models"
)

type inner struct {
	A int    `bson:"a"`
	B string `bson:"b,omitempty"`
}

type Embedded struct {
	E1 string `bson:"e1"`
	E2 int    `bson:"e2,omitempty"`
}

type hidden struct{ H int }

// every holds one of each shape the driver's struct codec has a rule for.
type every struct {
	Embedded `bson:",inline"`
	hidden
	NoTag      string
	Inner      inner             `bson:"inner,omitempty"`
	InnerPtr   *inner            `bson:"innerPtr"`
	InnerPtrO  *inner            `bson:"innerPtrO,omitempty"`
	T          time.Time         `bson:"t,omitempty"`
	T2         time.Time         `bson:"t2"`
	TP         *time.Time        `bson:"tp,omitempty"`
	OID        bson.ObjectID     `bson:"oid,omitempty"`
	OID2       bson.ObjectID     `bson:"oid2"`
	S          []string          `bson:"s,omitempty"`
	S2         []string          `bson:"s2"`
	Structs    []inner           `bson:"structs"`
	M          map[string]int    `bson:"m"`
	MS         map[string]*inner `bson:"ms"`
	Named      map[kind]string   `bson:"named"`
	Any        any               `bson:"any"`
	AnyMap     map[string]any    `bson:"anyMap"`
	F          float64           `bson:"f"`
	F32        float32           `bson:"f32"`
	D          time.Duration     `bson:"d"`
	U          uint32            `bson:"u"`
	Big        int64             `bson:"big"`
	Arr        [3]int            `bson:"arr"`
	Kind       kind              `bson:"kind"`
	Skip       string            `bson:"-"`
	Hidden     string            `bson:"hiddenFromClients" json:"-"`
	Doc        models.JSONDoc    `bson:"doc"`
	DocO       models.JSONDoc    `bson:"docO,omitempty"`
	DocP       *models.JSONDoc   `bson:"docP,omitempty"`
	Bools      []bool            `bson:"bools"`
	Nested     [][]int           `bson:"nested"`
	unexported int
}

type kind string

func TestEveryRuleMatchesTheDriver(t *testing.T) {
	doccodectest.Check(t, every{}, inner{}, bson.M{}, map[string]any{})
}

func TestEdgeValuesMatchTheDriver(t *testing.T) {
	zero := time.Time{}
	epoch := time.Unix(0, 0)
	before := time.Date(1912, 4, 15, 2, 20, 0, 999_999_999, time.UTC)
	for _, v := range []every{
		{T2: zero},
		{T2: epoch, TP: &epoch},
		{T2: before, TP: &before},
		{Big: 1<<63 - 1, F: 1e300, U: 1<<32 - 1},
		{Big: -1 << 63, F: -0.0},
		{NoTag: "\u2028\u2029\x00\x1f\"\\/é😀"},
		{Doc: models.JSONDoc(`{"seed":4611686018427387903,"s":"x\"y"}`)},
		{Doc: models.JSONDoc(`  [ ]`)},
		{Doc: models.JSONDoc(`"a bare string is stored as a string"`)},
		{Doc: models.JSONDoc(`{"truncated":`)},
		{Doc: models.JSONDoc{}},
		{S2: []string{}, M: map[string]int{}, Structs: []inner{}},
		{Any: bson.D{{Key: "nested", Value: bson.D{{Key: "a", Value: bson.A{int32(1), "two", nil}}}}}},
		{Any: int64(1) << 40},
		{Any: 2.5},
	} {
		doccodectest.CheckValue(t, v)
	}
}

// An ObjectID or a time inside an untyped value has no type in plain JSON, so
// it reads back as the string it was written as, where a legacy document read
// back a typed value. What a client is sent is the same either way, which is
// the one thing anything decoding into a map does with it (the replica).
func TestUntypedValuesRenderTheSameForAClient(t *testing.T) {
	v := map[string]any{"oid": bson.NewObjectID(), "when": time.UnixMilli(1700000000123).UTC(), "n": int32(3)}
	legacy, err := bson.MarshalExtJSON(v, false, false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := doccodec.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var fromLegacy, fromPlain bson.M
	if err := doccodec.Unmarshal(legacy, &fromLegacy); err != nil {
		t.Fatal(err)
	}
	if err := doccodec.Unmarshal(plain, &fromPlain); err != nil {
		t.Fatal(err)
	}
	var want bson.M
	if err := bson.UnmarshalExtJSON(legacy, false, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, fromLegacy) {
		t.Fatalf("legacy untyped values read as %#v, want %#v", fromLegacy, want)
	}
	a, _ := json.Marshal(fromLegacy)
	b, _ := json.Marshal(fromPlain)
	if string(a) != string(b) {
		t.Fatalf("a client would see\n%s\nfrom a legacy row and\n%s\nfrom a plain one", a, b)
	}
}

// What the bytes look like is a contract too: a person reading kdb-cli output,
// and the replica, which hands stored values to clients.
func TestPlainShape(t *testing.T) {
	id, _ := bson.ObjectIDFromHex("6ad083a30d675ec0fbfb3eff")
	v := every{
		OID2: id,
		T2:   time.Date(2026, 10, 15, 7, 0, 0, 123_456_789, time.FixedZone("CEST", 7200)),
		Doc:  models.JSONDoc(`{"a":1}`),
	}
	b, err := doccodec.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"oid2":"6ad083a30d675ec0fbfb3eff"`,
		`"t2":"2026-10-15T05:00:00.123Z"`,
		`"doc":{"a":1}`,
		`"hiddenFromClients":""`,
		`"e1":""`,
		`"notag":""`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("stored document lacks %s:\n%s", want, s)
		}
	}
	for _, unwanted := range []string{"$oid", "$date", "$numberLong", `"skip"`, `"h"`, `"unexported"`} {
		if strings.Contains(s, unwanted) {
			t.Errorf("stored document contains %s:\n%s", unwanted, s)
		}
	}
}

// A row written by an older build and then patched by a newer one holds both
// shapes at once.
func TestMixedDocumentReads(t *testing.T) {
	raw := `{"oid2":{"$oid":"6ad083a30d675ec0fbfb3eff"},"t2":"2026-10-15T05:00:00.123Z",` +
		`"tp":{"$date":{"$numberLong":"1700000000123"}},"big":{"$numberLong":"42"},` +
		`"f":{"$numberDouble":"Infinity"},"doc":"{\"legacy\":true}","docO":{"plain":true},"id":"engine-uuid"}`
	var v every
	if err := doccodec.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.OID2.Hex() != "6ad083a30d675ec0fbfb3eff" || v.Big != 42 || !v.TP.Equal(time.UnixMilli(1700000000123)) ||
		string(v.Doc) != `{"legacy":true}` || string(v.DocO) != `{"plain":true}` || v.T2.Nanosecond() != 123_000_000 {
		t.Fatalf("mixed document read as %+v", v)
	}
}

// Reading into a variable that already holds a document behaves as the driver
// does, which scans that reuse one variable rely on.
func TestDecodeIntoAUsedValue(t *testing.T) {
	first := `{"notag":"first","s2":["a","b"],"m":{"x":1},"innerPtr":{"a":5}}`
	second := `{"s2":["c"],"m":{"y":2}}`
	var want, got every
	for _, doc := range []string{first, second} {
		if err := bson.UnmarshalExtJSON([]byte(doc), false, &want); err != nil {
			t.Fatal(err)
		}
		if err := doccodec.Unmarshal([]byte(doc), &got); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("reuse differs\nwant %+v\ngot  %+v", want, got)
	}
}

func TestMalformedInputIsAnError(t *testing.T) {
	for _, doc := range []string{
		``, `{`, `{"a":}`, `{"notag":"unterminated}`, `{"notag":"x"} trailing`,
		`{"big":1.5}`, `{"oid2":"short"}`, `{"t2":"yesterday"}`, `{"u":-1}`,
		`{"notag":"\q"}`, "{\"notag\":\"\x01\"}", `{"big":{"$numberLong":"x"}}`,
	} {
		var v every
		if err := doccodec.Unmarshal([]byte(doc), &v); err == nil {
			t.Errorf("%q decoded without an error", doc)
		}
	}
	var v every
	err := doccodec.Unmarshal([]byte(`{"a"`), &v)
	if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("a syntax error should say so, got %v", err)
	}
	if err := doccodec.Unmarshal([]byte(`{}`), v); err == nil {
		t.Error("a non-pointer target should be refused")
	}
}

func TestUnsupportedShapesAreRefused(t *testing.T) {
	type badInline struct {
		X int `bson:",inline"`
	}
	type badOpt struct {
		X int `bson:"x,weird"`
	}
	type dup struct {
		A int `bson:"x"`
		B int `bson:"x"`
	}
	for _, v := range []any{badInline{}, badOpt{}, dup{}, map[int]string{1: "a"}, make(chan int)} {
		if _, err := doccodec.Marshal(v); err == nil {
			t.Errorf("%T stored without an error", v)
		}
	}
	_ = errors.New
}
