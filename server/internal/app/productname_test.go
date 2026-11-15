package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMailFromNameDefaultsToTheProduct(t *testing.T) {
	t.Setenv("SMTP_FROM_NAME", "")
	if got := LoadConfig().SMTP.FromName; got != "Jokerless" {
		t.Fatalf("SMTP From name = %q, want Jokerless", got)
	}
}

// JWT_REFRESH_SECRET was generated on every deploy and listed in both env
// examples, and nothing ever read it: refresh tokens are database rows. A
// secret that protects nothing invites someone to rotate it and believe
// sessions were revoked.
func TestNoRefreshTokenSecretIsConfigured(t *testing.T) {
	for _, p := range []string{"../../.env.example", "../../../deploy/env.production.example", "../../../scripts/deploy.sh", "config.go"} {
		raw, err := os.ReadFile(filepath.FromSlash(p))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "JWT_REFRESH_SECRET") {
			t.Errorf("%s still configures JWT_REFRESH_SECRET", p)
		}
	}
}
