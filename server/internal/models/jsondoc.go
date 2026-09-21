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
