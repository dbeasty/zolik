package doccodec

import (
	"encoding/base64"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Marshal renders v as the plain JSON the kdb backend stores.
func Marshal(v any) ([]byte, error) {
	buf := make([]byte, 0, 512)
	rv := reflect.ValueOf(v)
	// A stored row is always a document, so the driver writes a nil map at the
	// top as an empty one.
	if rv.Kind() == reflect.Map && rv.IsNil() {
		return append(buf, "{}"...), nil
	}
	return appendValue(buf, rv)
}

// storedTime is a time as BSON keeps one: UTC, to the millisecond. Writing it
// at the same precision means a value read back is the value Mongo would have
// read back, and a client sees the same string it saw before.
func storedTime(t time.Time) time.Time {
	return time.UnixMilli(t.UnixMilli()).UTC()
}

func appendValue(b []byte, v reflect.Value) ([]byte, error) {
	if !v.IsValid() {
		return append(b, "null"...), nil
	}
	t := v.Type()
	switch t {
	case objectIDType:
		id := v.Interface().(bson.ObjectID)
		b = append(b, '"')
		b = append(b, id.Hex()...)
		return append(b, '"'), nil
	case timeType:
		tm := storedTime(v.Interface().(time.Time))
		b = append(b, '"')
		b = tm.AppendFormat(b, time.RFC3339Nano)
		return append(b, '"'), nil
	}
	if t.Implements(marshalerType) {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return append(b, "null"...), nil
		}
		raw, err := v.Interface().(Marshaler).MarshalDocJSON()
		if err != nil {
			return nil, err
		}
		return append(b, raw...), nil
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			return append(b, "true"...), nil
		}
		return append(b, "false"...), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(b, v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.AppendUint(b, v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return appendFloat(b, v.Float(), t.Bits()), nil
	case reflect.String:
		return appendString(b, v.String()), nil
	case reflect.Interface:
		if v.IsNil() {
			return append(b, "null"...), nil
		}
		return appendValue(b, v.Elem())
	case reflect.Pointer:
		if v.IsNil() {
			return append(b, "null"...), nil
		}
		return appendValue(b, v.Elem())
	case reflect.Struct:
		return appendStruct(b, v)
	case reflect.Map:
		return appendMap(b, v)
	case reflect.Slice:
		if v.IsNil() {
			return append(b, "null"...), nil
		}
		if t == bsonDType {
			return appendD(b, v.Interface().(bson.D))
		}
		if t.Elem().Kind() == reflect.Uint8 {
			b = append(b, '"')
			b = base64.StdEncoding.AppendEncode(b, v.Bytes())
			return append(b, '"'), nil
		}
		return appendArray(b, v)
	case reflect.Array:
		return appendArray(b, v)
	}
	return nil, fmt.Errorf("doccodec: cannot store a %s", t)
}

func appendStruct(b []byte, v reflect.Value) ([]byte, error) {
	p, err := planFor(v.Type())
	if err != nil {
		return nil, err
	}
	b = append(b, '{')
	first := true
	for i := range p.fields {
		f := &p.fields[i]
		fv := fieldByIndex(v, f.index)
		if f.omitEmpty && isEmpty(fv) {
			continue
		}
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, f.key...)
		if b, err = appendValue(b, fv); err != nil {
			return nil, fmt.Errorf("%s.%s: %w", v.Type(), f.name, err)
		}
	}
	return append(b, '}'), nil
}

// appendMap writes a map's entries in key order, so a document's bytes do not
// change from one write to the next when its content has not.
func appendMap(b []byte, v reflect.Value) ([]byte, error) {
	if v.IsNil() {
		return append(b, "null"...), nil
	}
	if v.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("doccodec: map keys must be strings, not %s", v.Type().Key())
	}
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	b = append(b, '{')
	var err error
	for i, k := range keys {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendString(b, k.String())
		b = append(b, ':')
		if b, err = appendValue(b, v.MapIndex(k)); err != nil {
			return nil, err
		}
	}
	return append(b, '}'), nil
}

func appendD(b []byte, d bson.D) ([]byte, error) {
	b = append(b, '{')
	var err error
	for i, e := range d {
		if i > 0 {
			b = append(b, ',')
		}
		b = appendString(b, e.Key)
		b = append(b, ':')
		if b, err = appendValue(b, reflect.ValueOf(e.Value)); err != nil {
			return nil, err
		}
	}
	return append(b, '}'), nil
}

func appendArray(b []byte, v reflect.Value) ([]byte, error) {
	b = append(b, '[')
	var err error
	for i := 0; i < v.Len(); i++ {
		if i > 0 {
			b = append(b, ',')
		}
		if b, err = appendValue(b, v.Index(i)); err != nil {
			return nil, err
		}
	}
	return append(b, ']'), nil
}

// appendFloat writes a float as JSON can carry it. JSON has no NaN or
// infinity; those keep Extended JSON's spelling, which the reader accepts.
func appendFloat(b []byte, f float64, bits int) []byte {
	switch {
	case math.IsNaN(f):
		return append(b, `{"$numberDouble":"NaN"}`...)
	case math.IsInf(f, 1):
		return append(b, `{"$numberDouble":"Infinity"}`...)
	case math.IsInf(f, -1):
		return append(b, `{"$numberDouble":"-Infinity"}`...)
	}
	return strconv.AppendFloat(b, f, 'g', -1, bits)
}

const hexDigits = "0123456789abcdef"

// appendString writes s as a JSON string. Invalid UTF-8 becomes U+FFFD, as it
// does in encoding/json, so what is stored is always valid JSON.
func appendString(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c >= 0x20 && c != '"' && c != '\\' {
				i++
				continue
			}
			b = append(b, s[start:i]...)
			switch c {
			case '"', '\\':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b = append(b, s[start:i]...)
			b = append(b, `�`...)
			i += size
			start = i
			continue
		}
		// U+2028 and U+2029 are valid JSON but end a line in JavaScript.
		if r == ' ' || r == ' ' {
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '2', '0', '2', hexDigits[r&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}
