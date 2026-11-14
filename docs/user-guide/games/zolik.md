# Žolíky

A Czech rummy game played with jokers ("žolíky"). Lay your cards down in sets
and runs and be first to empty your hand. Whatever you're still holding when
someone else goes out counts against you.

- **Players:** 2–8
- **Variations:** Žolík Classic, Continental
- **Bots:** Easy, Medium, Hard and AI (a trained model)
- **In-app rules:** the game's page › **Rules ›**, or the rules button on the
  table

## Objective

Be the first to empty your hand by laying down valid sets and runs, and keep
as few penalty points as possible in the cards you're still holding when
someone else goes out. The player with the lowest total at the end wins.

## The cards and melds

The game uses full 52-card packs, each with two jokers: two packs for 2–4
players, three for 5–6, and four for 7–8.

- A **set** is 3 or more cards of the same rank. It can't hold more than 4
  cards, because a joker fills a missing suit and doesn't pad a full one. A
  set can't hold more jokers than natural cards.
- A **run** is consecutive cards of one suit: 3 or more in Classic, 4 or
  more in Continental. A run can hold up to 14 cards: the ace at the bottom,
  the twelve ranks above it, and the ace at the top. An ace sits above the
  king or below the two, but never bridges the two ends of a run.
- A joker stands for any card.

## How a turn works

1. **Draw** one card, from the stock or from the discard pile.
   - In **Classic**, any card in the discard pile may be taken, together with
     everything stacked above it.
   - In **Continental**, only the top card may be taken, and not before
     round 3.
   - A card taken from the discard pile must be played this turn, onto a meld
     on the table or into a new meld. Before you're down, it has to go into
     the melds that take you down. It can't be discarded again on the same
     turn.
2. **Lay down and add to melds** (see below).
3. **Discard** one card to end your turn. A joker can never be discarded,
   except as the exact card that empties your hand.

When the stock runs out, the discard pile is shuffled to make a new stock. If
both are empty, the deal ends.

### Getting down

You're "down" once you've laid the melds the deal requires:

- **Classic:** at least one of your runs must be completely joker-free
  (a "clean run"). There's no minimum point value.
- **Continental:** each of the 7 deals has its own contract, and your first
  melds must be worth at least 35 points. Jokers may be used freely in runs.

| Continental deal | Contract |
| --- | --- |
| 1 | 2 sets |
| 2 | 1 set and 1 run |
| 3 | 2 runs |
| 4 | 3 sets |
| 5 | 2 sets and 1 run |
| 6 | 1 set and 2 runs |
| 7 | 3 runs |

Towards the point minimum, each card counts what it would cost in hand,
except an ace: 15 in a set or above a king, 1 below a 2. A joker counts as
the card it stands for. Until you're down, every meld you lay must be one the
contract still needs. A lay-down you start must be finished, or taken back,
before you discard. The status line tells you what you still need ("You
still need 1 run to go down.").

### After you're down

- You may add cards to anyone's melds, but only after you've laid your own
  contract. A card added to a run must continue it at one end or the other.
- A joker in a meld on the table can be bought back with the exact card it
  stands for. By default, a joker bought back must be played into a meld the
  same turn.

## Scoring

When someone goes out, everyone else pays for the cards left in their hand:

| Card | Penalty |
| --- | --- |
| Joker | 50 |
| 10, J, Q, K | 10 |
| 2–9 | Its number |
| Ace | 25, or only 1 if a 2 of its suit, or both the Q and K of its suit, are also in your hand, or if it would extend a run on the table |

- **Classic:** deals continue until someone reaches 200 points. The lowest
  total wins.
- **Continental:** the match is 7 deals. The lowest total wins.

**Show score breakdown** on the results panel shows exactly how each hand was
counted.

## Variations and options

| Variation | Players | In short |
| --- | --- | --- |
| Žolík Classic | 2–8 | 13 cards each, runs of 3+, a clean run needed, no point minimum, discard pile open from the first round, any card in the pile may be taken |
| Continental | 2–8 | 12 cards each, runs of 4+, 7 deals with a contract each, a 35-point first meld, top card only, pile closed until round 3 |

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Meld value** | Off / 35 / 50 / 70 | Classic: Off; Continental: 35 | Minimum natural points across your first meld(s) before you're down. |
| **Discard pickup** | Open / Round 2 / Round 3 | Classic: Open; Continental: Round 3 | The round from which the discard pile may be drawn. |
| **Clean run** | Required / Off | Classic: Required; Continental: Off | Whether a joker-free run must be on the table before you count as down. |
| **Reclaimed joker** | Play same turn / May be kept | Play same turn | Whether a joker taken back off the table must be played the same turn, or may be kept in hand. |
| **Deal starter** | Rotate / Winner leads | Rotate | Who leads the next deal: the table rotating through every seat, or whoever just went out. |
| **Discard pile** | Open to look through / Top card only | Open to look through | Whether players may look through the whole discard pile. |

Also available: Pause between rounds, Opponents, When a player drops, Step
out, Hints. See [Options most games share](../playing.md#options-most-games-share).

## Tips

- Watch the ace rule: an ace in hand costs 25, but only 1 if you also hold
  its suit's 2, or its queen and king, or if it would fit a run on the table.
- Jokers cost 50 in hand. Get them onto the table early.
- In Classic, taking deep from the discard pile can win you a lot of cards,
  but every card you take has to be used this turn.
- In Continental, check the contract in the header ("Game 3 of 7: …") before
  you plan your hand.
