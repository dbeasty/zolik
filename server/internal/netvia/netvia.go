// Package netvia says which way a connection reached a table hosted on a
// device: from the device itself, over the room's Wi-Fi, over Bluetooth, or
// across the internet through the cloud's relay.
//
// It is a label for people, not a security boundary. A phone-hosted table
// shows each seat's route so everybody can see who goes if the internet does,
// and nothing is allowed or refused on the strength of it.
//
// The listeners mark the connections they accept (With): the device's own
// loopback listener as Self, the room's listener as WiFi. A tunnelled guest —
// Bluetooth or the relay — has its sockets dialled on the device's loopback by
// the tunnel itself, which says which route it is with Header; that header is
// believed on the loopback listener only, where nothing but this device can
// reach. A server in the cloud marks nothing, and its seats show no route.
package netvia

import (
	"context"
	"net/http"
)

// The routes a seat can be reached by.
const (
	Self      = "self"
	WiFi      = "wifi"
	Bluetooth = "bluetooth"
	Internet  = "internet"
)

// Header carries a tunnelled socket's route over the host's own loopback.
const Header = "X-Zolik-Via"

type key struct{}

// With marks a connection's context with the route it arrived by.
func With(ctx context.Context, via string) context.Context {
	return context.WithValue(ctx, key{}, via)
}

// From is the route a context was marked with, or "".
func From(ctx context.Context) string {
	v, _ := ctx.Value(key{}).(string)
	return v
}

// Of is the route a request arrived by: its listener's mark, or, on this
// device's own loopback, the route a tunnel says it carried.
func Of(r *http.Request) string {
	v := From(r.Context())
	if v == Self {
		switch h := r.Header.Get(Header); h {
		case Bluetooth, Internet, WiFi:
			return h
		}
	}
	return v
}
