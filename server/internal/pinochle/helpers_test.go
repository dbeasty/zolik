package pinochle

import (
	"testing"

	"zolik/server/internal/module"
)

func refs(n int) []module.PlayerRef {
	var out []module.PlayerRef
	for i := 1; i <= n; i++ {
		out = append(out, module.PlayerRef{ID: "p" + string(rune('0'+i))})
	}
	return out
}

func seatsOf(variation string) int { return tableOf(variation).seats }

var allVariations = []string{VarPartnership, VarCutthroat, VarTwoHanded, VarDoubleDeck}

func cfgOf(variation string, opts module.Options) module.MatchConfig {
	o := module.Options{module.OptPauseBetweenRounds: module.OptOff}
	for k, v := range opts {
		o[k] = v
	}
	return module.MatchConfig{Variation: variation, Options: o}
}

func newState(t *testing.T, variation string, opts module.Options) *GameState {
	t.Helper()
	raw, err := New().NewMatch(cfgOf(variation, opts), refs(seatsOf(variation)), 42)
	if err != nil {
		t.Fatalf("NewMatch: %v", err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func apply(t *testing.T, s *GameState, who string, a module.Action) (*GameState, error) {
	t.Helper()
	raw, err := encode(s)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := New().Apply(raw, who, a)
	if err != nil {
		return s, err
	}
	next, err := decode(out)
	if err != nil {
		t.Fatal(err)
	}
	return next, nil
}

func must(t *testing.T, s *GameState, who string, a module.Action) *GameState {
	t.Helper()
	next, err := apply(t, s, who, a)
	if err != nil {
		t.Fatalf("%s %s %v %v refused: %v", who, a.Verb, a.Cards, a.Params, err)
	}
	return next
}

func refused(t *testing.T, s *GameState, who string, a module.Action, code string) {
	t.Helper()
	if _, err := apply(t, s, who, a); module.CodeOf(err) != code {
		t.Fatalf("%s %s %v: got %v, want %s", who, a.Verb, a.Cards, err, code)
	}
}

func card(c string) module.Action {
	return module.Action{OfferID: OfferPlay, Verb: VerbPlay, Cards: []string{c}}
}

func bid(n string) module.Action {
	return module.Action{OfferID: OfferBid, Verb: VerbBid, Params: map[string]string{ParamAmount: n}}
}

var passAction = module.Action{OfferID: OfferPass, Verb: VerbPass}

func trumpAction(su string) module.Action {
	return module.Action{OfferID: OfferTrump, Verb: VerbTrump, Params: map[string]string{ParamSuit: su}}
}

// playingAt is a four-handed partnership hand in play with these hands, the
// bid 250 to p1, trumps named, p1 to lead.
func playingAt(t *testing.T, trump string, hands map[string][]string) *GameState {
	t.Helper()
	s := newState(t, VarPartnership, nil)
	s.FirstDealer, s.Deal = 3, 0 // p4 deals, p1 bids first
	s.Hands = hands
	s.Bid, s.Bidder, s.Trump = 250, "p1", trump
	s.Widow = nil
	s.startPlay()
	return s
}
