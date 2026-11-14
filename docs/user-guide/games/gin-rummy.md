# Gin Rummy

The classic two-player rummy. Form melds, cut your deadwood, and knock before
your opponent does.

- **Players:** 2
- **Variations:** Standard, Oklahoma
- **Bots:** Easy, Medium, Hard
- **In-app rules:** the game's page › **Rules ›**

## Setup

A 52-card deck. Each player is dealt 10 cards, and one more card is turned
face up to start the discard pile. Before the first draw, the non-dealer may
take that upcard, then the dealer may. If both pass, the non-dealer must draw
from the stock.

## Melds and deadwood

- A **meld** is a set of three or four cards of one rank, or a run of three
  or more in one suit.
- The ace is always low. There's no run from queen through ace.
- **Deadwood** is every card not in a meld. An ace counts 1, a 2 to 9 its
  number, and a 10, jack, queen or king 10.

## How a turn works

Draw one card, from the stock or the discard pile, then discard one.

**Knocking** takes the place of your discard, so it can only happen at the end
of your turn. You may knock once your deadwood is 10 or less (in Oklahoma,
the upcard sets the limit for the hand). Zero deadwood is **gin**, the best
possible knock.

After a knock that isn't gin, your opponent may **lay off** their own deadwood
onto your melds before the hands are compared.

If the stock runs down to its last two cards and nobody has knocked, the hand
is **dead**: nobody scores, and the same dealer deals again.

## Scoring

- **Knock:** if your deadwood is lower, you score the difference between the
  two deadwood counts.
- **Undercut:** if your opponent's deadwood is no higher than yours, they
  undercut you, scoring the difference plus 25.
- **Gin:** scores your opponent's whole deadwood, plus 25.
- **Big gin** (if switched on): all eleven cards meld with no discard, for a
  further 25.

The first player to pass the target score (100 by default) after a hand wins
the match. With **Line bonuses** on, each hand won is worth 25 at the end of
the match, and winning the match is worth a further 100 (200 if the loser
never scored a point).

## Options

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Target score** | 100 / 150 / 250 / 500 | 100 | The score a player must pass to win the match. |
| **Knock limit** | 10 / Oklahoma (the upcard sets it) | Standard: 10; Oklahoma: the upcard | The deadwood you may knock with. |
| **Big gin** | Off / On (+25) | Off | Eleven cards melding with no discard, for a bonus. |
| **Line bonuses** | On / Off | On | 25 per hand won, and 100 for the game (200 on a shutout). |
| **Discard pile** | Open to look through / Top card only | Top card only | Whether players may look through the whole pile. |

Also available: Pause between rounds, Opponents, When a player drops, Step
out, Hints.

## Tips

- Knock early with low deadwood rather than waiting for gin, unless you
  suspect your opponent is close to undercutting you.
- Watch what your opponent picks up from the discard pile, and avoid feeding
  their melds.
