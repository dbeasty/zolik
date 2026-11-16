package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func portOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestHealthcheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ok.Close()
	if code := healthcheck(portOf(t, ok)); code != 0 {
		t.Errorf("a healthy server: exit %d, want 0", code)
	}

	sick := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer sick.Close()
	if code := healthcheck(portOf(t, sick)); code == 0 {
		t.Error("a 503 from /healthz: exit 0, want non-zero")
	}

	gone := portOf(t, sick)
	sick.Close()
	if code := healthcheck(gone); code == 0 {
		t.Error("nothing listening: exit 0, want non-zero")
	}
}
