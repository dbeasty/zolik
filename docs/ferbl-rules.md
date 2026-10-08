# Ferbl — the pinned rules

The Czech pub vying game of four cards and suits.
- **Code:** `server/internal/ferbl`.
- **Card names:** Czech names stay Czech in every locale, as in Mariáš.

## Sources

- **[Ferbli]** John McLeod, [Ferbli](https://pagat.com/vying/ferbli.html): the Hungarian game Czech
  Ferbl comes from, and the only full written account of the family. The pack, the card values
  and the ranking of hands are taken from it as they stand.
- Publisher rules sheets in Slovak, German and Romanian describe the same hands with a simpler
  betting structure: two cards, a stake, two more, a final round of at most three raises. That
  structure is what the app plays.

No Czech source describes Ferbli's charging and counter-charging (*kóstáltatás*) the same way
twice, so the app plays plain limit betting instead (see below).

## The hands, best first

| Id | Hand |
|---|---|
| `fourKind` | Four of a kind: eso highest, seven lowest. |
| `banda` | Four of a suit, by its total: 41 down to 34. |
| `threeKind` | Three of a kind. |
| `threeSuited` | Three of a suit, by their total: 31 down to 24. |
| `twoAces` | Two aces, the only pair that counts. |
| `twoSuited` | Two of a suit, by their total: 21 down to 15. |
| `suits` | One card of each suit, by the highest card: 11 down to 8. |
| `ignored` | Cards outside the combination count for nothing. Equal hands go to the player first in turn from the dealer's left, as in Ferbli, where the pot is never split. |

Card values (`values`): eso 11; král, svršek, spodek and 10 count 10; 9, 8 and 7 their number.

## Betting

| Id | Rule |
|---|---|
| `ante` 🔧 | Everyone pays the vizi before the deal and gets two cards. |
| `round1` | On two cards, from the dealer's left: check or bet one vizi; then call, raise by one vizi, or fold. |
| `round2` | Two more cards each; a second round with bets and raises of two vizi. |
| `cap` | Each round closes after three raises. |
| `allIn` | A player short of chips calls with what they have and plays for the part of the pot they matched (side pots). |
| `showdown` | The hands still in are shown; the best takes each pot it is eligible for. A lone survivor takes it unseen, and a folded hand is never shown. |
| `end` 🔧 | The match lasts a set number of hands, or until one player has every chip; the most chips wins. |

## Simplifications

1. **Limit betting instead of charging.** Ferbli's *besszer* and *kóstáltatás* rounds, and its
   blind betting, are replaced by two limit rounds. The hands and their ranking are unchanged.
2. **Everyone antes,** rather than only the first player paying the vizi, and there is no
   *cukassza* (a hand with too few players in, carried over). If everyone checks the last round,
   the hands are shown.
3. **No wild cards, no grigáré, no shown cards** (all Ferbli variants on the Pagat page).

## Options

| Option | Choices | Default |
|---|---|---|
| `startingStack` | 100, 200, 500 | 100 |
| `vizi` | 1, 2, 5 | 2 |
| `hands` | 10, 20, 30 | 20 |
| `botSkill`, `pauseBetweenRounds` | stock | medium, pause |

## The bot

Every skill estimates its chance of holding the best hand at the showdown. It deals out the cards
it cannot see, sorted before shuffling so the real deal cannot leak in, and counts ties as half.

- **Easy** calls almost anything and never raises.
- **Medium** bets above 55%, raises above 72%, and calls when its chance covers the price (the call
  as a share of the pot after calling).
- **Hard** also reads the betting. A player who has bet or raised this hand is dealt, as far as
  the sampling can manage, a hand that would have bet: two aces or better after one bet or raise,
  three of a suit or better after two. Before that reading, hard played *worse* than medium (−7
  chips a match). After it, it plays clearly better.

Measured with `gamebench`, 200 duplicate pairs, three seats:
- medium beats easy by +28.4 chips a pair;
- hard beats medium by +25.3.

`botcost` puts hard at about 1.3 ms a decision at six seats.
