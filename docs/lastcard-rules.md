# Last Card — rules as built

The rules `server/internal/lastcard` implements, for reference. The players'
copy is the module's own `Rules()`, rendered by the rules screen per table, so
it only ever states what that table plays. This page states everything, with
each option marked. The plan and its reasoning are in
[lastcard-plan.md](lastcard-plan.md).

Last Card belongs to the Crazy Eights family, and so does Prší. It has its own
name, pack and art. No commercial game's name or design is used anywhere, and
`brand_test.go` fails the build if that name appears.

## The pack: 108 cards

- Four colours: **coral ●, teal ◆, violet ▲ and amber ■**. Each colour has its
  own shape, so you can tell colours apart without seeing colour.
- In each colour:
  - one 0;
  - two each of 1–9;
  - two each of **Skip**, **Reverse** and **Draw Two**.
- Four **Wilds** and four **Wild Draw Fours**.
- Card codes:
  - `<colour>-<face>` for coloured cards, e.g. `C-7`, `T-S`, `V-R`, `A-D`;
  - `W` and `W4` for the wilds.

## A deal

1. Deal 7 each (option: 5, 7 or 10) and turn one card face up.
   - An opening wild is buried and the next card turned.
   - An opening action card takes effect as if the dealer had played it.
2. The first player moves one seat round with each deal.
3. On your turn, play a card that matches the top card by colour, number or
   symbol, or play a Wild. A wild on top is matched by the colour its player
   named.
4. If you don't play, draw one. If it fits you may play it at once, or keep it
   ("Keep it"); either way your turn ends.
   - *Draw until you can play* (option, off by default): keep drawing until a
     card fits.
5. Action cards:
   - **Skip**: the next player misses a turn.
   - **Reverse**: play changes direction. With two players it acts as a Skip.
   - **Draw Two**: the next player draws two and misses a turn.
   - **Wild**: name the colour that follows.
   - **Wild Draw Four**: name the colour; the next player draws four and misses
     a turn. Honest only when you hold no card of the colour in play (see the
     challenge).
6. Empty your hand to win the deal. A draw card played as your last card still
   makes the next player draw.
7. If nobody can draw and nobody can play, the deal ends on the fewest cards.

## "Last card!" (option, on by default)

- Holding two cards on your turn, say "Last card!" (the button) before you play
  the second-to-last one.
- Forget, and the next player to move may **catch** you before making any other
  move: you draw two.
- Real tables let anyone catch you at any time. This version lets the next mover
  do it, on their own turn, so the engine never needs a move out of turn.
- A player who called shows a "Last card!" badge on their seat. One who forgot
  does not, which is the tell.

## The Wild Draw Four challenge (option, on by default)

- With the challenge on, a Wild Draw Four may be played at any time, so it can
  be a bluff. Its victim must **challenge** or **accept** before doing anything
  else.
- **Accept**: draw four, miss the turn.
- **Challenge**, and it was a bluff (its player held the colour in play): the
  bluffer draws four, and the challenger plays on normally.
- **Challenge**, and it was honest: the challenger draws six and misses the turn.
- Either way, the challenger alone is shown the hand the card came from.
- With the challenge off, the engine simply refuses a Wild Draw Four while you
  hold the colour in play.

## House rules (options, all off by default)

### Stacking draw cards

| Setting | What may answer a draw card |
|---|---|
| Off | Nothing |
| Draw Two on Draw Two | A Draw Two, on a Draw Two only |
| Any draw card | A Draw Two on a Draw Two, and a Wild Draw Four on either. Never a Draw Two on a Wild Draw Four |

- A stacked card passes the whole total to the next player, who stacks again or
  takes it all ("Take them") and misses the turn.
- A Wild Draw Four that *starts* a stack can still be challenged.
- One stacked on a pending draw is an answer, never a bluff.

### Sevens and zeros

- **7**: swap hands with whoever holds the fewest cards. On a tie, the nearest
  in the direction of play.
  - This is made automatic because a choice of player isn't something an offer
    can carry today. It is also the swap a player would almost always choose.
- **0**: every hand passes to the next player in the direction of play.

## Scoring (options: target one deal, 200 or 500 — default 500; scoring method — default winner takes)

| Card left in a hand | Points |
|---|---|
| Number cards | Face value |
| Skip, Reverse, Draw Two | 20 |
| Wild, Wild Draw Four | 50 |

There are two scoring methods, chosen per table with the **Scoring** option.

**Winner takes the points (default).**
- The deal's winner scores every card left in the other hands. Everyone else
  scores 0 for that deal.
- Totals only go up. The first player to reach the target wins.

**Lowest total wins.**
- Each player who didn't go out is charged the points left in their **own**
  hand. The player who went out is charged nothing.
- When anyone's total reaches the target, the **lowest** total wins. If the
  deal's winner is tied for lowest, they take it, then the earliest seat.
- Here holding fewer, cheaper cards when someone goes out matters. Medium and
  Hard bots shed their costliest card first once another player is down to two
  cards (+5 points over par in three-seat sweeps).

Either way:
- A draw card played as the last card still makes the next player draw first,
  and those cards count.
- Between deals the table pauses on a score sheet until everyone continues
  (option). Each line is broken down into number cards, action cards and wilds.
- In a single deal (target "One deal") the standings are cards left, fewest
  first.

## What the table says

- Every move is narrated in one line, with what it did, in the status box
  between the pile and the hand. For example: "Bo played ● Draw Two — Cy
  draws 2 and misses a turn", "… play now goes anticlockwise ↺", "… the colour
  is now ▲ Violet". The box lists every move since your own last one, then
  anything the table is waiting on you for.
- The box keeps one height (two lines on short windows), so the hand never
  moves under a finger.
- A wild on the pile shows the colour it named on the card itself: the frame,
  the named quarter of its wheel, and that colour's shape at the hub.
- Playing a wild, whether by dragging, tapping or the Play button, asks which
  colour first. The colour you hold most of is suggested.

## Bots

| Skill | Plays |
|---|---|
| Easy | Spends wilds first. Forgets "Last card!" one time in three |
| Medium | Keeps its wilds. Calls and catches |
| Hard | Plays Skip, Reverse and Draw Two first. Bluffs a Wild Draw Four only against a nearly-out player. Challenges a hand of 8+ cards. Swaps down with a 7 |

- Every knob was priced by a sweep; the numbers are in `bot.go`.
- `TestBotLadderIsOrdered` holds Easy < Medium < Hard.
- `go run ./cmd/gamebench -game lastcard …` benches Hard against itself with one
  judgement removed.
