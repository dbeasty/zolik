# Sedma — the pinned rules

The Czech and Slovak pub game in which a seven takes everything.
- **Code:** `server/internal/sedma`.
  - `rules.go` states the sentences by id.
  - `engine.go` plays them.
  - `bot.go` holds the three skills.
- **Source:** John McLeod, [Sedma](https://pagat.com/sedma/sedma.html).
- **Card names:** Czech names stay Czech in every locale, as in Mariáš.

## The rules, by id

| Id | Rule | Source |
|---|---|---|
| `goal` | Take aces and tens in tricks: 10 each, the last trick 10 more, 90 in a hand. | Pagat |
| `deck` | 32 German-suited cards; at three, 30, with two eights out (the app removes 8D and 8S). | Pagat |
| `points` | Eso and 10 are worth 10; nothing else counts. | Pagat |
| `teams` | Four play in partnerships, partners opposite; two or three play alone. | Pagat |
| `deal` | 4 cards each, the rest is the stock; left of the dealer leads; the deal passes left. | Pagat (see simplification 1) |
| `play` | No suit to follow: any card may be played. | Pagat |
| `capture` | A card of the rank led, or any 7, captures; the last such card played takes the trick. | Pagat |
| `continue` | After each full round the leader may continue with the led rank or a 7, and everyone plays again; otherwise the trick ends. | Pagat |
| `draw` | After each trick everyone draws one at a time, the winner first, back to 4 while the stock lasts. | Pagat |
| `score` | Most points: 1 stake; all 90 points: 2; every card: 3. | Pagat |
| `score3` | At three, two level on most points win 1 each; 30 each scores nothing. | Pagat |
| `end` 🔧 | First to {n} stakes wins (3, 5 or 10; 5 by default). | Pagat's "points instead of stakes" variant |

## Simplifications

1. **The deal passes to the left every hand, and the player to the dealer's left leads.** Pagat
   gives the next deal to the losers and the next lead to the winners. That varies between tables
   and changes nothing about play.
2. **No "burned" (spálená) hands and no eights-wild (osmová).** Both are regional variants on the
   Pagat page. Either could become a table option later.
3. **The leader's continue-or-stop decision is explicit.** When a round completes and the leader
   holds the led rank or a seven, play waits for them to continue or to let the trick go. A leader
   who holds neither ends the trick automatically.

## The bot

- **Easy** plays a legal card at random and usually lets a trick go.
- **Medium** plays rules of thumb:
  - capture a trick that has points in it, preferring the led rank to a seven;
  - throw an ace or ten onto a trick its side will keep;
  - lead a blank rank it holds twice;
  - continue only to win back points.
- **Hard** deals the cards it cannot see again, with the pool sorted first so the deal it really
  faces cannot leak into the sample. It plays each legal choice out to the end of the hand with
  medium in every seat, over the same 24 samples. It then picks the choice with the best stakes,
  then the best points.

Measured with `gamebench` (duplicate pairs, matches to 5):

| Pairing | Table | Margin |
|---|---|---|
| medium vs easy | four seats | +8.9 stakes a pair |
| hard vs medium | four seats | +4.1 |
| hard vs medium | two seats | +3.6 |

`botcost` puts hard at about 1.2 ms a decision.
