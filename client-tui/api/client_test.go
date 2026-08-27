package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// GetVersion is what the menu renders beside the client's own build (see
// ui.Root.loadServerBuild) — pinning its wire shape here means a server-side
// rename of "version"/"commit" breaks a fast, obvious test instead of showing
// up as a silently blank line in the TUI.
func TestGetVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"1.1.1.2","commit":"7feb025"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	build, err := c.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion() error = %v", err)
	}
	if build.Version != "1.1.1.2" {
		t.Errorf("Version = %q, want 1.1.1.2", build.Version)
	}
	if build.Commit != "7feb025" {
		t.Errorf("Commit = %q, want 7feb025", build.Commit)
	}
}
