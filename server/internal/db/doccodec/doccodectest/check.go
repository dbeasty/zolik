// Package doccodectest holds a stored model to the one promise doccodec makes
// about it: whatever the Mongo driver's Extended JSON codec made of a value,
// doccodec makes the same of it — reading the legacy bytes, and reading its own.
//
// Each package that stores documents on the kdb backend lists its stored types
// in a test that calls Check, so an unexported type is covered by the package
// that owns it, and a new stored type is one line to add.
package doccodectest

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec"
)

// Check round-trips a zero value and a few randomly filled values of each
// sample's type through both codecs and fails the test on any difference.
// Samples are values, not pointers: Check({}) of the type is enough.
func Check(t *testing.T, samples ...any) {
	t.Helper()
	for _, s := range samples {
		typ := reflect.TypeOf(s)
		t.Run(typ.String(), func(t *testing.T) {
			checkValue(t, reflect.New(typ).Elem().Interface(), "zero")
			for seed := uint64(1); seed <= 25; seed++ {
				v := reflect.New(typ).Elem()
				f := filler{r: rand.New(rand.NewPCG(seed, seed*7919))}
				f.fill(v, 0)
				checkValue(t, v.Interface(), fmt.Sprintf("seed %d", seed))
			}
		})
	}
}

// CheckValue is Check for one given value, for edge cases a random fill would
// rarely produce.
func CheckValue(t *testing.T, v any) {
	t.Helper()
	checkValue(t, v, "given")
}

func checkValue(t *testing.T, v any, label string) {
	t.Helper()
	typ := reflect.TypeOf(v)
	legacy, err := bson.MarshalExtJSON(v, false, false)
	if err != nil {
		t.Fatalf("%s: the legacy codec cannot store it: %v", label, err)
	}
	plain, err := doccodec.Marshal(v)
	if err != nil {
		t.Fatalf("%s: doccodec cannot store it: %v", label, err)
	}

	want := reflect.New(typ)
	if err := bson.UnmarshalExtJSON(legacy, false, want.Interface()); err != nil {
		t.Fatalf("%s: the legacy codec cannot read its own bytes: %v\n%s", label, err, legacy)
	}
	fromLegacy := reflect.New(typ)
	if err := doccodec.Unmarshal(legacy, fromLegacy.Interface()); err != nil {
		t.Fatalf("%s: doccodec cannot read a legacy document: %v\n%s", label, err, legacy)
	}
	if !reflect.DeepEqual(want.Elem().Interface(), fromLegacy.Elem().Interface()) {
		t.Fatalf("%s: a legacy document reads differently\nlegacy: %s\nwant: %#v\ngot:  %#v",
			label, legacy, want.Elem().Interface(), fromLegacy.Elem().Interface())
	}
	fromPlain := reflect.New(typ)
	if err := doccodec.Unmarshal(plain, fromPlain.Interface()); err != nil {
		t.Fatalf("%s: doccodec cannot read its own document: %v\n%s", label, err, plain)
	}
	if !reflect.DeepEqual(want.Elem().Interface(), fromPlain.Elem().Interface()) {
		t.Fatalf("%s: a plain document reads differently from the legacy one\nlegacy: %s\nplain:  %s\nwant: %#v\ngot:  %#v",
			label, legacy, plain, want.Elem().Interface(), fromPlain.Elem().Interface())
	}
}

// filler sets every settable field of a value to something random but storable
// by both codecs: whole numbers in range, times in the BSON date range, valid
// JSON in a JSONDoc, no NaN (DeepEqual cannot compare it).
type filler struct{ r *rand.Rand }

var (
	objectIDType  = reflect.TypeOf(bson.ObjectID{})
	timeType      = reflect.TypeOf(time.Time{})
	marshalerType = reflect.TypeOf((*doccodec.Marshaler)(nil)).Elem()
	emptyIface    = reflect.TypeOf((*any)(nil)).Elem()
)

const maxDepth = 4

func (f filler) str() string {
	const letters = "abcdefghijklmnopqrstuvwxyzÁéž \"\\\n<>&"
	rs := []rune(letters)
	n := f.r.IntN(12)
	out := make([]rune, n)
	for i := range out {
		out[i] = rs[f.r.IntN(len(rs))]
	}
	return string(out)
}

func (f filler) fill(v reflect.Value, depth int) {
	if !v.CanSet() {
		return
	}
	t := v.Type()
	switch t {
	case objectIDType:
		v.Set(reflect.ValueOf(bson.NewObjectIDFromTimestamp(time.Unix(f.r.Int64N(1<<31), 0))))
		return
	case timeType:
		// Nanoseconds on purpose: both codecs keep milliseconds, and both have
		// to drop the rest the same way.
		sec := f.r.Int64N(4_000_000_000) - 1_000_000_000
		v.Set(reflect.ValueOf(time.Unix(sec, f.r.Int64N(1e9)).In(time.FixedZone("Z+1", 3600))))
		return
	}
	if t.Implements(marshalerType) && t.Kind() == reflect.Slice {
		docs := []string{`{"a":1,"b":[true,null,"x"],"seed":4611686018427387903}`, `[1,2,3]`, `"just a string"`, `not json`, ``}
		if f.r.IntN(4) == 0 {
			v.SetZero()
			return
		}
		v.Set(reflect.ValueOf([]byte(docs[f.r.IntN(len(docs))])).Convert(t))
		return
	}
	switch t.Kind() {
	case reflect.Bool:
		v.SetBool(f.r.IntN(2) == 0)
	case reflect.Int, reflect.Int64:
		if f.r.IntN(3) == 0 {
			v.SetInt(f.r.Int64() - f.r.Int64())
		} else {
			v.SetInt(int64(f.r.Int32N(1000) - 500))
		}
	case reflect.Int8, reflect.Int16, reflect.Int32:
		v.SetInt(int64(f.r.IntN(100) - 50))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		v.SetUint(uint64(f.r.IntN(200)))
	case reflect.Uint, reflect.Uint64:
		v.SetUint(uint64(f.r.Int64N(1 << 40)))
	case reflect.Float32:
		v.SetFloat(float64(float32(f.r.Float64()*1000 - 500)))
	case reflect.Float64:
		v.SetFloat(f.r.NormFloat64() * 1e6)
	case reflect.String:
		v.SetString(f.str())
	case reflect.Pointer:
		if depth >= maxDepth || f.r.IntN(3) == 0 {
			return
		}
		p := reflect.New(t.Elem())
		f.fill(p.Elem(), depth+1)
		v.Set(p)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).IsExported() || t.Field(i).Anonymous {
				f.fill(v.Field(i), depth+1)
			}
		}
	case reflect.Slice:
		if depth >= maxDepth {
			return
		}
		switch f.r.IntN(4) {
		case 0:
			return // nil
		case 1:
			v.Set(reflect.MakeSlice(t, 0, 0))
			return
		}
		n := 1 + f.r.IntN(3)
		s := reflect.MakeSlice(t, n, n)
		for i := 0; i < n; i++ {
			f.fill(s.Index(i), depth+1)
		}
		v.Set(s)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			f.fill(v.Index(i), depth+1)
		}
	case reflect.Map:
		if depth >= maxDepth || t.Key().Kind() != reflect.String {
			return
		}
		switch f.r.IntN(4) {
		case 0:
			return
		case 1:
			v.Set(reflect.MakeMap(t))
			return
		}
		m := reflect.MakeMap(t)
		for i := 0; i < 1+f.r.IntN(3); i++ {
			k := reflect.New(t.Key()).Elem()
			k.SetString(fmt.Sprintf("k%d%s", i, f.str()))
			e := reflect.New(t.Elem()).Elem()
			f.fill(e, depth+1)
			m.SetMapIndex(k, e)
		}
		v.Set(m)
	case reflect.Interface:
		if t != emptyIface || depth >= maxDepth {
			return
		}
		switch f.r.IntN(5) {
		case 0:
			v.Set(reflect.ValueOf(f.str()))
		case 1:
			v.Set(reflect.ValueOf(int32(f.r.IntN(1000))))
		case 2:
			v.Set(reflect.ValueOf(bson.D{{Key: "x", Value: f.str()}, {Key: "n", Value: int32(7)}}))
		case 3:
			v.Set(reflect.ValueOf(bson.A{true, f.str()}))
		}
	}
}
