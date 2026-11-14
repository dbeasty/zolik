# Blackjack

Beat the dealer without going over twenty-one. Played for chips, never real
money. The app marks Blackjack "18+", and on phones it's hidden from players
under 18.

- **Players:** 2–7 seats (the dealer is the house, not a seat)
- **Variations:** Vegas Strip, Atlantic City, Single deck
- **Bots:** Easy, Medium, Hard
- **In-app rules:** the game's page › **Rules ›**

## The table

Every seat sits down with the same chips (500 by default), and the table has
a minimum stake (10 by default). Player cards are dealt face up. The dealer
keeps one card face down until everyone has played.

## How a round works

1. **Stakes up.** One stake per round, and once it's up it can't be changed.
   You can only stake chips you actually hold.
2. **Cards dealt.**
3. **Insurance**, if the dealer shows an ace: you may insure for half your
   stake, and it pays 2:1 if the dealer has blackjack.
4. **Each seat plays its hand in turn.** Draw as many cards as you like
   (**hit**), or **stand**. Going over twenty-one loses at once, whatever the
   dealer does afterwards.

Card values: 2 to 9 count their number; 10, jack, queen and king count 10; an
ace counts 11 while that fits, and 1 when it doesn't. An ace with a ten-value
card, as your first two cards, is a **blackjack**.

- **Double:** on your first two cards, double your stake and take exactly one
  more card.
- **Split:** two cards of the same value may be split into hands of their
  own, each with its own stake. Split aces take one card each and then stand,
  and twenty-one made that way isn't a blackjack.
- **Surrender** (Atlantic City): give up your first hand for half its stake,
  once the dealer has checked for blackjack.

**The dealer** draws to seventeen and then stands. Whether it draws to a soft
seventeen (one made with an ace) depends on the table. Showing an ace or a
ten, the dealer checks for blackjack before anybody plays.

## How the match ends

The table plays a fixed number of rounds (10 by default), and whoever holds
the most chips at the end wins. A seat that can no longer cover the minimum
sits out the rest of the match.

You may get up whenever you like. With cards in front of you, you play the
round out first. Your chips go with you as your result. You may also sit down
at any time with the starting chips, and mid-round you're in from the next
round. A player whose connection drops for longer than **Leave if away for**
leaves the table with their chips.

## Variations

| | Vegas Strip | Atlantic City | Single deck |
| --- | --- | --- | --- |
| Decks | 6 | 8 | 1 |
| Blackjack pays | 3:2 | 3:2 | 6:5 |
| Dealer on soft 17 | Stands | Stands | Hits |
| Splitting | Up to three times (four hands) | Up to three times | Once (two hands) |
| Double after split | Allowed | Allowed | Not allowed |
| Surrender | Off | Late surrender | Off |

## Options

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Starting chips** | 200 / 500 / 2000 | 500 | Chips each seat sits down with. |
| **Table minimum** | 5 / 10 / 25 | 10 | The smallest stake for a round. |
| **Rounds** | 5 / 10 / 20 | 10 | How many rounds the table plays. |
| **Decks** | 1 / 2 / 6 / 8 | per variation | Decks in the shoe. |
| **Dealer on soft 17** | Stands / Hits | per variation | Whether the dealer draws to a soft seventeen. |
| **Blackjack pays** | 3:2 / 6:5 / Even money | per variation | What a natural twenty-one pays. |
| **Splitting** | No splitting / Once (two hands) / Three times (four hands) | per variation | How many times a pair may be split. |
| **Double after split** | Allowed / Not allowed | per variation | Whether a split hand may be doubled. |
| **Surrender** | Off / Late surrender | per variation | Whether a first hand may be given up for half the stake. |
| **Insurance** | Offered / Not offered | Offered | Whether a dealer ace offers insurance at 2:1. |
| **Leave if away for** | 2 / 5 / 10 minutes / Never | 5 minutes | How long a disconnected player is sat out before leaving with their chips. |

Also available: Pause between rounds, Opponents, Hints.

## Tips

- Stand on hard 17 or more. Hit 11 or less.
- Always split aces and eights. Never split tens or fives.
- **Hint** asks a bot what it would do. It's a handy way to learn basic
  strategy.
