package module

// Drop-in tables: games where nobody has to be connected all the time.
//
// A hand of poker or blackjack does not need every seat's attention to carry
// on. Žolíky, Canasta and the rest do — a meld is a decision nobody else can
// make — so those still pause the whole table when the seat they are waiting on
// goes quiet. A module that is DropIn does not pause: a seat that has been away
// past a grace period is *sat out*, and the runtime plays it as passively as the
// rules allow until its player comes back. It is not a tournament rule; nobody
// is eliminated for a bad connection, and the seat resumes the moment its
// player is heard from again.
type DropIn interface {
	// SitOut names the verbs a sat-out seat plays, most preferred first — the
	// ones that commit the least: check before fold, stand before hit. Anything
	// not listed is still tried last, so a seat can never be stranded.
	SitOut() []string
}

// SitOutVerbs reports whether m is a drop-in game, and how it plays an absent
// seat.
func SitOutVerbs(m GameModule) ([]string, bool) {
	d, ok := m.(DropIn)
	if !ok {
		return nil, false
	}
	return d.SitOut(), true
}
