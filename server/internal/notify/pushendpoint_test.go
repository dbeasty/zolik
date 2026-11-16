package notify

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCheckPushEndpointAcceptsOnlyPublicHTTPS(t *testing.T) {
	good := []string{
		"https://fcm.googleapis.com/fcm/send/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/x",
		"https://web.push.apple.com/QW",
		"https://8.8.8.8/push",
	}
	for _, e := range good {
		if err := CheckPushEndpoint(e); err != nil {
			t.Errorf("%s refused: %v", e, err)
		}
	}
	bad := []string{
		"http://fcm.googleapis.com/fcm/send/abc", // not https
		"ftp://example.com/x",
		"https://localhost/x",
		"https://admin.localhost/x",
		"https://printer.local/x",
		"https://metadata.google.internal/computeMetadata/v1/",
		"https://127.0.0.1:8091/admin",
		"https://[::1]/x",
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.5/x",
		"https://192.168.13.13/x",
		"https://172.16.0.1/x",
		"https://100.64.0.1/x",
		"https://0.0.0.0/x",
		"https://[fd00::1]/x",
		"https://[::ffff:127.0.0.1]/x",
		"https://user:pw@fcm.googleapis.com/x",
		"not a url",
		"",
	}
	for _, e := range bad {
		if err := CheckPushEndpoint(e); err == nil {
			t.Errorf("%q accepted", e)
		}
	}
}

func TestRegisterDeviceRefusesAnInsideEndpoint(t *testing.T) {
	f := newFixture(t)
	for _, e := range []string{"http://169.254.169.254/latest/meta-data/", "https://127.0.0.1:8091/admin"} {
		_, err := f.svc.RegisterDevice(context.Background(), bob, DeviceRegistration{
			Kind: DeviceWebPush, Subscription: &Subscription{Endpoint: e},
		})
		if code(err) != "PUSH_UNAVAILABLE" {
			t.Errorf("%s: want PUSH_UNAVAILABLE, got %v", e, err)
		}
	}
	if ds, _ := f.repo.DevicesFor(context.Background(), KeyFor(bob)); len(ds) != 0 {
		t.Fatalf("an inside endpoint was stored: %v", ds)
	}
}

func testDevice(endpoint string) Device {
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	d := Device{Subscription: &Subscription{Endpoint: endpoint}}
	d.Subscription.Keys.P256dh = base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes())
	d.Subscription.Keys.Auth = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	return d
}

// A device row stored before registration checked endpoints is not a way
// past the check: the sender refuses it without making the request.
func TestWebPushNeverPostsToAnInsideEndpoint(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	pub, priv, _ := GenerateVAPIDKeys()
	sender, _ := NewWebPushSender(pub, priv, "mailto:x@example.com")
	if err := sender.Send(context.Background(), testDevice(srv.URL+"/internal"), Push{}); !errors.Is(err, ErrDeviceGone) {
		t.Fatalf("want ErrDeviceGone, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the server POSTed to a loopback http endpoint")
	}
}

// The default client checks the address it actually connects to, so a name
// that resolves inward is refused at the dial.
func TestPushClientRefusesToDialAnInsideAddress(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()
	_, err := pushClient().Get(srv.URL)
	if !errors.Is(err, ErrPushEndpoint) {
		t.Fatalf("dialling %s: %v, want ErrPushEndpoint", srv.URL, err)
	}
}

func TestWebPushDoesNotFollowRedirects(t *testing.T) {
	var followed atomic.Int32
	inside := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		followed.Add(1)
	}))
	defer inside.Close()
	push := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inside.URL+"/admin", http.StatusTemporaryRedirect)
	}))
	defer push.Close()

	pub, priv, _ := GenerateVAPIDKeys()
	sender, _ := NewWebPushSender(pub, priv, "mailto:x@example.com")
	sender.Client, sender.allowPrivate = push.Client(), true
	if err := sender.Send(context.Background(), testDevice(push.URL), Push{}); err == nil {
		t.Fatal("a redirected push was reported delivered")
	}
	if followed.Load() != 0 {
		t.Fatal("the push followed a redirect")
	}
}
