# Pinochle — the pinned rules

The American trick-and-meld game descended from Bezique, in four forms.
- **Code:** `server/internal/pinochle`.
  - `rules.go` states the sentences by id.
  - `engine.go` plays them.
  - `meld.go` holds the meld table.
  - `bot.go` and `sampling.go` hold the three skills.
- **Copyright:** Pinochle is a traditional game from the nineteenth century. Its rules are public
  domain, and game mechanics are not copyrightable in any case. The rules text here and in the app
  is our own wording. The name is generic, and the cards are the app's ordinary French faces.
- **Sources:** John McLeod, pagat.com:
  - [Partnership Auction Pinochle](https://www.pagat.com/marriage/pinmain.html)
  - [Cutthroat (three-handed auction) Pinochle](https://www.pagat.com/marriage/pin3.html)
  - [Two-hand Pinochle](https://www.pagat.com/marriage/pin2.html)
  - [Double-deck Pinochle](https://www.pagat.com/marriage/pindd.html)

  Where they disagree, or where tables differ, the rule became a table option. The default is the
  most common form.

## The tables

| Variation | Seats | Pack | Hands | Extra |
|---|---|---|---|---|
| `partnership` (default) | 4, partners opposite | 48: two of each A T K Q J 9 | 12 | optional pass of 3 or 4 between partners |
| `cutthroat` | 3, each alone | 48 | 15 | 3-card widow to the bidder, who buries 3 |
| `twohanded` | 2 | 48 | 12 | stock and turned-up trump; meld after each trick won |
| `doubledeck` | 4, partners opposite | 80: four of each A T K Q J, no nines | 20 | the larger double, triple and quadruple melds |

## The rules, by id

Every id below is `pinochle.rules.<id>`.

| Id | Rule |
|---|---|
| `order.single` / `order.double` | Rank within a suit: A T K Q J 9 (no 9 on the double pack). |
| `points.counters` 🔧 | Default. Aces, tens and kings are counters, 10 each. |
| `points.classic` 🔧 | A 11, T 10, K 4, Q 3, J 2. The hand is worth the same under either option. |
| `points.blank` / `points.blankDouble` / `points.nine` | What scores nothing, said outright: Q, J and 9 under counters (Q and J on the double pack), the 9 under classic. |
| `lastTrick` | The last trick is worth 10 more (20 on the double pack): 250 a hand, or 500 on the double pack. |
| `auction` 🔧 | Bidding starts left of the dealer. The minimum is 250 (500 on the double pack). |
| `bidStep` | Each bid beats the last by a multiple of 10. |
| `passFinal` | A pass is final for the hand. |
| `allPass.stuck` 🔧 | Default. If all pass, the dealer takes the hand at the minimum. |
| `allPass.redeal` 🔧 | If all pass, the hand is thrown in and the deal moves on. |
| `trump` | The bidder names any suit. |
| `takeWidow`, `bury` | Cutthroat. The widow is shown to all and goes into the bidder's hand. The bidder buries 3, and any counters among them score for the bidder. |
| `pass` 🔧 | Partnership tables. The bidder's partner passes N to the bidder, who passes N back. Off by default. |
| `meld.*` | Run 150. Royal marriage 40. Marriage 20. Dix 10. Arounds: A 100, K 80, Q 60, J 40. Pinochle 40. Doubles: run 1500; aces 1000, kings 800, queens 600, jacks 400; pinochle 300. Triples and quadruples on the double pack: aces 1500 and 2000, kings 1200 and 1600, queens 900 and 1200, jacks 600 and 800; pinochle 600 and 900. |
| `meld.doublesNeed` | A double needs two of every card in it and scores instead of the single meld: a double run is 1500, not 1650. |
| `meld.classes` | A card may count in several melds only if they are of different classes: sequence/marriage/dix, arounds, pinochle. A run's K and Q are not also a royal marriage. |
| `meld.classesExample` | The Q♠ may count in a spade marriage, queens around and a pinochle at once; a second trump K and Q beside a run is still a royal marriage. |
| `meld.shown` | Three or four players: the meld is counted automatically when trumps are named and shown through the first trick. |
| `meld.needsTrick` 🔧 / `meld.needsCounter` 🔧 | A side's meld counts only if it takes a trick (default), or a counter. |
| `lead.bidder` | The bidder leads to the first trick. |
| `follow`, `beat`, `trumpIn` | Follow suit. Beat the best card if you can. If void, trump, and overtrump if you can. Once the led suit is trumped, any card of the suit will do. This is `tricks.FollowBeatTrump`. |
| `identical` | Of two identical cards, the first played wins. |
| `made` / `set` | The bidding side scores meld plus tricks if that reaches the bid; otherwise it loses the bid. |
| `defenders.*` | The other side or sides score their meld and tricks. |
| `throwIn` | Before the first lead the bidder may concede. The bid is lost, and the others score their meld only. |
| `end`, `bidderOut`, `tiePlaysOn` | The match goes to the target, 1500 by default. If more than one side is over, the bidding side wins. A tie at the top plays on. |

### Two-handed

| Id | Rule |
|---|---|
| `turnUp`, `turnUpDix` | After 12 each, the next card is turned for trumps and lies under the stock; it is drawn last. A turned dix scores 10 for the dealer. |
| `lead.twohanded` | The non-dealer leads. |
| `stockFree` | While the stock lasts, any card may be played. |
| `meld.onWin`, `meld.onePerTurn` | The trick's winner may lay one meld before drawing. Melded cards stay in hand and may be played. |
| `meld.exchange` | Exchanging the dix for the turned trump is that turn's meld and scores 10. A second dix may simply be shown for 10. |
| `draw` | The winner draws first. |
| `late`, `meld.staysInHand` | When the stock is gone, melds go back into hand and the last 12 tricks are played under `follow`/`beat`/`trumpIn`. |
| `score.twohanded` | Meld scores when laid. Tricks score at the end of the hand. The target is 1000 by default. |

On the table, each meld is labelled with its name and its points (`Group.Value`), so a seat's meld total can be read off the cards.

## Simplifications

1. **Meld is counted for you** in the three- and four-handed games. Every combination a hand holds
   is counted under the class rule, which is also the most it can score. Nobody can forget a meld,
   and nobody has to declare one.
2. **The cutthroat bidder buries before the meld is counted.** Pagat has the bidder meld first and
   then bury cards that were not melded. Counting after the bury gives the same score as the best
   legal bury.
3. **Two-handed meld reuse follows the class rule only.** Some tables also allow a card to move up
   within its class, for example a royal marriage later grown into a run. The app does not.
4. **No "shooting the moon", no "double-or-nothing", no partner-plays-alone.** These are local
   variants. Any of them could become an option.

## The bot

- **Easy:**
  - bids on its meld and a third of the hand, with noise;
  - names its best meld suit;
  - plays a random legal card half the time.
- **Medium:**
  - bids its best meld plus a share of the tricks, weighted by aces and trump length, plus an
    allowance for a partner's or the widow's meld;
  - raises by the least step, and does not bid against a partner without a clearly stronger hand;
  - cashes boss cards, pulls trumps when its side bid and is long in them;
  - smears counters on a partner's sure trick, otherwise wins cheaply or sheds its cheapest card;
  - buries counters from short suits;
  - passes its partner trumps and aces;
  - throws in a hopeless bid.
- **Hard:**
  - bids and handles the widow and pass like Medium;
  - for card play, samples deals consistent with everything public: melds still held, cards it
    passed, the widow, and voids and "could not beat" caps run backwards from the follow rules;
  - plays each candidate out with Medium in every seat and scores the hand exactly as the engine
    would;
  - from 20 cards left (18 in Cutthroat), solves each sample exactly with `tricks/search`;
  - in two-handed after the stock is gone, solves the one possible deal exactly.

  The solver knows a card by its code, so duplicates are renamed apart first (`QS`, `QS'`) and
  named back afterwards. See `renameApart`.
