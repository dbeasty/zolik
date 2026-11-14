# Prší — rules as built

The rules `server/internal/prsi` implements, for reference. The players' copy
is the module's own `Rules()`, rendered by the rules screen per table. Prší is
the Czech member of the Crazy Eights family, played with the 32-card
German-suited pack.

## The pack: 32 cards

- 7, 8, 9, 10, Spodek (J), Svršek (Q), Král (K) and Eso (A) in four suits.
- Card codes are the server's usual ones: `7H`, `TS`, `QD`, `AC`.

## The deal

1. Deal 4 each (option: 4, 5 or 6) and turn one card face up.
   - An opening svršek is buried and the next card turned. If only svršeks
     are left to turn, one is turned after all and its own suit is in play.
   - An opening 7 or ace takes effect on the first player, as if it had been
     played at them.
2. **The deal must leave a card to turn up and one to draw.** Seats times
   cards dealt may be at most 30, so six players are dealt at most five each.
   A table set to six players at six cards is refused when it is dealt
   (`DEAL_TOO_LARGE`, pointing at rule `prsi.rules.dealLimit`) rather than
   dealt short.
3. The first player is chosen by the seed.

## A turn

One move a turn, and every move ends it:

1. **Play** a card that matches the top card by suit or rank, or a svršek. A
   svršek on top is matched by the suit its player named, never by rank.
2. **Draw** one card instead. When the draw pile runs out, everything under
   the top of the discard pile is shuffled to make a new one.
3. **Nothing left to draw.** When the draw pile is empty and the discard pile
   is down to its top card, and none of your cards fits, drawing passes your
   turn without a card (rule `prsi.rules.turn.nothingToDraw`). A player who
   *can* play may not draw from an empty pile (`NOTHING_TO_DRAW`): the empty
   draw is the way out of a dead position, not a way to stall.

## Special cards

- **7**: the next player draws two, or answers with a 7 of their own, which
  passes on the whole debt plus two (rule `prsi.rules.sevens`). Taking the
  debt is the draw action; when the pile cannot supply all of it, they take
  what there is — possibly nothing — and the turn ends.
- **Ace**: the next player loses their turn (the pass action), or answers
  with an ace of their own (rule `prsi.rules.aces`).
- **Svršek (Q)**: wild; play it on anything and name the suit that follows
  (rule `prsi.rules.queens`).

## The end

- The match ends the moment someone's hand is empty; they win. A 7 that
  empties your hand wins before anyone has to draw.
- **Blocked table** (rule `prsi.rules.end.blocked`): if every seat in turn
  passes with nothing to draw — no card played in between, and nobody paying
  a 7 or an ace on that turn — nobody can ever move again. The match ends:
  the fewest cards in hand wins, and players tied on the fewest share the
  win. A turn spent paying a 7's debt or an ace's skip does not count towards
  that circle, because the player has not yet had a free turn on that pile.

## Options

| Option | Values | Default |
|---|---|---|
| Cards dealt | 4, 5, 6 (six players: at most 5) | 4 |
| Open discard pile | on / off | off |
