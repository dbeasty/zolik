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
type bot struct {
	// style, when set, is the profile this bot plays whatever skill its seat
	// was given — a style from Styles rather than a rung of the ladder.
	style *profile
}

var _ module.Bot = bot{}

func (b bot) Act(raw module.State, seat module.BotSeat, offers []module.ActionOffer) (module.Action, bool) {
	// The undos are for people, and a bot must not see them.
	//
	// Not squeamishness about bots changing their minds — it is that they
	// cannot. An undo restores the position exactly, and every path below is a
	// function of the position, so a bot that undid a move would be handed the
	// same state and make the same move again, forever. The one position where
	// it would ever reach for one is the wedge MeldLaid exists for, and that is
	// precisely the position it would oscillate in.
	//
	// What keeps a bot out of that wedge is opening.go: an unopened side only
	// captures, melds or lays off once it has found, through the engine, the
	// whole of an opening that still leaves it a discard. This leaves bots
	// exactly where they were before the undos existed, which is the point.
	offers = withoutUndos(offers)

	s, err := decode(raw)
	if err != nil {
		return module.ChooseAction(offers, nil)
	}
	if s.Break.Open || s.Status != "active" || s.Current != seat.PlayerID {
		return module.ChooseAction(offers, nil)
	}
	p := profileFor(seat.Skill)
	if b.style != nil {
		p = *b.style
	}
	mn := menuOf(offers)
	tb := b.read(s, seat.PlayerID, p)

	if s.Phase == phaseDraw {
		if a, ok := b.draw(raw, s, seat.PlayerID, p, mn); ok {
			return a, true
		}
	}
	if a, ok := b.build(raw, s, seat.PlayerID, p, tb, mn); ok {
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
	// racesForPartners changes two things at a partnership table, and
	// nothing heads-up: banksPoints is set aside, so the side goes out the
	// first turn it may, and hoardsWilds is relaxed, so a wild goes where it
	// gets the side to its canastas and then out. See partnered.
	racesForPartners bool
	// infers prices each discard by the chance the next seat captures the
	// pile with it, read off what that seat probably holds (infer.go: its
	// captures, discards and passes this deal), times the cards the pile
	// would hand it. Below feeding a melded rank, above the card's own value.
	infers bool
	// closer races for the side's canasta quota and then out: see
	// closerProfile. It changes where wilds go, when the pile is worth taking
	// and which card is shed, never what is legal.
	closer bool
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
		// Banking and hoarding are heads-up judgements. At a partnership
		// table they lose to every profile that simply races — see partnered.
		racesForPartners: true,
		infers:           true,
		// A card earlier than the default. A seat holding three cards goes out
		// next turn on a lay-off and a discard; by the time it holds two the
		// expensive cards this side is carrying are already lost.
		endgameAt: 3,
	},
}

// closerProfile is the closer: a style, not a rung of the ladder, for the
// learning pool and the bench (learnGame.Styles). It is the player who ends
// deals early — modelled on Žolíky's ai.CloserProfile — and it exists because
// the ladder does not have one: Hard banks points (worthGoingOut) and hoards
// its wilds for natural canastas, so a deal against it runs long and nobody
// ever punishes a big hand left over at the end.
//
// It reads the table as Hard does. What it changes:
//
//	the pile     taken at any size while the side is short of its canastas,
//	             because a capture is the fastest way to seven of a rank;
//	             once the quota is met, only when the capture does not leave
//	             the hand bigger than it was.
//	the wilds    spent to finish the quota canasta — laid off onto any meld
//	             of four or more, or in a new meld of seven — and, once the
//	             quota is met, onto anything that takes them. A mixed canasta
//	             now beats a natural one later.
//	the close    always worth it (no banksPoints): once the side may go out,
//	             every lay-off and meld that empties the hand is made.
//	the discard  once the quota is met, the dead cards go first — a rank the
//	             side can no longer meld, then singles before pairs — and the
//	             dearest of them, since the hand is about to be counted.
var closerProfile = profile{
	skill:           module.SkillHard,
	takesPile:       true,
	pileWorthTaking: 1,
	readsDanger:     true,
	hoardsWilds:     true,
	readsHandCounts: true,
	endgameAt:       3,
	closer:          true,
}

// hasQuota reports that this side already has the canastas it needs to go out.
func hasQuota(s *GameState, t *Team) bool { return t != nil && canGoOut(s, t) }

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
	// ending is whether the deal is close enough to over that a canasta
	// finished today beats a better one finished later. It is what licenses
	// the one wild card this bot ever spends on a meld it would rather have
	// left natural — see wildMayJoin.
	//
	// Unlike pressed it is not a strength setting. It is read off the stock,
	// which is a public count every client already draws, so a beginner
	// profile is as entitled to it as an expert: not knowing how many cards
	// are left is not a way of playing badly, it is a way of not looking at
	// the table.
	ending bool
	// racing is a partnership table, at a profile that races there: see
	// partnered. quota is that the side already has the canastas it needs to
	// go out; it is read at such a table and by the closer, and false
	// otherwise, so a heads-up Hard plays as it did without it.
	racing bool
	quota  bool
}

// read builds that judgement.
//
// The bot is handed the whole state — every seat's hand — because that is what
// the runtime has, and nothing here reads a card out of one that is not its
// own. Sizes only, which is what every client already draws on screen, and
// which is the difference between a strong opponent and a cheating one.
// TestBotDoesNotPeek pins it.
func (b bot) read(s *GameState, playerID string, p profile) table {
	out := table{ending: len(s.DrawPile) <= lastTurnsStock(s)}
	if p.racesForPartners && partnered(s, playerID) {
		out.racing = true
		t := s.team(playerID)
		out.quota = t != nil && canGoOut(s, t)
	}
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
	out.ending = out.ending || out.pressed
	out.closing = out.pressed || !p.banksPoints || out.racing || worthGoingOut(s, playerID)
	if p.closer {
		out.quota = hasQuota(s, s.team(playerID))
	}
	return out
}

// partnered reports that this seat plays with a partner, which is where
// worthGoingOut's bargain and the wild hoard both stop paying.
//
// Holding back to build is a wager that the side will be the one to end the
// deal later, and at a partnership table it is a worse wager twice over: there
// are two opposing hands each a few turns from going out instead of one, and
// when one of them does, the partner's whole hand is counted against this side
// as well as this seat's. A wild held for a natural canasta is the dearest card
// in that hand, and the hoard was what mostly kept Hard from going out: in
// nearly every deal it was offered the chance and let it pass, each way out
// needed a wild the hoard would not spend. Measured over 300 seeds, Hard at
// four seats lost to Medium by 448 a match and to the closer style by 475
// while beating both heads-up; racing turns those into wins of 308 and 486.
// So both judgements stay for one-on-one play and are relaxed here — see
// wildMayJoin and worthAWild for where the wilds now go.
func partnered(s *GameState, playerID string) bool {
	t := s.team(playerID)
	return t != nil && len(t.Players) >= 2
}

// lastTurnsStock is the stock a table this size draws in two more turns each.
//
// Below it, "wait for the seventh natural" is a plan with no turns left to
// happen in, and the two hundred points it is holding out for are two hundred
// points nobody is going to score. Scaled by the table rather than fixed,
// because eight cards is a couple of turns at a two-handed Canasta table and
// not even one circuit of a six-handed Samba drawing two apiece.
func lastTurnsStock(s *GameState) int {
	seats := len(s.TurnOrder)
	if seats <= 0 {
		seats = 2
	}
	draw := s.rules().DrawCount
	if draw <= 0 {
		draw = 1
	}
	return seats * draw * 2
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

// withoutUndos drops the take-backs from a bot's view of the offer list. See
// Act for why a bot must never be handed one.
func withoutUndos(offers []module.ActionOffer) []module.ActionOffer {
	out := make([]module.ActionOffer, 0, len(offers))
	for _, o := range offers {
		switch o.Verb {
		case VerbUndoTakePile, VerbUndoLayOff, VerbUndoLayMeld:
			continue
		}
		out = append(out, o)
	}
	return out
}

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
// paying for at any size — provided the rest of the opening is there to be
// laid. See captureOpens.
func (b bot) draw(raw module.State, s *GameState, playerID string, p profile, mn menu) (module.Action, bool) {
	if p.takesPile {
		if o, ok := b.bestCapture(raw, s, playerID, p, mn); ok {
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
func (b bot) bestCapture(raw module.State, s *GameState, playerID string, p profile, mn menu) (module.ActionOffer, bool) {
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
	t := s.team(playerID)
	opening := t != nil && !t.HasMelded
	if opening {
		floor = 1
	}
	if len(s.DiscardPile) < floor {
		return module.ActionOffer{}, false
	}
	quota := p.closer && hasQuota(s, t)
	// Cheapest capture first: the fewest cards out of hand, and among equals
	// the fewest wilds, because a capture paid for with a wild has spent the
	// most valuable card in the hand on a card that was free to whoever went
	// before.
	opts = append([]module.ActionOffer(nil), opts...)
	sort.SliceStable(opts, func(i, j int) bool { return cheaperCapture(opts[i], opts[j]) })
	for _, o := range opts {
		// The capture is the first lay of an opening, and the engine only
		// asks whether the floor is reachable in value, not whether reaching
		// it leaves the two cards a side that cannot go out must end on. So
		// the rest of the opening is found before the pile is touched, or the
		// pile is left where it is.
		if opening {
			a, ok := module.SubmissionFor(o)
			if !ok || !captureOpens(raw, playerID, a) {
				continue
			}
		}
		// The closer with its canastas made wants a smaller hand, not a
		// bigger table: the pile is worth it only if the hand does not grow.
		// The top card goes onto the meld; the rest come into the hand.
		if quota {
			if spent, _ := captureCost(o); len(s.DiscardPile)-1 > spent {
				continue
			}
		}
		return o, true
	}
	return module.ActionOffer{}, false
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
func (b bot) build(raw module.State, s *GameState, playerID string, p profile, tb table, mn menu) (module.Action, bool) {
	t := s.team(playerID)
	held := len(s.Hands[playerID])

	// Never start an opening this turn cannot finish, and never take a step in
	// one that leaves it unfinishable. See opening.go: every strength is held
	// to this, lay-offs included, because the position being avoided is not a
	// weak move but a turn with no legal move in it. That overrides meldsEarly
	// and the lay-off preference below, which are tastes about *which* good
	// move to make, and there is no good version of walking into a dead turn.
	if t != nil && !t.HasMelded {
		return openingMove(raw, s, playerID, mn)
	}

	// Lay-offs first. A lay-off grows a meld the side already owns, which is
	// the only way a canasta ever gets finished, and unlike a new meld it can
	// never split the hand's material across two ranks that each then stall at
	// three cards.
	if o, cards, ok := b.bestLayOff(s, t, p, tb, mn); ok {
		if tb.closing || !emptiesHand(held, len(cards)) {
			cards = layOffTogether(raw, s, playerID, t, o, cards, held, tb.closing)
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
		if p.hoardsWilds && spendsWild(o) && !worthAWildFor(t, p, tb, o) {
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

// worthAWild reports that this new meld is one of the two places a wild card
// earns its keep: it is what opens the side's account, or it lands a canasta
// that would not otherwise be one.
//
// Everywhere else a wild in a three-card meld is a card taken out of play. The
// meld it is in still needs four more naturals to be worth anything beyond its
// face value, and the wild could have been the seventh card of a rank the hand
// already holds five of. That is the trade this test refuses to make.
//
// The canasta clause is wildMayJoin's rule in the other direction, and says
// the same thing for the same reason: a hand laying six naturals and a wild in
// one motion is buying three hundred points with the card that would have made
// it five, so it is worth it once the deal is ending and not before.
func worthAWild(t *Team, tb table, o module.ActionOffer) bool {
	if t != nil && !t.HasMelded {
		return true
	}
	// Racing with the canastas made, every meld is a step out of the hand.
	if tb.racing && tb.quota {
		return true
	}
	if len(meldCards(o)) < canastaSize {
		return false
	}
	return tb.ending
}

// worthAWildFor is worthAWild with the closer's answer: a wild in a new meld
// is worth it once the quota is met (it empties the hand) or when the meld is
// itself a canasta, whatever the stock says.
func worthAWildFor(t *Team, p profile, tb table, o module.ActionOffer) bool {
	if !p.closer {
		return worthAWild(t, tb, o)
	}
	if t != nil && !t.HasMelded {
		return true
	}
	return tb.quota || len(meldCards(o)) >= canastaSize
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
// Naturals before wilds, and among equals the meld the card actually advances:
// the seventh card of a meld is worth several hundred points, the fourth card
// of one that will never get there is worth ten, and the eighth card of a
// finished canasta is worth ten and a risk. See layOffReach.
func (b bot) bestLayOff(s *GameState, t *Team, p profile, tb table, mn menu) (module.ActionOffer, []string, bool) {
	type option struct {
		offer module.ActionOffer
		card  string
		reach int // how much closer to a canasta this meld gets; see layOffReach
		wild  bool
	}
	var best *option
	for _, o := range mn.byVerb(VerbLayOff) {
		if o.Source == nil || o.Target == nil {
			continue
		}
		var target *Meld
		if t != nil {
			for i := range t.Melds {
				if t.Melds[i].ID == o.Target.MeldID {
					target = &t.Melds[i]
				}
			}
		}
		size := 0
		if target != nil {
			size = len(target.Cards)
		}
		for _, c := range o.Source.Cards {
			wild := isWild(c)
			// Where a wild may go, and it is not many places. An offer whose
			// meld this side does not own is not one either: a wild that
			// cannot be priced is a wild that stays in the hand.
			if wild && (target == nil || !wildMayJoinFor(*target, p, tb)) {
				continue
			}
			cand := option{offer: o, card: c, reach: layOffReach(size), wild: wild}
			if best == nil || betterLayOff(cand.wild, cand.reach, cand.card, best.wild, best.reach, best.card) {
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

// layOffTogether turns a natural lay-off onto a group into one lay-off of every
// natural of that rank the offer accepts for the same meld.
//
// One card per action was a stall, not a style. The runtime gives up on a seat
// that takes more than botMaxStall (30) actions in a row without the turn
// moving on (match/bots.go), and a Samba hand that has just captured a big pile
// can owe thirty cards to its own melds: Hard self-play at six seats, seed 823,
// took the pile and then laid off twenty-six cards one at a time — the run was
// thirty-two actions long, and a live table would have frozen on it. The turn
// lays the same cards either way (bestLayOff comes back to each of them in
// turn), so sending them together changes how many actions the turn takes and
// nothing about what is laid.
//
// Naturals onto a group only. A wild is a decision of its own (wildMayJoin)
// and stays one card at a time; a Samba sequence takes cards that continue it,
// which same-rank copies do not. Short of closing, the batch leaves the hand
// the two cards the single lay-off was already required to leave it
// (emptiesHand). The engine has the last word: a batch it refuses is the
// single card the caller chose.
func layOffTogether(raw module.State, s *GameState, playerID string, t *Team, o module.ActionOffer, cards []string, held int, closing bool) []string {
	if len(cards) != 1 || isWild(cards[0]) || o.Source == nil || o.Target == nil || t == nil {
		return cards
	}
	var target *Meld
	for i := range t.Melds {
		if t.Melds[i].ID == o.Target.MeldID {
			target = &t.Melds[i]
		}
	}
	if target == nil || target.kind() == meldRun {
		return cards
	}
	batch := []string{cards[0]}
	skipped := false
	for _, c := range o.Source.Cards {
		if c == cards[0] && !skipped {
			skipped = true // the copy already in the batch
			continue
		}
		if !isWild(c) && rankOf(c) == rankOf(cards[0]) {
			batch = append(batch, c)
		}
	}
	if !closing && len(batch) > held-2 {
		batch = batch[:max(held-2, 1)]
	}
	if len(batch) < 2 || !hasCards(s.Hands[playerID], batch) {
		return cards
	}
	if ok, _ := probe(New(), raw, playerID, module.Action{
		OfferID: o.ID, Verb: VerbLayOff, Target: o.Target.MeldID, Cards: batch,
	}); !ok {
		return cards
	}
	return batch
}

// layOffReach orders melds by what one more card does for them: 1 means the
// card finishes a canasta, 4 means the meld is still four cards away, and a
// meld that is already a canasta sorts last of all.
//
// Last, not first, and that inversion is half of the bug this section exists
// for. The old ordering was "cards short of seven, fewest first", which a meld
// already at seven satisfies better than any other meld on the table — so the
// finished canasta was the preferred destination for every card in the hand,
// wild ones included. Where a group canasta stays open (Samba; see
// ruleset.GroupCanastaCloses) that is where the cards went.
func layOffReach(size int) int {
	if size >= canastaSize {
		return canastaSize
	}
	return canastaSize - size
}

// wildMayJoin answers where a wild card is allowed to go on this side's own
// table, and the answer is "almost nowhere" for a reason the scoring states
// plainly: seven cards of a rank pay five hundred, and the same seven with a
// wild among them pay three.
//
//	already a canasta   Never, at any strength. The meld is finished, so the
//	                    wild buys no bonus at all — and on a natural canasta
//	                    it takes two hundred points back off a side that had
//	                    already earned them. That is not weak play, it is
//	                    unmaking points that were on the table, which is why
//	                    no profile is allowed it: the same footing as never
//	                    discarding a wild.
//	one short, mixed    Yes. A meld that already holds a wild can never be
//	                    natural, three hundred is the whole of what it can be
//	                    worth, and the wild in hand is what collects it.
//	one short, natural  Only once the deal is ending. Six naturals is a
//	                    five-hundred-point canasta waiting on one card of its
//	                    rank; closing it with a wild books three hundred and
//	                    gives up two. Worth doing when there are no turns left
//	                    to draw the seventh in — table.ending — and a loss
//	                    before then.
//	further off         A strength setting, unchanged: a hoarding profile
//	                    keeps the wild for somewhere it finishes something, a
//	                    weaker one spends it on four cards of a rank. That is
//	                    ordinary beginner play rather than a bug, and whether
//	                    this seat is a beginner is the profile's business.
//
//	racing              At a partnership table (see partnered) the hoard is
//	                    relaxed: a wild may join a meld of four or more, a
//	                    step to the canasta the side needs rather than a card
//	                    out of play; and once the side has its canastas,
//	                    any meld short of one, natural six included — going
//	                    out is worth more than the two hundred.
func wildMayJoin(m Meld, p profile, tb table) bool {
	if m.isCanasta() {
		return false
	}
	if len(m.Cards) == canastaSize-1 {
		return m.wilds() > 0 || tb.ending || tb.quota
	}
	if tb.racing && (tb.quota || len(m.Cards) >= canastaSize-3) {
		return true
	}
	return !p.hoardsWilds
}

// wildMayJoinFor is wildMayJoin with the closer's answer: never onto a
// finished canasta, onto anything once the quota is met, and before then onto
// a meld of four or more, where the wild is a step towards the canasta the
// quota needs rather than a card taken out of play.
func wildMayJoinFor(m Meld, p profile, tb table) bool {
	if !p.closer {
		return wildMayJoin(m, p, tb)
	}
	if m.isCanasta() {
		return false
	}
	return tb.quota || len(m.Cards) >= canastaSize-3
}

func betterLayOff(xWild bool, xReach int, xCard string, yWild bool, yReach int, yCard string) bool {
	if xWild != yWild {
		return !xWild
	}
	if xReach != yReach {
		return xReach < yReach
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
	shedding := p.closer && tb.quota
	// dead is a rank this side can no longer put on the table: its meld of
	// that rank is closed and no second one may be started. Such a card can
	// only ever leave the hand as a discard.
	dead := map[string]bool{}
	if t := s.team(playerID); shedding && t != nil {
		r := s.rules()
		for _, m := range t.Melds {
			if m.kind() == meldSet && m.closed(r) && t.rankIsFull(r, m.Rank) && t.openGroup(r, m.Rank) == nil {
				dead[m.Rank] = true
			}
		}
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

	var inf *inference
	if p.infers {
		inf = infer(s, playerID)
	}
	cands := make([]discardCandidate, 0, len(o.Source.Cards))
	for _, c := range o.Source.Cards {
		gives, givesMany := 0, 0
		if inf != nil {
			cards := inf.captureRisk(s, c) * float64(len(s.DiscardPile)+1)
			gives, givesMany = giveawayBucket(cards), int(cards/giveawayMany)
		}
		cands = append(cands, discardCandidate{
			card: c,
			wild: isWild(c),
			// Two or more of a rank in hand is a meld waiting for a third, and
			// breaking one up to save five points is how a hand never melds.
			building: counts[rankOf(c)] >= 2,
			// A black three cannot be captured with and blocks the pile for
			// the next player, so it is the safest card in the deck to throw
			// and its five points are worth spending to keep a fat pile shut.
			blocks:    isBlackThree(c),
			feeds:     danger[rankOf(c)],
			dead:      dead[rankOf(c)],
			gives:     gives,
			givesMany: givesMany,
			value:     cardValue(c),
			ordinal:   c,
		})
	}
	better := func(i, j int) bool { return betterDiscard(cands[i], cands[j], s, tb) }
	if shedding {
		better = func(i, j int) bool { return betterShed(cands[i], cands[j]) }
	}
	sort.SliceStable(cands, better)
	return module.Action{OfferID: o.ID, Verb: VerbDiscard, Cards: []string{cands[0].card}}, true
}

type discardCandidate struct {
	card     string
	wild     bool
	building bool
	blocks   bool
	feeds    bool
	dead     bool
	// gives is the expected cards handed to the next seat by a capture this
	// discard makes possible, in buckets of giveawayCards; zero without the
	// inference.
	gives int
	// givesMany is the same in buckets of giveawayMany, and outranks keeping
	// a pair: a pile of that size is worth more than the meld a pair might
	// become.
	givesMany int
	value     int
	ordinal   string
}

// giveawayCards is how many expected cards of pile one bucket of risk is: a
// discard is only judged riskier than another when it hands over at least
// that many more cards on average.
const giveawayCards = 1.5

// giveawayMany is the expected cards of pile that outweigh breaking a pair.
const giveawayMany = 4.0

func giveawayBucket(cards float64) int { return int(cards / giveawayCards) }

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
	if x.givesMany != y.givesMany {
		return x.givesMany < y.givesMany
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
	if x.gives != y.gives {
		return x.gives < y.gives
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

// betterShed is the closer's discard once its side may go out. The hand is now
// a bill, not an investment, and the question is only which card gets it
// closer to empty: never a wild (it finishes or extends a meld); then a card
// no meld can take; then a single before a pair, since a pair is one card
// from a meld; then, among what is left, not the rank the other side is
// collecting; and the dearest first.
func betterShed(x, y discardCandidate) bool {
	if x.wild != y.wild {
		return !x.wild
	}
	if x.dead != y.dead {
		return x.dead
	}
	if x.building != y.building {
		return !x.building
	}
	if x.feeds != y.feeds {
		return !x.feeds
	}
	if x.value != y.value {
		return x.value > y.value
	}
	return x.ordinal < y.ordinal
}
