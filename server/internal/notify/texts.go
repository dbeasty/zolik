package notify

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// texts.json holds the few sentences the operating system shows on the
// server's behalf — a push is rendered by the phone or the browser, not by the
// app, so it cannot be worded from the client's bundle at display time.
//
// It is generated from those same bundles (scripts/gen-notify-texts.js), and a
// client test fails if it drifts, so the words in a push are the words the app
// would have used.
//
//go:embed texts.json
var textsJSON []byte

var texts = func() map[string]map[string]string {
	var out map[string]map[string]string
	if err := json.Unmarshal(textsJSON, &out); err != nil {
		panic("notify: texts.json: " + err.Error())
	}
	return out
}()

// text words key in locale, falling back to English and then to the key, and
// fills {name} placeholders from params.
func text(locale, key string, params map[string]string) string {
	s := lookup(locale, key)
	for k, v := range params {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

func lookup(locale, key string) string {
	for _, l := range []string{locale, baseLocale(locale), "en"} {
		if s, ok := texts[l][key]; ok && s != "" {
			return s
		}
	}
	return key
}

// baseLocale turns "pt-BR" or "de_AT" into the bundle name it falls back to.
func baseLocale(l string) string {
	l = strings.ToLower(l)
	if i := strings.IndexAny(l, "-_"); i > 0 {
		return l[:i]
	}
	return l
}

// gameName is a module's name in locale, or its id when nobody has worded it
// — which is also what the client shows for a game it has never heard of.
func gameName(locale, moduleID string) string {
	key := "module." + moduleID
	if s := lookup(locale, key); s != key {
		return s
	}
	return moduleID
}
