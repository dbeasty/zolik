package gamemcp

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"zolik/server/internal/module"
)

// The engine speaks in message keys and card codes; a model reads English.
// This file is the translation, and it follows the client's own rules
// (client-react-native/src/lib/labels.ts) so a model reads what a person at
// the table would: the English bundle's wording where there is one, the key's
// own shape where there is not, player ids as names and card codes as cards.

//go:generate go run ./genmessages

//go:embed messages_en.json
var messagesJSON []byte

var messages = func() map[string]string {
	var m map[string]string
	if err := json.Unmarshal(messagesJSON, &m); err != nil {
		panic("gamemcp: messages_en.json: " + err.Error())
	}
	return m
}()

var (
	placeholder = regexp.MustCompile(`\{(\w+)\}`)
	keyShaped   = regexp.MustCompile(`^[a-z][A-Za-z0-9]*(\.[A-Za-z0-9]+)+$`)
)

// namer turns player ids into the names a table uses for them.
type namer map[string]string

// humanise is labels.ts's fallback: the last segment of a key, spaced.
func humanise(key string) string {
	last := key
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		last = key[i+1:]
	}
	var b strings.Builder
	prev := rune(0)
	for _, r := range last {
		if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			b.WriteRune(' ')
		}
		if r == '_' || r == '-' {
			r = ' '
		}
		b.WriteRune(r)
		prev = r
	}
	s := strings.TrimSpace(b.String())
	if s == "" {
		return key
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// label is a key in words, with its placeholders filled.
func label(key string, params map[string]string) string {
	if key == "" {
		return ""
	}
	tmpl, ok := messages[key]
	if !ok {
		return humanise(key)
	}
	return placeholder.ReplaceAllStringFunc(tmpl, func(whole string) string {
		if v, ok := params[whole[1:len(whole)-1]]; ok {
			return v
		}
		return whole
	})
}

// token is one value the server sent, as a reader should see it.
func (n namer) token(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		if name, ok := n[x]; ok {
			return name
		}
		if isCardCode(x) {
			return cardText(x)
		}
		if keyShaped.MatchString(x) {
			return label(x, nil)
		}
		return x
	case []string:
		return n.list(x)
	case []any:
		out := make([]string, len(x))
		for i, e := range x {
			out[i] = n.token(e)
		}
		sep := ", "
		if allCards(x) {
			sep = " "
		}
		return strings.Join(out, sep)
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprint(int64(x))
		}
		return fmt.Sprint(x)
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + " " + n.token(x[k])
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

func (n namer) list(xs []string) string {
	out := make([]string, len(xs))
	cards := true
	for i, x := range xs {
		out[i] = n.token(x)
		if !isCardCode(x) {
			cards = false
		}
	}
	if cards {
		return strings.Join(out, " ")
	}
	return strings.Join(out, ", ")
}

func allCards(xs []any) bool {
	if len(xs) == 0 {
		return false
	}
	for _, x := range xs {
		s, ok := x.(string)
		if !ok || !isCardCode(s) {
			return false
		}
	}
	return true
}

// fact is labels.ts's factText: the key's wording, and the value after it
// unless the wording already places it.
func (n namer) fact(f module.Fact) string {
	params := map[string]string{}
	for k, v := range f.Params {
		params[k] = n.token(v)
	}
	value := ""
	if f.Value != "" {
		value = n.token(f.Value)
		params["value"] = value
	}
	text := label(f.LabelKey, params)
	if tmpl, ok := messages[f.LabelKey]; ok && placeholder.MatchString(tmpl) {
		return text
	}
	if value == "" {
		return text
	}
	return text + " " + value
}

func (n namer) facts(fs []module.Fact) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		if s := n.fact(f); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// --- cards ------------------------------------------------------------------

var suitGlyph = map[byte]string{'H': "♥", 'D': "♦", 'C': "♣", 'S': "♠"}

// isCardCode is cards.ts's: a rank and a suit, or a joker — or one of Last
// Card's own codes ("C-7", "T-S", "W4").
func isCardCode(s string) bool {
	if strings.HasPrefix(s, "JOKER") || lastCardText(s) != "" {
		return true
	}
	return len(s) == 2 && strings.IndexByte("A23456789TJQK", s[0]) >= 0 && strings.IndexByte("HDCS", s[1]) >= 0
}

// cardText is a card as it reads in a sentence: "7♠", "10♦", "JK".
func cardText(c string) string {
	if strings.HasPrefix(c, "JOKER") {
		return "JK"
	}
	if t := lastCardText(c); t != "" {
		return t
	}
	if !isCardCode(c) {
		return c
	}
	rank := string(c[0])
	if rank == "T" {
		rank = "10"
	}
	return rank + suitGlyph[c[1]]
}

var lastCardColours = map[string]string{"C": "coral", "T": "teal", "V": "violet", "A": "amber"}
var lastCardFaces = map[string]string{"S": "skip", "R": "reverse", "D": "draw-two"}

// lastCardText names a Last Card code in words a model reads easily —
// "coral 7", "teal skip", "wild draw-four" — or "" for anything else.
func lastCardText(c string) string {
	switch c {
	case "W":
		return "wild"
	case "W4":
		return "wild draw-four"
	}
	colour, face, ok := strings.Cut(c, "-")
	name, isColour := lastCardColours[colour]
	if !ok || !isColour {
		return ""
	}
	if f, isAction := lastCardFaces[face]; isAction {
		return name + " " + f
	}
	if len(face) == 1 && face[0] >= '0' && face[0] <= '9' {
		return name + " " + face
	}
	return ""
}

func cardsText(cs []string) string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = cardText(c)
	}
	return strings.Join(out, " ")
}

const rankOrder = "23456789TJQKA"

// bySuit is a hand grouped by suit, low to high, jokers last: the layout a
// player sorts a rummy hand into, and the one in which a model can see a run.
func bySuit(cs []string) string {
	groups := map[byte][]string{}
	var jokers, other []string
	for _, c := range cs {
		switch {
		case strings.HasPrefix(c, "JOKER"):
			jokers = append(jokers, "JK")
		case lastCardText(c) != "":
			// Last Card is not a rummy hand: no runs to show, and its names
			// already say the colour.
			other = append(other, lastCardText(c))
		case isCardCode(c):
			groups[c[1]] = append(groups[c[1]], c)
		default:
			other = append(other, c)
		}
	}
	var parts []string
	for _, s := range []byte("SHDC") {
		g := groups[s]
		if len(g) == 0 {
			continue
		}
		sort.SliceStable(g, func(i, j int) bool {
			return strings.IndexByte(rankOrder, g[i][0]) < strings.IndexByte(rankOrder, g[j][0])
		})
		ranks := make([]string, len(g))
		for i, c := range g {
			ranks[i] = strings.TrimSuffix(cardText(c), suitGlyph[s])
		}
		parts = append(parts, suitGlyph[s]+" "+strings.Join(ranks, " "))
	}
	if len(jokers) > 0 {
		parts = append(parts, strings.Join(jokers, " "))
	}
	parts = append(parts, other...)
	return strings.Join(parts, " | ")
}

func viewCards(cv []module.CardView) []string {
	out := make([]string, 0, len(cv))
	for _, c := range cv {
		if c.FaceDown || c.Card == "" {
			out = append(out, "??")
			continue
		}
		out = append(out, c.Card)
	}
	return out
}
