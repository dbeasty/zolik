package geo

import "testing"

func TestCountryForTimeZone(t *testing.T) {
	for tz, want := range map[string]string{
		"Europe/Prague":     "CZ",
		"Europe/Bratislava": "SK",
		"America/New_York":  "US",
		"Europe/Kiev":       "UA",
		"Europe/Kyiv":       "UA",
		"UTC":               "",
		"Etc/GMT+2":         "",
		"":                  "",
	} {
		if got := CountryForTimeZone(tz); got != want {
			t.Errorf("CountryForTimeZone(%q) = %q, want %q", tz, got, want)
		}
	}
}

func TestSaneTimeZone(t *testing.T) {
	for s, want := range map[string]bool{
		"Europe/Prague":          true,
		"America/Port-au-Prince": true,
		"Etc/GMT+2":              true,
		"":                       false,
		"<script>":               false,
		"Europe/Prague; drop":    false,
	} {
		if got := SaneTimeZone(s); got != want {
			t.Errorf("SaneTimeZone(%q) = %v, want %v", s, got, want)
		}
	}
}
