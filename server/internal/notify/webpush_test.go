package notify

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// decryptWebPush is the browser's half of RFC 8291, so the test checks the
// encryption against the specification rather than against itself.
func decryptWebPush(t *testing.T, body []byte, ua *ecdh.PrivateKey, secret []byte) []byte {
	t.Helper()
	salt := body[:16]
	rs := binary.BigEndian.Uint32(body[16:20])
	idlen := int(body[20])
	asPublicRaw := body[21 : 21+idlen]
	record := body[21+idlen:]
	if rs != 4096 || idlen != 65 {
		t.Fatalf("header: rs=%d idlen=%d", rs, idlen)
	}
	asPublic, err := ecdh.P256().NewPublicKey(asPublicRaw)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := ua.ECDH(asPublic)
	if err != nil {
		t.Fatal(err)
	}
	keyInfo := append(append([]byte("WebPush: info\x00"), ua.PublicKey().Bytes()...), asPublicRaw...)
	ikm, _ := hkdf.Key(sha256.New, shared, secret, string(keyInfo), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, record, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain[len(plain)-1] != 0x02 {
		t.Fatal("missing last-record delimiter")
	}
	return plain[:len(plain)-1]
}

func TestWebPushEncryptsForTheSubscriptionAndSignsWithVAPID(t *testing.T) {
	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	enc := base64.RawURLEncoding

	var gotBody []byte
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	sender, err := NewWebPushSender(pub, priv, "mailto:ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	d := Device{ID: "d", Kind: DeviceWebPush, Subscription: &Subscription{Endpoint: srv.URL + "/push/abc"}}
	d.Subscription.Keys.P256dh = enc.EncodeToString(ua.PublicKey().Bytes())
	d.Subscription.Keys.Auth = enc.EncodeToString(secret)

	err = sender.Send(context.Background(), d, Push{
		Title: "Anna opened a table", Body: "Canasta", URL: "https://x/join/ABC", Tag: "invite:1",
		Data: map[string]any{"type": "table_invite"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(decryptWebPush(t, gotBody, ua, secret), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["title"] != "Anna opened a table" || payload["type"] != "table_invite" || payload["tag"] != "invite:1" {
		t.Fatalf("payload: %v", payload)
	}
	if gotHeaders.Get("Content-Encoding") != "aes128gcm" || len(gotHeaders.Get("Topic")) != 32 {
		t.Fatalf("headers: %v", gotHeaders)
	}

	// The VAPID token verifies against the public key and names the origin.
	auth := gotHeaders.Get("Authorization")
	tok := strings.TrimSuffix(strings.TrimPrefix(auth, "vapid t="), ", k="+pub)
	raw, _ := enc.DecodeString(pub)
	key := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(raw[1:33]), Y: new(big.Int).SetBytes(raw[33:])}
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(tok, claims, func(*jwt.Token) (any, error) { return key, nil }); err != nil {
		t.Fatalf("vapid token: %v", err)
	}
	if claims["aud"] != srv.URL {
		t.Fatalf("aud %v, want %s", claims["aud"], srv.URL)
	}
}

func TestWebPushGoneMeansDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()
	pub, priv, _ := GenerateVAPIDKeys()
	sender, _ := NewWebPushSender(pub, priv, "mailto:x@example.com")
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	d := Device{Subscription: &Subscription{Endpoint: srv.URL}}
	d.Subscription.Keys.P256dh = base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes())
	d.Subscription.Keys.Auth = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	if err := sender.Send(context.Background(), d, Push{}); err != ErrDeviceGone {
		t.Fatalf("want ErrDeviceGone, got %v", err)
	}
}

func TestVAPIDKeysMustMatch(t *testing.T) {
	pub, _, _ := GenerateVAPIDKeys()
	_, priv, _ := GenerateVAPIDKeys()
	if _, err := NewWebPushSender(pub, priv, "mailto:x@example.com"); err == nil {
		t.Fatal("a mismatched pair must be refused at startup")
	}
}

func TestExpoDeviceNotRegisteredMeansDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg map[string]any
		_ = json.NewDecoder(r.Body).Decode(&msg)
		if msg["to"] != "ExponentPushToken[x]" || msg["channelId"] != "invites" {
			t.Errorf("message: %v", msg)
		}
		_, _ = w.Write([]byte(`{"data":{"status":"error","message":"gone","details":{"error":"DeviceNotRegistered"}}}`))
	}))
	defer srv.Close()
	err := ExpoSender{Endpoint: srv.URL}.Send(context.Background(), Device{Token: "ExponentPushToken[x]"}, Push{Title: "t"})
	if err != ErrDeviceGone {
		t.Fatalf("want ErrDeviceGone, got %v", err)
	}
}
