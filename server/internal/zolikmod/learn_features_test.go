package zolikmod

import (
	"reflect"
	"slices"
	"testing"

	"zolik/server/internal/learn"
	"zolik/server/internal/module"
	"zolik/server/internal/rules"
)

// learnPosition deals a match and hands its state to set, which arranges the
// seat on turn's position; the state is then encoded back.
func learnPosition(t *testing.T, variation string, seats int, set func(s *matchState, me, next string)) (*Module, module.State, string) {
	t.Helper()
	m := New()
	raw, err := m.NewMatch(learnGame{}.Config(seats, variation), learn.Players(seats), 7)
	if err != nil {
		t.Fatal(err)
	}
	s, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	me := s.Rules.CurrentTurn
	order := seatsFrom(s.Rules.TurnOrder, me)
	s.Ledger.Deal = s.Rules.GameNumber
	set(s, me, order[1])
	raw, err = encode(s)
	if err != nil {
		t.Fatal(err)
	}
	return m, raw, me
}

func candidatesAt(t *testing.T, m *Module, raw module.State, seat string) []learn.Candidate {
	t.Helper()
	cands, err := learnGame{}.Candidates(raw, seat, learnOffers(t, m, raw, seat))
	if err != nil {
		t.Fatal(err)
	}
	return cands
}

// takeOf is the pickup candidate naming this card, if there is one.
func takeOf(cands []learn.Candidate, card string) (learn.Candidate, bool) {
	for _, c := range cands {
		if c.Action.OfferID == rules.OfferDrawDiscard && slices.Equal(c.Action.Cards, []string{card}) {
			return c, true
		}
	}
	return learn.Candidate{}, false
}

func discardOf(cands []learn.Candidate, card string) (learn.Candidate, bool) {
	for _, c := range cands {
		if c.Action.Verb == string(rules.VerbDiscard) && slices.Equal(c.Action.Cards, []string{card}) {
			return c, true
		}
	}
	return learn.Candidate{}, false
}

// Evidence from a real game: 2 seats, classic, a clean run required, and the
// network holding 6♠ 7♠ never going down. With an 8♠ (on top) or a 5♠
// (deeper, the cards above it coming too) on the pile, the pickup is offered,
// and it says the clean run is a card nearer: one card missing before, none
// after. The state says the same before the pickup: one card from a clean run,
// and the run still live.
func TestLearnTakeTowardCleanRun(t *testing.T) {
	hand := []string{"6S", "7S", "KH", "KD", "2H", "9C", "JD", "4D", "QS", "3C", "TH", "JOKER1", "JOKER2"}
	for _, pile := range [][]string{{"TC", "4C", "8S"}, {"TC", "5S", "4C"}} {
		m, raw, me := learnPosition(t, varClassic, 2, func(s *matchState, me, _ string) {
			s.Rules.Hands[me] = append([]string(nil), hand...)
			s.Rules.DiscardPile = append([]string(nil), pile...)
		})
		obs, err := learnGame{}.Encode(raw, me)
		if err != nil {
			t.Fatal(err)
		}
		if got := obs[offDist]; got != 0.25 {
			t.Errorf("pile %v: clean-run distance %v, want one card (0.25)", pile, got)
		}
		if obs[offDist+1] != 1 {
			t.Errorf("pile %v: the clean run is not live", pile)
		}
		card := pile[1]
		if pile[2] == "8S" {
			card = "8S"
		}
		c, ok := takeOf(candidatesAt(t, m, raw, me), card)
		if !ok {
			t.Fatalf("pile %v: no candidate takes %s", pile, card)
		}
		if c.Features[fDClean] != 0.25 {
			t.Errorf("take %s: clean-run change %v, want one card nearer (0.25)", card, c.Features[fDClean])
		}
		if c.Features[fGoesDown] != 1 || c.Features[fDistAfter] != 0 {
			t.Errorf("take %s: goes down %v, distance after %v", card, c.Features[fGoesDown], c.Features[fDistAfter])
		}
	}
}

// And the other half of that game: it threw 7♠ while holding 6♠. The discard
// candidate for a card of the nearest clean run says it moves the run away.
func TestLearnDiscardAwayFromCleanRun(t *testing.T) {
	m, raw, me := learnPosition(t, varClassic, 2, func(s *matchState, me, _ string) {
		s.Rules.Hands[me] = []string{"5S", "6S", "KH", "KD", "2H", "9C", "JD", "4D", "QS", "3C", "TH", "JOKER1", "7S", "8C"}
		s.Rules.Phase = rules.PhaseMeld
		s.Rules.DiscardPile = []string{"TC"}
	})
	cands := candidatesAt(t, m, raw, me)
	seven, ok := discardOf(cands, "7S")
	if !ok {
		t.Fatal("no discard of 7S")
	}
	if seven.Features[fDClean] >= 0 {
		t.Errorf("discarding 7S from 5S 6S 7S: clean-run change %v, want negative", seven.Features[fDClean])
	}
	loose, _ := discardOf(cands, "9C")
	if loose.Features[fDClean] != 0 {
		t.Errorf("discarding 9C moves the clean run by %v", loose.Features[fDClean])
	}
}

// Once down, a pickup that makes no meld and lays off nowhere is still a
// candidate (the adapter no longer judges it), marked as building when it
// joins a card of its suit in hand — while the deal's pickups are few.
func TestLearnLoosePickupOnceDown(t *testing.T) {
	setup := func(pickups int) (*Module, module.State, string) {
		return learnPosition(t, varClassic, 2, func(s *matchState, me, _ string) {
			s.Rules.Hands[me] = []string{"6H", "KD", "2C", "9C"}
			s.Rules.DiscardPile = []string{"TC", "4C", "8H"}
			s.Rules.RoundReqMet[me] = true
			s.Ledger.Pickups = map[string]int{me: pickups}
		})
	}
	m, raw, me := setup(0)
	c, ok := takeOf(candidatesAt(t, m, raw, me), "8H")
	if !ok {
		t.Fatal("a down seat is not offered 8H")
	}
	if c.Features[fMakesMeld] != 0 || c.Features[fBuilds] != 1 {
		t.Errorf("8H to 6H: makes %v builds %v, want a building card", c.Features[fMakesMeld], c.Features[fBuilds])
	}
	if _, ok := takeOf(candidatesAt(t, m, raw, me), "4C"); !ok {
		t.Error("the deeper 4C is not offered")
	}
	m, raw, me = setup(maxLoosePickups)
	if _, ok := takeOf(candidatesAt(t, m, raw, me), "8H"); ok {
		t.Errorf("a loose pickup offered after %d this deal", maxLoosePickups)
	}
}

// Evidence from real games: J♦ thrown to an opponent who had visibly taken
// 9♦ and 10♦, and 8♦ thrown into an opponent's 9♦ 10♦ run. Discards say they
// feed the next seat; a card of nothing it collects does not.
func TestLearnDiscardFlagsFeedingTheOpponent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		feeds []string
		set   func(s *matchState, next string)
	}{
		{"taken off the pile", []string{"8D", "JD"}, func(s *matchState, next string) {
			s.Ledger.Held = map[string][]string{next: {"9D", "TD"}}
		}},
		{"on the table", []string{"8D", "QD"}, func(s *matchState, next string) {
			s.Rules.Melds[next] = [][]string{{"9D", "TD", "JD"}}
			s.Rules.MeldMeta[next] = []rules.MeldInfo{{MeldID: "m-feed", Type: rules.MeldRun, OwnerID: next}}
			s.Rules.RoundReqMet[next] = true
		}},
	} {
		m, raw, me := learnPosition(t, varClassic, 2, func(s *matchState, me, next string) {
			s.Rules.Hands[me] = []string{"JD", "8D", "QD", "3C", "KS", "KH", "5H", "9S"}
			s.Rules.Phase = rules.PhaseMeld
			s.Rules.DiscardPile = []string{"TC"}
			if s.Rules.Melds == nil {
				s.Rules.Melds = map[string][][]string{}
			}
			if s.Rules.MeldMeta == nil {
				s.Rules.MeldMeta = map[string][]rules.MeldInfo{}
			}
			tc.set(s, next)
		})
		cands := candidatesAt(t, m, raw, me)
		for _, card := range tc.feeds {
			c, ok := discardOf(cands, card)
			if !ok {
				t.Fatalf("%s: no discard of %s", tc.name, card)
			}
			if c.Features[fFeedsNext] != 1 || c.Features[fFeedsAny] != 1 {
				t.Errorf("%s: discarding %s feeds next %v any %v, want 1", tc.name, card, c.Features[fFeedsNext], c.Features[fFeedsAny])
			}
		}
		safe, _ := discardOf(cands, "3C")
		if safe.Features[fFeedsNext] != 0 || safe.Features[fNextSuit] != 0 {
			t.Errorf("%s: 3C feeds next %v, suit %v", tc.name, safe.Features[fFeedsNext], safe.Features[fNextSuit])
		}
		obs, _ := learnGame{}.Encode(raw, me)
		if obs[offRivals+2*(numSuits+numRanks)+cardSlot("8D")] != 1 {
			t.Errorf("%s: the next seat's wants do not show 8D", tc.name)
		}
	}
	// And the pickup evidence itself, taken: the suit and nearness signals.
	m, raw, me := learnPosition(t, varClassic, 2, func(s *matchState, me, next string) {
		s.Rules.Hands[me] = []string{"JD", "3C", "KS", "KH", "5H", "9S"}
		s.Rules.Phase = rules.PhaseMeld
		s.Rules.DiscardPile = []string{"TC"}
		s.Ledger.Held = map[string][]string{next: {"9D", "TD"}}
	})
	jd, _ := discardOf(candidatesAt(t, m, raw, me), "JD")
	if jd.Features[fNextSuit] != 1 || jd.Features[fFeedsNext] != 1 {
		t.Errorf("JD after 9D 10D taken: suit near %v, feeds %v", jd.Features[fNextSuit], jd.Features[fFeedsNext])
	}
}

// The candidates, like the encoding, are a function of what the seat can see:
// scrambling every hidden card leaves every candidate's action and features
// as they were.
func TestLearnCandidatesDoNotPeek(t *testing.T) {
	g := learnGame{}
	checked := 0
	for i, tb := range learnTables {
		w := newLearnWalker(t, tb.variation, tb.seats, 700+int64(i))
		for n := 0; n < 200; n++ {
			seat := w.actor()
			if seat == "" {
				w.deal()
				continue
			}
			cands, _ := g.Candidates(w.state, seat, learnOffers(t, w.m, w.state, seat))
			if n%4 == 0 {
				other := scrambleHidden(t, w.state, seat, w.rnd)
				again, _ := g.Candidates(other, seat, learnOffers(t, w.m, other, seat))
				if !reflect.DeepEqual(cands, again) {
					t.Fatalf("%s/%d seat %s: candidates changed with only hidden cards moved", tb.variation, tb.seats, seat)
				}
				checked++
			}
			if !w.move(seat, cands) {
				w.deal()
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d positions checked", checked)
	}
}

// The closer digs: with the card that takes it down under the top of the
// pile, it takes that card where Hard draws from the stock.
func TestCloserDigsThePile(t *testing.T) {
	m, raw, me := learnPosition(t, varClassic, 2, func(s *matchState, me, _ string) {
		s.Rules.Hands[me] = []string{"6S", "7S", "KH", "KD", "2H", "9C", "JD", "4D", "QS", "3C", "TH", "JOKER1", "JOKER2"}
		s.Rules.DiscardPile = []string{"TC", "8S", "4C"}
	})
	offers := learnOffers(t, m, raw, me)
	seat := module.BotSeat{PlayerID: me, Skill: module.SkillHard, Seed: 1}
	hard, _ := heuristicBot{}.Act(raw, seat, offers)
	closer, _ := learnGame{}.Styles()["closer"].Act(raw, seat, offers)
	if hard.OfferID != rules.OfferDrawDeck {
		t.Errorf("hard: %+v, want the stock", hard)
	}
	if closer.OfferID != rules.OfferDrawDiscard || !slices.Equal(closer.Cards, []string{"8S"}) {
		t.Errorf("closer: %+v, want 8S off the pile", closer)
	}
}
