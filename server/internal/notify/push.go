package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ErrDeviceGone is what a sender reports when the push service says the
// device will never be reachable again — an uninstalled app, a revoked
// browser subscription. The caller deletes the row; retrying would only
// repeat the answer.
var ErrDeviceGone = errors.New("notify: device no longer registered")

// Push is one notification, already worded for the device's language.
//
// Data is what the app receives alongside it: the same object the socket
// carries, so a client that is open when the push lands treats the two as one
// invite. Tag groups notifications about the same table, so a later
// revocation can replace or close the one already showing.
type Push struct {
	Title string
	Body  string
	URL   string
	Tag   string
	Data  map[string]any
	// Silent marks a push that only updates what is showing — a revocation —
	// and should never itself interrupt anybody.
	Silent bool
}

// Sender delivers one push to one device.
type Sender interface {
	Send(ctx context.Context, d Device, p Push) error
}

// Senders routes each device to the service that reaches it. A kind with no
// sender configured is skipped quietly: a deployment without VAPID keys
// simply has no browser push, and says so in /notify/config.
type Senders map[DeviceKind]Sender

func (s Senders) Send(ctx context.Context, d Device, p Push) error {
	sender, ok := s[d.Kind]
	if !ok || sender == nil {
		return nil
	}
	return sender.Send(ctx, d, p)
}

// LogSender writes each push to the log instead of sending it — development,
// and any deployment that has not configured a push service.
type LogSender struct{}

func (LogSender) Send(_ context.Context, d Device, p Push) error {
	log.Printf("notify push (log only) kind=%s device=%s title=%q body=%q url=%s silent=%v",
		d.Kind, d.ID, p.Title, p.Body, p.URL, p.Silent)
	return nil
}

// ExpoSender posts to Expo's push service, which forwards to APNs and FCM.
//
// One request per push rather than batching: an invite goes to a handful of
// devices, and a per-device answer is what tells us which token died.
type ExpoSender struct {
	Endpoint    string // defaults to Expo's production endpoint
	AccessToken string // optional; required once "enhanced security" is on in the Expo project
	Client      *http.Client
}

const expoEndpoint = "https://exp.host/--/api/v2/push/send"

func (s ExpoSender) Send(ctx context.Context, d Device, p Push) error {
	if d.Token == "" {
		return ErrDeviceGone
	}
	msg := map[string]any{
		"to":        d.Token,
		"data":      p.Data,
		"channelId": "invites",
		"priority":  "high",
	}
	if p.Silent {
		// A data-only message: the app hears about it if it is running, and
		// the phone shows nothing. Expo cannot retract a notification already
		// displayed, so this is the best a revocation can do natively.
		msg["_contentAvailable"] = true
		msg["priority"] = "normal"
	} else {
		msg["title"] = p.Title
		msg["body"] = p.Body
		msg["sound"] = "default"
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	endpoint := s.Endpoint
	if endpoint == "" {
		endpoint = expoEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.AccessToken)
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
	var out struct {
		Data struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details struct {
				Error string `json:"error"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("expo push: status %d, unreadable answer: %w", resp.StatusCode, err)
	}
	if out.Data.Status == "error" {
		if out.Data.Details.Error == "DeviceNotRegistered" {
			return ErrDeviceGone
		}
		return fmt.Errorf("expo push: %s (%s)", out.Data.Message, out.Data.Details.Error)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("expo push: status %d", resp.StatusCode)
	}
	return nil
}
