package models

import "testing"

func TestSanitizeAvatar(t *testing.T) {
	kept := []string{"p-violet", "m-steel", "a", "m-verdigris", "x0-9"}
	for _, s := range kept {
		if got := SanitizeAvatar(s); got != s {
			t.Errorf("SanitizeAvatar(%q) = %q, want it kept", s, got)
		}
	}

	// Everything a client has no business sending. None of these is an error;
	// they simply stop being a slug, and the face is derived instead.
	dropped := []string{
		"",
		"P-Violet",                     // upper case
		"p_violet",                     // underscore
		"p violet",                     // space
		"<script>",                     // markup
		"p-violet\n",                   // control character
		"ünicode",                      // outside the agreed alphabet
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaa", // past the ceiling
	}
	for _, s := range dropped {
		if got := SanitizeAvatar(s); got != "" {
			t.Errorf("SanitizeAvatar(%q) = %q, want it dropped", s, got)
		}
	}
}
