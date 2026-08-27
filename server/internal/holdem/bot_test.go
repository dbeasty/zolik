package holdem

import (
	"math/rand"
	"testing"

	"zolik/server/internal/module"
)

// botAct asks the module's own bot what it would do, through exactly the path
// the runtime uses: the offer list from LegalActions, and nothing else handed
// in.
func botAct(t *testing.T, raw module.State, playerID string) module.Action {
	t.Helper()
	m := New()
	offers, err := m.LegalActions(raw, playerID)
	if err != nil {
		t.Fatalf("LegalActions: %v", err)
	}
	a, ok := m.Bot().Act(raw, playerID, offers)
	if !ok {
		t.Fatalf("bot had no move for %s", playerID)
	}
	return a
}

// --- reading two cards -------------------------------------------------------

// TestChenScoresTheKnownHands pins the preflop score against the published
// numbers rather than against itself, so the formula is a citation and a typo
// in it cannot pass as a taste change.
func TestChenScoresTheKnownHands(t *testing.T) {
	cases := []struct {
		hole []string
		want float64
	}{
		{[]string{"AS", "AD"}, 20},   // the best hand there is
		{[]string{"KS", "KD"}, 16},   //
		{[]string{"QS", "QD"}, 14},   //
		{[]string{"JS", "JD"}, 12},   //
		{[]string{"TS", "TD"}, 10},   //
		{[]string{"2S", "2D"}, 5},    // a pair is worth five however small
		{[]string{"AS", "KS"}, 12},   // suited: two points
		{[]string{"AS", "KD"}, 10},   // offsuit: none
		{[]string{"AS", "QS"}, 11},   // one gap: a point back off
		{[]string{"7S", "2D"}, -1.5}, // the worst hand there is
	}
	for _, tc := range cases {
		if got := chen(tc.hole); got != tc.want {
			t.Errorf("chen(%v) = %v, want %v", tc.hole, got, tc.want)
		}
	}
}

// --- before the flop ---------------------------------------------------------

// preflopTable seats three, puts a raise to `raisedTo` in front of seat 0 and
// gives it `hole`. Seat 0 is on turn.
func preflopTable(hole []string, raisedTo int) module.State {
	return table(3, func(s *GameState) {
		s.Current = 0
		s.Seats[0].Hole = hole
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 990
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 980
		s.CurrentBet, s.MinRaise = 20, 20
		if raisedTo > 20 {
			s.Seats[2].Bet, s.Seats[2].Committed = raisedTo, raisedTo
			s.Seats[2].Stack = 1000 - raisedTo
			s.CurrentBet, s.MinRaise = raisedTo, raisedTo-20
		}
	})
}

// TestBotFoldsTrashToARaise is the headline: the old bot called here, every
// time, with anything.
func TestBotFoldsTrashToARaise(t *testing.T) {
	trash := [][]string{
		{"7C", "2D"},
		{"9C", "3D"},
		{"JC", "4H"},
		{"8S", "2H"},
	}
	for _, hole := range trash {
		got := botAct(t, preflopTable(hole, 60), "p1")
		if got.Verb != VerbFold {
			t.Errorf("with %v facing a raise to 60: %s, want fold", hole, got.Verb)
		}
	}
}

// TestBotRaisesPremiumHands is the other half of it: the old bot never raised
// while a call was on the menu, so aces got played exactly like seven-deuce.
func TestBotRaisesPremiumHands(t *testing.T) {
	premium := [][]string{
		{"AC", "AD"},
		{"KC", "KD"},
		{"AC", "KC"},
		{"QC", "QD"},
	}
	for _, hole := range premium {
		got := botAct(t, preflopTable(hole, 0), "p1")
		if got.Verb != VerbRaise {
			t.Errorf("with %v in an unraised pot: %s, want raise", hole, got.Verb)
		}
	}
}

// TestBotAnswersARaiseByItsSize.
//
// Three ways to play the same king-queen, decided by the price alone: aces
// re-raise, king-queen calls three big blinds and folds fifteen. The middle
// line is the one no threshold-free bot can find — it is neither the best hand
// nor a fold, and the old one had no way to express it.
func TestBotAnswersARaiseByItsSize(t *testing.T) {
	if got := botAct(t, preflopTable([]string{"AC", "AD"}, 60), "p1"); got.Verb != VerbRaise {
		t.Errorf("aces facing a raise: %s, want raise", got.Verb)
	}
	if got := botAct(t, preflopTable([]string{"KC", "QD"}, 60), "p1"); got.Verb != VerbCall {
		t.Errorf("king-queen facing a raise to 60: %s, want call", got.Verb)
	}
	if got := botAct(t, preflopTable([]string{"KC", "QD"}, 300), "p1"); got.Verb != VerbFold {
		t.Errorf("king-queen facing a raise to 300: %s, want fold", got.Verb)
	}
}

// TestBotTakesAFreeCardRatherThanFolding.
//
// Nothing is at stake and folding costs the pot for no reason. The offer bot
// got this right by accident (fold was its last preference); this one has to
// get it right on purpose, because it is capable of deciding a hand is
// worthless.
func TestBotTakesAFreeCardRatherThanFolding(t *testing.T) {
	raw := table(3, func(s *GameState) {
		s.Current = 0
		s.Seats[0].Hole = []string{"7C", "2D"}
		s.Seats[0].Bet, s.Seats[0].Committed, s.Seats[0].Stack = 20, 20, 980
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 20, 20, 980
		s.Seats[2].Folded = true
		s.CurrentBet, s.MinRaise = 20, 20
	})
	if got := botAct(t, raw, "p1"); got.Verb != VerbCheck {
		t.Errorf("with nothing to call: %s, want check", got.Verb)
	}
}

// TestBotShovesAShortStack: ten big blinds is past the point of playing a flop,
// and a bot that keeps folding into the blinds finishes second.
func TestBotShovesAShortStack(t *testing.T) {
	raw := table(3, func(s *GameState) {
		s.Current = 0
		s.Seats[0].Hole = []string{"AC", "QD"}
		s.Seats[0].Stack = 180
		s.Seats[1].Bet, s.Seats[1].Committed, s.Seats[1].Stack = 10, 10, 990
		s.Seats[2].Bet, s.Seats[2].Committed, s.Seats[2].Stack = 20, 20, 980
		s.CurrentBet, s.MinRaise = 20, 20
	})
	got := botAct(t, raw, "p1")
	if got.Verb != VerbRaise {
		t.Fatalf("short stack with ace-queen: %s, want raise", got.Verb)
	}
	if got.Params[ParamAmount] != "180" {
		t.Errorf("raised to %s, want all in for 180", got.Params[ParamAmount])
	}
}

// --- after the flop ----------------------------------------------------------

// facing seats two on the given board with a bet of `bet` in front of seat 0
// to answer, and two hundred already in the pot. A bet of zero is a table
// checked to seat 0.
func facing(hole, board []string, bet int) module.State {
	return table(2, func(s *GameState) {
		switch len(board) {
		case 3:
			s.Street = streetFlop
		case 4:
			s.Street = streetTurn
		default:
			s.Street = streetRiver
		}
		s.Board = board
		s.Current = 0
		s.Pot = 200
		s.Seats[0].Hole = hole
		s.Seats[0].Committed, s.Seats[0].Stack = 100, 900
		s.Seats[1].Hole = []string{"5C", "5D"}
		s.Seats[1].Committed, s.Seats[1].Stack = 100+bet, 900-bet
		s.Seats[1].Bet, s.Seats[1].Acted = bet, true
		s.CurrentBet, s.MinRaise = bet, s.BigBlind
	})
}

// TestBotFoldsAHopelessRiver. Two cards that play no part in the hand, against
// a bet that needs a third of the pot to call: the fold is arithmetic, not
// nerve.
func TestBotFoldsAHopelessRiver(t *testing.T) {
	raw := facing([]string{"2C", "3D"}, []string{"AH", "KD", "QC", "JS", "9H"}, 100)
	if got := botAct(t, raw, "p1"); got.Verb != VerbFold {
		t.Errorf("no hand, bet of half the pot: %s, want fold", got.Verb)
	}
}

// TestBotRaisesTheNuts. The old bot called with quads, which is how a calling
// station loses money holding the best hand possible.
func TestBotRaisesTheNuts(t *testing.T) {
	raw := facing([]string{"AS", "AD"}, []string{"AH", "AC", "KD", "8S", "3C"}, 60)
	got := botAct(t, raw, "p1")
	if got.Verb != VerbRaise {
		t.Fatalf("four aces facing a bet: %s, want raise", got.Verb)
	}
	if got.Params[ParamAmount] == "" {
		t.Error("raise carried no amount")
	}
}

// TestBotPricesTheSameDrawTwoWays is the pot-odds rule itself, pinned.
//
// A flush draw on the turn wins about three times in ten. That is a call at a
// tenth of the pot and a fold at three times it — same cards, same board, same
// opponent, and the only thing that changed is the price. Nothing that reads
// the offer list can tell these two spots apart; they are the same four offers.
func TestBotPricesTheSameDrawTwoWays(t *testing.T) {
	hole := []string{"QH", "JH"}
	board := []string{"AH", "7H", "2C", "KD"}

	if got := botAct(t, facing(hole, board, 20), "p1"); got.Verb == VerbFold {
		t.Errorf("a flush draw offered ten to one: fold, want a call")
	}
	if got := botAct(t, facing(hole, board, 600), "p1"); got.Verb != VerbFold {
		t.Errorf("a flush draw against a bet of three times the pot: %s, want fold", got.Verb)
	}
}

// TestBotBetsWhenCheckedTo: a hand strong enough to be called by worse is worth
// money only if somebody puts it in.
func TestBotBetsWhenCheckedTo(t *testing.T) {
	raw := facing([]string{"AS", "AD"}, []string{"AH", "AC", "KD", "8S", "3C"}, 0)
	if got := botAct(t, raw, "p1"); got.Verb != VerbRaise {
		t.Errorf("four aces checked to: %s, want a bet", got.Verb)
	}
}

// --- the disciplines ---------------------------------------------------------

// TestBotDoesNotPeek.
//
// The bot is handed the whole state — every seat's hole cards — because that is
// what the runtime has. Nothing but this test stops it reading them, and a bot
// that quietly did would be undetectable from the outside and would ruin the
// game.
func TestBotDoesNotPeek(t *testing.T) {
	hole := []string{"AC", "AD"}
	board := []string{"7H", "8D", "2C", "KS", "3H"}

	honest := facing(hole, board, 100)
	rigged := table(2, func(s *GameState) {
		s.Street = streetRiver
		s.Board = board
		s.Current = 0
		s.Pot = 200
		s.Seats[0].Hole = hole
		s.Seats[0].Committed, s.Seats[0].Stack = 100, 900
		// The one difference: the opponent now holds a straight rather than a
		// small pair. A bot reading it would fold; ours cannot see it.
		s.Seats[1].Hole = []string{"9C", "TD"}
		s.Seats[1].Committed, s.Seats[1].Stack = 200, 800
		s.Seats[1].Bet, s.Seats[1].Acted = 100, true
		s.CurrentBet, s.MinRaise = 100, s.BigBlind
	})

	a, b := botAct(t, honest, "p1"), botAct(t, rigged, "p1")
	if a.Verb != b.Verb || a.Params[ParamAmount] != b.Params[ParamAmount] {
		t.Errorf("bot played %v against a pair and %v against a straight: it is reading hole cards", a, b)
	}
}

// TestBotIsDeterministic. The bot bluffs and varies its sizing, and both come
// from a seed derived from the position rather than from the clock — so a
// replayed match replays, and a failing test can be run twice.
func TestBotIsDeterministic(t *testing.T) {
	raw := facing([]string{"QC", "JD"}, []string{"AH", "KD", "8S", "4C", "2H"}, 0)
	first := botAct(t, raw, "p1")
	for i := 0; i < 5; i++ {
		again := botAct(t, raw, "p1")
		if again.Verb != first.Verb || again.Params[ParamAmount] != first.Params[ParamAmount] {
			t.Fatalf("run %d played %v, first run played %v", i+2, again, first)
		}
	}
}

// --- whole matches -----------------------------------------------------------

// playBots runs a match with a bot at every seat and returns the final state.
//
// Every action goes through Apply, so an illegal one fails here rather than
// being quietly swapped for a legal one the way the runtime's recovery path
// would — which is the only way to find out whether the bot's own moves are
// legal or whether it has been living off that safety net.
func playBots(t *testing.T, state module.State, players []module.PlayerRef, seats map[string]module.Bot, maxActions int) *GameState {
	t.Helper()
	m := New()
	for step := 0; step < maxActions; step++ {
		s := mustDecode(t, state)
		if s.Status != "active" {
			return s
		}
		actor := module.ActiveSeat(m, state, players[0].ID, players)
		if actor == "" {
			return s
		}
		offers, err := m.LegalActions(state, actor)
		if err != nil {
			t.Fatalf("LegalActions(%s): %v", actor, err)
		}
		a, ok := seats[actor].Act(state, actor, offers)
		if !ok {
			t.Fatalf("%s had no move", actor)
		}
		next, _, err := m.Apply(state, actor, a)
		if err != nil {
			t.Fatalf("%s proposed an illegal %v: %v", actor, a, err)
		}
		state = next
	}
	t.Fatalf("match did not finish in %d actions", maxActions)
	return nil
}

func botSeats(players []module.PlayerRef, b module.Bot) map[string]module.Bot {
	seats := map[string]module.Bot{}
	for _, p := range players {
		seats[p.ID] = b
	}
	return seats
}

// TestBotPlaysOnlyLegalMovesToTheEnd.
//
// The bot chooses a raise *amount*, which is the one thing in this module a bot
// can get numerically wrong — a chip under the minimum raise or a chip over the
// stack is refused, and the runtime would paper over it. So: whole matches, at
// every table size the module allows, with every action asserted legal on the
// spot, and the chips counted at the end.
func TestBotPlaysOnlyLegalMovesToTheEnd(t *testing.T) {
	for _, seats := range []int{2, 3, 6, 9} {
		ids := make([]string, 0, seats)
		for i := 1; i <= seats; i++ {
			ids = append(ids, "p"+string(rune('0'+i)))
		}
		players := refs(ids...)
		m := New()
		for seed := int64(1); seed <= 3; seed++ {
			state, err := m.NewMatch(module.MatchConfig{
				Variation: "timed",
				Options:   module.Options{OptStartingStack: 400, OptBigBlind: 20, OptHandLimit: 8},
			}, players, seed)
			if err != nil {
				t.Fatalf("NewMatch: %v", err)
			}
			final := playBots(t, state, players, botSeats(players, m.Bot()), 4000)

			total := 0
			for i := range final.Seats {
				total += final.Seats[i].Stack
			}
			if want := 400 * seats; total != want {
				t.Errorf("%d seats seed %d: %d chips in play, want %d", seats, seed, total, want)
			}
		}
	}
}

// TestBotBeatsTheCallingStation is the whole point, measured.
//
// The opponent is the bot this replaced, unchanged: module.OfferBot preferring
// call, then check, then raise. Folding correctly and raising with the best of
// it is worth chips against it, and if it is not then none of the reasoning
// above is doing anything and this file should not exist.
func TestBotBeatsTheCallingStation(t *testing.T) {
	m := New()
	station := module.OfferBot(VerbCall, VerbCheck, VerbRaise, VerbFold)
	players := refs("thinker", "station")

	won, chips := 0, 0
	const matches = 8
	for seed := int64(1); seed <= matches; seed++ {
		state, err := m.NewMatch(module.MatchConfig{
			Variation: "timed",
			Options:   module.Options{OptStartingStack: 1000, OptBigBlind: 20, OptHandLimit: 15},
		}, players, seed)
		if err != nil {
			t.Fatalf("NewMatch: %v", err)
		}
		final := playBots(t, state, players, map[string]module.Bot{
			"thinker": m.Bot(),
			"station": station,
		}, 6000)

		var mine int
		for i := range final.Seats {
			if final.Seats[i].PlayerID == "thinker" {
				mine = final.Seats[i].Stack
			}
		}
		chips += mine - 1000
		if mine > 1000 {
			won++
		}
	}

	// Poker is a game of chance over fifteen hands, so this is deliberately a
	// loose bar: it fails on a bot that is not better, not on one having a bad
	// run. The chip count is the number that moves.
	if won*2 <= matches {
		t.Errorf("won %d of %d matches against a calling station (net %+d chips)", won, matches, chips)
	}
	if chips <= 0 {
		t.Errorf("net %+d chips against a calling station over %d matches, want a profit", chips, matches)
	}
	t.Logf("won %d of %d, net %+d chips", won, matches, chips)
}

// BenchmarkBotThinking is here because the rollout count is a judgement call
// with a deadline behind it: the runtime pauses a bot for four hundred to
// thirteen hundred milliseconds before it acts, so a decision has to fit inside
// that with room to spare or the pause stops being cosmetic.
//
//	go test ./internal/holdem/ -run XXX -bench Thinking
func BenchmarkBotThinking(b *testing.B) {
	for _, tc := range []struct {
		name      string
		opponents int
	}{
		{"heads up", 1},
		{"six handed", 5},
	} {
		b.Run(tc.name, func(b *testing.B) {
			r := rand.New(rand.NewSource(1))
			for i := 0; i < b.N; i++ {
				equity([]string{"AS", "KD"}, []string{"2C", "7H", "TS"},
					tc.opponents, rollouts(tc.opponents), pair, r)
			}
		})
	}
}
