package match_test

import (
	"strings"
	"testing"

	"zolik/server/internal/match"
	"zolik/server/internal/models"
	"zolik/server/internal/module"
	"zolik/server/internal/prsi"
)

// The invite link is the whole feature in one string: whatever this returns is
// what a host pastes into a chat, and what somebody else opens on a device
// that has never heard of this server. So the cases worth pinning are the ones
// where a wrong answer is worse than no answer — a link built from nothing, or
// a link that quietly loses the code it exists to carry.

func newLinkManager(t *testing.T, base string) *match.Manager {
	t.Helper()
	m := match.NewManager(nil, module.NewRegistry(prsi.New()), nil)
	m.SetInviteBaseURL(base)
	return m
}

func TestInviteURLBuildsAShareableLink(t *testing.T) {
	m := newLinkManager(t, "https://play.limidus.com")
	if got, want := m.InviteURL("ABC123"), "https://play.limidus.com/join/ABC123"; got != want {
		t.Fatalf("InviteURL = %q, want %q", got, want)
	}
}

// A trailing slash on the configured base is the single most likely way for
// this to be misconfigured, and "https://host//join/X" is a 404 on some
// proxies. Normalising once here means no caller has to think about it.
func TestInviteURLNormalisesTheBase(t *testing.T) {
	for _, base := range []string{
		"https://play.limidus.com/",
		"https://play.limidus.com//",
		"  https://play.limidus.com  ",
	} {
		m := newLinkManager(t, base)
		if got, want := m.InviteURL("ABC123"), "https://play.limidus.com/join/ABC123"; got != want {
			t.Errorf("base %q: InviteURL = %q, want %q", base, got, want)
		}
	}
}

// No base configured is the ordinary state of a development build and of every
// test in this package. It must produce nothing at all rather than a relative
// path or a link to "/join/ABC123" on some host nobody named — a client shows
// the code instead, which is what it always did.
func TestInviteURLIsEmptyWithoutABase(t *testing.T) {
	m := newLinkManager(t, "")
	if got := m.InviteURL("ABC123"); got != "" {
		t.Fatalf("InviteURL without a base = %q, want empty", got)
	}
}

func TestInviteURLIsEmptyWithoutAJoinCode(t *testing.T) {
	m := newLinkManager(t, "https://play.limidus.com")
	if got := m.InviteURL(""); got != "" {
		t.Fatalf("InviteURL without a code = %q, want empty", got)
	}
}

// The state message is where both clients read the link from, so the field has
// to survive the trip through BuildStateMsg — a link the server mints and then
// forgets to send is the same as no link.
func TestStateMessageCarriesTheInviteLink(t *testing.T) {
	m := newLinkManager(t, "https://play.limidus.com")
	msg := m.BuildStateMsg(models.Match{
		ModuleID: "prsi",
		Status:   "lobby",
		JoinCode: "ZZ9000",
	}, "viewer")

	if got, want := msg.InviteURL, "https://play.limidus.com/join/ZZ9000"; got != want {
		t.Fatalf("state InviteURL = %q, want %q", got, want)
	}
	// The code rides along unchanged: the link is an addition, not a
	// replacement, and a host who wants to read six characters out loud still
	// can.
	if msg.JoinCode != "ZZ9000" {
		t.Fatalf("state JoinCode = %q, want the code to survive alongside the link", msg.JoinCode)
	}
}

func TestStateMessageOmitsTheLinkWhenUnconfigured(t *testing.T) {
	m := newLinkManager(t, "")
	msg := m.BuildStateMsg(models.Match{ModuleID: "prsi", Status: "lobby", JoinCode: "ZZ9000"}, "viewer")
	if msg.InviteURL != "" {
		t.Fatalf("state InviteURL = %q, want empty on an unconfigured server", msg.InviteURL)
	}
}

// The path is shared with two clients that build routes against it, so it is
// asserted rather than left to a comment: changing it is a breaking change for
// every link already pasted into somebody's chat history.
func TestInvitePathIsTheClientRoute(t *testing.T) {
	if match.InvitePath != "/join/" {
		t.Fatalf("InvitePath = %q — links already shared point at /join/", match.InvitePath)
	}
	if !strings.HasPrefix(match.InvitePath, "/") || !strings.HasSuffix(match.InvitePath, "/") {
		t.Fatalf("InvitePath %q must be a rooted path ending in a separator", match.InvitePath)
	}
}
