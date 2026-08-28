package db

// Engine names for the storage-backend feature flag. Mongo is the engine
// every existing deployment runs; KDB is the embedded in-process engine that
// needs no database server at all.
const (
	EngineMongo = "mongo"
	EngineKDB   = "kdb"
)
