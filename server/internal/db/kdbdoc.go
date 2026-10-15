package db

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"go.mongodb.org/mongo-driver/v2/bson"

	"zolik/server/internal/db/doccodec"
)

// What the KDB backend stores a model struct as.
//
// The Mongo backend never comes through here: its driver encodes the structs
// to binary BSON and sends that to its server. The KDB backend stores JSON
// text, and for a long time that text was MongoDB Extended JSON made by the
// same driver ({"$oid": ...}, {"$date": ...}) — the bson tags name the stored
// fields, and several fields are `json:"-"` precisely because they must never
// leave the server on the wire, so encoding/json could not stand in. Parsing
// that text was the largest single cost in a loaded server's profile.
//
// doccodec reads the same bson tags and writes plain JSON. It reads both
// shapes, value by value, so every database written before it keeps working:
// rows convert as they are rewritten, and an old row is never a problem.
//
// KDB_DOC_WRITE_FORMAT chooses what is written:
//
//	json     plain JSON (the default)
//	extjson  the Extended JSON earlier builds wrote, and the only shape a
//	         build from before doccodec can read
//
// extjson is an escape hatch, not a mode to run in: it exists for the one case
// where a node that cannot read plain JSON has to keep reading what this one
// writes — rolling a server back past doccodec, or a client that cannot be
// updated. It does not convert rows already written as plain JSON.

// DocWriteFormat names what MarshalDoc writes.
type DocWriteFormat int32

const (
	DocWriteJSON DocWriteFormat = iota
	DocWriteExtJSON
)

func (f DocWriteFormat) String() string {
	if f == DocWriteExtJSON {
		return "extjson"
	}
	return "json"
}

// ParseDocWriteFormat reads KDB_DOC_WRITE_FORMAT's spellings. Empty is the
// default.
func ParseDocWriteFormat(s string) (DocWriteFormat, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "json":
		return DocWriteJSON, nil
	case "extjson":
		return DocWriteExtJSON, nil
	}
	return 0, fmt.Errorf("kdb: KDB_DOC_WRITE_FORMAT=%q: want \"extjson\" or \"json\"", s)
}

var (
	docWrite     atomic.Int32
	docWriteOnce sync.Once
)

// SetDocWriteFormat chooses what MarshalDoc writes from now on. The server
// sets it once at startup from the environment (KDBStorageFromEnv); tests set
// it to exercise both shapes.
func SetDocWriteFormat(f DocWriteFormat) {
	docWriteOnce.Do(func() {}) // an explicit choice outranks the lazy read
	docWrite.Store(int32(f))
}

// CurrentDocWriteFormat is what MarshalDoc writes.
func CurrentDocWriteFormat() DocWriteFormat {
	docWriteOnce.Do(func() {
		// A process that never opened the database through KDBStorageFromEnv
		// — a tool, a test — still honours the variable. A value it cannot
		// read leaves the default here; the server refuses to start on it.
		if f, err := ParseDocWriteFormat(os.Getenv("KDB_DOC_WRITE_FORMAT")); err == nil {
			docWrite.Store(int32(f))
		}
	})
	return DocWriteFormat(docWrite.Load())
}

// MarshalDoc renders a model struct as the bytes the KDB backend stores.
func MarshalDoc(v any) ([]byte, error) {
	if CurrentDocWriteFormat() == DocWriteExtJSON {
		return bson.MarshalExtJSON(v, false, false)
	}
	return doccodec.Marshal(v)
}

// UnmarshalDoc decodes bytes stored by MarshalDoc — in either shape, or a mix
// of both — back into a model struct.
func UnmarshalDoc(data []byte, v any) error {
	return doccodec.Unmarshal(data, v)
}
