package notify

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// A Web Push subscription's endpoint is a URL the browser hands over, and the
// server then POSTs to it on its own: whoever registers a "device" chooses
// where this server's requests go. Left unchecked that is a blind SSRF - a
// way to make the server knock on its own admin port, the cloud metadata
// address or anything else on its network. Real push services are public
// HTTPS hosts, so that is all an endpoint may be.

// ErrPushEndpoint is an endpoint this server will not send to.
var ErrPushEndpoint = errors.New("web push: endpoint must be a public https URL")

// CheckPushEndpoint reports whether raw is somewhere a push may be sent:
// https, a host name or a public address, no credentials. A name is resolved
// only at send time, where the dialer checks the address it actually
// connects to (see publicOnlyDialer), so a name that later resolves inward is
// caught there.
func CheckPushEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return ErrPushEndpoint
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return ErrPushEndpoint
	}
	if ip, err := netip.ParseAddr(host); err == nil && !publicAddr(ip) {
		return ErrPushEndpoint
	}
	return nil
}

// cgnat is the shared address space (RFC 6598), private in all but name.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// publicAddr reports whether ip is an address on the public internet.
func publicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() && ip.IsGlobalUnicast() && !ip.IsPrivate() &&
		!ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !cgnat.Contains(ip) &&
		!(ip.Is4() && ip.As4()[0] == 0)
}

// publicOnlyDialer refuses, at connect time, any address that is not public.
// Checking here rather than only on the URL is what defeats a host name that
// resolves (or is rebound) to an inside address.
var publicOnlyDialer = &net.Dialer{
	Timeout: 5 * time.Second,
	Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !publicAddr(ip) {
			return fmt.Errorf("%w (dialled %s)", ErrPushEndpoint, address)
		}
		return nil
	},
}

// noRedirects keeps a push on the endpoint it was addressed to. A push
// service answers a push; it never redirects one, and following a 3xx would
// be a way around every check above.
func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// pushClient is the client a WebPushSender uses when it is given none: no
// proxy from the environment, public addresses only, no redirects.
func pushClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return publicOnlyDialer.DialContext(ctx, network, addr)
			},
			TLSHandshakeTimeout: 5 * time.Second,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: noRedirects,
	}
}
