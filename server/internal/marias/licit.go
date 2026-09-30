package marias

import (
	"strconv"

	"zolik/server/internal/module"
)

// Licitovaný: the auction, the ladder it climbs, and announcing a contract
// from it. Everything after the talon — doubling, play, settlement — is the
// same code volený uses. See docs/marias-licitovany-rules.md.

// The ladder, lowest first (ČSM licitovaný I). A contract kind and whether
// its trumps are hearts give its rung; betl and durch have no trumps and one
// rung each.
const (
	kindSedma       = "sedma"
	kindSto         = "sto"
	kindStoSedma    = "stoSedma"
	kindBetl        = "betl"
	kindDurch       = "durch"
	kindDveSedmy    = "dveSedmy"
	kindDveSedmySto = "dveSedmySto"
)

const topRung = 12

func contractRung(kind string, red bool) int {
	pick := func(plain, hearts int) int {
		if red {
			return hearts
		}
		return plain
	}
	switch kind {
	case kindSedma:
		return pick(1, 2)
	case kindSto:
		return pick(3, 5)
	case kindStoSedma:
		return pick(4, 6)
	case kindBetl:
		return 7
	case kindDurch:
		return 8
	case kindDveSedmy:
		return pick(9, 11)
	case kindDveSedmySto:
		return pick(10, 12)
	}
	return 0
}

func kindOfOffer(offerID string) string {
	switch offerID {
	case OfferLSedma:
		return kindSedma
	case OfferLSto:
		return kindSto
	case OfferLStoSedma:
		return kindStoSedma
	case OfferLBetl:
		return kindBetl
	case OfferLDurch:
		return kindDurch
	case OfferLDveSedmy:
		return kindDveSedmy
	case OfferLDveSedmySto:
		return kindDveSedmySto
	}
	return ""
}

func trumpKind(kind string) bool { return kind != kindBetl && kind != kindDurch }

// dealLicit deals ten to each seat and the talon face down to the middle,
// and opens the auction: the zadák (the dealer) bids against the forhont,
// and the middle player waits (ČSM general VII/3).
func dealLicit(s *GameState, deck []string) {
	s.Hands = map[string][]string{}
	forhont := s.chooser()
	seat := forhont
	for i := 0; i < len(s.Players); i++ {
		s.Hands[seat] = append([]string(nil), deck[i*handSize:(i+1)*handSize]...)
		sortHand(s.Hands[seat])
		seat = s.next(seat)
	}
	s.Talon = append([]string(nil), deck[len(s.Players)*handSize:]...)
	s.Unseen = nil

	middle := s.next(forhont)
	s.Holder, s.Bidder, s.Waiting = forhont, s.next(middle), middle
	s.Phase, s.Current = phaseAuction, s.Bidder
	s.Declarer, s.Game, s.Sedma = "", "", false
	s.ProtiSedma, s.ProtiSto = "", ""
	s.Fleks, s.Quiet = map[string]int{}, 0
	s.Trick, s.LastTrick = nil, nil
	s.TricksWon, s.Points, s.Marriages = map[string]int{}, map[string]int{}, map[string][]string{}
}

// bid names a rung above the one standing.
func (s *GameState) bid(p string, a module.Action) error {
	if err := s.inPhase(phaseAuction); err != nil {
		return err
	}
	if p == s.Holder {
		return errCode(ErrYouHold)
	}
	if p != s.Bidder {
		return errCode(ErrNotYourTurn)
	}
	rung, err := strconv.Atoi(a.Params["rung"])
	if err != nil || rung <= s.Rung || rung > topRung {
		return errCode(ErrBidTooLow)
	}
	s.Rung = rung
	s.Current = s.Holder
	return nil
}

// hold is "mám": the holder will play the rung just bid, and at equal rungs
// the holder wins.
func (s *GameState) hold(p string) error {
	if err := s.inPhase(phaseAuction); err != nil {
		return err
	}
	if p == s.Bidder {
		return errCode(ErrYouBid)
	}
	if p != s.Holder || s.Rung == 0 {
		return errCode(ErrNotYourTurn)
	}
	s.Current = s.Bidder
	return nil
}

// auctionPass drops a player from the auction. The middle player, still
// waiting, takes the place of the first to drop: bidding against the holder
// if the bidder dropped, holding against the bidder if the holder did. The
// last player left has won.
func (s *GameState) auctionPass(p string) error {
	switch p {
	case s.Bidder:
		if s.Waiting != "" {
			s.Bidder, s.Waiting = s.Waiting, ""
			s.Current = s.Bidder
			return nil
		}
		s.endAuction(s.Holder)
	case s.Holder:
		if s.Rung == 0 {
			return errCode(ErrNotYourTurn) // nothing has been bid to answer
		}
		if s.Waiting != "" {
			s.Holder, s.Waiting = s.Waiting, ""
			s.Current = s.Holder
			return nil
		}
		s.endAuction(s.Bidder)
	default:
		return errCode(ErrNotYourTurn)
	}
	return nil
}

// endAuction gives the talon to the winner, at the rung won — rung 1, sedma,
// if nobody bid at all.
func (s *GameState) endAuction(winner string) {
	if s.Rung == 0 {
		s.Rung = 1
	}
	s.Holder, s.Bidder, s.Waiting = "", "", ""
	s.Declarer = winner
	s.Hands[winner] = append(s.Hands[winner], s.Talon...)
	s.Talon = nil
	sortHand(s.Hands[winner])
	s.Phase, s.Current = phaseAnnounce, winner
}

// announceLicit names the contract: a kind, its trumps (and for dvě sedmy
// its helper suit), at or above the rung won. Above plain sedma it need not
// match the cards (ČSM licitovaný II/16); plain sedma needs the trump seven
// (II/17).
func (s *GameState) announceLicit(a module.Action) error {
	if err := s.inPhase(phaseAnnounce); err != nil {
		return err
	}
	kind := kindOfOffer(a.OfferID)
	if kind == "" {
		return module.Error{Code: ErrUnknownAction, Message: a.OfferID}
	}
	trump, helper := "", ""
	if trumpKind(kind) {
		trump = a.Params["trump"]
		if !isSuit(trump) {
			return errCode(ErrUnknownSuit)
		}
	}
	if kind == kindDveSedmy || kind == kindDveSedmySto {
		helper = a.Params["helper"]
		if !isSuit(helper) {
			return errCode(ErrUnknownSuit)
		}
		if helper == trump {
			return errCode(ErrHelperIsTrump)
		}
	}
	if contractRung(kind, trump == string(suitRed)) < s.Rung {
		return errCode(ErrBelowTheBid)
	}
	if kind == kindSedma && !hasCard(s.Hands[s.Declarer], "7"+trump) {
		return errCode(ErrNoSeven)
	}

	s.Trump, s.Helper = trump, helper
	s.Sedma = kind == kindSedma || kind == kindStoSedma
	s.WithSto = kind == kindDveSedmySto
	switch kind {
	case kindSedma:
		s.Game = gameHra
	case kindSto, kindStoSedma:
		s.Game = gameSto
	case kindBetl:
		s.Game = gameBetl
	case kindDurch:
		s.Game = gameDurch
	default:
		s.Game = gameDveSedmy
	}
	s.Phase = phaseTalon
	return nil
}

// omyl folds a contract held to plain sedma before anybody plays: the
// declarer pays and the next deal is dealt (ČSM licitovaný II/17).
func (s *GameState) omyl() ([]module.Event, error) {
	if err := s.inPhase(phaseAnnounce); err != nil {
		return nil, err
	}
	if !s.licit() || s.Rung != 1 {
		return nil, errCode(ErrNoOmyl)
	}
	s.Game = gameOmyl
	return s.settle(), nil
}

// firstLeader is who leads to the first trick. In licitovaný that is the
// forhont, except in betl and durch, where the declarer does (ČSM general
// II/6). In volený it is always the declarer, who is the forhont in every
// trump game anyway.
func (s *GameState) firstLeader() string {
	if !s.licit() || s.Game == gameBetl || s.Game == gameDurch {
		return s.Declarer
	}
	return s.chooser()
}

func isSuit(v string) bool {
	return len(v) == 1 && (v[0] == 'H' || v[0] == 'D' || v[0] == 'C' || v[0] == 'S')
}

// rungKey names a rung, for a bid button and the header.
func rungKey(rung int) string {
	switch rung {
	case 1:
		return "marias.rung.1"
	case 2:
		return "marias.rung.2"
	case 3:
		return "marias.rung.3"
	case 4:
		return "marias.rung.4"
	case 5:
		return "marias.rung.5"
	case 6:
		return "marias.rung.6"
	case 7:
		return "marias.rung.7"
	case 8:
		return "marias.rung.8"
	case 9:
		return "marias.rung.9"
	case 10:
		return "marias.rung.10"
	case 11:
		return "marias.rung.11"
	}
	return "marias.rung.12"
}

// suitsHolding lists the suits in which hand holds the seven.
func suitsHolding(hand []string) []string {
	var out []string
	for _, suit := range []string{"H", "D", "C", "S"} {
		if hasCard(hand, "7"+suit) {
			out = append(out, suit)
		}
	}
	return out
}
