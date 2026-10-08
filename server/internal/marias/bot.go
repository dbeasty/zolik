package marias

import (
	"math/rand"
	"sort"
	"strconv"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

// How a seat nobody is sitting at plays Mariáš: the textbook rules of thumb,
// no search. Step 6 of docs/marias-plan.md replaces the card play with
// sampling; this is what every table plays against until then.
//
// It reads only what its seat may know — its own hand, the cards on the
// table, the contract once it is public, and (as the chooser) its own trump
// card. TestBotDoesNotPeek holds it to that.
//
// Legality is never its business. It picks among the cards the offer list
// already says are legal, so a bad judgement is a weak move and never a
// refused one.
type bot struct {
	// skill, if set, overrides the seat's: a bench style is always the
	// skill it was built as.
	skill module.Skill
	// limits bound a hard seat's search; zero is defaultLimits.
	limits searchLimits
	// classicContracts is the betl and durch judgement as it was before
	// contracts.go: betl only on a hand of nothing but low cards, durch
	// never. Kept for the bench, to measure the judgement against.
	classicContracts bool
}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil || s.Status != "active" || s.Intermission.Open || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, []string{module.VerbContinue, VerbPass, VerbGood})
	}
	me := seat.PlayerID
	skill := seat.Skill
	if b.skill != "" {
		skill = b.skill
	}
	if skill == "" {
		skill = module.SkillMedium
	}
	r := rand.New(rand.NewSource(seat.Seed + int64(s.Deal)*1009 + int64(len(s.Hands[me]))*31 + int64(len(s.Trick))))
	send := func(id, verb string, cards ...string) (module.Action, bool) {
		return module.Action{OfferID: id, Verb: verb, Cards: cards}, true
	}
	enabled := func(id string) *module.ActionOffer {
		for i := range offers {
			if offers[i].ID == id && offers[i].Enabled {
				return &offers[i]
			}
		}
		return nil
	}
	hand := s.Hands[me]

	switch s.Phase {
	case phaseAuction:
		limit := biddingLimit(hand, skill)
		if b.classicContracts {
			if skill != module.SkillEasy && betlHand(hand) {
				limit = max(limit, contractRung(kindBetl, false))
			}
		} else {
			// Ten cards and a talon unseen: the hand has to stand as it is.
			switch game, _ := b.noTrumpGame(hand, 0, skill, r); game {
			case gameDurch:
				limit = max(limit, contractRung(kindDurch, false))
			case gameBetl:
				limit = max(limit, contractRung(kindBetl, false))
			}
		}
		switch {
		case me == s.Holder && s.Rung <= limit && enabled(OfferHold) != nil:
			return send(OfferHold, VerbHold)
		case me == s.Bidder && s.Rung+1 <= limit && enabled(OfferBid) != nil:
			return module.Action{OfferID: OfferBid, Verb: VerbBid, Params: map[string]string{"rung": strconv.Itoa(s.Rung + 1)}}, true
		}
		return send(OfferAuctionPass, VerbPass)

	case phaseTrump:
		if o := enabled(OfferTrump); o != nil {
			return send(OfferTrump, VerbChooseTrump, trumpCard(o.Source.Cards))
		}

	case phaseAnnounce:
		game := ""
		if !b.classicContracts {
			game, _ = b.noTrumpGame(hand, 2, skill, r)
		}
		if s.licit() {
			return licitContract(hand, skill, offers, game, b.classicContracts)
		}
		id := announcement(hand, tricks.Suit(s.TrumpCard), skill)
		switch {
		case b.classicContracts:
		case game == gameDurch:
			id = OfferDurch
		case game == gameBetl:
			id = OfferBetl
		case id == OfferBetl:
			id = OfferHra // betlHand's all-low hand, judged and found wanting
		}
		if enabled(id) == nil {
			id = OfferHra
		}
		return send(id, VerbAnnounce)

	case phaseTalon:
		if o := enabled(OfferTalon); o != nil {
			if (s.Game == gameBetl || s.Game == gameDurch) && !b.classicContracts {
				if pair := noTrumpTalon(s.Game, hand, o.Source.Cards); pair != nil {
					return send(OfferTalon, VerbDiscard, pair...)
				}
			}
			return send(OfferTalon, VerbDiscard, talonPair(s, hand, o.Source.Cards)...)
		}

	case phaseAnswer:
		if b.classicContracts {
			if skill != module.SkillEasy && betlHand(hand) && enabled(OfferBadBetl) != nil {
				return send(OfferBadBetl, VerbTakeOver)
			}
			return send(OfferGood, VerbGood)
		}
		// Špatná takes the talon unseen, and it is the announcer's own
		// discard: the ten cards in hand have to make the game by themselves.
		switch game, _ := b.noTrumpGame(hand, 0, skill, r); {
		case game == gameDurch && enabled(OfferBadDurch) != nil:
			return send(OfferBadDurch, VerbTakeOver)
		case game == gameBetl && enabled(OfferBadBetl) != nil:
			return send(OfferBadBetl, VerbTakeOver)
		}
		return send(OfferGood, VerbGood)

	case phaseFlek:
		if skill != module.SkillEasy {
			if id, verb := b.doubling(s, me, skill, enabled); id != "" {
				return send(id, verb)
			}
		}
		return send(OfferPass, VerbPass)

	case phasePlay:
		if o := enabled(OfferPlay); o != nil {
			return send(OfferPlay, VerbPlay, b.playCard(s, me, skill, o.Source.Cards, r))
		}
	}
	return module.ChooseAction(offers, []string{VerbPass, VerbGood})
}

// --- the auction --------------------------------------------------------------

// suitScore is how good a suit looks as trumps: length first, then the
// cards that win tricks and score.
func suitScore(hand []string, suit byte) int {
	n, hasK, hasQ := 0, false, false
	for _, c := range hand {
		if tricks.Suit(c) != suit {
			continue
		}
		n += 3
		switch tricks.Rank(c) {
		case 'A':
			n += 3
		case 'T':
			n += 2
		case 'K':
			n++
			hasK = true
		case 'Q':
			n++
			hasQ = true
		}
	}
	if hasK && hasQ {
		n += 2
	}
	return n
}

// trumpCard names trumps by the lowest card of the best-looking suit.
func trumpCard(seven []string) string {
	best, bestScore := byte(0), -1
	for _, suit := range []byte("HDCS") {
		if sc := suitScore(seven, suit); sc > bestScore {
			best, bestScore = suit, sc
		}
	}
	card := ""
	for _, c := range seven {
		if tricks.Suit(c) == best && (card == "" || orderTrump.Beats(tricks.Rank(card), tricks.Rank(c))) {
			card = c
		}
	}
	return card
}

func count(hand []string, keep func(string) bool) int {
	n := 0
	for _, c := range hand {
		if keep(c) {
			n++
		}
	}
	return n
}

// announcement picks a game from the chooser's twelve cards.
func announcement(hand []string, trump byte, skill module.Skill) string {
	if skill == module.SkillEasy {
		return OfferHra
	}
	if betlHand(hand) {
		return OfferBetl
	}
	trumps := count(hand, func(c string) bool { return tricks.Suit(c) == trump })
	seven := hasCard(hand, "7"+string(trump))
	switch {
	case stoHand(hand, trump) && seven:
		return OfferStoSedma
	case stoHand(hand, trump):
		return OfferSto
	case seven && trumps >= 5:
		return OfferHraSedma
	}
	return OfferHra
}

// betlHand is a hand that can lose every trick: no ace, král or svršek, and
// every suit held starts from the seven or the eight.
func betlHand(hand []string) bool {
	low := map[byte]bool{}
	for _, c := range hand {
		switch tricks.Rank(c) {
		case 'A', 'K', 'Q':
			return false
		case '7', '8':
			low[tricks.Suit(c)] = true
		}
	}
	for _, c := range hand {
		if !low[tricks.Suit(c)] {
			return false
		}
	}
	return true
}

// talonPair picks the two cards to lay away from those the rules allow.
//
// In a trump game: short plain suits first, low cards first, never a
// trump or half a marriage. In betl the most dangerous cards go — the
// highest — and in durch the weakest.
func talonPair(s *GameState, hand, allowed []string) []string {
	trump := s.trump()
	suitLen := map[byte]int{}
	for _, c := range hand {
		suitLen[tricks.Suit(c)]++
	}
	keep := func(c string) int {
		strength := len(orderPlain) - rankIndex(orderPlain, tricks.Rank(c))
		switch s.Game {
		case gameBetl:
			return -strength
		case gameDurch:
			return strength
		}
		v := suitLen[tricks.Suit(c)]*10 + len(orderTrump) - rankIndex(orderTrump, tricks.Rank(c))
		if tricks.Suit(c) == trump {
			v += 1000
		}
		if r := tricks.Rank(c); (r == 'K' && hasCard(hand, "Q"+string(tricks.Suit(c)))) || (r == 'Q' && hasCard(hand, "K"+string(tricks.Suit(c)))) {
			v += 100
		}
		return v
	}
	sorted := append([]string(nil), allowed...)
	sort.SliceStable(sorted, func(i, j int) bool { return keep(sorted[i]) < keep(sorted[j]) })
	return sorted[:2]
}

func rankIndex(o tricks.Order, r byte) int {
	for i := 0; i < len(o); i++ {
		if o[i] == r {
			return i
		}
	}
	return len(o)
}

// doubling decides one doubling-round move, or none.
func (b bot) doubling(s *GameState, me string, _ module.Skill, enabled func(string) *module.ActionOffer) (string, string) {
	if !s.trumpGame() {
		return "", ""
	}
	hand := s.Hands[me]
	trump := s.trump()
	trumps := count(hand, func(c string) bool { return tricks.Suit(c) == trump })
	topTrump := hasCard(hand, "A"+string(trump)) || hasCard(hand, "T"+string(trump))
	if me == s.Declarer {
		if trumps >= 6 && topTrump && enabled(OfferFlekGame) != nil {
			return OfferFlekGame, VerbFlek
		}
		return "", ""
	}
	if trumps >= 4 && topTrump && enabled(OfferFlekGame) != nil {
		return OfferFlekGame, VerbFlek
	}
	return "", ""
}

// --- the play -----------------------------------------------------------------

func (b bot) playCard(s *GameState, me string, skill module.Skill, legal []string, r *rand.Rand) string {
	if len(legal) == 1 {
		return legal[0]
	}
	// An easy seat is a player who does not always see the good card.
	if skill == module.SkillEasy && r.Float64() < 0.3 {
		return legal[r.Intn(len(legal))]
	}
	// A hard seat searches: sampled deals, solved open-handed (sampling.go).
	// The rules of thumb below are its fallback if no sample could be.
	if skill == module.SkillHard {
		lim := b.limits
		if lim == (searchLimits{}) {
			lim = defaultLimits
		}
		if c, _, ok := s.searchPlay(me, legal, r, lim); ok {
			return c
		}
	}
	order, trump := s.order(), s.trump()
	strength := func(c string) int { return len(order) - rankIndex(order, tricks.Rank(c)) }
	lowest := func(cards []string) string {
		best := cards[0]
		for _, c := range cards[1:] {
			if strength(c) < strength(best) {
				best = c
			}
		}
		return best
	}
	highest := func(cards []string) string {
		best := cards[0]
		for _, c := range cards[1:] {
			if strength(c) > strength(best) {
				best = c
			}
		}
		return best
	}

	switch s.Game {
	case gameBetl:
		if me == s.Declarer {
			// Under whatever is winning, as high as it can go; on the lead,
			// the lowest card there is.
			if len(s.Trick) == 0 {
				return lowest(legal)
			}
			if losers := b.losing(s, me, legal); len(losers) > 0 {
				return highest(losers)
			}
			return lowest(legal)
		}
		return lowest(legal)
	case gameDurch:
		if me == s.Declarer {
			return highest(legal)
		}
		return lowest(legal)
	}

	// A trump game.
	isTrump := func(c string) bool { return tricks.Suit(c) == trump }
	// cheap is what to throw away: no points, not a trump, low.
	cheap := func(cards []string) string {
		best, bestV := "", 1<<30
		for _, c := range cards {
			v := cardPoints(c)*10 + strength(c)
			if isTrump(c) {
				v += 50
			}
			if v < bestV {
				best, bestV = c, v
			}
		}
		return best
	}

	if len(s.Trick) == 0 {
		hand := s.Hands[me]
		trumps := count(hand, isTrump)
		if me == s.Declarer && trumps >= 3 {
			var ts []string
			for _, c := range legal {
				if isTrump(c) {
					ts = append(ts, c)
				}
			}
			if len(ts) > 0 && (hasCard(ts, "A"+string(trump)) || hasCard(ts, "T"+string(trump))) {
				return highest(ts)
			}
		}
		// A svršek whose král is in hand: leading it announces the marriage.
		for _, c := range legal {
			if tricks.Rank(c) == 'Q' && hasCard(hand, "K"+string(tricks.Suit(c))) {
				return c
			}
		}
		for _, c := range legal {
			if tricks.Rank(c) == 'A' && !isTrump(c) {
				return c
			}
		}
		return cheap(legal)
	}

	win, _ := tricks.Trick{Plays: s.Trick}.Winning(trump, order)
	winner := s.Players[win.Seat]
	friendly := (winner == s.Declarer) == (me == s.Declarer)
	last := len(s.Trick) == len(s.active())-1

	if friendly {
		if last {
			// The trick is ours: put the points on it.
			best, bestV := legal[0], -1
			for _, c := range legal {
				v := cardPoints(c)*10 - strength(c)
				if isTrump(c) {
					v -= 50
				}
				if v > bestV {
					best, bestV = c, v
				}
			}
			return best
		}
		return cheap(legal)
	}
	if winners := b.winning(s, me, legal); len(winners) > 0 {
		return lowest(winners)
	}
	return cheap(legal)
}

// winning is the cards that would take the trick from where it stands.
func (b bot) winning(s *GameState, me string, cards []string) []string {
	var out []string
	for _, c := range cards {
		t := tricks.Trick{Plays: append(append([]tricks.Play(nil), s.Trick...), tricks.Play{Seat: s.seat(me), Card: c})}
		if w, _ := t.Winning(s.trump(), s.order()); w.Seat == s.seat(me) {
			out = append(out, c)
		}
	}
	return out
}

// losing is the cards that would not take it.
func (b bot) losing(s *GameState, me string, cards []string) []string {
	wins := b.winning(s, me, cards)
	return removeCards(append([]string(nil), cards...), wins...)
}

// --- licitovaný -----------------------------------------------------------------

// biddingLimit is the highest rung a seat will bid or hold on its ten cards
// in a trump game, before it has seen the talon: sedma where it holds a
// seven in a long suit, sto with a trump marriage behind it. Zero is "pass".
// Betl and durch are noTrumpGame's to judge.
func biddingLimit(hand []string, skill module.Skill) int {
	limit := 0
	for _, suit := range []byte("HDCS") {
		trumps := count(hand, func(c string) bool { return tricks.Suit(c) == suit })
		red := suit == suitRed
		if hasCard(hand, "7"+string(suit)) && trumps >= 4 {
			limit = max(limit, contractRung(kindSedma, red))
		}
		if skill == module.SkillEasy {
			continue
		}
		if stoHand(hand, suit) {
			limit = max(limit, contractRung(kindSto, red))
		}
	}
	return limit
}

// licitContract picks what to announce with the talon in hand: durch or
// betl on a hand judged to make one, sto or sedma in the strongest suit the offers allow, omyl when
// held to plain sedma without a seven to play it — and otherwise the lowest
// contract the offers allow, in the first suit they list.
//
// game is the betl or durch the hand was judged to make (noTrumpGame), and
// classic is the judgement before that existed: betlHand, never durch.
func licitContract(hand []string, skill module.Skill, offers []module.ActionOffer, game string, classic bool) (module.Action, bool) {
	find := func(id string) *module.ActionOffer {
		for i := range offers {
			if offers[i].ID == id && offers[i].Enabled {
				return &offers[i]
			}
		}
		return nil
	}
	choices := func(o *module.ActionOffer, name string) []string {
		var out []string
		for _, p := range o.Params {
			if p.Name == name {
				for _, c := range p.Choices {
					out = append(out, c.Value)
				}
			}
		}
		return out
	}
	// The offered suit this hand would rather have as trumps.
	bestSuit := func(suits []string) string {
		best, bestScore := "", -1
		for _, suit := range suits {
			if sc := suitScore(hand, suit[0]); sc > bestScore {
				best, bestScore = suit, sc
			}
		}
		return best
	}
	withTrump := func(o *module.ActionOffer) (module.Action, bool) {
		a := module.Action{OfferID: o.ID, Verb: o.Verb, Params: map[string]string{}}
		if trumps := choices(o, "trump"); len(trumps) > 0 {
			a.Params["trump"] = bestSuit(trumps)
		}
		if helpers := choices(o, "helper"); len(helpers) > 0 {
			for _, h := range helpers {
				if h != a.Params["trump"] {
					a.Params["helper"] = h
					break
				}
			}
		}
		return a, true
	}

	if o := find(OfferLDurch); o != nil && !classic && game == gameDurch {
		return withTrump(o)
	}
	if o := find(OfferLBetl); o != nil && (classic && skill != module.SkillEasy && betlHand(hand) || !classic && game == gameBetl) {
		return withTrump(o)
	}
	if o := find(OfferLSto); o != nil && skill != module.SkillEasy {
		if suit := bestSuit(choices(o, "trump")); suit != "" && stoHand(hand, suit[0]) {
			return withTrump(o)
		}
	}
	if o := find(OfferLSedma); o != nil {
		return withTrump(o)
	}
	if o := find(OfferOmyl); o != nil {
		return module.Action{OfferID: OfferOmyl, Verb: VerbFold}, true
	}
	for _, id := range []string{OfferLSto, OfferLStoSedma, OfferLBetl, OfferLDurch, OfferLDveSedmy, OfferLDveSedmySto} {
		if o := find(id); o != nil {
			return withTrump(o)
		}
	}
	return module.ChooseAction(offers, nil)
}

// stoHand is a hand that can count to a hundred with trump as trumps: the
// trump marriage (40 of it), the trump ace to hold the suit, six trumps, and
// five aces and tens between them for the other sixty. A failed sto pays for
// every ten points short, so this errs on the side of not announcing one.
func stoHand(hand []string, trump byte) bool {
	t := string(trump)
	if !hasCard(hand, "K"+t) || !hasCard(hand, "Q"+t) || !hasCard(hand, "A"+t) {
		return false
	}
	trumps := count(hand, func(c string) bool { return tricks.Suit(c) == trump })
	sharps := count(hand, func(c string) bool { return cardPoints(c) > 0 })
	return trumps >= 6 && sharps >= 5
}
