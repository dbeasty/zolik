package db

import (
	"go.mongodb.org/mongo-driver/v2/bson"
)

// The KDB backend stores each document as Extended JSON produced from the
// same structs the Mongo driver persists. Marshalling through bson rather
// than encoding/json is load-bearing: the models carry bson tags naming their
// stored fields, and several fields are `json:"-"` precisely because they
// must never leave the server on the wire — a plain json.Marshal would both
// rename and drop them. Relaxed (non-canonical) form keeps the payload
// human-readable in kdb-cli.

// MarshalDoc renders a model struct as the bytes the KDB backend stores.
func MarshalDoc(v any) ([]byte, error) {
	return bson.MarshalExtJSON(v, false, false)
}

// UnmarshalDoc decodes bytes stored by MarshalDoc back into a model struct.
func UnmarshalDoc(data []byte, v any) error {
	return bson.UnmarshalExtJSON(data, false, v)
}
