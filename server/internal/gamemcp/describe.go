package gamemcp

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"zolik/server/internal/module"
)

// describer turns actions into the short phrases a move list shows:
// "discard 7♠", "take 8♥ from the pile, then lay meld 8♥ 8♠ 8♦",
// "raise to 60". It reads the board it is given for what an action points
// at — the meld a card goes onto, the pile's top card — and nothing else.
type describer struct {
	names  namer
	vm     module.ViewModel
	offers []module.ActionOffer
}

func (d describer) steps(steps []module.Action) string {
	parts := make([]string, len(steps))
	for i, a := range steps {
		parts[i] = d.action(a)
	}
	return strings.Join(parts, ", then ")
}

func (d describer) action(a module.Action) string {
	cards := cardsText(a.Cards)
	with := func(s string) string {
		if cards == "" {
			return s
		}
		return s + " " + cards
	}
	var out string
	switch a.Verb {
	case "draw":
		if strings.Contains(a.OfferID, "discard") || strings.Contains(a.OfferID, "pile") || a.Target == "discard_pile" {
			if cards == "" {
				cards = cardText(d.pileTop())
			}
			out = "take " + cards + " from the pile"
			if n := d.pileAbove(a.Cards); n > 0 {
				out += " (and the " + plural(n, "card") + " above it)"
			}
		} else {
			out = "draw from the stock"
		}
	case "discard":
		out = with("discard")
	case "lay_meld":
		out = with("lay meld")
	case "lay_off":
		out = with("lay off") + d.onto(a.Target)
	case "swap_joker":
		out = "swap " + cards + " for the joker" + strings.Replace(d.onto(a.Target), " onto", " in", 1)
	case "take_pile":
		out = "take the pile"
		if cards != "" {
			out += " with " + cards + " from hand"
		}
		out += d.onto(a.Target)
	case "take_top":
		out = "take the top card of the pile" + d.onto(a.Target)
	case "raise", "bet":
		if v, ok := a.Params["amount"]; ok {
			out = a.Verb + " to " + v
		} else {
			out = a.Verb
		}
	default:
		out = with(strings.ReplaceAll(a.Verb, "_", " "))
		if a.Target != "" {
			out += d.onto(a.Target)
		}
	}
	for _, k := range sortedKeys(a.Params) {
		if k == "amount" && (a.Verb == "raise" || a.Verb == "bet") {
			continue
		}
		out += " (" + humanise(k) + " " + d.names.token(a.Params[k]) + ")"
	}
	return out
}

// onto names the group an action targets, from the board.
func (d describer) onto(target string) string {
	if target == "" {
		return ""
	}
	for _, z := range d.vm.Zones {
		for _, g := range z.Groups {
			if g.ID != target {
				continue
			}
			whose := ""
			if z.OwnerID != "" {
				whose = d.names.token(z.OwnerID) + "'s "
			}
			return " onto " + whose + "meld " + cardsText(g.Cards) + " [" + g.ID + "]"
		}
	}
	// Not on the board yet: laid earlier in the same move.
	return " onto meld [" + target + "]"
}

func (d describer) pile() []string {
	for _, z := range d.vm.Zones {
		if z.Kind == module.ZonePile {
			return viewCards(z.Cards)
		}
	}
	return nil
}

func (d describer) pileTop() string {
	p := d.pile()
	if len(p) == 0 {
		return "the top card"
	}
	return p[len(p)-1]
}

// pileAbove is how many cards lie above the deepest copy of the named card —
// the engine takes that copy and everything on it.
func (d describer) pileAbove(cards []string) int {
	if len(cards) != 1 {
		return 0
	}
	p := d.pile()
	for i, c := range p {
		if c == cards[0] {
			return len(p) - 1 - i
		}
	}
	return 0
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// sameAction compares what two submissions do, not which offer they came
// from, and a meld's cards in any order.
//
// Its one use that matters is matching a bot's single action to the listed
// moves, and a false match there only mislabels advice.
func sameAction(a, b module.Action) bool {
	if a.Verb != b.Verb || len(a.Cards) != len(b.Cards) {
		return false
	}
	// A target one side leaves implicit (the pile a draw comes from) is not
	// a difference; two different targets are.
	if a.Target != b.Target && a.Target != "" && b.Target != "" {
		return false
	}
	x, y := append([]string(nil), a.Cards...), append([]string(nil), b.Cards...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	for k, v := range a.Params {
		if b.Params[k] != v {
			return false
		}
	}
	return len(a.Params) == len(b.Params)
}

// softmax at temperature 1, stable against large logits.
func softmax(logits []float32) []float64 {
	p := make([]float64, len(logits))
	hi := math.Inf(-1)
	for _, l := range logits {
		hi = math.Max(hi, float64(l))
	}
	sum := 0.0
	for i, l := range logits {
		p[i] = math.Exp(float64(l) - hi)
		sum += p[i]
	}
	for i := range p {
		p[i] /= sum
	}
	return p
}

// topK is the indices of the k largest values, largest first.
func topK(xs []float64, k int) []int {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return xs[idx[a]] > xs[idx[b]] })
	if len(idx) > k {
		idx = idx[:k]
	}
	return idx
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
