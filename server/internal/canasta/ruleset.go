package canasta

// A variation's whole shape, in one place.
//
// This started as three ints — hand size, target, canastas to go out — because
// the two variations that shipped first differed in nothing else. Samba differs
// in the deck, the draw, the wild limits, the meld kinds, the pile and every
// bonus (docs/samba-plan.md §3.1), and the alternative to widening this struct
// was a second copy of the engine with those clauses inlined. One struct, read
// once, is the cheaper half of that trade.
//
// Fields are exported because a resolved ruleset is stored on the state: a match
// is dealt under the rules it was created with and keeps them, so shipping a
// change to a variation cannot alter a deal already in progress. That is the same
// property `Pause` has, for the same reason.
type ruleset struct {
	HandSize        int `json:"handSize"`
	TargetScore     int `json:"targetScore"`
	CanastasToGoOut int `json:"canastasToGoOut"`

	// HandSizeAt overrides HandSize at a given seat count, for the games that
	// deal fewer cards to a fuller table.
	HandSizeAt map[int]int `json:"handSizeAt,omitempty"`

	// MaxSeats is what this variation can actually seat, which is not always
	// what the module can: see view.go's descriptor.
	MaxSeats int `json:"maxSeats"`

	Decks         int `json:"decks"`
	JokersPerDeck int `json:"jokersPerDeck"`
	DrawCount     int `json:"drawCount"`

	// Sequences turns on runs as a second kind of meld. Off, this is Canasta;
	// on, the module also knows what a samba is.
	Sequences bool `json:"sequences,omitempty"`

	// The wild limits inside a group. MaxWilds and MinNaturals are absolute;
	// NaturalsPerWild is a ratio (0 = no ratio), and Samba's "twice as many
	// naturals as wilds" is the only reason it exists.
	MaxWilds        int `json:"maxWilds"`
	MinNaturals     int `json:"minNaturals"`
	NaturalsPerWild int `json:"naturalsPerWild,omitempty"`

	// GroupsPerRank caps how many separate groups of one rank a side may have.
	// 0 means no cap, which is Samba.
	GroupsPerRank int `json:"groupsPerRank"`
	// GroupCanastaCloses is whether a group stops accepting cards at seven. A
	// sequence always closes at seven — that is what makes it a samba — but
	// Samba lets a group canasta keep growing.
	GroupCanastaCloses bool `json:"groupCanastaCloses"`
	// PileAlwaysFrozen makes every capture require two naturals from hand, for
	// the whole deal, against everyone.
	PileAlwaysFrozen bool `json:"pileAlwaysFrozen,omitempty"`

	// BlackThreeMeld is whether a group of black threes may ever go down.
	//
	// Where it is on, the meld is still only legal as the move that empties a
	// hand — three or four of them, straight from the hand, never with a wild
	// among them. Where it is off, a black three has no route to the table at
	// all: it is a stop card when discarded and 100 against you when it is not,
	// and that is the whole of it. Modern American is the variation that forbids
	// it, and forbidding it there is not a detail — the pile is the game at
	// thirteen cards and two canastas, so a hand that cannot shed its blockers
	// on the way out plays differently from one that can.
	BlackThreeMeld bool `json:"blackThreeMeld,omitempty"`

	// MeldFloors are the initial-meld minimums, ascending by the score they
	// start at. Below zero every variation asks for negativeMeldFloor.
	MeldFloors []floor `json:"meldFloors"`

	// RedThreesNeed is how many canastas a side must have before its red threes
	// count up rather than down. Not derived from CanastasToGoOut: Modern
	// American needs two canastas to go out but has always paid for red threes
	// after one, and deriving it would silently change a shipped variation.
	RedThreesNeed int `json:"redThreesNeed"`
	// RedThreePenaltyFlat charges 100 a three when they count down, rather than
	// negating the all-of-them bonus. Samba's, so that a side which never opened
	// owes 600 rather than 1000 for having been dealt well.
	RedThreePenaltyFlat bool `json:"redThreePenaltyFlat,omitempty"`

	NaturalCanastaBonus int `json:"naturalCanastaBonus"`
	MixedCanastaBonus   int `json:"mixedCanastaBonus"`
	SambaBonus          int `json:"sambaBonus,omitempty"`
	GoingOutBonus       int `json:"goingOutBonus"`
	// ConcealedBonus replaces GoingOutBonus for a hand melded in one turn. Zero
	// means the variation has no such bonus, which is Samba.
	ConcealedBonus int `json:"concealedBonus,omitempty"`
}

// floor is one band of the initial-meld minimum: at Above points and up, a side
// must lay Min in a turn to open.
type floor struct {
	Above int `json:"above"`
	Min   int `json:"min"`
}

// negativeMeldFloor is what a side below zero must lay. Shared by every
// variation, which is why it is here rather than in each of them.
const negativeMeldFloor = 15

// meldFloor is the minimum this side must lay in one turn to get on the table.
//
// It rises with the side's own accumulated score, which is the mechanism that
// keeps a match from running away: being ahead is what makes opening harder.
func (r ruleset) meldFloor(score int) int {
	if score < 0 {
		return negativeMeldFloor
	}
	min := negativeMeldFloor
	for _, f := range r.MeldFloors {
		if score >= f.Above {
			min = f.Min
		}
	}
	return min
}

// handSizeFor is how many cards a table of this size is dealt.
func (r ruleset) handSizeFor(seats int) int {
	if n, ok := r.HandSizeAt[seats]; ok {
		return n
	}
	return r.HandSize
}

// redThrees is how many red threes the deck holds, and redThreeAllBonus what
// holding every one of them pays.
//
// Both are derived from the deck rather than declared, because they are not
// independent of it: two decks means four threes and 800, three means six and
// 1000. Declaring them separately would be two facts that can disagree.
func (r ruleset) redThrees() int { return r.Decks * 2 }

func (r ruleset) redThreeAllBonus() int {
	if r.redThrees() >= 6 {
		return 1000
	}
	return 800
}

// classicFloors are the three bands Canasta has always had; Samba adds a fourth
// in its own table below.
var classicFloors = []floor{{Above: 0, Min: 50}, {Above: 1500, Min: 90}, {Above: 3000, Min: 120}}

var variations = map[string]ruleset{
	// Classic: eleven cards, one canasta buys the right to go out, 5000 wins.
	"classic": {
		HandSize: 11, TargetScore: 5000, CanastasToGoOut: 1, MaxSeats: 4,
		Decks: 2, JokersPerDeck: 2, DrawCount: 1,
		MaxWilds: 3, MinNaturals: 2, GroupsPerRank: 1, GroupCanastaCloses: true,
		BlackThreeMeld:      true,
		MeldFloors:          classicFloors,
		RedThreesNeed:       1,
		NaturalCanastaBonus: 500, MixedCanastaBonus: 300,
		GoingOutBonus: 100, ConcealedBonus: 200,
	},
	// Modern American: thirteen cards and two canastas to go out, which makes
	// deals longer and the discard pile far more valuable — and black threes
	// that can never be melded, only discarded or paid for.
	"modern_american": {
		HandSize: 13, TargetScore: 5000, CanastasToGoOut: 2, MaxSeats: 4,
		Decks: 2, JokersPerDeck: 2, DrawCount: 1,
		MaxWilds: 3, MinNaturals: 2, GroupsPerRank: 1, GroupCanastaCloses: true,
		MeldFloors:          classicFloors,
		RedThreesNeed:       1,
		NaturalCanastaBonus: 500, MixedCanastaBonus: 300,
		GoingOutBonus: 100, ConcealedBonus: 200,
	},
}

// sambaFloors add a fourth band above the three Canasta has: past 7000 a side
// needs 150 to open, which is what keeps a 10,000-point match from being decided
// halfway through it.
var sambaFloors = []floor{
	{Above: 0, Min: 50}, {Above: 1500, Min: 90},
	{Above: 3000, Min: 120}, {Above: 7000, Min: 150},
}

func init() {
	// Samba: three decks, fifteen cards, sequences, and a pile nobody can take
	// cheaply. Registered here rather than in the literal above because it is
	// long enough that a reader deserves the fields named (docs/samba-plan.md §2).
	variations["samba"] = ruleset{
		HandSize: 15,
		// Six seats deal thirteen: ninety cards off a 162-card deck would leave
		// a stock too thin for six players drawing two a turn.
		HandSizeAt:      map[int]int{6: 13},
		TargetScore:     10000,
		CanastasToGoOut: 2,
		MaxSeats:        6,

		Decks: 3, JokersPerDeck: 2, DrawCount: 2,

		Sequences: true,
		// Two wilds at most, and twice as many naturals as wilds — so a group
		// with two wilds needs four naturals and cannot exist below six cards.
		MaxWilds: 2, MinNaturals: 2, NaturalsPerWild: 2,
		// Black threes go down on the way out, as in Classic.
		BlackThreeMeld: true,
		// No cap: a side may keep several groups of one rank, separately. And a
		// group canasta is not closed by its seventh card, unlike a sequence,
		// whose seventh card is what makes it a samba.
		GroupsPerRank: 0, GroupCanastaCloses: false,
		PileAlwaysFrozen: true,

		MeldFloors: sambaFloors,

		// Two canastas — the same two that let a side go out — before red threes
		// count up, and a flat 100 each when they count down.
		RedThreesNeed: 2, RedThreePenaltyFlat: true,

		NaturalCanastaBonus: 500, MixedCanastaBonus: 300, SambaBonus: 1500,
		// 200 for going out, and no concealed bonus: Samba does not have one, so
		// melding a whole hand in a turn pays the ordinary 200.
		GoingOutBonus: 200, ConcealedBonus: 0,
	}
}

func resolveVariation(variation string) ruleset {
	if v, ok := variations[variation]; ok {
		return v
	}
	return variations["classic"]
}
