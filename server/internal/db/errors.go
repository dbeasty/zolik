package db

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ErrDuplicateKey is what a non-Mongo backend wraps when a write violates a
// unique constraint. Mongo surfaces the same condition as its own server
// error, so callers must classify through IsDuplicateKey rather than
// comparing against either form directly.
var ErrDuplicateKey = errors.New("duplicate key")

// ErrNotFound is what a non-Mongo backend returns when a lookup finds
// nothing, standing in for mongo.ErrNoDocuments. Packages that already have
// their own not-found sentinel (auth, stats) keep it; this one exists for the
// repositories whose Mongo implementations return the driver error raw.
var ErrNotFound = errors.New("not found")

// IsDuplicateKey reports whether err is a unique-constraint violation from
// any backend. It is the engine-neutral replacement for
// mongo.IsDuplicateKeyError at call sites that turn "already taken" into a
// 409 rather than a 500.
func IsDuplicateKey(err error) bool {
	return errors.Is(err, ErrDuplicateKey) || mongo.IsDuplicateKeyError(err)
}

// IsNotFound reports whether err means "no such document" from any backend.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, mongo.ErrNoDocuments)
}
