package canasta

import (
	"sort"

	"zolik/server/internal/module"
)

// How a seat nobody is sitting at plays Canasta.
//
// This module used to answer module.OfferBot(VerbLayMeld, VerbLayOff,
// VerbTakePile, VerbTakeTop, VerbDraw, VerbDiscard), and the comment beside it
// said that was enough here where it would not be for Žolíky, because a
// Canasta meld ships as exact cards rather than as a shape to solve. That is
// true of *melding* and it was never true of the rest of the turn. An
// offer-preference bot takes the first enabled offer of its favourite verb,
// and a turn has to end in a discard, so the question "which card" was being
// answered by which one sorted first.
//
// Card notation sorts digits before letters. So the first discardable card in
// a hand holding a deuce is the deuce — a wild card, twenty points, one of the
// eight things in the deck that can finish a canasta — and the old bot threw
// one away on every turn it held one, before it threw any of its ordinary
// cards. It also handed the pile to whoever wanted it, because a wild discard
// freezes the pile for everyone including the side that just melded.
//
// So this is a module.Botted implementation. It is built entirely on the offer
// list and the decoded state — it never decides a move is legal, only which of
// the legal ones to make — and the judgements it adds are the ones a person
// makes at this game:
//
//	the pile      a captured pile is the biggest single swing in Canasta, and
//	              the whole of the endgame is about who gets the last one.
//	the wilds     a wild is not twenty points, it is the card that turns six
//	              of a rank into a canasta. Spending one to fill a meld that
//	              is nowhere near seven is the commonest beginner error and
//	              was this bot's default.
//	the discard   which card is least use to the player it is being handed to.
//	the close     whether to end the deal at all. Going out is worth a
//	              hundred; the canasta it interrupts is worth three or five,
//	              and the pile still to be taken is worth whatever is in it.
//	              See worthGoingOut, which is the judgement Samba needs most
//	              and the one an offer list can never carry.
type bot struct{}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	s, err := decode(raw)
	if err != nil {
		return module.ChooseAction(offers, nil)
	}
	if s.Break.Open || s.Status != "active" || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, nil)
	}
	p := profileFor(seat.Skill)
	mn := menuOf(offers)
	tb := b.read(s, seat.PlayerID, p)

	if s.Phase == phaseDraw {
		if a, ok := b.draw(s, seat.PlayerID, p, mn); ok {
			return a, true
		}
	}
	if a, ok := b.build(s, seat.PlayerID, p, tb, mn); ok {
		return a, true
	}
	if a, ok := b.discard(s, seat.PlayerID, p, tb, mn); ok {
		return a, true
	}
	// Nothing this file has an opinion about: the offer list is the whole
	// answer, exactly as it was before this file existed.
	return module.ChooseAction(offers, []string{VerbLayMeld, VerbLayOff, VerbTakePile, VerbTakeTop, VerbDraw, VerbDiscard})
}

// --- how well to play it -----------------------------------------------------

// profile is what a skill setting changes.
//
// Deliberately small, and the thing that is *not* in it is the point: no
// profile discards a wild card while it holds anything else. That is not a
// strength setting, it is the difference between a weak player and a broken
// one — a beginner at this game hoards wilds, they do not fire them into the
// pile — and making it a knob would only be a way of shipping the old bug
// under a new name.
type profile struct {
	skill module.Skill

	// takesPile is whether this profile will capture the discard pile at all
	// when the capture is not forced on it by being the only move. A pile is
	// cards, and cards are what this game is scored on.
	takesPile bool
	// pileWorthTaking is how many cards the pile must hold before capturing it
	// beats drawing two. Zero means the default.
	pileWorthTaking int
	// readsDanger is whether the discard avoids feeding a rank the opposing
	// side has already melded — which is how a pile gets captured, and the
	// Canasta equivalent of "the AI just fed me my run".
	readsDanger bool
	// hoardsWilds is whether a wild is held back from an ordinary meld and
	// spent only where it finishes a canasta or opens the side's account.
	// Below this, wilds are still never discarded — they are simply melded as
	// soon as they fit anywhere.
	hoardsWilds bool
	// meldsEarly lays every meld the moment it is legal. Weak, and weak in the
	// way real beginners are: it announces the hand and strands wilds in melds
	// that will never reach seven.
	meldsEarly bool
	// banksPoints is whether this profile weighs closing the deal against what
	// is still on the table to be won. See worthGoingOut.
	banksPoints bool
	// readsHandCounts notices that an opponent is nearly out of cards, and
	// switches from building to shedding the cards that would be scored
	// against the side. Reads sizes only, never contents — see handCounts.
	readsHandCounts bool
	// endgameAt is how few cards an opponent must hold for that switch. Zero
	// means the default.
	endgameAt int
}

var profiles = map[module.Skill]profile{
	module.SkillEasy: {
		skill:      module.SkillEasy,
		meldsEarly: true,
		// Takes a pile, but only an obvious one. An earlier draft had this
		// profile never take the pile at all, and that is not a weak player,
		// it is a broken one: without captures a side never assembles a
		// canasta, without a canasta it can never go out, and every deal ends
		// on an exhausted stock with both scores drifting downwards. A table
		// of them never reaches a target score at all.
		takesPile:       true,
		pileWorthTaking: 10,
	},
	module.SkillMedium: {
		skill:           module.SkillMedium,
		takesPile:       true,
		pileWorthTaking: 5,
		readsDanger:     true,
		readsHandCounts: true,
	},
	module.SkillHard: {
		skill:           module.SkillHard,
		takesPile:       true,
		pileWorthTaking: 3,
		readsDanger:     true,
		hoardsWilds:     true,
		banksPoints:     true,
		readsHandCounts: true,
		// A card earlier than the default. A seat holding three cards goes out
		// next turn on a lay-off and a discard; by the time it holds two the
		// expensive cards this side is carrying are already lost.
		endgameAt: 3,
	},
}

// endgameHandSize is the default for profile.endgameAt.
const endgameHandSize = 2

// profileFor is the strength a skill plays at. An unknown or empty skill is
// Medium — module.BotSeat's compatibility rule.
func profileFor(s module.Skill) profile {
	if p, ok := profiles[s]; ok {
		return p
	}
	return profiles[module.SkillMedium]
}

// --- reading the table -------------------------------------------------------

// table is what the bot has worked out about the position before it picks a
// move: who is close to ending the deal, and whether ending it is this side's
// interest.
type table struct {
	// pressed is true when a player on the other side is close enough to going
	// out that the cards still in hand have stopped being an investment and
	// started being a bill.
	pressed bool
	// closing is whether this side is better off ending the deal than going on
	// building in it.
	closing bool
}

// read builds that judgement.
//
// The bot is handed the whole state — every seat's hand — because that is what
// the runtime has, and nothing here reads a card out of one that is not its
// own. Sizes only, which is what every client already draws on screen, and
// which is the difference between a strong opponent and a cheating one.
// TestBotDoesNotPeek pins it.
func (b bot) read(s *GameState, playerID string, p profile) table {
	out := table{}
	if !p.readsHandCounts {
		return out
	}
	at := p.endgameAt
	if at <= 0 {
		at = endgameHandSize
	}
	mine := s.TeamOf[playerID]
	shortest := 1 << 30
	for id, hand := range s.Hands {
		if s.TeamOf[id] == mine {
			continue
		}
		if len(hand) < shortest {
			shortest = len(hand)
		}
	}
	out.pressed = shortest <= at || len(s.DrawPile) <= 2
	out.closing = out.pressed || !p.banksPoints || worthGoingOut(s, playerID)
	return out
}

// worthGoingOut answers the question this game is actually about, and the one
// an offer list cannot ask.
//
// Going out is worth a hundred points, two hundred if the hand was concealed.
// A canasta is worth three hundred, or five hundred with no wild in it, and a
// captured pile is worth whatever happens to be in it — which late in a deal
// is routinely more than either. So closing is not the goal, it is one of the
// ways a deal can end, and ending it while the other side is still behind on
// melded cards throws away the part of the deal that was going well.
//
// This matters most in Samba, where the extra deck, the sequences and the
// two-canasta quota mean a deal has far more left in it at the moment going
// out first becomes legal: a side that closes the instant it may is trading a
// hundred points for several hundred it had already half-built.
//
// So: close when it wins the deal, or when somebody else is about to close it
// anyway. Otherwise keep playing — the cards are still coming.
func worthGoingOut(s *GameState, playerID string) bool {
	mine := s.team(playerID)
	if mine == nil {
		return true
	}
	r := s.rules()
	best := 0
	for i := range s.Teams {
		if s.Teams[i].ID == mine.ID {
			continue
		}
		if v := meldedValue(r, &s.Teams[i]); v > best {
			best = v
		}
	}
	return meldedValue(r, mine) >= best
}

// meldedValue is what a side's table is worth: the cards on it, plus the
// bonuses the deal will pay for the canastas among them. Read off the
// variation's own ruleset rather than from constants, so Samba's sequences and
// its larger bonuses are counted at what Samba pays for them.
func meldedValue(r ruleset, t *Team) int {
	total := 0
	for i := range t.Melds {
		m := t.Melds[i]
		total += handValue(m.Cards)
		if !m.isCanasta() {
			continue
		}
		if containsWild(m.Cards) {
			total += r.MixedCanastaBonus
		} else {
			total += r.NaturalCanastaBonus
		}
	}
	return total
}

func containsWild(cards []string) bool {
	for _, c := range cards {
		if isWild(c) {
			return true
		}
	}
	return false
}

// --- the menu ----------------------------------------------------------------

// menu is the enabled offers, kept whole rather than indexed by verb: this
// module offers several take_pile and several lay_meld at once, and which one
// is the entire decision.
type menu struct{ enabled []module.ActionOffer }

func menuOf(offers []module.ActionOffer) menu {
	mn := menu{}
	for _, o := range offers {
		if o.Enabled {
			mn.enabled = append(mn.enabled, o)
		}
	}
	return mn
}

// byVerb is every enabled offer for one verb, in the order the module listed
// them.
func (mn menu) byVerb(verb string) []module.ActionOffer {
	var out []module.ActionOffer
	for _, o := range mn.enabled {
		if o.Verb == verb {
			out = append(out, o)
		}
	}
	return out
}

// --- drawing, or taking the pile ---------------------------------------------

// draw decides how the turn starts.
//
// Taking the pile is the strongest move in Canasta and it is also the only one
// that can be wrong for a reason other than the cards: a capture spends
// naturals out of the hand to claim the top card, and a pile of one is not
// worth two of them. So it is priced in cards, against a floor the profile
// sets, with one exception that overrides the floor — a side that has not
// melded yet is buying its initial meld as well as the pile, and that is worth
// paying for at any size.
func (b bot) draw(s *GameState, playerID string, p profile, mn menu) (module.Action, bool) {
	if p.takesPile {
		if o, ok := b.bestCapture(s, playerID, p, mn); ok {
			return module.SubmissionFor(o)
		}
	}
	// Samba's one-card capture: it costs nothing out of hand and leaves the
	// pile standing, so it is taken whenever it is on offer.
	if tops := mn.byVerb(VerbTakeTop); len(tops) > 0 {
		return module.SubmissionFor(tops[0])
	}
	for _, o := range mn.byVerb(VerbDraw) {
		return module.SubmissionFor(o)
	}
	return module.Action{}, false
}

// bestCapture picks which pile capture to make, if any is worth making.
func (b bot) bestCapture(s *GameState, playerID string, p profile, mn menu) (module.ActionOffer, bool) {
	opts := mn.byVerb(VerbTakePile)
	if len(opts) == 0 {
		return module.ActionOffer{}, false
	}
	floor := p.pileWorthTaking
	if floor <= 0 {
		floor = 5
	}
	// A side still short of its initial meld is buying two things with one
	// move, so the pile does not have to be big to be worth it.
	if t := s.team(playerID); t != nil && !t.HasMelded {
		floor = 1
	}
	if len(s.DiscardPile) < floor {
		return module.ActionOffer{}, false
	}
	// Cheapest capture first: the fewest cards out of hand, and among equals
	// the fewest wilds, because a capture paid for with a wild has spent the
	// most valuable card in the hand on a card that was free to whoever went
	// before.
	best, found := module.ActionOffer{}, false
	for _, o := range opts {
		if !found || cheaperCapture(o, best) {
			best, found = o, true
		}
	}
	return best, found
}

// captureCost is what a take-pile offer spends out of hand: how many cards,
// and how many of them are wild.
func captureCost(o module.ActionOffer) (cards, wilds int) {
	if o.Source == nil || o.Source.Zone != module.FromHand {
		// A capture onto a meld already on the table pays nothing out of hand.
		return 0, 0
	}
	for _, c := range o.Source.Cards {
		cards++
		if isWild(c) {
			wilds++
		}
	}
	return cards, wilds
}

func cheaperCapture(x, y module.ActionOffer) bool {
	xc, xw := captureCost(x)
	yc, yw := captureCost(y)
	if xw != yw {
		return xw < yw
	}
	if xc != yc {
		return xc < yc
	}
	return x.ID < y.ID
}

// --- building the table ------------------------------------------------------

// build lays a meld or extends one, and is where wild cards are actually spent.
func (b bot) build(s *GameState, playerID string, p profile, tb table, mn menu) (module.Action, bool) {
	t := s.team(playerID)
	held := len(s.Hands[playerID])

	// Lay-offs first. A lay-off grows a meld the side already owns, which is
	// the only way a canasta ever gets finished, and unlike a new meld it can
	// never split the hand's material across two ranks that each then stall at
	// three cards.
	if o, cards, ok := b.bestLayOff(s, t, p, mn); ok {
		if tb.closing || !emptiesHand(held, len(cards)) {
			return module.Action{OfferID: o.ID, Verb: VerbLayOff, Target: o.Target.MeldID, Cards: cards}, true
		}
	}

	melds := mn.byVerb(VerbLayMeld)
	if len(melds) == 0 {
		return module.Action{}, false
	}
	if p.meldsEarly {
		return module.SubmissionFor(melds[0])
	}
	best, found := module.ActionOffer{}, false
	for _, o := range melds {
		if p.hoardsWilds && spendsWild(o) && !worthAWild(s, t, o) {
			continue
		}
		// The move that ends the deal, declined. See worthGoingOut: a side
		// still behind on melded cards has more to win by playing on than the
		// hundred points closing pays, and in Samba a great deal more.
		if !tb.closing && emptiesHand(held, len(meldCards(o))) {
			continue
		}
		if !found || betterMeld(o, best) {
			best, found = o, true
		}
	}
	if !found {
		return module.Action{}, false
	}
	return module.SubmissionFor(best)
}

// emptiesHand reports that playing this many cards leaves nothing but the card
// the turn has to end with — which is to say, going out.
func emptiesHand(held, played int) bool { return held-played <= 1 }

// meldCards is what an offer would put on the table.
func meldCards(o module.ActionOffer) []string {
	if o.Source == nil {
		return nil
	}
	if len(o.Source.Submit) > 0 {
		return o.Source.Submit
	}
	return o.Source.Cards
}

func spendsWild(o module.ActionOffer) bool {
	for _, c := range meldCards(o) {
		if isWild(c) {
			return true
		}
	}
	return false
}

// worthAWild reports that this meld is one of the two places a wild card earns
// its keep: it finishes a canasta, or it is what opens the side's account.
//
// Everywhere else a wild in a three-card meld is a card taken out of play. The
// meld it is in still needs four more naturals to be worth anything beyond its
// face value, and the wild could have been the seventh card of a rank the hand
// already holds five of. That is the trade this test refuses to make.
func worthAWild(s *GameState, t *Team, o module.ActionOffer) bool {
	if t != nil && !t.HasMelded {
		return true
	}
	return len(meldCards(o)) >= canastaSize
}

// betterMeld orders the melds a hand could lay: the biggest first, and among
// equals the one that spends no wild.
//
// Size before value on purpose. Four cards of a rank is four sevenths of a
// canasta and three is three sevenths, and a canasta is where all the points in
// this game actually are — five hundred for a natural one, against ten a card
// for the cards themselves.
func betterMeld(x, y module.ActionOffer) bool {
	xc, yc := meldCards(x), meldCards(y)
	if len(xc) != len(yc) {
		return len(xc) > len(yc)
	}
	xw, yw := spendsWild(x), spendsWild(y)
	if xw != yw {
		return !xw
	}
	if v, w := handValue(xc), handValue(yc); v != w {
		return v > w
	}
	return x.ID < y.ID
}

// bestLayOff picks a card to add to one of the side's own melds.
//
// Naturals before wilds, and a meld that is close to seven before one that is
// not: the last card of a canasta is worth several hundred points and the
// fourth card of a meld that will never get there is worth ten.
func (b bot) bestLayOff(s *GameState, t *Team, p profile, mn menu) (module.ActionOffer, []string, bool) {
	type option struct {
		offer module.ActionOffer
		card  string
		room  int // how many cards short of a canasta this meld still is
		wild  bool
	}
	var best *option
	for _, o := range mn.byVerb(VerbLayOff) {
		if o.Source == nil || o.Target == nil {
			continue
		}
		size := 0
		if t != nil {
			for i := range t.Melds {
				if t.Melds[i].ID == o.Target.MeldID {
					size = len(t.Melds[i].Cards)
				}
			}
		}
		short := canastaSize - size
		for _, c := range o.Source.Cards {
			wild := isWild(c)
			// A wild goes onto a meld only where it completes a canasta —
			// anywhere else it is being spent to save a card that had
			// somewhere better to be. Below the hoarding profiles it still
			// goes on last, after every natural that fits.
			if wild && p.hoardsWilds && short > 1 {
				continue
			}
			cand := option{offer: o, card: c, room: short, wild: wild}
			if best == nil || betterLayOff(cand.wild, cand.room, cand.card, best.wild, best.room, best.card) {
				c := cand
				best = &c
			}
		}
	}
	if best == nil {
		return module.ActionOffer{}, nil, false
	}
	return best.offer, []string{best.card}, true
}

func betterLayOff(xWild bool, xRoom int, xCard string, yWild bool, yRoom int, yCard string) bool {
	if xWild != yWild {
		return !xWild
	}
	if xRoom != yRoom {
		return xRoom < yRoom
	}
	return xCard < yCard
}

// --- ending the turn ---------------------------------------------------------

// discard chooses the card to end the turn with, which is the decision this
// whole file exists for.
//
// The order is: never a wild while anything else is legal; then keep the
// material the hand is building with; then do not hand the opposing side the
// rank they are collecting; then let the cheapest card go.
func (b bot) discard(s *GameState, playerID string, p profile, tb table, mn menu) (module.Action, bool) {
	offers := mn.byVerb(VerbDiscard)
	if len(offers) == 0 {
		return module.Action{}, false
	}
	o := offers[0]
	if o.Source == nil || len(o.Source.Cards) == 0 {
		return module.SubmissionFor(o)
	}

	hand := s.Hands[playerID]
	counts := map[string]int{}
	for _, c := range hand {
		counts[rankOf(c)]++
	}
	danger := map[string]bool{}
	if p.readsDanger {
		mine := s.TeamOf[playerID]
		for i := range s.Teams {
			if s.Teams[i].ID == mine {
				continue
			}
			for _, m := range s.Teams[i].Melds {
				if m.Rank != "" {
					danger[m.Rank] = true
				}
			}
		}
	}

	cands := make([]discardCandidate, 0, len(o.Source.Cards))
	for _, c := range o.Source.Cards {
		cands = append(cands, discardCandidate{
			card: c,
			wild: isWild(c),
			// Two or more of a rank in hand is a meld waiting for a third, and
			// breaking one up to save five points is how a hand never melds.
			building: counts[rankOf(c)] >= 2,
			// A black three cannot be captured with and blocks the pile for
			// the next player, so it is the safest card in the deck to throw
			// and its five points are worth spending to keep a fat pile shut.
			blocks:  isBlackThree(c),
			feeds:   danger[rankOf(c)],
			value:   cardValue(c),
			ordinal: c,
		})
	}
	sort.SliceStable(cands, func(i, j int) bool { return betterDiscard(cands[i], cands[j], s, tb) })
	return module.Action{OfferID: o.ID, Verb: VerbDiscard, Cards: []string{cands[0].card}}, true
}

type discardCandidate struct {
	card     string
	wild     bool
	building bool
	blocks   bool
	feeds    bool
	value    int
	ordinal  string
}

func betterDiscard(x, y discardCandidate, s *GameState, tb table) bool {
	// The rule this file was written for. A wild is never the card a turn ends
	// with unless it is the only card the engine will take, and the engine has
	// already filtered this list for that.
	if x.wild != y.wild {
		return !x.wild
	}
	// A black three shuts the pile. Worth doing when there is a pile worth
	// shutting, and not worth the five points when there is not.
	if x.blocks != y.blocks && len(s.DiscardPile) >= 5 {
		return x.blocks
	}
	// Material worth keeping — until somebody is about to end the deal, at
	// which point a pair that was an investment two turns ago is two cards
	// about to be counted against this side.
	if x.building != y.building && !tb.pressed {
		return !x.building
	}
	if x.feeds != y.feeds {
		return !x.feeds
	}
	if x.value != y.value {
		// Ordinarily the cheap card goes and the aces stay for melding. Under
		// pressure it inverts: every point still in hand when somebody goes
		// out is a point subtracted from this side's deal, so the expensive
		// card is the one to be rid of.
		if tb.pressed {
			return x.value > y.value
		}
		return x.value < y.value
	}
	return x.ordinal < y.ordinal
}
