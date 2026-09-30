package marias

// The payment table (sazebník). Every figure a deal settles comes out of
// here, so the numbers a player agreed to at the lobby are written in one
// place and tested against the association's table.
//
// A deal is settled part by part — the game, and each seven — because each
// part is announced, doubled and won on its own: a player can make the game
// and lose the seven. Each part is paid between the declarer and each
// defender separately, in units.

// Tariff is the base value of each part, in units.
type Tariff struct {
	Hra   int `json:"hra"`
	Sedma int `json:"sedma"`
	Sto   int `json:"sto"`
	Betl  int `json:"betl"`
	Durch int `json:"durch"`
	// DveSedmy and Omyl exist only at a licitovaný table.
	DveSedmy int `json:"dveSedmy,omitempty"`
	Omyl     int `json:"omyl,omitempty"`
}

var tariffs = map[int]Tariff{
	TariffCSM: {Hra: 1, Sedma: 2, Sto: 4, Betl: 15, Durch: 30, DveSedmy: 40, Omyl: 6},
	TariffPub: {Hra: 1, Sedma: 2, Sto: 4, Betl: 5, Durch: 10, DveSedmy: 40, Omyl: 6},
}

func tariffFor(v int) Tariff {
	if t, ok := tariffs[v]; ok {
		return t
	}
	return tariffs[TariffCSM]
}

// tensPast and tensShort count whole tens beyond, or short of, a hundred.
// Counts are always multiples of ten (card points and marriages both are);
// rounding up on the short side is only a guard.
func tensPast(count int) int {
	if count <= hundred {
		return 0
	}
	return (count - hundred) / 10
}

func tensShort(count int) int {
	if count >= hundred {
		return 0
	}
	return (hundred - count + 9) / 10
}

// hraValue is what a plain game (hra) pays, win or lose, given the winning
// side's total — card points and every marriage it announced.
//
// A side that reaches a hundred without announcing it (tiché sto) doubles
// the game, and is paid that doubled game again for every ten points past
// the hundred: 2, 4, 6 … units at 100, 110, 120 (ČSM general rules V/7).
// Linear, not doubling — see docs/marias-licitovany-rules.md §8, question 1.
func (t Tariff) hraValue(winnerTotal int) int {
	if winnerTotal < hundred {
		return t.Hra
	}
	return 2 * t.Hra * (1 + tensPast(winnerTotal))
}

// stoValue is what an announced hundred pays (ČSM general rules V/6). Made,
// it is worth the sto tariff and the tariff again for every ten points past
// the hundred. Failed, it pays the tariff for every ten points short of the
// hundred and for every ten points of marriages the other side announced.
//
// count is the announcing side's card points and the first marriage it
// announced; opponentsMarriages is everything the other side announced.
func (t Tariff) stoValue(won bool, count, opponentsMarriages int) int {
	if won {
		return t.Sto * (1 + tensPast(count))
	}
	return t.Sto * (tensShort(count) + opponentsMarriages/10)
}

// payLimit is the most a single deal pays between the declarer and any one
// defender (ČSM general rules V/9: 500 times the base).
const payLimit = 500

// quietSeven is the unannounced seven: won, or killed in the last trick.
// Half an announced one.
func (t Tariff) quietSeven() int { return t.Sedma / 2 }

// multiplier is what flek and red trumps do to a part: each doubling
// doubles it, and hearts double it once more.
func multiplier(doublings int, red bool) int {
	m := 1 << doublings
	if red {
		m *= 2
	}
	return m
}

// flekNames are the doublings in the order they are said. Past the list a
// doubling has no name of its own and is shown as its multiplier.
var flekNames = []string{"flek", "re", "tutti", "boty", "kalhoty", "kaiser"}
