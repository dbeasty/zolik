# Šnaps and Šedesát šest — the pinned rules

Two-player marriage games with the German-suited pack, as one module with two variations.
- **Code:** `server/internal/snaps`.
  - `rules.go` states the sentences by id.
  - `snaps.go` holds the switches that tell the two games apart (`ruleset`).
  - `score.go` is the scoring, shared by the engine and the hard bot's search.
- **Wording:** Czech card names (eso, král, svršek, spodek) stay Czech in every locale, as in
  Mariáš.

## Sources

- **[Šnaps]** John McLeod, [Schnapsen](https://www.pagat.com/marriage/schnaps.html), the Austrian
  game played in Czechia as Šnaps: 20 cards, 5 each.
- **[66]** John McLeod, [Sixty-six](https://www.pagat.com/marriage/66.html): 24 cards with the
  nines, 6 each.

## What the two variations share

| Id | Rule | Source |
|---|---|---|
| `goal` | Be first to 66 card points each deal; a deal is worth 1, 2 or 3 game points. | both |
| `points`, `points66` | Eso 11, 10 10, král 4, svršek 3, spodek 2 (9 nothing): 120 in all. | both |
| `deal.turns` | The deal alternates; the non-dealer leads first; whoever takes a trick leads next. | Šnaps (66: the winner deals; see below) |
| `play.open` | While the talon is open any card may be played; higher card of the suit led, or higher trump, wins. | both |
| `play.draw` | Winner draws first, then the other; the last card drawn is the turned-up trump. | both |
| `play.strict` | Talon closed or used up: follow suit and beat if you can, else trump, else anything. | both |
| `marriage` | Lead král or svršek holding the other: 20, or 40 in trumps; counts once a trick is taken. | both |
| `out` | Reaching 66 on taking a trick or announcing a marriage ends the deal at once. | both (see simplification 1) |
| `score.out` | Going out: 3 if the other has no trick, 2 if under 33, else 1. | both |
| `close` | On lead with the talon open you may close it: no drawing, strict rules, and you must reach 66. | both |
| `score.closerFailed` | A closer who runs out short of 66 gives the other 2, or 3 if they had no trick at closing. | both |
| `end` 🔧 | The match is first to {n} game points. | option (7 by default, both sources) |

## Where they differ

| | Šnaps | Šedesát šest |
|---|---|---|
| Cards, hand | 20, 5 each (`deck20`, `deal5`) | 24, 6 each (`deck24`, `deal6`) |
| Exchanged trump | spodek, by the leader (`exchange`) | 9, by a leader who has taken a trick (`exchange66`) |
| Marriages after closing or running out | allowed | not allowed (`marriageOpenOnly`) |
| A closer who goes out is paid on | the other's tricks and count **at closing** (`score.closedAtClosing`) | the other's count **at the end** (`score.closedAtEnd`) |
| The closer's opponent goes out first | 2, or 3 if they had no trick at closing | 2 |
| Talon used up, nobody at 66 | the last trick wins the deal for 1 (`lastTrickSnaps`) | last trick +10; the higher count wins, scored as going out; 65–65 scores nothing (`lastTrick66`) |

## Simplifications (where the app differs from a table)

1. **Going out is automatic.** The moment a player may claim 66 (on taking a trick or announcing a
   marriage) the deal ends in their favour. Nobody can miscount, so the penalty for a false claim
   never arises. At a real table a player could choose not to claim, but there is almost never a
   reason to.
2. **A marriage is announced by leading one of its cards while holding the other**, as Mariáš
   does it. The other card is shown, so it counts as known to the opponent until played.
3. **The deal alternates in both games.** In Sixty-six the winner of a deal deals the next; an
   app match is short enough that the difference is not worth a rule.
4. **Closing is only on the lead, with a full hand.** Sixty-six also allows closing before drawing
   after winning a trick, and the opponent may exchange the trump nine right after a close. Both
   are left out.

## Options

| Option | Choices | Default |
|---|---|---|
| `target` | 3, 7, 11 game points | 7 |
| `showCardPoints` | hidden, shown | hidden. Your own count is always shown to you; this shows the other player's too. |
| `botSkill`, `pauseBetweenRounds` | stock | medium, pause |

## The bot

- **Easy** plays a legal card at random.
- **Medium** plays rules of thumb: take an ace or ten led, keep trumps and marriages, lead low,
  cash top cards once play is strict.
- **Hard** plays like medium while the talon is open. Once the talon is closed or used up it solves
  the rest exactly (`solve.go`): alpha-beta over the two hands, checked against plain minimax and
  against the engine's own scoring. With the talon used up the position is open; with it closed,
  the other hand is sampled from the unseen cards, respecting voids shown. Hard decides whether to
  close by solving the closed deal over those samples, and closes when it wins at least 80% of
  them.

Measured with `gamebench`, 150 duplicate pairs each:
- medium beats easy by +12.6 game points a pair;
- hard beats medium by +5.6 a pair.

`botcost` puts hard at about 1 ms a decision (Šedesát šest, p99 6 ms).
