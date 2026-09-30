package marias

import (
	"math/rand"
	"sort"

	"zolik/server/internal/tricks"
	"zolik/server/internal/tricks/search"
)

// The hard bot's card play: sample the cards it cannot see, consistent with
// everything the table has shown, solve each sample open-handed
// (tricks/search), and play the card with the best average. Step 6 of
// docs/marias-plan.md.
//
// What it may read is exactly what its seat may: its own hand, the cards
// played (History and Trick), how many cards each hand holds, the contract
// and its announcements, and — as the declarer — its own talon. Everything
// else it deals itself. TestBotDoesNotPeek holds it to that.

const (
	samplesPerMove = 20
	// nodeBudget bounds the search for one decision across all its samples,
	// so a bot's think time has a ceiling whatever the position. Counted in
	// nodes rather than milliseconds, so a match replays the same moves on
	// any machine.
	nodeBudget = 2_000_000
	// sampleNodeLimit abandons one sample's solve if it runs long; a
	// half-searched sample is dropped rather than counted.
	sampleNodeLimit = 400_000
)

// knowledge is what the table has shown about the cards a seat has not.
type knowledge struct {
	// forbidden cards a player cannot hold: shown void, or shown unable to
	// beat.
	forbidden map[string]map[string]bool
	// required cards a player must still hold: the král of an announced
	// marriage, an announced seven.
	required map[string][]string
}

// infer reads the history of the deal for voids, can't-beats and promises.
func (s *GameState) infer() knowledge {
	k := knowledge{forbidden: map[string]map[string]bool{}, required: map[string][]string{}}
	for _, p := range s.Players {
		k.forbidden[p] = map[string]bool{}
	}
	trump := s.trump()
	order := s.order()
	suitCards := func(suit byte) []string {
		var out []string
		for _, r := range orderTrump {
			out = append(out, string(r)+string(suit))
		}
		return out
	}
	forbidSuit := func(p string, suit byte) {
		for _, c := range suitCards(suit) {
			k.forbidden[p][c] = true
		}
	}
	forbidAbove := func(p string, best string) {
		for _, c := range suitCards(tricks.Suit(best)) {
			if order.Beats(tricks.Rank(c), tricks.Rank(best)) {
				k.forbidden[p][c] = true
			}
		}
	}

	all := append(append([][]tricks.Play(nil), s.History...), s.Trick)
	for _, trick := range all {
		for i := 1; i < len(trick); i++ {
			p, c := s.Players[trick[i].Seat], trick[i].Card
			led := tricks.Suit(trick[0].Card)
			best, _ := tricks.Trick{Plays: trick[:i]}.Winning(trump, order)
			switch {
			case tricks.Suit(c) != led:
				// Did not follow: void in the suit led — and, not having
				// trumped either, void in trumps too.
				forbidSuit(p, led)
				if trump != tricks.NoTrump && tricks.Suit(c) != trump {
					forbidSuit(p, trump)
				}
				if trump != tricks.NoTrump && tricks.Suit(c) == trump && tricks.Suit(best.Card) == trump &&
					!order.Beats(tricks.Rank(c), tricks.Rank(best.Card)) {
					forbidAbove(p, best.Card) // trumped under: held no higher trump
				}
			case tricks.Suit(best.Card) == led && !order.Beats(tricks.Rank(c), tricks.Rank(best.Card)):
				forbidAbove(p, best.Card) // followed without beating: held nothing higher
			}
		}
	}

	played := map[string]bool{}
	for _, trick := range all {
		for _, pl := range trick {
			played[pl.Card] = true
		}
	}
	for _, m := range s.Announced {
		if king := "K" + m.Suit; !played[king] {
			k.required[m.Player] = append(k.required[m.Player], king)
		}
	}
	// Sevens that had to be held to be announced: volený's sedma, the
	// licitovaný plain sedma, and sedma proti. Above plain sedma a
	// licitovaný seven may be a bluff, so it promises nothing.
	if seven := s.trumpSeven(); seven != "" && !played[seven] {
		switch {
		case s.Sedma && (!s.licit() || s.Game == gameHra):
			k.required[s.Declarer] = append(k.required[s.Declarer], seven)
		case s.ProtiSedma != "":
			k.required[s.ProtiSedma] = append(k.required[s.ProtiSedma], seven)
		}
	}
	return k
}

// sample deals the cards me cannot see to the other hands (and the talon,
// unless me laid it), consistent with k. It reports false if no consistent
// deal was found, which a caller treats as "stop sampling".
func (s *GameState) sample(me string, k knowledge, r *rand.Rand) (map[string][]string, bool) {
	seen := map[string]bool{}
	for _, c := range s.Hands[me] {
		seen[c] = true
	}
	for _, trick := range append(append([][]tricks.Play(nil), s.History...), s.Trick) {
		for _, pl := range trick {
			seen[pl.Card] = true
		}
	}
	talonKnown := me == s.Declarer
	if talonKnown {
		for _, c := range s.Talon {
			seen[c] = true
		}
	}
	var unknown []string
	for _, c := range buildDeck() {
		if !seen[c] {
			unknown = append(unknown, c)
		}
	}

	// The slots to fill: every other hand, and the talon when unseen.
	type slot struct {
		name   string
		need   int
		forbid map[string]bool
	}
	var slots []*slot
	for _, p := range s.Players {
		if p != me {
			slots = append(slots, &slot{name: p, need: len(s.Hands[p]), forbid: k.forbidden[p]})
		}
	}
	if !talonKnown && len(s.Talon) > 0 {
		forbid := map[string]bool{}
		if s.trumpGame() {
			// No ace or ten goes to the talon in a trump game, nor an
			// announced seven.
			for _, c := range buildDeck() {
				if cardPoints(c) > 0 {
					forbid[c] = true
				}
			}
			forbid[s.trumpSeven()] = true
		}
		slots = append(slots, &slot{name: "talon", need: len(s.Talon), forbid: forbid})
	}

	for attempt := 0; attempt < 40; attempt++ {
		deal := map[string][]string{}
		left := map[*slot]int{}
		for _, sl := range slots {
			left[sl] = sl.need
		}
		placed := map[string]bool{}
		ok := true
		// Promised cards first.
		for _, sl := range slots {
			for _, c := range k.required[sl.name] {
				if placed[c] || left[sl] == 0 || seen[c] {
					continue
				}
				deal[sl.name] = append(deal[sl.name], c)
				placed[c] = true
				left[sl]--
			}
		}
		rest := make([]string, 0, len(unknown))
		for _, c := range unknown {
			if !placed[c] {
				rest = append(rest, c)
			}
		}
		r.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
		// The most constrained cards first: fewer dead ends.
		eligible := func(c string) []*slot {
			var out []*slot
			for _, sl := range slots {
				if left[sl] > 0 && !sl.forbid[c] {
					out = append(out, sl)
				}
			}
			return out
		}
		sort.SliceStable(rest, func(i, j int) bool { return len(eligible(rest[i])) < len(eligible(rest[j])) })
		// In a real deal the unseen cards exactly fill the slots. A position
		// built by hand can hold fewer cards than a deal, and its surplus
		// is simply left out.
		room := 0
		for _, sl := range slots {
			room += left[sl]
		}
		spare := len(rest) - room
		for _, c := range rest {
			opts := eligible(c)
			if len(opts) == 0 {
				if spare > 0 {
					spare--
					continue
				}
				ok = false
				break
			}
			// Weighted by room left, so a hand needing six takes more of
			// the loose cards than a talon needing one.
			total := 0
			for _, sl := range opts {
				total += left[sl]
			}
			pick := r.Intn(total)
			for _, sl := range opts {
				if pick < left[sl] {
					deal[sl.name] = append(deal[sl.name], c)
					left[sl]--
					break
				}
				pick -= left[sl]
			}
		}
		if ok {
			return deal, true
		}
	}
	return nil, false
}

// searchPlay is the hard bot's card: the legal card with the best average
// over sampled deals, or ok false if no sample could be solved.
func (s *GameState) searchPlay(me string, legal []string, r *rand.Rand) (string, bool) {
	if len(legal) == 1 {
		return legal[0], true
	}
	k := s.infer()
	decl := s.seat(s.Declarer)
	maximizer := make([]bool, len(s.Players))
	maximizer[decl] = true
	var reserves []search.Reserve
	if h := s.sevenHolder(); h != "" && s.trumpSeven() != "" {
		reserves = append(reserves, search.Reserve{Seat: s.seat(h), Card: s.trumpSeven(), UntilLeft: 1})
	}
	if hs := s.helperSeven(); hs != "" {
		reserves = append(reserves, search.Reserve{Seat: decl, Card: hs, UntilLeft: 2})
	}
	leader := s.seat(me)
	if len(s.Trick) > 0 {
		leader = s.Trick[0].Seat
	}

	// Marriages score as the svršek goes down with its král still in hand.
	var marriages []search.Pair
	if s.trumpGame() {
		for _, suit := range []string{"H", "D", "C", "S"} {
			marriages = append(marriages, search.Pair{First: "Q" + suit, Second: "K" + suit, Value: s.marriageValue(suit)})
		}
	}

	totals := map[string]int{}
	counted, nodes := 0, 0
	for i := 0; i < samplesPerMove && nodes < nodeBudget; i++ {
		deal, ok := s.sampleLikely(me, k, r)
		if !ok {
			break
		}
		hands := make([][]string, len(s.Players))
		for seat, p := range s.Players {
			if p == me {
				hands[seat] = append([]string(nil), s.Hands[me]...)
			} else {
				hands[seat] = deal[p]
			}
		}
		res := search.Solve(search.Problem{
			Hands:      hands,
			Trick:      s.Trick,
			Leader:     leader,
			Trump:      s.trump(),
			Order:      s.order(),
			Policy:     tricks.FollowBeatTrump,
			Maximizer:  maximizer,
			Reserves:   reserves,
			Pairs:      marriages,
			TrickValue: s.trickValue(),
			CardClass:  s.cardClass(),
			NodeLimit:  sampleNodeLimit,
		})
		nodes += res.Nodes
		if res.Exhausted {
			continue
		}
		for c, v := range res.Values {
			totals[c] += v
		}
		counted++
	}
	if counted == 0 {
		return "", false
	}

	// The declarer raises the value; a defender lowers it. Ties go to the
	// cheaper card, so an even choice does not throw a point away.
	best, bestV := "", 0
	forDeclarer := me == s.Declarer
	for _, c := range legal {
		v, ok := totals[c]
		if !ok {
			continue
		}
		better := best == "" ||
			(forDeclarer && v > bestV) || (!forDeclarer && v < bestV) ||
			(v == bestV && cardPoints(c) < cardPoints(best))
		if better {
			best, bestV = c, v
		}
	}
	return best, best != ""
}

// trickValue is what a trick is worth to the declarer's side, for the
// search: the card points of each trick it takes, and a large weight on the
// contract's own conditions — a betl or durch broken, a seven won or lost.
func (s *GameState) trickValue() func([]tricks.Play, int, int) (int, bool) {
	decl := s.seat(s.Declarer)
	trumpSeven, helperSeven := s.trumpSeven(), s.helperSeven()
	sevenBy := func(plays []tricks.Play, card string, winner int) (played, won bool, by int) {
		for _, pl := range plays {
			if pl.Card == card {
				return true, pl.Seat == winner, pl.Seat
			}
		}
		return false, false, -1
	}
	protiSeat := -1
	if s.ProtiSedma != "" {
		protiSeat = s.seat(s.ProtiSedma)
	}
	return func(plays []tricks.Play, winner, left int) (int, bool) {
		switch s.Game {
		case gameBetl:
			if winner == decl {
				return -1000, true
			}
			return 0, false
		case gameDurch:
			if winner != decl {
				return -1000, true
			}
			return 0, false
		}
		v := 0
		if winner == decl {
			for _, pl := range plays {
				v += cardPoints(pl.Card)
			}
			if left == 0 {
				v += pointsLastTrick
			}
		}
		switch {
		case s.Game == gameDveSedmy && left == 1:
			if _, won, by := sevenBy(plays, helperSeven, winner); won && by == decl {
				v += 80
			} else {
				v -= 80
			}
		case s.Game == gameDveSedmy && left == 0:
			if _, won, by := sevenBy(plays, trumpSeven, winner); won && by == decl {
				v += 80
			} else {
				v -= 80
			}
		case left == 0 && trumpSeven != "":
			played, won, by := sevenBy(plays, trumpSeven, winner)
			switch {
			case s.Sedma:
				if won && by == decl {
					v += 60
				} else {
					v -= 60
				}
			case protiSeat >= 0:
				if won && by == protiSeat {
					v -= 60
				} else {
					v += 60
				}
			case played:
				// A quiet seven, won or killed.
				if won == (by == decl) {
					v += 10
				} else {
					v -= 10
				}
			}
		}
		return v, false
	}
}

// cardClass groups cards the search may treat as one move: by card points,
// with the sevens that carry a contract kept apart.
func (s *GameState) cardClass() func(string) int {
	trumpSeven, helperSeven := s.trumpSeven(), s.helperSeven()
	marriages := s.trumpGame()
	return func(c string) int {
		if c == trumpSeven || c == helperSeven {
			return 100 + int(tricks.Suit(c))
		}
		// A svršek and its král are never one move: which goes first
		// decides whether a marriage is announced.
		if r := tricks.Rank(c); marriages && (r == 'Q' || r == 'K') {
			return 200 + int(r)*8 + int(tricks.Suit(c))
		}
		return cardPoints(c)
	}
}

// declarerMinTrumps is how many trumps a declarer is taken to have held: a
// trump game is chosen on length, and a defender who samples the declarer
// short in trumps is solving a deal that almost never happens.
const declarerMinTrumps = 4

// sampleLikely is sample, redrawn a few times until the declarer holds the
// trump length their contract implies. A defender's samples are otherwise
// uniform over hands the declarer would rarely have chosen trumps with. If
// no redraw satisfies it, the last consistent deal stands.
func (s *GameState) sampleLikely(me string, k knowledge, r *rand.Rand) (map[string][]string, bool) {
	deal, ok := s.sample(me, k, r)
	if !ok || me == s.Declarer || !s.trumpGame() {
		return deal, ok
	}
	trump := s.trump()
	played := 0
	for _, trick := range append(append([][]tricks.Play(nil), s.History...), s.Trick) {
		for _, pl := range trick {
			if s.Players[pl.Seat] == s.Declarer && tricks.Suit(pl.Card) == trump {
				played++
			}
		}
	}
	for try := 0; try < 12; try++ {
		n := played
		for _, c := range deal[s.Declarer] {
			if tricks.Suit(c) == trump {
				n++
			}
		}
		if n >= declarerMinTrumps {
			return deal, true
		}
		next, ok := s.sample(me, k, r)
		if !ok {
			break
		}
		deal = next
	}
	return deal, true
}
