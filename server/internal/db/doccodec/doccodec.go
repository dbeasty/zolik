// Package doccodec is how the kdb backend turns a model struct into the JSON
// it stores, and back.
//
// It reads the same `bson:` tags the Mongo driver does, because those tags are
// what name a stored field — and several fields are `json:"-"` precisely
// because they are stored but must never reach a client, so the `json:` tags
// cannot be the storage contract. What it writes is plain JSON: an ObjectID is
// its hex string, a time is an RFC 3339 string, a module's state is the raw
// JSON it already is. There is no BSON on this path; the Mongo driver is only
// ever involved on the Mongo backend, which speaks binary BSON to its server
// and never sees any of this.
//
// Before this package the kdb backend stored MongoDB Extended JSON, produced
// and parsed by the driver's text codec ({"$oid": ...}, {"$date": ...},
// {"$numberLong": ...}). That parser was the largest single cost in a loaded
// server's profile. Every database written before this package holds that
// shape, so Unmarshal reads it as well as plain JSON, per value, with no
// detection pass: a legacy document and a new one go down the same path, and a
// document mixing both (a legacy row patched by a newer build) reads too.
//
// The rules follow the Mongo driver's struct codec, because a document written
// by one and read by the other has to mean the same thing, and the tests in
// this package hold them to it field by field:
//
//   - the key is the tag's name, or the field name lowercased;
//   - `omitempty` skips nil, zero numbers, false, empty strings, slices and
//     maps, and anything whose IsZero says so (a zero time, a zero ObjectID) —
//     but not a struct that is merely all zeros;
//   - `inline` flattens an embedded struct; `-` and unexported fields are
//     skipped;
//   - a nil slice or map is written as null, an empty one as [] or {};
//   - times are stored in UTC to the millisecond, as BSON stores them;
//   - decoding into a value that already holds data leaves fields the document
//     does not mention alone, merges into an existing map, and replaces a
//     slice — which is what the driver does, and what callers that reuse a
//     variable across a scan have been relying on.
package doccodec

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Marshaler is implemented by a type that stores itself as a JSON value it
// renders on its own: models.JSONDoc, a module's state, which is already JSON
// and is embedded as it stands.
type Marshaler interface {
	MarshalDocJSON() ([]byte, error)
}

// Unmarshaler is the reading half of Marshaler. It is handed the raw JSON of
// the value, which for a document written before this package may be a string
// holding JSON rather than the JSON itself; the implementation accepts both.
type Unmarshaler interface {
	UnmarshalDocJSON([]byte) error
}

// TextUnmarshaler is an Unmarshaler that can also be handed a string value
// already unescaped — the content, not the JSON of it. Optional: without it an
// Unmarshaler gets the quoted string.
type TextUnmarshaler interface {
	UnmarshalDocText(content []byte) error
}

type zeroer interface{ IsZero() bool }

var (
	objectIDType        = reflect.TypeOf(bson.ObjectID{})
	timeType            = reflect.TypeOf(time.Time{})
	marshalerType       = reflect.TypeOf((*Marshaler)(nil)).Elem()
	unmarshalerType     = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*TextUnmarshaler)(nil)).Elem()
	zeroerType          = reflect.TypeOf((*zeroer)(nil)).Elem()
	bsonDType           = reflect.TypeOf(bson.D{})
	bsonMType           = reflect.TypeOf(bson.M{})
	bsonAType           = reflect.TypeOf(bson.A{})
)

// field is one stored field of a struct.
type field struct {
	name      string
	key       []byte // `"name":`, pre-rendered
	index     []int  // path through inlined structs
	omitEmpty bool
	typ       reflect.Type
}

// structPlan is a struct type's stored fields, in declaration order.
type structPlan struct {
	fields []field
	byName map[string]*field
}

var plans sync.Map // reflect.Type -> *structPlan

func planFor(t reflect.Type) (*structPlan, error) {
	if p, ok := plans.Load(t); ok {
		return p.(*structPlan), nil
	}
	p := &structPlan{byName: map[string]*field{}}
	if err := collectFields(t, nil, p); err != nil {
		return nil, err
	}
	for i := range p.fields {
		f := &p.fields[i]
		if _, dup := p.byName[f.name]; dup {
			return nil, fmt.Errorf("doccodec: %s: two fields are stored as %q", t, f.name)
		}
		p.byName[f.name] = f
	}
	actual, _ := plans.LoadOrStore(t, p)
	return actual.(*structPlan), nil
}

func collectFields(t reflect.Type, prefix []int, p *structPlan) error {
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag, hasTag := sf.Tag.Lookup("bson")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		inline, omitEmpty := false, false
		for _, o := range strings.Split(opts, ",") {
			switch o {
			case "inline":
				inline = true
			case "omitempty":
				omitEmpty = true
			case "", "minsize", "truncate":
			default:
				return fmt.Errorf("doccodec: %s.%s: unsupported bson tag option %q", t, sf.Name, o)
			}
		}
		if !sf.IsExported() {
			// The driver skips an unexported field, and an embedded struct of
			// an unexported type with it, unless the struct is inlined.
			if !(sf.Anonymous && inline) {
				continue
			}
		}
		index := append(append([]int(nil), prefix...), i)
		if inline {
			ft := sf.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() != reflect.Struct {
				return fmt.Errorf("doccodec: %s.%s: only a struct can be inlined", t, sf.Name)
			}
			if sf.Type.Kind() == reflect.Pointer {
				return fmt.Errorf("doccodec: %s.%s: an inlined pointer is not supported", t, sf.Name)
			}
			if err := collectFields(ft, index, p); err != nil {
				return err
			}
			continue
		}
		if !hasTag || name == "" {
			name = strings.ToLower(sf.Name)
		}
		key := appendString(nil, name)
		key = append(key, ':')
		p.fields = append(p.fields, field{name: name, key: key, index: index, omitEmpty: omitEmpty, typ: sf.Type})
	}
	return nil
}

// isEmpty is the driver's omitempty test.
func isEmpty(v reflect.Value) bool {
	if v.Type().Implements(zeroerType) {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return true
		}
		return v.Interface().(zeroer).IsZero()
	}
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

// fieldByIndex walks an inlined path for reading, reporting false where an
// embedded pointer on the way is nil. Inlined pointers are refused when the
// plan is built, so in practice every path is all values.
func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	if len(index) == 1 {
		return v.Field(index[0])
	}
	return v.FieldByIndex(index)
}

// errSyntax is wrapped by every parse error, so a caller can tell a document
// that is not JSON from one that does not fit the model.
var errSyntax = errors.New("doccodec: invalid JSON")
