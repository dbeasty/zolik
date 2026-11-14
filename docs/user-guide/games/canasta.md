# Canasta

The partnership rummy game. Build melds of the same rank, complete canastas
(melds of seven), and go out before the other side.

- **Players:** 2–6 (Classic and Modern American up to 4; Samba and CanastaX
  up to 6). Four players form two partnerships and six form three, with
  partners sitting opposite. With two, three or five players, everyone plays
  for themselves.
- **Variations:** Classic, Modern American, Samba, CanastaX
- **Bots:** Easy, Medium, Hard and AI (a trained model)
- **In-app rules:** the game's page › **Rules ›**

## Objective

The first side (a partnership, or a single player) to pass the target score (5000 in
Classic and Modern American, 10000 in Samba and CanastaX) wins the match.
Melds belong to the partnership: either partner may extend them, and neither
side may touch the other's.

## The cards

Classic and Modern American use 108 cards (2 decks plus jokers). Samba and
CanastaX use 162 cards (3 decks plus jokers). Jokers and 2s are **wild**.

| Card | Counts |
| --- | --- |
| Joker | 50 |
| 2 | 20 |
| Ace | 20 |
| King down to 8 | 10 |
| 7 down to 4 | 5 |
| Black three | 100 |

**Red threes** are bonus cards. One in your hand is shown immediately and
scores as a bonus, unless your side never completes a canasta, in which case
it counts against you. Red threes never count towards a meld.

**Black threes** block the discard pile: nobody may take the pile while a
black three is on top. One left in your hand costs 100.

## How a turn works

1. **Draw** from the stock (two cards in Samba and CanastaX), **or take the
   whole discard pile**. You can take the pile only with a move that uses its
   top card straight away.
2. **Meld**: lay down new melds or add to your side's melds.
3. **Discard** one card. The discard ends your turn, so always keep a card
   to spare for it. Never meld away your whole hand unless that's the move
   that goes out.

### Melds

- A meld is 3 or more cards of the same rank. A **canasta** is a meld of 7 or
  more.
- **Classic and Modern American:** a meld may hold at most 3 wild cards, and
  never fewer than 2 natural ones. Your side keeps one meld of each rank, and
  a canasta of 7 is complete and takes no more cards.
- **Samba and CanastaX:** a meld needs 2 natural cards for every wild one,
  and never more than 2 wilds. Your side may have several melds of the same
  rank. A meld can also be a **sequence**, three or more cards of one suit in
  a row. A sequence of seven is a **samba**, worth 1500.

### Your first meld

Your side's first meld must reach a minimum that rises with your score:

| Your side's score | Minimum (Classic, Modern American) | Minimum (Samba, CanastaX) |
| --- | --- | --- |
| Below zero | 15 | 15 |
| Up to 1500 | 50 | 50 |
| Up to 3000 | 90 | 90 |
| Up to 7000 | 120 | 120 |
| Beyond that | 120 | 150 |

Until your side has made its first meld, it may add only natural cards, and
only to melds it laid that same turn. Those count towards the minimum.

### The discard pile

- **Classic:** if your side already has an unfinished meld of the top card's
  rank, you can take the whole pile to add that card to it. A wild card buried
  in the pile **freezes** it against everyone: taking it then costs two
  natural cards of the top card's rank, from your own hand. Three or more
  black threes may be melded, but only as part of going out.
- **Modern American:** a meld already on the table can't take the pile. You
  must match the top card with two cards from your hand. Black threes are
  never melded.
- **Samba and CanastaX:** the pile is frozen all deal. To take it you must
  match its top card with two natural cards from your hand.

Once the stock is empty, play goes on only while the player whose turn it is
can take the discard pile and still finish their turn. When they can't, the
deal ends.

## Going out and scoring

- **Going out** needs 1 completed canasta in Classic, and 2 in the other
  variations.
- Going out is worth 100 (200 if your side goes out "concealed", in a single
  turn with nothing melded before) in Classic and Modern American, and a flat
  200 in Samba and CanastaX.
- At the end of a deal each side scores the cards it laid, its canasta
  bonuses (a natural canasta 500, a mixed one 300; a samba 1500), its red
  threes and its going-out bonus. It loses the cards still in its hands.
  **Show score breakdown** lists every line.

## Variations

| Variation | Players | Cards each | Target | In short |
| --- | --- | --- | --- | --- |
| Classic | 2–4 | 11 | 5000 | One canasta to go out; frozen pile only when a wild is buried |
| Modern American | 2–4 | 13 | 5000 | Two canastas to go out; black threes never melded; melds can't take the pile |
| Samba | 2–6 | 15 | 10000 | Three decks, draw two, sequences and sambas, pile always frozen |
| CanastaX | 2–6 | 15 | 10000 | Samba plus every house rule below switched on |

### CanastaX house rules

CanastaX switches on all of the following. You can also turn them on
individually at other tables (wild sequences need a variation that has
sequences):

- **Dirty sequences**: sequences may hold wild cards, at most two and never
  more than half as many as the naturals. Seven with a wild among them is a
  dirty samba, worth 700.
- **Meld of 2s**: once your side has opened it may lay one meld of 2s and
  jokers in any mix. Seven cards is a canasta: 2250 if all 2s, 1500 with
  jokers.
- **Rearrange melds**: on your turn, once your side has opened, move cards
  between your side's melds as long as every meld stays legal. A canasta
  broken this way is no longer a canasta.
- **Swap for wilds**: on your turn, once your side has opened, swap a natural
  card from your hand for the wild it stands for in any meld on the table,
  and keep the wild.
- **Top card or whole pile**: whenever you could take the discard pile, you
  may take just its top card instead.

Moves made with Rearrange and Swap can be taken back with **Undo move** and
**Undo swap**.

## Options

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Cards dealt** | 11 / 13 / 15 | Classic 11; Modern American 13; Samba, CanastaX 15 | Cards each player starts a deal with. |
| **Target score** | 500 (short) / 1000 / 3000 / 5000 / 10000 | Classic, Modern American 5000; Samba, CanastaX 10000 | The score a partnership must pass to win. |
| **Canastas to go out** | 1 / 2 | Classic 1; others 2 | How many canastas a side needs before it may go out. |
| **Dirty sequences** | Off / On | CanastaX On | See above. |
| **Meld of 2s** | Off / On | CanastaX On | See above. |
| **Rearrange melds** | Off / On | CanastaX On | See above. |
| **Swap for wilds** | Off / On | CanastaX On | See above. |
| **Top card or whole pile** | Off / On | CanastaX On | See above. |
| **Discard pile** | Open to look through / Top card only | Top card only | Whether players may look through the whole pile. |

Also available: Pause between rounds, Opponents, When a player drops, Step
out, Hints.

## Tips

- A frozen pile is the big prize late in a deal. Keep pairs of cards your
  opponents discard.
- Bury a black three on the pile to stop the next player taking it.
- Don't meld your last cards until you can go out: you always need one to
  discard.
