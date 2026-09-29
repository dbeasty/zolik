package marias

import (
	"math/rand"

	"zolik/server/internal/module"
	"zolik/server/internal/tricks"
)

var _ module.GameModule = (*Module)(nil)

const handSize = 10

// buildDeck is the 32 cards, in the server's codes.
func buildDeck() []string {
	out := make([]string, 0, 32)
	for _, s := range "HDCS" {
		for _, r := range orderTrump {
			out = append(out, string(r)+string(s))
		}
	}
	return out
}

// NewMatch seats three players and deals the first hand.
func (m *Module) NewMatch(cfg module.MatchConfig, players []module.PlayerRef, seed int64) (module.State, error) {
	if len(players) != 3 {
		return nil, module.Error{Code: "TOO_FEW_PLAYERS", Message: "mariáš is played by three"}
	}
	c := resolve(cfg)
	s := &GameState{
		Status:         "active",
		Seed:           seed,
		Deals:          c.deals,
		Tariff:         c.tariff,
		RedDoubles:     c.redDoubles,
		FlekLimit:      c.flekLimit,
		ZLidu:          c.zLidu,
		ShowCardPoints: c.showCardPoints,
		Pause:          cfg.PauseBetweenRounds(true),
		FirstChooser:   module.StartingSeat(seed, len(players)),
		Scores:         map[string]int{},
	}
	for _, p := range players {
		s.Players = append(s.Players, p.ID)
		s.Scores[p.ID] = 0
	}
	startDeal(s)
	return encode(s)
}

// startDeal shuffles and deals the deal numbered s.Deal.
//
// The chooser gets seven cards to name trumps from and five more they may
// not look at yet; the other two get ten each (ČSM B/4). The packets a real
// dealer gives make no difference to a shuffled deck, so only the counts
// are kept.
func startDeal(s *GameState) {
	deck := buildDeck()
	r := rand.New(rand.NewSource(s.Seed + int64(s.Deal)*7919))
	r.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })

	chooser := s.chooser()
	s.Hands = map[string][]string{chooser: append([]string(nil), deck[:7]...)}
	s.Unseen = append([]string(nil), deck[7:12]...)
	rest := deck[12:]
	for p := s.next(chooser); p != chooser; p = s.next(p) {
		s.Hands[p] = append([]string(nil), rest[:handSize]...)
		rest = rest[handSize:]
		sortHand(s.Hands[p])
	}
	sortHand(s.Hands[chooser])

	s.Phase, s.Current = phaseTrump, chooser
	s.TrumpCard, s.Talon = "", nil
	s.Declarer, s.Game, s.Sedma = chooser, "", false
	s.ProtiSedma, s.ProtiSto = "", ""
	s.Fleks, s.Quiet = map[string]int{}, 0
	s.Trick, s.LastTrick = nil, nil
	s.TricksWon, s.Points, s.Marriages = map[string]int{}, map[string]int{}, map[string][]string{}
}

// Apply validates and applies one move. The state is decoded fresh on every
// call, so a refused action leaves the caller's bytes untouched — which is
// what lets the offer list probe it.
func (m *Module) Apply(raw module.State, playerID string, a module.Action) (module.State, []module.Event, error) {
	s, err := decode(raw)
	if err != nil {
		return raw, nil, err
	}
	if s.Status != "active" {
		return raw, nil, errCode(ErrGameNotActive)
	}

	if s.Intermission.Open {
		if a.Verb != module.VerbContinue {
			return raw, nil, errCode(ErrNotNow)
		}
		if err := s.Intermission.Mark(s.Players, playerID); err != nil {
			return raw, nil, err
		}
		var events []module.Event
		if s.Intermission.Settled(s.Players) {
			s.Intermission.Close()
			startDeal(s)
			events = []module.Event{{Type: "deal_started", Data: map[string]any{"deal": s.Deal + 1}}}
		}
		out, err := encode(s)
		return out, events, err
	}

	if s.seat(playerID) < 0 || s.Current != playerID {
		return raw, nil, errCode(ErrNotYourTurn)
	}

	var events []module.Event
	switch a.Verb {
	case VerbChooseTrump:
		err = s.chooseTrump(playerID, a)
	case VerbAnnounce:
		err = s.announce(a)
	case VerbDiscard:
		err = s.discard(playerID, a.Cards)
	case VerbGood:
		err = s.good()
	case VerbTakeOver:
		err = s.takeOver(playerID, a.OfferID)
	case VerbFlek:
		err = s.flek(playerID, a.OfferID)
	case VerbProti:
		err = s.proti(playerID, a.OfferID)
	case VerbPass:
		err = s.pass()
	case VerbPlay:
		events, err = s.play(playerID, a.Cards)
	default:
		return raw, nil, module.Error{Code: ErrUnknownAction, Message: a.Verb}
	}
	if err != nil {
		return raw, nil, err
	}
	out, err := encode(s)
	return out, events, err
}

func (s *GameState) inPhase(p string) error {
	if s.Phase != p {
		return errCode(ErrNotNow)
	}
	return nil
}

// chooseTrump names trumps by a card from the first seven, or z lidu by the
// first of the five unseen, and hands the chooser the rest of their cards.
func (s *GameState) chooseTrump(p string, a module.Action) error {
	if err := s.inPhase(phaseTrump); err != nil {
		return err
	}
	if a.OfferID == OfferZLidu {
		if !s.ZLidu {
			return errCode(ErrNoZLidu)
		}
		s.TrumpCard = s.Unseen[0] // the five are already shuffled
	} else {
		if len(a.Cards) != 1 {
			return errCode(ErrPickOneCard)
		}
		if !hasCard(s.Hands[p], a.Cards[0]) {
			return errCode(ErrCardNotInHand)
		}
		s.TrumpCard = a.Cards[0]
	}
	s.Hands[p] = append(s.Hands[p], s.Unseen...)
	s.Unseen = nil
	sortHand(s.Hands[p])
	s.Phase = phaseAnnounce
	return nil
}

// announce names the declarer's game, with or without a seven.
func (s *GameState) announce(a module.Action) error {
	if err := s.inPhase(phaseAnnounce); err != nil {
		return err
	}
	game, sedma := "", false
	switch a.OfferID {
	case OfferHra:
		game = gameHra
	case OfferHraSedma:
		game, sedma = gameHra, true
	case OfferSto:
		game = gameSto
	case OfferStoSedma:
		game, sedma = gameSto, true
	case OfferBetl:
		game = gameBetl
	case OfferDurch:
		game = gameDurch
	default:
		return module.Error{Code: ErrUnknownAction, Message: a.OfferID}
	}
	if sedma && !hasCard(s.Hands[s.Declarer], "7"+string(tricks.Suit(s.TrumpCard))) {
		// A seven is announced only if it is held (ČSM B/13).
		return errCode(ErrNoSeven)
	}
	s.Game, s.Sedma = game, sedma
	s.Phase = phaseTalon
	return nil
}

// discard lays two cards away to the talon.
func (s *GameState) discard(p string, cards []string) error {
	if err := s.inPhase(phaseTalon); err != nil {
		return err
	}
	if len(cards) != 2 || cards[0] == cards[1] {
		return errCode(ErrDiscardTwo)
	}
	for _, c := range cards {
		if err := s.talonRefusal(p, c); err != nil {
			return err
		}
	}
	s.Hands[p] = removeCards(s.Hands[p], cards...)
	s.Talon = append([]string(nil), cards...)
	if s.Game == gameDurch {
		// Nothing goes over durch (ČSM B/14), so there is nobody to ask.
		s.openDoubling()
		return nil
	}
	s.Phase, s.Current = phaseAnswer, s.next(s.Declarer)
	return nil
}

// talonRefusal is why one card may not go to the talon, or nil.
func (s *GameState) talonRefusal(p, c string) error {
	if !hasCard(s.Hands[p], c) {
		return errCode(ErrCardNotInHand)
	}
	if s.trumpGame() && cardPoints(c) > 0 {
		return errCode(ErrSharpInTalon) // ČSM C/13
	}
	if s.Sedma && c == s.trumpSeven() {
		return errCode(ErrSevenInTalon)
	}
	return nil
}

// good accepts the game as announced.
func (s *GameState) good() error {
	if err := s.inPhase(phaseAnswer); err != nil {
		return err
	}
	s.Current = s.next(s.Current)
	if s.Current == s.Declarer {
		s.openDoubling()
	}
	return nil
}

// takeOver is špatná: a higher game, played by the player who says it, with
// the talon.
func (s *GameState) takeOver(p, offerID string) error {
	if err := s.inPhase(phaseAnswer); err != nil {
		return err
	}
	game := ""
	switch offerID {
	case OfferBadBetl:
		game = gameBetl
	case OfferBadDurch:
		game = gameDurch
	default:
		return module.Error{Code: ErrUnknownAction, Message: offerID}
	}
	if gameRank(game) <= gameRank(s.Game) {
		return errCode(ErrNotHigher)
	}
	s.Declarer, s.Game, s.Sedma = p, game, false
	s.Hands[p] = append(s.Hands[p], s.Talon...)
	s.Talon = nil
	sortHand(s.Hands[p])
	s.Phase = phaseTalon
	return nil
}

// openDoubling starts the doubling round with the player after the
// declarer.
func (s *GameState) openDoubling() {
	s.Phase, s.Current, s.Quiet = phaseFlek, s.next(s.Declarer), 0
}

// parts are the doublable parts of the contract as it stands.
func (s *GameState) parts() []string {
	out := []string{partGame}
	if s.Sedma {
		out = append(out, partSedma)
	}
	if s.ProtiSedma != "" {
		out = append(out, partProtiSedma)
	}
	if s.ProtiSto != "" {
		out = append(out, partProtiSto)
	}
	return out
}

func (s *GameState) hasPart(part string) bool {
	for _, p := range s.parts() {
		if p == part {
			return true
		}
	}
	return false
}

// declarerOwns is whether a part is the declarer's to win.
func declarerOwns(part string) bool { return part == partGame || part == partSedma }

func partOfOffer(offerID string) string {
	switch offerID {
	case OfferFlekGame:
		return partGame
	case OfferFlekSedma:
		return partSedma
	case OfferFlekPSedma:
		return partProtiSedma
	case OfferFlekPSto:
		return partProtiSto
	}
	return ""
}

// flek doubles one part. The side that did not announce it doubles first,
// then the two sides take turns: flek, re, tutti…
func (s *GameState) flek(p, offerID string) error {
	if err := s.inPhase(phaseFlek); err != nil {
		return err
	}
	part := partOfOffer(offerID)
	if part == "" || !s.hasPart(part) {
		return errCode(ErrNoSuchPart)
	}
	if err := s.flekRefusal(p, part); err != nil {
		return err
	}
	s.Fleks[part]++
	s.Quiet = 0
	return nil
}

func (s *GameState) flekRefusal(p, part string) error {
	n := s.Fleks[part]
	ownerSide := declarerOwns(part) == (p == s.Declarer)
	// Even counts are the opponents' to raise, odd ones the owner's.
	if ownerSide != (n%2 == 1) {
		return errCode(ErrNotYoursToFlek)
	}
	if s.FlekLimit != FlekUnlimited && n >= s.FlekLimit {
		return errCode(ErrFlekLimit)
	}
	return nil
}

// proti announces a defenders' seven or hundred.
func (s *GameState) proti(p, offerID string) error {
	if err := s.inPhase(phaseFlek); err != nil {
		return err
	}
	if !s.defender(p) || !s.trumpGame() {
		return errCode(ErrProtiNotNow)
	}
	switch offerID {
	case OfferProtiSedma:
		if s.Sedma || s.ProtiSedma != "" {
			return errCode(ErrProtiNotNow)
		}
		if !hasCard(s.Hands[p], s.trumpSeven()) {
			return errCode(ErrNoSeven)
		}
		s.ProtiSedma = p
	case OfferProtiSto:
		if s.Game != gameHra || s.ProtiSto != "" {
			return errCode(ErrProtiNotNow)
		}
		s.ProtiSto = p
	default:
		return module.Error{Code: ErrUnknownAction, Message: offerID}
	}
	s.Quiet = 0
	return nil
}

// pass ends a player's say in the doubling round. A full circle of passes
// in a row closes it, and the declarer leads.
func (s *GameState) pass() error {
	if err := s.inPhase(phaseFlek); err != nil {
		return err
	}
	s.Quiet++
	if s.Quiet >= len(s.Players) {
		s.Phase, s.Current = phasePlay, s.Declarer
		return nil
	}
	s.Current = s.next(s.Current)
	return nil
}

// legal is the cards p may play now: the trick's own obligations, and a
// seven announced is kept back for the last trick while anything else is
// legal.
func (s *GameState) legal(p string) []string {
	hand := s.Hands[p]
	out := tricks.Legal(hand, tricks.Trick{Plays: s.Trick}, s.trump(), s.order(), tricks.FollowBeatTrump)
	if p == s.sevenHolder() && len(out) > 1 {
		out = removeCards(out, s.trumpSeven())
	}
	return out
}

// playRefusal says which obligation a card breaks.
func (s *GameState) playRefusal(p, card string) error {
	if !hasCard(s.Hands[p], card) {
		return errCode(ErrCardNotInHand)
	}
	if hasCard(s.legal(p), card) {
		return nil
	}
	hand, trick := s.Hands[p], tricks.Trick{Plays: s.Trick}
	if hasCard(tricks.Legal(hand, trick, s.trump(), s.order(), tricks.FollowBeatTrump), card) {
		return errCode(ErrKeepSeven)
	}
	led := trick.Led()
	followers := 0
	for _, c := range hand {
		if tricks.Suit(c) == led {
			followers++
		}
	}
	switch {
	case followers > 0 && tricks.Suit(card) != led:
		return errCode(ErrMustFollow)
	case followers > 0:
		return errCode(ErrMustBeat)
	case tricks.Suit(card) != s.trump():
		return errCode(ErrMustTrump)
	}
	return errCode(ErrMustOvertrump)
}

// play puts one card on the trick, and settles the trick and the deal when
// they are over.
func (s *GameState) play(p string, cards []string) ([]module.Event, error) {
	if err := s.inPhase(phasePlay); err != nil {
		return nil, err
	}
	if len(cards) != 1 {
		return nil, errCode(ErrPickOneCard)
	}
	card := cards[0]
	if err := s.playRefusal(p, card); err != nil {
		return nil, err
	}

	// A svršek played while its král is still in hand is a marriage, and is
	// counted as said (docs/marias-rules.md, deviation 7).
	if s.trumpGame() && tricks.Rank(card) == 'Q' && hasCard(s.Hands[p], "K"+string(tricks.Suit(card))) {
		s.Marriages[p] = append(s.Marriages[p], string(tricks.Suit(card)))
	}
	s.Hands[p] = removeCards(s.Hands[p], card)
	s.Trick = append(s.Trick, tricks.Play{Seat: s.seat(p), Card: card})
	events := []module.Event{{Type: "card_played", Data: map[string]any{"playerId": p, "card": card}}}

	if len(s.Trick) < len(s.Players) {
		s.Current = s.next(p)
		return events, nil
	}

	win, _ := tricks.Trick{Plays: s.Trick}.Winning(s.trump(), s.order())
	winner := s.Players[win.Seat]
	for _, pl := range s.Trick {
		s.Points[winner] += cardPoints(pl.Card)
	}
	s.TricksWon[winner]++
	s.LastTrick, s.Trick = s.Trick, nil
	s.Current = winner
	last := len(s.Hands[winner]) == 0
	if last {
		s.Points[winner] += pointsLastTrick
	}
	events = append(events, module.Event{Type: "trick_won", Data: map[string]any{"playerId": winner}})

	if last || s.decidedEarly() {
		events = append(events, s.settle()...)
	}
	return events, nil
}

// decidedEarly is a betl or durch whose outcome can no longer change.
func (s *GameState) decidedEarly() bool {
	switch s.Game {
	case gameBetl:
		return s.TricksWon[s.Declarer] > 0
	case gameDurch:
		taken := 0
		for _, n := range s.TricksWon {
			taken += n
		}
		return s.TricksWon[s.Declarer] < taken
	}
	return false
}

// Finished reports whether the match is over, and who is ahead on units —
// two seats level on the most are both winners.
func (m *Module) Finished(raw module.State) (bool, []string, error) {
	s, err := decode(raw)
	if err != nil {
		return false, nil, err
	}
	if s.Status != "completed" {
		return false, nil, nil
	}
	best := s.Scores[s.Players[0]]
	for _, p := range s.Players {
		if s.Scores[p] > best {
			best = s.Scores[p]
		}
	}
	var winners []string
	for _, p := range s.Players {
		if s.Scores[p] == best {
			winners = append(winners, p)
		}
	}
	return true, winners, nil
}
