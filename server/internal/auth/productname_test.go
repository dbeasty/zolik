package auth_test

import (
	"net/http"
	"strings"
	"testing"

	"zolik/server/internal/auth"
)

// The sign-in mail used to say "Žolíky", the module's name, to people who had
// signed up for Jokerless.
func TestSignInMailIsBrandedJokerlessByDefault(t *testing.T) {
	h := newTestHarnessWith(t, func(d *auth.Deps) { d.AppName = "" })
	if res := h.do(http.MethodPost, "/auth/email/start", "", map[string]any{"email": "brand@example.com"}); res.status != http.StatusOK {
		t.Fatalf("email/start: %d %s", res.status, res.raw)
	}
	h.mailer.mu.Lock()
	defer h.mailer.mu.Unlock()
	m := h.mailer.sent[len(h.mailer.sent)-1]
	if !strings.HasPrefix(m.Subject, "Jokerless ") || !strings.Contains(m.Text, "Jokerless") {
		t.Fatalf("mail: subject %q body %q", m.Subject, m.Text)
	}
	if strings.Contains(m.Subject+m.Text, "Žolíky") {
		t.Fatal("the mail still says Žolíky")
	}
}
