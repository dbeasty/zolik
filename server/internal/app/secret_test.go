package app

import (
	"strings"
	"testing"

	"zolik/server/internal/db"
)

// A non-local deployment without its own signing key must not start: the
// fallback key is in the repository, so anyone could sign in as anyone.
func TestNewRefusesProductionWithoutAccessSecret(t *testing.T) {
	t.Setenv("JWT_ACCESS_SECRET", "")
	a, err := New(Config{Env: "production", DBEngine: db.EngineKDB, KDBPath: t.TempDir()})
	if err == nil {
		_ = a.Close(t.Context())
		t.Fatal("app.New started in production with no JWT_ACCESS_SECRET")
	}
	if !strings.Contains(err.Error(), "JWT_ACCESS_SECRET") {
		t.Fatalf("error does not name the variable: %v", err)
	}
}
