// Package geo answers "roughly where is this player" without looking anybody
// up anywhere.
//
// The signal is the device's time zone, which every client already knows and
// sends with its session refresh. It names a country for almost every zone
// people actually set, costs no IP database and no third-party call, and says
// nothing finer than the country — which is all the operator console asks.
//
// zones_gen.go is the tz database's zone.tab, regenerated with:
//
//	grep -v '^#' /usr/share/zoneinfo/zone.tab | awk -F'\t' '{printf "\t\"%s\": \"%s\",\n", $3, $1}' | sort
package geo

import "strings"

// aliases are the old names still in use on devices that have not updated
// their tz data. zone.tab lists only canonical zones, so without these a
// phone that still says Europe/Kiev would land in "unknown".
var aliases = map[string]string{
	"Europe/Kiev":                      "UA",
	"Europe/Uzhgorod":                  "UA",
	"Europe/Zaporozhye":                "UA",
	"Asia/Calcutta":                    "IN",
	"Asia/Saigon":                      "VN",
	"Asia/Katmandu":                    "NP",
	"Asia/Rangoon":                     "MM",
	"America/Buenos_Aires":             "AR",
	"America/Indianapolis":             "US",
	"America/Louisville":               "US",
	"America/Montreal":                 "CA",
	"Atlantic/Faeroe":                  "FO",
	"Pacific/Truk":                     "FM",
	"Pacific/Ponape":                   "FM",
	"Australia/ACT":                    "AU",
	"Australia/NSW":                    "AU",
	"Australia/Canberra":               "AU",
	"US/Eastern":                       "US",
	"US/Central":                       "US",
	"US/Mountain":                      "US",
	"US/Pacific":                       "US",
	"Canada/Eastern":                   "CA",
	"Canada/Pacific":                   "CA",
	"Europe/Belfast":                   "GB",
	"GB":                               "GB",
	"Eire":                             "IE",
	"Asia/Istanbul":                    "TR",
	"Turkey":                           "TR",
	"Japan":                            "JP",
	"Singapore":                        "SG",
	"Hongkong":                         "HK",
	"Israel":                           "IL",
	"Poland":                           "PL",
	"Portugal":                         "PT",
	"America/Argentina/ComodRivadavia": "AR",
}

// CountryForTimeZone returns the ISO 3166 alpha-2 country an IANA zone
// belongs to, or "" for a zone that names no country ("UTC", "Etc/GMT+2")
// or one this table does not know.
func CountryForTimeZone(tz string) string {
	tz = strings.TrimSpace(tz)
	if c, ok := zoneCountry[tz]; ok {
		return c
	}
	return aliases[tz]
}

// SaneTimeZone reports whether s has the shape of an IANA zone name, so a
// client cannot park arbitrary text on an account. Shape only: an unknown
// but well-formed zone is kept, and simply counts as unknown until the table
// learns it.
func SaneTimeZone(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '/' || r == '_' || r == '-' || r == '+':
		default:
			return false
		}
	}
	return true
}
