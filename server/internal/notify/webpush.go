package notify

import (
	"bytes"
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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// WebPushSender delivers to browsers: RFC 8291 message encryption (aes128gcm)
// under RFC 8292 VAPID authentication.
//
// Written here rather than taken from a library because it is two short RFCs
// and the whole of it is below — a dependency would be more code to audit
// than the thing it replaced.
type WebPushSender struct {
	// PublicKey is the uncompressed P-256 point, base64url, exactly as the
	// browser is handed it for PushManager.subscribe.
	PublicKey string
	// PrivateKey is the 32-byte scalar, base64url.
	PrivateKey string
	// Subject is the contact push services use when something goes wrong: a
	// mailto: or https: URL.
	Subject string
	Client  *http.Client

	key *ecdsa.PrivateKey
}

// NewWebPushSender parses and checks the VAPID key pair once, at startup, so
// a mistyped key stops the server instead of failing every push.
func NewWebPushSender(public, private, subject string) (*WebPushSender, error) {
	d, err := b64(private)
	if err != nil {
		return nil, fmt.Errorf("VAPID private key: %w", err)
	}
	key, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), d)
	if err != nil {
		return nil, fmt.Errorf("VAPID private key: %w", err)
	}
	pub, err := key.PublicKey.Bytes()
	if err != nil {
		return nil, err
	}
	if want, err := b64(public); err != nil || !bytes.Equal(want, pub) {
		return nil, errors.New("VAPID public key does not match the private key")
	}
	if subject == "" {
		return nil, errors.New("VAPID subject (a mailto: or https: URL) is required")
	}
	return &WebPushSender{PublicKey: public, PrivateKey: private, Subject: subject, key: key}, nil
}

// GenerateVAPIDKeys makes a fresh key pair, base64url encoded.
func GenerateVAPIDKeys() (public, private string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	pub, err := key.PublicKey.Bytes()
	if err != nil {
		return "", "", err
	}
	priv, err := key.Bytes()
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(pub), enc.EncodeToString(priv), nil
}

func b64(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if out, err := enc.DecodeString(s); err == nil {
			return out, nil
		}
	}
	return nil, errors.New("not base64")
}

func (s *WebPushSender) Send(ctx context.Context, d Device, p Push) error {
	if d.Subscription == nil || d.Subscription.Endpoint == "" {
		return ErrDeviceGone
	}
	payload, err := json.Marshal(map[string]any{
		"type":   p.Data["type"],
		"invite": p.Data["invite"],
		"id":     p.Data["id"],
		"title":  p.Title,
		"body":   p.Body,
		"url":    p.URL,
		"tag":    p.Tag,
		"silent": p.Silent,
	})
	if err != nil {
		return err
	}
	body, err := encryptWebPush(payload, d.Subscription.Keys.P256dh, d.Subscription.Keys.Auth)
	if err != nil {
		return err
	}
	auth, err := s.vapidHeader(d.Subscription.Endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Subscription.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", strconv.Itoa(int(time.Hour/time.Second)))
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", auth)
	if p.Tag != "" {
		// A pending message with the same topic is replaced rather than
		// delivered alongside, so a phone that was offline while a table
		// opened and closed wakes to the revocation, not to both.
		req.Header.Set("Topic", topicFor(p.Tag))
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrDeviceGone
	case resp.StatusCode >= 300:
		return fmt.Errorf("web push: %s answered %d", req.URL.Host, resp.StatusCode)
	}
	return nil
}

// topicFor squeezes a tag into the 32 URL-safe characters a Topic may hold.
func topicFor(tag string) string {
	sum := sha256.Sum256([]byte(tag))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:32]
}

func (s *WebPushSender) vapidHeader(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"aud": u.Scheme + "://" + u.Host,
		"exp": time.Now().Add(12 * time.Hour).Unix(),
		"sub": s.Subject,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(s.key)
	if err != nil {
		return "", err
	}
	return "vapid t=" + token + ", k=" + s.PublicKey, nil
}

// encryptWebPush is RFC 8291 §3.4 with a single aes128gcm record (RFC 8188).
func encryptWebPush(plaintext []byte, p256dh, authSecret string) ([]byte, error) {
	uaPublicRaw, err := b64(p256dh)
	if err != nil {
		return nil, fmt.Errorf("subscription p256dh: %w", err)
	}
	secret, err := b64(authSecret)
	if err != nil {
		return nil, fmt.Errorf("subscription auth: %w", err)
	}
	curve := ecdh.P256()
	uaPublic, err := curve.NewPublicKey(uaPublicRaw)
	if err != nil {
		return nil, fmt.Errorf("subscription p256dh: %w", err)
	}
	asPrivate, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := asPrivate.ECDH(uaPublic)
	if err != nil {
		return nil, err
	}
	asPublic := asPrivate.PublicKey().Bytes()

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	keyInfo := append(append([]byte("WebPush: info\x00"), uaPublicRaw...), asPublic...)
	ikm, err := hkdf.Key(sha256.New, shared, secret, string(keyInfo), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// 0x02 marks the last (and only) record; no padding after it.
	record := gcm.Seal(nil, nonce, append(plaintext, 0x02), nil)

	const recordSize = 4096
	if len(record) > recordSize-16 {
		return nil, errors.New("web push payload too large for one record")
	}
	header := make([]byte, 0, 16+4+1+len(asPublic))
	header = append(header, salt...)
	header = binary.BigEndian.AppendUint32(header, recordSize)
	header = append(header, byte(len(asPublic)))
	header = append(header, asPublic...)
	return append(header, record...), nil
}
