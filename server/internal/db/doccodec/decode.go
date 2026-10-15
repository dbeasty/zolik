package doccodec

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Unmarshal decodes a stored document into v, which must be a non-nil
// pointer. It reads plain JSON and the relaxed Extended JSON documents were
// stored as before this package, value by value.
func Unmarshal(data []byte, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("doccodec: Unmarshal needs a non-nil pointer, not %T", v)
	}
	d := decoder{data: data}
	d.skipSpace()
	if err := d.value(rv.Elem()); err != nil {
		return err
	}
	d.skipSpace()
	if d.off != len(d.data) {
		return d.syntax("trailing data")
	}
	return nil
}

type decoder struct {
	data []byte
	off  int
	buf  []byte // scratch for strings with escapes
}

func (d *decoder) syntax(what string) error {
	return fmt.Errorf("%w at offset %d: %s", errSyntax, d.off, what)
}

func (d *decoder) skipSpace() {
	for d.off < len(d.data) {
		switch d.data[d.off] {
		case ' ', '\t', '\n', '\r':
			d.off++
		default:
			return
		}
	}
}

func (d *decoder) peek() byte {
	d.skipSpace()
	if d.off >= len(d.data) {
		return 0
	}
	return d.data[d.off]
}

func (d *decoder) expect(c byte) error {
	if d.peek() != c {
		return d.syntax(fmt.Sprintf("expected %q", c))
	}
	d.off++
	return nil
}

func (d *decoder) literal(word string) error {
	if !bytes.HasPrefix(d.data[d.off:], []byte(word)) {
		return d.syntax("bad literal")
	}
	d.off += len(word)
	return nil
}

// str reads a JSON string. The result aliases the input when the string has no
// escapes and the scratch buffer when it does, so it is valid only until the
// next call; callers that keep it copy it (string(...)).
func (d *decoder) str() ([]byte, error) {
	if err := d.expect('"'); err != nil {
		return nil, err
	}
	start := d.off
	for d.off < len(d.data) {
		c := d.data[d.off]
		if c == '"' {
			s := d.data[start:d.off]
			d.off++
			return s, nil
		}
		if c == '\\' {
			return d.strEscaped(start)
		}
		if c < 0x20 {
			return nil, d.syntax("control character in string")
		}
		d.off++
	}
	return nil, d.syntax("unterminated string")
}

func (d *decoder) strEscaped(start int) ([]byte, error) {
	d.buf = append(d.buf[:0], d.data[start:d.off]...)
	for d.off < len(d.data) {
		// Copy the run up to the next quote, backslash or control character
		// in one append: a module state stored as a string is mostly runs.
		run := d.off
		for run < len(d.data) {
			c := d.data[run]
			if c == '"' || c == '\\' || c < 0x20 {
				break
			}
			run++
		}
		d.buf = append(d.buf, d.data[d.off:run]...)
		d.off = run
		if d.off >= len(d.data) {
			break
		}
		c := d.data[d.off]
		if c == '"' {
			d.off++
			return d.buf, nil
		}
		if c < 0x20 {
			return nil, d.syntax("control character in string")
		}
		d.off++ // the backslash
		if d.off >= len(d.data) {
			break
		}
		e := d.data[d.off]
		d.off++
		switch e {
		case '"', '\\', '/':
			d.buf = append(d.buf, e)
		case 'b':
			d.buf = append(d.buf, '\b')
		case 'f':
			d.buf = append(d.buf, '\f')
		case 'n':
			d.buf = append(d.buf, '\n')
		case 'r':
			d.buf = append(d.buf, '\r')
		case 't':
			d.buf = append(d.buf, '\t')
		case 'u':
			r, ok := d.hex4()
			if !ok {
				return nil, d.syntax("bad \\u escape")
			}
			if utf16.IsSurrogate(r) {
				r2 := utf8.RuneError
				if bytes.HasPrefix(d.data[d.off:], []byte(`\u`)) {
					save := d.off
					d.off += 2
					if lo, ok := d.hex4(); ok {
						r2 = lo
					} else {
						d.off = save
					}
				}
				if dec := utf16.DecodeRune(r, r2); dec != utf8.RuneError {
					r = dec
				} else {
					r = utf8.RuneError
				}
			}
			d.buf = utf8.AppendRune(d.buf, r)
		default:
			return nil, d.syntax("bad escape")
		}
	}
	return nil, d.syntax("unterminated string")
}

func (d *decoder) hex4() (rune, bool) {
	if d.off+4 > len(d.data) {
		return 0, false
	}
	var r rune
	for _, c := range d.data[d.off : d.off+4] {
		switch {
		case '0' <= c && c <= '9':
			c -= '0'
		case 'a' <= c && c <= 'f':
			c = c - 'a' + 10
		case 'A' <= c && c <= 'F':
			c = c - 'A' + 10
		default:
			return 0, false
		}
		r = r*16 + rune(c)
	}
	d.off += 4
	return r, true
}

// number reads a JSON number literal.
func (d *decoder) number() ([]byte, error) {
	d.skipSpace()
	start := d.off
	if d.off < len(d.data) && d.data[d.off] == '-' {
		d.off++
	}
	digits := 0
	for d.off < len(d.data) {
		c := d.data[d.off]
		if ('0' <= c && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			if '0' <= c && c <= '9' {
				digits++
			}
			d.off++
			continue
		}
		break
	}
	if digits == 0 {
		return nil, d.syntax("expected a number")
	}
	return d.data[start:d.off], nil
}

// skip passes over one value of any kind.
func (d *decoder) skip() error {
	switch d.peek() {
	case '{':
		d.off++
		if d.peek() == '}' {
			d.off++
			return nil
		}
		for {
			if _, err := d.str(); err != nil {
				return err
			}
			if err := d.expect(':'); err != nil {
				return err
			}
			if err := d.skip(); err != nil {
				return err
			}
			switch d.peek() {
			case ',':
				d.off++
			case '}':
				d.off++
				return nil
			default:
				return d.syntax("expected , or }")
			}
		}
	case '[':
		d.off++
		if d.peek() == ']' {
			d.off++
			return nil
		}
		for {
			if err := d.skip(); err != nil {
				return err
			}
			switch d.peek() {
			case ',':
				d.off++
			case ']':
				d.off++
				return nil
			default:
				return d.syntax("expected , or ]")
			}
		}
	case '"':
		_, err := d.str()
		return err
	case 't':
		return d.literal("true")
	case 'f':
		return d.literal("false")
	case 'n':
		return d.literal("null")
	default:
		_, err := d.number()
		return err
	}
}

// raw returns the bytes of the next value, unparsed.
func (d *decoder) raw() ([]byte, error) {
	d.skipSpace()
	start := d.off
	if err := d.skip(); err != nil {
		return nil, err
	}
	return d.data[start:d.off], nil
}

// wrapper reads an Extended JSON wrapper object, {"$name": value}, already
// positioned at its '{'. It returns the key, and leaves the decoder at the
// wrapped value; finishWrapper consumes the closing brace.
func (d *decoder) wrapper() (string, error) {
	if err := d.expect('{'); err != nil {
		return "", err
	}
	k, err := d.str()
	if err != nil {
		return "", err
	}
	if err := d.expect(':'); err != nil {
		return "", err
	}
	return string(k), nil
}

func (d *decoder) finishWrapper() error { return d.expect('}') }

func (d *decoder) null() bool {
	if d.peek() == 'n' && bytes.HasPrefix(d.data[d.off:], []byte("null")) {
		d.off += 4
		return true
	}
	return false
}

// value decodes the next value into v, which is settable.
func (d *decoder) value(v reflect.Value) error {
	t := v.Type()
	switch t {
	case objectIDType:
		return d.objectID(v)
	case timeType:
		return d.timeValue(v)
	}
	if v.CanAddr() && reflect.PointerTo(t).Implements(unmarshalerType) {
		if d.peek() == '"' && reflect.PointerTo(t).Implements(textUnmarshalerType) {
			// A value stored as a string of itself (every module state written
			// before this package): unescape it once, here, rather than hand
			// the raw string over to be parsed a second time.
			s, err := d.str()
			if err != nil {
				return err
			}
			return v.Addr().Interface().(TextUnmarshaler).UnmarshalDocText(s)
		}
		raw, err := d.raw()
		if err != nil {
			return err
		}
		return v.Addr().Interface().(Unmarshaler).UnmarshalDocJSON(raw)
	}
	switch t.Kind() {
	case reflect.Pointer:
		if d.null() {
			v.SetZero()
			return nil
		}
		if v.IsNil() {
			v.Set(reflect.New(t.Elem()))
		}
		return d.value(v.Elem())
	case reflect.Interface:
		if t.NumMethod() != 0 {
			return fmt.Errorf("doccodec: cannot decode into %s", t)
		}
		x, err := d.generic()
		if err != nil {
			return err
		}
		if x == nil {
			v.SetZero()
		} else {
			v.Set(reflect.ValueOf(x))
		}
		return nil
	}
	if d.null() {
		// The driver reads null as the zero value, for anything it can.
		v.SetZero()
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		return d.object(v)
	case reflect.Map:
		return d.mapValue(v)
	case reflect.Slice:
		if t == bsonDType {
			x, err := d.generic()
			if err != nil {
				return err
			}
			if dd, ok := x.(bson.D); ok {
				v.Set(reflect.ValueOf(dd))
				return nil
			}
			return fmt.Errorf("doccodec: expected a document for bson.D")
		}
		if t.Elem().Kind() == reflect.Uint8 {
			return d.bytesValue(v)
		}
		return d.slice(v)
	case reflect.Array:
		return d.array(v)
	case reflect.String:
		s, err := d.str()
		if err != nil {
			return err
		}
		v.SetString(string(s))
		return nil
	case reflect.Bool:
		switch d.peek() {
		case 't':
			v.SetBool(true)
			return d.literal("true")
		case 'f':
			v.SetBool(false)
			return d.literal("false")
		}
		return d.syntax("expected a boolean")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := d.intValue()
		if err != nil {
			return err
		}
		if v.OverflowInt(n) {
			return fmt.Errorf("doccodec: %d overflows %s", n, t)
		}
		v.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := d.intValue()
		if err != nil {
			return err
		}
		if n < 0 || v.OverflowUint(uint64(n)) {
			return fmt.Errorf("doccodec: %d overflows %s", n, t)
		}
		v.SetUint(uint64(n))
		return nil
	case reflect.Float32, reflect.Float64:
		f, err := d.floatValue()
		if err != nil {
			return err
		}
		v.SetFloat(f)
		return nil
	}
	return fmt.Errorf("doccodec: cannot decode into %s", t)
}

func (d *decoder) object(v reflect.Value) error {
	p, err := planFor(v.Type())
	if err != nil {
		return err
	}
	if err := d.expect('{'); err != nil {
		return err
	}
	if d.peek() == '}' {
		d.off++
		return nil
	}
	for {
		k, err := d.str()
		if err != nil {
			return err
		}
		f := p.byName[string(k)]
		if err := d.expect(':'); err != nil {
			return err
		}
		if f == nil {
			// The engine's own "id", and fields a newer build wrote.
			if err := d.skip(); err != nil {
				return err
			}
		} else if err := d.value(v.FieldByIndex(f.index)); err != nil {
			return fmt.Errorf("%s.%s: %w", v.Type(), f.name, err)
		}
		switch d.peek() {
		case ',':
			d.off++
		case '}':
			d.off++
			return nil
		default:
			return d.syntax("expected , or }")
		}
	}
}

func (d *decoder) mapValue(v reflect.Value) error {
	t := v.Type()
	if t.Key().Kind() != reflect.String {
		return fmt.Errorf("doccodec: map keys must be strings, not %s", t.Key())
	}
	if err := d.expect('{'); err != nil {
		return err
	}
	// Into an existing map, as the driver does: entries the document does not
	// name are kept.
	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}
	if d.peek() == '}' {
		d.off++
		return nil
	}
	elem := reflect.New(t.Elem()).Elem()
	for {
		k, err := d.str()
		if err != nil {
			return err
		}
		key := reflect.ValueOf(string(k)).Convert(t.Key())
		if err := d.expect(':'); err != nil {
			return err
		}
		elem.SetZero()
		if err := d.value(elem); err != nil {
			return err
		}
		v.SetMapIndex(key, elem)
		switch d.peek() {
		case ',':
			d.off++
		case '}':
			d.off++
			return nil
		default:
			return d.syntax("expected , or }")
		}
	}
}

func (d *decoder) slice(v reflect.Value) error {
	if err := d.expect('['); err != nil {
		return err
	}
	t := v.Type()
	// A fresh slice, as the driver does: a reused variable does not keep the
	// tail of a longer slice it held before.
	out := reflect.MakeSlice(t, 0, 4)
	if d.peek() == ']' {
		d.off++
		v.Set(out)
		return nil
	}
	for {
		out = reflect.Append(out, reflect.Zero(t.Elem()))
		if err := d.value(out.Index(out.Len() - 1)); err != nil {
			return err
		}
		switch d.peek() {
		case ',':
			d.off++
		case ']':
			d.off++
			v.Set(out)
			return nil
		default:
			return d.syntax("expected , or ]")
		}
	}
}

func (d *decoder) array(v reflect.Value) error {
	if err := d.expect('['); err != nil {
		return err
	}
	i := 0
	if d.peek() == ']' {
		d.off++
		return nil
	}
	for {
		if i < v.Len() {
			if err := d.value(v.Index(i)); err != nil {
				return err
			}
		} else if err := d.skip(); err != nil {
			return err
		}
		i++
		switch d.peek() {
		case ',':
			d.off++
		case ']':
			d.off++
			return nil
		default:
			return d.syntax("expected , or ]")
		}
	}
}

func (d *decoder) bytesValue(v reflect.Value) error {
	var enc []byte
	if d.peek() == '{' {
		// {"$binary":{"base64":"...","subType":"00"}}
		k, err := d.wrapper()
		if err != nil {
			return err
		}
		if k != "$binary" {
			return fmt.Errorf("doccodec: expected $binary, got %s", k)
		}
		var inner struct {
			Base64 string `bson:"base64"`
		}
		if err := d.object(reflect.ValueOf(&inner).Elem()); err != nil {
			return err
		}
		if err := d.finishWrapper(); err != nil {
			return err
		}
		enc = []byte(inner.Base64)
	} else {
		s, err := d.str()
		if err != nil {
			return err
		}
		enc = s
	}
	// Never nil: an empty binary value reads back empty, as the driver reads it.
	out, err := base64.StdEncoding.AppendDecode(make([]byte, 0, base64.StdEncoding.DecodedLen(len(enc))), enc)
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(out).Convert(v.Type()))
	return nil
}

func (d *decoder) objectID(v reflect.Value) error {
	if d.null() {
		v.SetZero()
		return nil
	}
	wrapped := d.peek() == '{'
	if wrapped {
		k, err := d.wrapper()
		if err != nil {
			return err
		}
		if k != "$oid" {
			return fmt.Errorf("doccodec: expected $oid, got %s", k)
		}
	}
	s, err := d.str()
	if err != nil {
		return err
	}
	var id bson.ObjectID
	if len(s) != 24 {
		return fmt.Errorf("doccodec: %q is not an ObjectID", s)
	}
	if _, err := hex.Decode(id[:], s); err != nil {
		return fmt.Errorf("doccodec: %q is not an ObjectID", s)
	}
	v.Set(reflect.ValueOf(id))
	if wrapped {
		return d.finishWrapper()
	}
	return nil
}

func (d *decoder) timeValue(v reflect.Value) error {
	if d.null() {
		v.SetZero()
		return nil
	}
	tm, err := d.timeLiteral()
	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(tm))
	return nil
}

// timeLiteral reads a time in any of the shapes one has been stored as: an
// RFC 3339 string, {"$date": "RFC 3339"}, {"$date": {"$numberLong": "ms"}}, or
// {"$date": ms}. The result is UTC to the millisecond, as the driver returns.
func (d *decoder) timeLiteral() (time.Time, error) {
	if d.peek() == '"' {
		s, err := d.str()
		if err != nil {
			return time.Time{}, err
		}
		tm, err := time.Parse(time.RFC3339Nano, string(s))
		if err != nil {
			return time.Time{}, err
		}
		return storedTime(tm), nil
	}
	k, err := d.wrapper()
	if err != nil {
		return time.Time{}, err
	}
	if k != "$date" {
		return time.Time{}, fmt.Errorf("doccodec: expected $date, got %s", k)
	}
	var ms int64
	switch d.peek() {
	case '"':
		s, err := d.str()
		if err != nil {
			return time.Time{}, err
		}
		tm, err := time.Parse(time.RFC3339Nano, string(s))
		if err != nil {
			return time.Time{}, err
		}
		ms = tm.UnixMilli()
	default:
		if ms, err = d.intValue(); err != nil {
			return time.Time{}, err
		}
	}
	if err := d.finishWrapper(); err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(ms).UTC(), nil
}

// intValue reads an integer: a number, or one of the Extended JSON wrappers.
// A whole number written with a fraction (2.0) is accepted, as the driver
// accepts it; a real fraction is refused rather than truncated.
func (d *decoder) intValue() (int64, error) {
	if d.peek() == '{' {
		k, err := d.wrapper()
		if err != nil {
			return 0, err
		}
		s, err := d.str()
		if err != nil {
			return 0, err
		}
		var n int64
		switch k {
		case "$numberLong", "$numberInt":
			n, err = strconv.ParseInt(string(s), 10, 64)
		case "$numberDouble":
			var f float64
			if f, err = strconv.ParseFloat(string(s), 64); err == nil {
				n, err = wholeFloat(f)
			}
		default:
			err = fmt.Errorf("doccodec: expected a number, got %s", k)
		}
		if err != nil {
			return 0, err
		}
		return n, d.finishWrapper()
	}
	lit, err := d.number()
	if err != nil {
		return 0, err
	}
	if n, err := strconv.ParseInt(string(lit), 10, 64); err == nil {
		return n, nil
	}
	f, err := strconv.ParseFloat(string(lit), 64)
	if err != nil {
		return 0, err
	}
	return wholeFloat(f)
}

func wholeFloat(f float64) (int64, error) {
	if f != math.Trunc(f) || f < math.MinInt64 || f >= math.MaxInt64 {
		return 0, fmt.Errorf("doccodec: %v is not an integer", f)
	}
	return int64(f), nil
}

func (d *decoder) floatValue() (float64, error) {
	if d.peek() == '{' {
		k, err := d.wrapper()
		if err != nil {
			return 0, err
		}
		s, err := d.str()
		if err != nil {
			return 0, err
		}
		var f float64
		switch k {
		case "$numberDouble", "$numberLong", "$numberInt":
			switch string(s) {
			case "Infinity":
				f = math.Inf(1)
			case "-Infinity":
				f = math.Inf(-1)
			case "NaN":
				f = math.NaN()
			default:
				f, err = strconv.ParseFloat(string(s), 64)
			}
		default:
			err = fmt.Errorf("doccodec: expected a number, got %s", k)
		}
		if err != nil {
			return 0, err
		}
		return f, d.finishWrapper()
	}
	lit, err := d.number()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(string(lit), 64)
}

// generic decodes a value into the types the driver produces for an empty
// interface: bson.D for a document, bson.A for an array, int32 or int64 for a
// whole number, float64, string, bool, nil — and, for a legacy document, an
// ObjectID or a bson.DateTime where Extended JSON spelled one. A plain-JSON document
// carries no type for those, so they come back as the strings they were
// written as; the one caller that decodes into a map (the replica's client
// view) renders both the same way.
func (d *decoder) generic() (any, error) {
	switch d.peek() {
	case '{':
		return d.genericObject()
	case '[':
		d.off++
		out := bson.A{}
		if d.peek() == ']' {
			d.off++
			return out, nil
		}
		for {
			x, err := d.generic()
			if err != nil {
				return nil, err
			}
			out = append(out, x)
			switch d.peek() {
			case ',':
				d.off++
			case ']':
				d.off++
				return out, nil
			default:
				return nil, d.syntax("expected , or ]")
			}
		}
	case '"':
		s, err := d.str()
		if err != nil {
			return nil, err
		}
		return string(s), nil
	case 't':
		return true, d.literal("true")
	case 'f':
		return false, d.literal("false")
	case 'n':
		return nil, d.literal("null")
	}
	lit, err := d.number()
	if err != nil {
		return nil, err
	}
	if n, err := strconv.ParseInt(string(lit), 10, 64); err == nil {
		if n >= math.MinInt32 && n <= math.MaxInt32 {
			return int32(n), nil
		}
		return n, nil
	}
	return strconv.ParseFloat(string(lit), 64)
}

func (d *decoder) genericObject() (any, error) {
	start := d.off
	d.off++ // '{'
	out := bson.D{}
	if d.peek() == '}' {
		d.off++
		return out, nil
	}
	first := true
	for {
		k, err := d.str()
		if err != nil {
			return nil, err
		}
		key := string(k)
		if err := d.expect(':'); err != nil {
			return nil, err
		}
		if first && len(key) > 1 && key[0] == '$' {
			// An Extended JSON wrapper: rewind and read it as the type it names.
			save := d.off
			d.off = start
			if x, ok, err := d.genericWrapper(key); ok || err != nil {
				return x, err
			}
			d.off = save
		}
		first = false
		x, err := d.generic()
		if err != nil {
			return nil, err
		}
		out = append(out, bson.E{Key: key, Value: x})
		switch d.peek() {
		case ',':
			d.off++
		case '}':
			d.off++
			return out, nil
		default:
			return nil, d.syntax("expected , or }")
		}
	}
}

func (d *decoder) genericWrapper(key string) (any, bool, error) {
	switch key {
	case "$oid":
		var id bson.ObjectID
		err := d.objectID(reflect.ValueOf(&id).Elem())
		return id, true, err
	case "$date":
		// The driver reads a date it has no type for as a DateTime, not a time.
		tm, err := d.timeLiteral()
		return bson.NewDateTimeFromTime(tm), true, err
	case "$numberLong":
		n, err := d.intValue()
		return n, true, err
	case "$numberInt":
		n, err := d.intValue()
		return int32(n), true, err
	case "$numberDouble":
		f, err := d.floatValue()
		return f, true, err
	}
	return nil, false, nil
}
