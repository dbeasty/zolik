package models

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// JSONDoc is a JSON document the runtime stores without reading: a module's
// state, one move's action.
//
// Stored as text. As a plain json.RawMessage the driver writes it as BSON
// binary, which extended JSON spells as base64 inside a {"$binary": ...}
// wrapper: a state a third larger, and a 71-byte action stored as 243 bytes.
// As text it costs only the escaping of its quotes, and nothing parses its
// numbers — the seeds six modules keep in their state are near 2^62.
type JSONDoc json.RawMessage

// MarshalBSONValue writes the document as a string, or null when empty.
func (d JSONDoc) MarshalBSONValue() (byte, []byte, error) {
	if d == nil {
		return byte(bson.TypeNull), nil, nil
	}
	return byte(bson.TypeString), bsoncore.AppendString(nil, string(d)), nil
}

// UnmarshalBSONValue reads a string or null.
func (d *JSONDoc) UnmarshalBSONValue(typ byte, data []byte) error {
	switch bson.Type(typ) {
	case bson.TypeNull, bson.TypeUndefined:
		*d = nil
		return nil
	case bson.TypeString:
		s, _, ok := bsoncore.ReadString(data)
		if !ok {
			return fmt.Errorf("models: JSONDoc: malformed string")
		}
		*d = JSONDoc(s)
		return nil
	default:
		return fmt.Errorf("models: JSONDoc: cannot decode BSON type %s", bson.Type(typ))
	}
}

// MarshalDocJSON is how the kdb backend stores the document (db/doccodec): as
// the JSON it is, embedded in the stored row, rather than as a string of it.
//
// Only an object or an array is embedded, and only when it is valid JSON;
// anything else is stored as a string, which is how every document was stored
// before. That keeps the reading side unambiguous: a stored string is always
// the document's text, and a stored object or array is always the document.
// A state that happened to be a bare JSON string would otherwise be
// indistinguishable from a legacy row.
func (d JSONDoc) MarshalDocJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	if embeddable(d) {
		return d, nil
	}
	return json.Marshal(string(d))
}

func embeddable(d []byte) bool {
	// Exactly an object or an array, with nothing around it: what is embedded
	// must read back byte for byte, and the reader takes a value without the
	// whitespace on either side of it.
	if len(d) < 2 {
		return false
	}
	first, last := d[0], d[len(d)-1]
	if !(first == '{' && last == '}') && !(first == '[' && last == ']') {
		return false
	}
	return json.Valid(d)
}

// UnmarshalDocJSON reads what MarshalDocJSON wrote, and what was stored before
// it: a string holding the document's text, an embedded object or array, or
// null.
func (d *JSONDoc) UnmarshalDocJSON(raw []byte) error {
	switch {
	case len(raw) == 0 || string(raw) == "null":
		*d = nil
	case raw[0] == '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fmt.Errorf("models: JSONDoc: %w", err)
		}
		*d = JSONDoc(s)
	default:
		*d = append(JSONDoc(nil), raw...)
	}
	return nil
}

// UnmarshalDocText is UnmarshalDocJSON for a string the reader has already
// unescaped: the document's text itself.
func (d *JSONDoc) UnmarshalDocText(content []byte) error {
	*d = append(JSONDoc{}, content...)
	return nil
}

// MarshalJSON writes the document as itself, as json.RawMessage does. A named
// type does not inherit json.RawMessage's methods, and without these a JSONDoc
// on the wire would be base64.
func (d JSONDoc) MarshalJSON() ([]byte, error) {
	return json.RawMessage(d).MarshalJSON()
}

// UnmarshalJSON keeps the document verbatim.
func (d *JSONDoc) UnmarshalJSON(b []byte) error {
	return (*json.RawMessage)(d).UnmarshalJSON(b)
}
