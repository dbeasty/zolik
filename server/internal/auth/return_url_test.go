package auth

import "testing"

func TestReturnURLAllowedByStopsAtAnOriginBoundary(t *testing.T) {
	cases := []struct {
		candidate, allowed string
		want               bool
	}{
		{"https://jokerless.com", "https://jokerless.com", true},
		{"https://jokerless.com/", "https://jokerless.com", true},
		{"https://jokerless.com/auth/done?x=1", "https://jokerless.com", true},
		{"https://jokerless.com?x=1", "https://jokerless.com", true},
		{"https://jokerless.com#x", "https://jokerless.com", true},
		{"clientreactnative://auth", "clientreactnative://", true},
		{"https://play.limidus.com/app/x", "https://play.limidus.com/app/", true},

		// The reason the function exists.
		{"https://jokerless.com.evil.example/steal", "https://jokerless.com", false},
		{"https://jokerless.comevil.example", "https://jokerless.com", false},
		{"https://jokerless.com:8443/", "https://jokerless.com", false},
		{"https://jokerless.com@evil.example/", "https://jokerless.com", false},

		{"https://jokerless.org/", "https://jokerless.com", false},
		{"https://jokerless.com/", "", false},
	}
	for _, c := range cases {
		if got := returnURLAllowedBy(c.candidate, c.allowed); got != c.want {
			t.Errorf("returnURLAllowedBy(%q, %q) = %v, want %v", c.candidate, c.allowed, got, c.want)
		}
	}
}

func TestResolveReturnToAcceptsEveryDeclaredDomain(t *testing.T) {
	h := &Handlers{allowedReturnURLs: []string{
		"https://jokerless.com", "https://jokerless.org", "https://play.limidus.com", "clientreactnative://",
	}}
	for _, ok := range []string{
		"https://jokerless.com/", "https://jokerless.org/auth/done", "https://play.limidus.com/", "clientreactnative://auth",
	} {
		if _, err := h.resolveReturnTo(ok); err != nil {
			t.Errorf("resolveReturnTo(%q) refused: %v", ok, err)
		}
	}
	if _, err := h.resolveReturnTo("https://jokerless.org.evil.example/"); err == nil {
		t.Error("resolveReturnTo accepted a look-alike host")
	}
	if got, _ := h.resolveReturnTo(""); got != "https://jokerless.com" {
		t.Errorf("empty returnTo = %q, want the first declared entry", got)
	}
}
