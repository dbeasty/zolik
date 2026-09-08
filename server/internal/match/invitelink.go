package match

import (
	"net/url"
	"strings"
)

// InvitePath is the client route a shared invite link points at.
//
// It lives here, on the server, rather than in each client, because the link
// is minted here: a host copies a URL out of one client and pastes it to
// somebody who will open it in another. If the two disagreed about the path
// the link would be a 404 for exactly the person it was written for.
//
// The shape is deliberately short and typeable — "/join/ABC123" — because it
// is read out loud over the phone as often as it is pasted, which is also why
// the join code it carries stays six characters.
const InvitePath = "/join/"

// SetInviteBaseURL tells the runtime where the outside world reaches this
// deployment, so it can mint links that work when they arrive somewhere else.
//
// It is configuration rather than something derived from the request that
// asked, for the same reason the OAuth redirect URI is: the Host header is
// attacker-controlled, and a link built from it would be a link to wherever
// the attacker said. An unset base means no link is offered at all — clients
// fall back to reading the join code out, which is what they did before links
// existed.
func (m *Manager) SetInviteBaseURL(base string) {
	m.inviteBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

// InviteURL is the link that seats whoever opens it at this table.
//
// Empty when there is nothing honest to say — no configured base, or a match
// with no join code — because a half-built URL in a "copy this" box is worse
// than no box at all.
func (m *Manager) InviteURL(joinCode string) string {
	if m == nil || m.inviteBaseURL == "" || joinCode == "" {
		return ""
	}
	return m.inviteBaseURL + InvitePath + url.PathEscape(joinCode)
}
