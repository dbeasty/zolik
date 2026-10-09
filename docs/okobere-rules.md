# Oko bere — the pinned rules

The Czech pub twenty-one, played with the Mariáš pack against a bank that goes round the table.
- **Code:** `server/internal/okobere`.
- **Card names:** Czech names stay Czech in every locale, as in Mariáš.

## Sources

The game has no association behind it, and the Czech sources agree on the core but differ in the
details. Where they differ, the app either picks the majority reading or makes it an option.

- [karetnihry.blogspot.com: Oko / Jednadvacet](https://karetnihry.blogspot.com/2011/04/oko-jednadvacet.html):
  card values, the bank, two aces paying three times, ties to the banker.
- [hazardni-hry.eu: Oko bere s malou domů](https://hazardni-hry.eu/karty/oko-bere.html): the bank
  as a pot, stakes matched from it, va banque, bets up to the bank, ties to the banker, a bust
  bank paying everyone standing.
- [gameday.cz: Oko bere](https://www.gameday.cz/oko-bere/): the banker playing last, "… bere!",
  and the variants.

## The rules, by id

| Id | Rule | Source |
|---|---|---|
| `goal` | Get closer to 21 than the bank without going over. | all |
| `deck` | 32 German-suited cards, shuffled again every round. | all |
| `values` | 7–10 at face value; spodek and svršek 1, král 2, eso 11. | karetnihry, hazardni-hry (gameday counts the svršek 2) |
| `oko` | Two aces as the first two cards are oko and win at once. | all |
| `bank` 🔧 | One player stakes a bank of {bank} chips (or all they have); a round's stakes together can be no more than the bank. | hazardni-hry |
| `rotation` 🔧 | The bank passes left after {n} rounds, or at once when broken. | see simplification 3 |
| `deal` | One card face down each; punters play in turn from the banker's left, the banker last. | all |
| `stake` 🔧 | Look at your card, then stake from the minimum up to your chips and what the bank can still cover. | hazardni-hry |
| `turn` | Take cards one at a time ("ještě") and stop when you have enough ("stačí"). | all |
| `bust` | Over 21 is bust ("přes"): the stake goes to the bank at once. | all |
| `okoPays3` / `okoPays1` 🔧 | A punter's oko is paid three times the stake (default) or the stake. | karetnihry and gameday say three times; hazardni-hry only "wins" |
| `banker` | The banker turns up their cards and draws the same way. | all |
| `settle` | Against a bank of 21 or under, more wins the stake and the same or less loses it; a bust bank pays everyone standing. | all |
| `bankerOko` | A banker's oko wins every stake still on the table. | hazardni-hry, gameday |
| `end` 🔧 | Everyone starts with {chips}; the match ends after the bank has gone round {n} times; most chips wins. | the app's |

## Simplifications

1. **No seven exchange.** Some tables let a player swap a first-card 7 (and then an 8). It is a
   house rule the sources word differently, so it is left out.
2. **No raising mid-hand (přisadit).** The stake is set once, on the first card.
3. **The bank passes on a schedule.** It moves after a set number of rounds, or as soon as it is
   broken, rather than by "malá domů", the banker's call to end their bank on a king of hearts or
   an eight.
4. **Oko pays out of the bank**, and never more than the bank holds.

## Options

| Option | Choices | Default |
|---|---|---|
| `startingStack` | 100, 200, 500 | 100 |
| `minBet` | 1, 2, 5 | 2 |
| `bank` | 20, 50, 100 | 50 |
| `roundsPerBank` | 3, 5, 8 | 5 |
| `bankTurns` | 1, 2, 3 | 1 |
| `okoPays` | the stake, three times | three times |
| `botSkill`, `pauseBetweenRounds` | stock | medium, pause |

## The bot

- **Easy** stakes the minimum and draws to 14.
- **Medium** stakes more on an eso or a ten, stands from 15, and banks to 16.
- **Hard** counts only what it may see: its own cards, and every bust or oko turned up. It draws
  by its chance of going over for the total it holds, and sizes its stake from a quick simulation
  of the hand.

Measured with `gamebench`, 300 duplicate pairs, three seats:
- medium beats easy by +3.5 chips a pair;
- hard beats medium by +5.0.

The game is mostly luck, which is why these margins are small next to the 100-chip stacks.
`botcost` puts hard at about 0.1 ms a decision.
