package models

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/x/bsonx/bsoncore"
)

// JSONDoc is a JSON document the runtime stores without reading: a module's
// state, one action, a checkpoint's board.
//
// It exists for how it is stored. As a plain json.RawMessage the driver
// writes it as BSON binary, which extended JSON then spells as base64 inside a
// {"$binary": ...} wrapper — state grew by a third, and a 71-byte action was
// stored as 243 bytes. As text it costs only the escaping of its quotes.
//
// Reading accepts every form a stored document can hold: binary (everything
// written before this type, and every historical version the store keeps,
// which BoardAfter still decodes), text, and null.
type JSONDoc json.RawMessage

// jsonDocWritesText selects the stored form: text, now that the release
// before this one reads it.
var jsonDocWritesText = true

// SetJSONDocWritesText switches the stored form and returns the previous
// setting, so a test can write as one release and read back as another.
func SetJSONDocWritesText(on bool) (was bool) {
	was, jsonDocWritesText = jsonDocWritesText, on
	return was
}

// MarshalBSONValue writes the document as text, or as binary while
// jsonDocWritesText is off. Empty is null, as a nil json.RawMessage was.
func (d JSONDoc) MarshalBSONValue() (byte, []byte, error) {
	if d == nil {
		return byte(bson.TypeNull), nil, nil
	}
	if jsonDocWritesText {
		return byte(bson.TypeString), bsoncore.AppendString(nil, string(d)), nil
	}
	return byte(bson.TypeBinary), bsoncore.AppendBinary(nil, bson.TypeBinaryGeneric, d), nil
}

// UnmarshalBSONValue reads binary, text or null.
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
	case bson.TypeBinary:
		_, b, _, ok := bsoncore.ReadBinary(data)
		if !ok {
			return fmt.Errorf("models: JSONDoc: malformed binary")
		}
		*d = append(JSONDoc(nil), b...)
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
