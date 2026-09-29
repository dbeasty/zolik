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
}

var tariffs = map[int]Tariff{
	TariffCSM: {Hra: 1, Sedma: 2, Sto: 4, Betl: 15, Durch: 30},
	TariffPub: {Hra: 1, Sedma: 2, Sto: 4, Betl: 5, Durch: 10},
}

func tariffFor(v int) Tariff {
	if t, ok := tariffs[v]; ok {
		return t
	}
	return tariffs[TariffCSM]
}

// overHundred doubles base once for every ten points past a hundred:
// base at 100, 2×base at 110, 4×base at 120. points below a hundred count as
// a hundred.
func overHundred(base, points int) int {
	for p := hundred; p+10 <= points; p += 10 {
		base *= 2
	}
	return base
}

// hraValue is what a plain game (hra) pays, win or lose, given the winning
// side's count — card points plus one marriage, the same count sto uses.
//
// A side that reaches a hundred without having announced it has made a quiet
// hundred (tiché sto): it doubles the game, and doubles again for every ten
// past (2, 4, 8 … units at 100, 110, 120 …).
func (t Tariff) hraValue(winnerHundredCount int) int {
	if winnerHundredCount < hundred {
		return t.Hra
	}
	return overHundred(2*t.Hra, winnerHundredCount)
}

// stoValue is what an announced hundred pays. Won, it is worth the sto
// tariff doubled for every ten past a hundred. Lost, it pays the sto tariff
// flat: the association's table does not scale a failure, and "how far short"
// is not a figure any Czech source agrees on (docs/marias-rules.md, open
// question 1).
func (t Tariff) stoValue(won bool, count int) int {
	if !won {
		return t.Sto
	}
	return overHundred(t.Sto, count)
}

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
