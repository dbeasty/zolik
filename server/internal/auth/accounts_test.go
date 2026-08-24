package auth

import (
	"strings"
	"testing"

	"zolik/server/internal/identity"
)

func TestSuggestUsername(t *testing.T) {
	cases := []struct {
		name   string
		claims identity.Claims
		want   string
	}{
		{
			name:   "the provider's name is preferred",
			claims: identity.Claims{Name: "Ada Lovelace", Email: "ada@example.com"},
			want:   "Ada Lovelace",
		},
		{
			name:   "falls back to the address's local part",
			claims: identity.Claims{Email: "ada.lovelace@example.com"},
			want:   "ada.lovelace",
		},
		{
			name:   "an Apple private-relay user with nothing at all still gets a name",
			claims: identity.Claims{},
			want:   "Player",
		},
		{
			name:   "control characters and punctuation are stripped",
			claims: identity.Claims{Name: "Ada <script>"},
			want:   "Ada script",
		},
		{
			name:   "a name of pure punctuation is not a name",
			claims: identity.Claims{Name: "***", Email: "real@example.com"},
			want:   "real",
		},
		{
			name:   "over-long names are trimmed rather than rejected",
			claims: identity.Claims{Name: strings.Repeat("a", 100)},
			want:   strings.Repeat("a", 24),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := suggestUsername(tc.claims); got != tc.want {
				t.Errorf("suggestUsername = %q, want %q", got, tc.want)
			}
		})
	}
}

// A guest id travels into subject keys, match records and BSON map keys, so
// anything a client sends has to be exactly the shape this server issues.
func TestSanitizeGuestIDAcceptsOnlyIssuedShapes(t *testing.T) {
	good := "0123456789abcdef0123456789abcdef"
	if got := sanitizeGuestID(good); got != good {
		t.Errorf("a well-formed guest id was rejected: %q", got)
	}

	bad := []string{
		"",
		"short",
		strings.Repeat("a", 33),
		"0123456789ABCDEF0123456789ABCDEF", // upper case is not what we mint
		"0123456789abcdef0123456789abcde!", // stray punctuation
		"$where0123456789abcdef0123456789", // a BSON operator prefix
		"user.0123456789abcdef0123456789a", // a dot would break a map key
	}
	for _, id := range bad {
		if got := sanitizeGuestID(id); got != "" {
			t.Errorf("sanitizeGuestID(%q) = %q, want it rejected", id, got)
		}
	}
}

func TestCleanUsernameKeepsLettersAcrossAlphabets(t *testing.T) {
	// The game ships in Czech as well as English; stripping non-ASCII letters
	// would mangle most Czech names.
	if got := cleanUsername("Žofie Nováková"); got != "Žofie Nováková" {
		t.Errorf("cleanUsername mangled a Czech name: %q", got)
	}
}

func TestClaimsValidity(t *testing.T) {
	if (identity.Claims{Provider: "google"}).Valid() {
		t.Error("claims with no subject were reported valid — there is nothing to key an account on")
	}
	if (identity.Claims{Subject: "s"}).Valid() {
		t.Error("claims with no provider were reported valid")
	}
	if !(identity.Claims{Provider: "google", Subject: "s"}).Valid() {
		t.Error("a provider and subject should be enough")
	}
}
