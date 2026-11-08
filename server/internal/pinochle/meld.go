package pinochle

import (
	"sort"

	"zolik/server/internal/tricks"
)

// Meld kinds. Each is its own message key on the client
// (pinochle.meld.<kind>), so they are spelled once, here.
const (
	MeldRun           = "run"
	MeldDoubleRun     = "doubleRun"
	MeldRoyalMarriage = "royalMarriage"
	MeldMarriage      = "marriage"
	MeldDix           = "dix"
	MeldAces          = "aces"
	MeldKings         = "kings"
	MeldQueens        = "queens"
	MeldJacks         = "jacks"
	MeldPinochle      = "pinochle"
)

// The three classes of meld. A card may count in more than one meld only
// when the melds are of different classes: the queen of spades can sit in a
// pinochle, a marriage and queens around at once, but the king and queen of
// a run are not also a royal marriage.
const (
	classSequence = 'A' // runs, marriages, the dix
	classAround   = 'B' // aces, kings, queens, jacks around
	classPinochle = 'C' // the pinochle
)

func classOf(kind string) byte {
	switch kind {
	case MeldAces, MeldKings, MeldQueens, MeldJacks:
		return classAround
	case MeldPinochle:
		return classPinochle
	}
	return classSequence
}

// The meld table, in points (the same scale as card points: a run is worth
// fifteen counters).
const (
	pointsRun           = 150
	pointsDoubleRun     = 1500
	pointsRoyalMarriage = 40
	pointsMarriage      = 20
	pointsDix           = 10
)

// aroundPoints is n of a rank in every suit — once around, twice, three or
// four times (the last two on the double pack only).
var aroundPoints = map[byte][5]int{
	'A': {0, 100, 1000, 1500, 2000},
	'K': {0, 80, 800, 1200, 1600},
	'Q': {0, 60, 600, 900, 1200},
	'J': {0, 40, 400, 600, 800},
}

// pinochlePoints is n queens of spades each with a jack of diamonds.
var pinochlePoints = [5]int{0, 40, 300, 600, 900}

func aroundKind(r byte) string {
	switch r {
	case 'A':
		return MeldAces
	case 'K':
		return MeldKings
	case 'Q':
		return MeldQueens
	}
	return MeldJacks
}

// MeldItem is one scoring combination.
type MeldItem struct {
	Kind string `json:"kind"`
	// Suit is the suit of a run, a marriage or a dix; empty otherwise.
	Suit string `json:"suit,omitempty"`
	// Times is how many times over: two for double pinochle, eight aces
	// as aces twice around. Always 1 for a marriage or a dix, which are
	// listed once each instead.
	Times  int      `json:"times"`
	Points int      `json:"points"`
	Cards  []string `json:"cards"`
}

// Meld is everything one hand shows.
type Meld struct {
	Items []MeldItem `json:"items,omitempty"`
	Total int        `json:"total"`
}

// counts is a hand as a multiset.
type counts map[string]int

func countOf(hand []string) counts {
	c := counts{}
	for _, card := range hand {
		c[card]++
	}
	return c
}

func repeat(cards []string, n int) []string {
	out := make([]string, 0, len(cards)*n)
	for i := 0; i < n; i++ {
		out = append(out, cards...)
	}
	return out
}

// BestMeld is what a hand melds with trump as trumps, taking every
// combination it holds under the sharing rule above. It is a deterministic
// count, not a choice: with the classes kept apart the most a hand can show
// is simply everything of each class.
func BestMeld(hand []string, trump byte, t table) Meld {
	c := countOf(hand)
	var m Meld
	add := func(it MeldItem) {
		m.Items = append(m.Items, it)
		m.Total += it.Points
	}
	ts := string(trump)

	// Class A in trumps: runs first, then royal marriages from the kings
	// and queens the runs did not use, then the nines.
	if trump != 0 {
		runs := c["A"+ts]
		for _, r := range "TKQJ" {
			runs = min(runs, c[string(r)+ts])
		}
		run := []string{"A" + ts, "T" + ts, "K" + ts, "Q" + ts, "J" + ts}
		for i := 0; i+1 < runs; i += 2 {
			add(MeldItem{Kind: MeldDoubleRun, Suit: ts, Times: 2, Points: pointsDoubleRun, Cards: repeat(run, 2)})
		}
		if runs%2 == 1 {
			add(MeldItem{Kind: MeldRun, Suit: ts, Times: 1, Points: pointsRun, Cards: run})
		}
		for i := 0; i < min(c["K"+ts], c["Q"+ts])-runs; i++ {
			add(MeldItem{Kind: MeldRoyalMarriage, Suit: ts, Times: 1, Points: pointsRoyalMarriage, Cards: []string{"K" + ts, "Q" + ts}})
		}
		if t.hasNines() {
			for i := 0; i < c["9"+ts]; i++ {
				add(MeldItem{Kind: MeldDix, Suit: ts, Times: 1, Points: pointsDix, Cards: []string{"9" + ts}})
			}
		}
	}
	// Class A outside trumps: the plain marriages.
	for _, s := range suits {
		if byte(s) == trump {
			continue
		}
		ss := string(s)
		for i := 0; i < min(c["K"+ss], c["Q"+ss]); i++ {
			add(MeldItem{Kind: MeldMarriage, Suit: ss, Times: 1, Points: pointsMarriage, Cards: []string{"K" + ss, "Q" + ss}})
		}
	}
	// Class B: arounds.
	for _, r := range []byte("AKQJ") {
		n := 4
		for _, s := range suits {
			n = min(n, c[string(r)+string(s)])
		}
		if n > 0 {
			var cards []string
			for _, s := range suits {
				cards = append(cards, string(r)+string(s))
			}
			add(MeldItem{Kind: aroundKind(r), Times: n, Points: aroundPoints[r][n], Cards: repeat(cards, n)})
		}
	}
	// Class C: pinochles, all at once.
	if n := min(c["QS"], c["JD"], 4); n > 0 {
		add(MeldItem{Kind: MeldPinochle, Times: n, Points: pinochlePoints[n], Cards: repeat([]string{"QS", "JD"}, n)})
	}
	return m
}

func (t table) hasNines() bool { return len(t.ranks) == 6 }

// MeldCards is every card a meld shows, each counted as often as the most
// any one class needs it — the cards a player lays on the table.
func (m Meld) MeldCards() []string {
	byClass := map[byte]counts{}
	for _, it := range m.Items {
		cl := classOf(it.Kind)
		if byClass[cl] == nil {
			byClass[cl] = counts{}
		}
		for _, c := range it.Cards {
			byClass[cl][c]++
		}
	}
	need := counts{}
	for _, cs := range byClass {
		for c, n := range cs {
			need[c] = max(need[c], n)
		}
	}
	var out []string
	for c, n := range need {
		for i := 0; i < n; i++ {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// --- two-handed: one meld at a time ----------------------------------------

// Used is, per class, how many of each card a two-handed player has already
// melded in that class. A card still counts while it stays in hand, so the
// numbers fall when a melded card is played.
type Used map[string]map[string]int // class letter -> card -> count

func (u Used) of(class byte) map[string]int {
	k := string(class)
	if u[k] == nil {
		u[k] = map[string]int{}
	}
	return u[k]
}

// shown is every card on the table for this player: the most of each that
// any class has used.
func (u Used) shown() []string {
	need := map[string]int{}
	for _, cs := range u {
		for c, n := range cs {
			need[c] = max(need[c], n)
		}
	}
	var out []string
	for c, n := range need {
		for i := 0; i < n; i++ {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// release is a card leaving the hand: no class may count more of it than
// the hand still holds.
func (u Used) release(hand []string, card string) {
	have := 0
	for _, c := range hand {
		if c == card {
			have++
		}
	}
	for _, cs := range u {
		if cs[card] > have {
			cs[card] = have
		}
	}
}

// Option is one meld a two-handed player may lay after winning a trick.
type Option struct {
	ID string // the offer id, "meld.<kind>[.<suit>]"
	MeldItem
}

// twoHandOptions is every single meld a hand can lay now, given what it has
// laid already. Doubles (double pinochle, aces twice around) are offered only
// when every card of them is fresh to that class, since they score as one
// meld, not two.
func twoHandOptions(hand []string, used Used, trump byte) []Option {
	have := countOf(hand)
	free := func(class byte, cards []string) bool {
		need := countOf(cards)
		u := used.of(class)
		for c, n := range need {
			if have[c]-u[c] < n {
				return false
			}
		}
		return true
	}
	var out []Option
	addIf := func(id string, it MeldItem) {
		if free(classOf(it.Kind), it.Cards) {
			out = append(out, Option{ID: id, MeldItem: it})
		}
	}
	ts := string(trump)
	addIf("meld.run", MeldItem{Kind: MeldRun, Suit: ts, Times: 1, Points: pointsRun,
		Cards: []string{"A" + ts, "T" + ts, "K" + ts, "Q" + ts, "J" + ts}})
	addIf("meld.royalMarriage", MeldItem{Kind: MeldRoyalMarriage, Suit: ts, Times: 1, Points: pointsRoyalMarriage,
		Cards: []string{"K" + ts, "Q" + ts}})
	for _, s := range suits {
		if byte(s) == trump {
			continue
		}
		ss := string(s)
		addIf(marriageOfferID(ss), MeldItem{Kind: MeldMarriage, Suit: ss, Times: 1, Points: pointsMarriage,
			Cards: []string{"K" + ss, "Q" + ss}})
	}
	for _, r := range []byte("AKQJ") {
		var cards []string
		for _, s := range suits {
			cards = append(cards, string(r)+string(s))
		}
		kind := aroundKind(r)
		addIf(aroundOfferID(kind, 2), MeldItem{Kind: kind, Times: 2, Points: aroundPoints[r][2], Cards: repeat(cards, 2)})
		addIf(aroundOfferID(kind, 1), MeldItem{Kind: kind, Times: 1, Points: aroundPoints[r][1], Cards: cards})
	}
	addIf("meld.pinochle.2", MeldItem{Kind: MeldPinochle, Times: 2, Points: pinochlePoints[2], Cards: []string{"QS", "JD", "QS", "JD"}})
	addIf("meld.pinochle", MeldItem{Kind: MeldPinochle, Times: 1, Points: pinochlePoints[1], Cards: []string{"QS", "JD"}})
	return out
}

// Offer ids are written out in full so that each is a literal the key
// manifest can read.
func marriageOfferID(suit string) string {
	switch suit {
	case "H":
		return "meld.marriage.H"
	case "C":
		return "meld.marriage.C"
	case "D":
		return "meld.marriage.D"
	}
	return "meld.marriage.S"
}

func aroundOfferID(kind string, times int) string {
	switch kind {
	case MeldAces:
		if times == 2 {
			return "meld.aces.2"
		}
		return "meld.aces"
	case MeldKings:
		if times == 2 {
			return "meld.kings.2"
		}
		return "meld.kings"
	case MeldQueens:
		if times == 2 {
			return "meld.queens.2"
		}
		return "meld.queens"
	}
	if times == 2 {
		return "meld.jacks.2"
	}
	return "meld.jacks"
}

// suitOf reads a meld's suit back out of a card list, for display.
func suitOf(cards []string) byte {
	if len(cards) == 0 {
		return 0
	}
	return tricks.Suit(cards[0])
}
