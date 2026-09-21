package notify

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The colon in a subject key arrives percent-encoded from every client that
// builds its URLs properly, and has to mean the same key.
func TestKeyParamUnescapes(t *testing.T) {
	var got string
	r := chi.NewRouter()
	r.Post("/notify/circle/{key}/mute", func(_ http.ResponseWriter, req *http.Request) { got = keyParam(req) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/notify/circle/guest%3Aabc/mute", nil))
	if got != "guest:abc" {
		t.Fatalf("got %q", got)
	}
}
