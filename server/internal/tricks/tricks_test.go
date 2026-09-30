package tricks

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

const (
	trumpOrder Order = "ATKQJ987" // Mariáš with trumps
	betlOrder  Order = "AKQJT987" // Mariáš's Betl and Durch
	bridge     Order = "AKQJT98765432"
)

// trick builds a trick from space-separated cards, seats numbered from 0.
func trick(cards string) Trick {
	var t Trick
	for i, c := range strings.Fields(cards) {
		t.Plays = append(t.Plays, Play{Seat: i, Card: c})
	}
	return t
}

func TestLegal(t *testing.T) {
	cases := []struct {
		name   string
		hand   string
		played string
		trump  byte
		order  Order
		policy Policy
		want   string
	}{
		{"lead: anything", "7H AS KC", "", 'S', trumpOrder, FollowBeatTrump, "7H AS KC"},

		// Following suit.
		{"must beat the lead when able", "AH 7H 8S", "KH", 'S', trumpOrder, FollowBeatTrump, "AH"},
		{"ten beats king in the trump order", "TH 7H", "KH", 'S', trumpOrder, FollowBeatTrump, "TH"},
		{"any higher card will do", "AH TH 7H", "KH", 'S', trumpOrder, FollowBeatTrump, "AH TH"},
		{"cannot beat: any card of the suit", "7H 9H 8S", "AH", 'S', trumpOrder, FollowBeatTrump, "7H 9H"},
		{"beat the best card, not the lead", "KH AH 7H", "9H TH", 'S', trumpOrder, FollowBeatTrump, "AH"},
		{"an off-suit discard sets no bar", "8H 9H", "7H AC", NoTrump, betlOrder, FollowBeatTrump, "8H 9H"},
		{"led suit trumped: no need to beat", "AH 7H 8S", "9H 7S", 'S', trumpOrder, FollowBeatTrump, "AH 7H"},

		// Void in the led suit.
		{"void: must trump", "7S 8C", "AH", 'S', trumpOrder, FollowBeatTrump, "7S"},
		{"void: must overtrump when able", "7S AS KC", "9H 8S", 'S', trumpOrder, FollowBeatTrump, "AS"},
		{"void: cannot overtrump, still trumps", "7S KC", "9H AS", 'S', trumpOrder, FollowBeatTrump, "7S"},
		{"void, no trumps: anything", "7C KD", "AH", 'S', trumpOrder, FollowBeatTrump, "7C KD"},

		// Trumps led.
		{"trump led: beat it", "AS 7S KH", "9S", 'S', trumpOrder, FollowBeatTrump, "AS"},
		{"trump led, none held: anything", "KH 7C", "9S", 'S', trumpOrder, FollowBeatTrump, "KH 7C"},

		// No trumps (Betl, Durch).
		{"no trump: beat in the betl order", "TH QH", "JH", NoTrump, betlOrder, FollowBeatTrump, "QH"},
		{"no trump, void: anything", "7S 8C", "AH", NoTrump, betlOrder, FollowBeatTrump, "7S 8C"},

		// FollowOnly (Whist, Bridge).
		{"follow only: any card of the suit", "2H AH 3C", "KH", 'S', bridge, FollowOnly, "2H AH"},
		{"follow only, void: no obligation to trump", "2S 3C", "KH", 'S', bridge, FollowOnly, "2S 3C"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Legal(strings.Fields(c.hand), trick(c.played), c.trump, c.order, c.policy)
			if want := strings.Fields(c.want); !reflect.DeepEqual(got, want) {
				t.Errorf("Legal(%s | %s) = %v, want %v", c.hand, c.played, got, want)
			}
		})
	}
}

func TestWinning(t *testing.T) {
	cases := []struct {
		name   string
		played string
		trump  byte
		order  Order
		seat   int
	}{
		{"highest of the led suit", "KH TH 7H", 'S', trumpOrder, 1},
		{"discards never win", "7H AC AD", 'S', trumpOrder, 0},
		{"a trump takes it", "AH 7S TH", 'S', trumpOrder, 1},
		{"overtrumped", "AH 7S 8S", 'S', trumpOrder, 2},
		{"trump led", "9S TS AH", 'S', trumpOrder, 1},
		{"no trump: suit only", "JH TH AS", NoTrump, betlOrder, 0},
		{"four seats", "2H KH AH 3S", 'S', bridge, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := trick(c.played).Winning(c.trump, c.order)
			if !ok || got.Seat != c.seat {
				t.Errorf("Winning(%s) = seat %d (ok %v), want seat %d", c.played, got.Seat, ok, c.seat)
			}
		})
	}
}

func TestEmptyTrick(t *testing.T) {
	var tr Trick
	if tr.Led() != 0 {
		t.Error("an empty trick has no led suit")
	}
	if _, ok := tr.Winning('S', trumpOrder); ok {
		t.Error("an empty trick has no winner")
	}
}

// Legal must never hand back the caller's slice: a module keeps the result
// in an offer while the hand it came from changes.
func TestLegalDoesNotAliasHand(t *testing.T) {
	hand := []string{"7H", "AS"}
	got := Legal(hand, Trick{}, 'S', trumpOrder, FollowBeatTrump)
	got[0] = "XX"
	if hand[0] != "7H" {
		t.Error("Legal returned the hand's own backing array")
	}
}

// TestLegalInvariants deals random positions and checks what every answer
// must satisfy whatever the rules: a player is never left with no card, never
// offered one they do not hold, and never let off following suit.
func TestLegalInvariants(t *testing.T) {
	var deck []string
	for _, s := range "HDCS" {
		for _, r := range trumpOrder {
			deck = append(deck, string(r)+string(s))
		}
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		r.Shuffle(len(deck), func(a, b int) { deck[a], deck[b] = deck[b], deck[a] })
		played := trick(strings.Join(deck[:r.Intn(3)], " "))
		hand := deck[3 : 4+r.Intn(10)]
		trump := []byte{'H', 'D', 'C', 'S', NoTrump}[r.Intn(5)]
		policy := []Policy{FollowOnly, FollowBeatTrump}[r.Intn(2)]

		got := Legal(hand, played, trump, trumpOrder, policy)
		if len(got) == 0 {
			t.Fatalf("no legal card: hand %v, trick %v", hand, played.Plays)
		}
		held := map[string]bool{}
		for _, c := range hand {
			held[c] = true
		}
		for _, c := range got {
			if !held[c] {
				t.Fatalf("offered %s not in hand %v", c, hand)
			}
			if led := played.Led(); led != 0 && len(ofSuit(hand, led)) > 0 && Suit(c) != led {
				t.Fatalf("let off following %c with %s: hand %v", led, c, hand)
			}
		}
	}
}
