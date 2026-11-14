# Black Peter — the pinned rules

The children's pairing game: Černý Petr in Czech, Schwarzer Peter in German, and the same family as
Old Maid and the French Pouilleux. Played here with an ordinary 52-card pack.
- **Code:** `server/internal/blackpeter`.
- **Pack:** French. Special picture packs exist (a Peter card and pairs of pictures); the standard
  pack with one jack removed is the common home substitute, and it needs no art of its own.

## Sources

The game has no governing body, and the many printed and online descriptions agree on the core:
remove one card so that one card is left unpaired, deal everything, put pairs down, take cards blind
from a neighbour, and whoever ends with the odd card loses. They differ on which card is removed,
on whether pairs need the same colour, on which neighbour you take from, and on how a match is
scored. The written rules below are our own words for the common reading, with the differences
made options where a table would reasonably want either.

- Wikipedia, "Black Peter (card game)": with a standard pack the jack of clubs comes out and the
  jack of spades is Black Peter; the loser is marked.
- Pagat, "Old Maid": the same family of games, pairs by rank or by rank and colour, taking cards
  from a neighbour.
- Czech and German household descriptions of Černý Petr / Schwarzer Peter: the loser carries a mark
  (traditionally a soot smudge on the face).
- `nextDeal` (the loser deals) is a common house custom rather than a printed rule; it is stated so
  the table always says who starts.

## The rules, by id

| Id | Rule |
|---|---|
| `goal` | Get rid of every card by putting them down in pairs; whoever is left with Black Peter loses the round. |
| `deck` | 52-card pack less the jack of clubs: 51 cards. The jack of spades has no partner and is Black Peter. |
| `pairsRank` / `pairsColour` 🔧 | A pair is two cards of the same rank (default), or of the same rank and colour. Black Peter pairs with nothing either way. |
| `deal` | Every card is dealt one at a time from the dealer's left; hands may differ by one card. |
| `layPairs` | Pairs go down face up as soon as a player holds them: after the deal, and after every card taken. |
| `draw` | On your turn take one card, unseen, from the nearest player on your left who still holds cards. |
| `pass` | The turn then passes to the player you took from (or past them, if they just went out). |
| `out` | A player with no cards is out of the round and safe. |
| `lose` | The round ends when one player still holds cards; they hold Black Peter alone and take a mark. |
| `nextDeal` | The loser deals the next round; the player on their left takes the first card. |
| `match` 🔧 | The match is a set number of rounds (1, 3, 5 or 10; default 5). Fewest marks win; ties share. |

🔧 = a table option.

## Simplifications, and why

1. **Pairs go down by themselves.** At a real table a child may miss a pair, and the others may
   point it out. Here nothing is gained by holding a pair (there is no rule that rewards it), so the
   table puts every pair down at once rather than make the player do bookkeeping.
2. **The taker chooses a position, not a card.** The neighbour's hand is shown as a fan of backs; a
   tap on one takes that card (`module.PickParam`). The control bar's "Take a card" takes one at
   random. Both are the same draw, since every card is face down.
3. **Where a taken card lands is random.** At a table the holder rearranges and shuffles their hand.
   Here a card that does not pair goes into a random place in the taker's hand, so the seat that gave
   it cannot point back at it next time it is their turn to take.
4. **No skill levels that differ.** There is no information to play on, so every bot strength takes
   a random card. The bot-skill option is still offered because it chooses the bots' names and seats.
