package app

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitRequestBodyStopsAnOversizedBody(t *testing.T) {
	var read int
	var readErr error
	h := LimitRequestBody(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		read, readErr = len(b), err
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 4096))))
	var tooBig *http.MaxBytesError
	if !errors.As(readErr, &tooBig) {
		t.Fatalf("reading 4096 bytes through a 1024 cap: err = %v, want *http.MaxBytesError", readErr)
	}
	if read > 1024 {
		t.Fatalf("handler read %d bytes, past the cap", read)
	}
}

func TestLimitRequestBodyLeavesAnHonestBodyAlone(t *testing.T) {
	var got string
	h := LimitRequestBody(1024)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}`)))
	if got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
	// And a request with no body at all (a GET, a WebSocket upgrade) is untouched.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
