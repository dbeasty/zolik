# Poker

Three classic poker games, played for chips: Texas Hold'em, Pot-Limit Omaha
and Five-Card Draw. Chips are never real money. The app marks poker "18+",
and on phones it's hidden from players under 18.

- **Players:** 2–9 (Five-Card Draw 2–6)
- **Variations:** Texas Hold'em, Pot-Limit Omaha, Five-Card Draw
- **Bots:** Easy, Medium, Hard and AI (a trained model)
- **In-app rules:** the game's page › **Rules ›**

## Objective

Win chips by having the best hand at showdown, or by being the only player
left in the hand. Your hand is always the best five cards you can make.

## How a hand works

Every seat starts with the same stack (1000 by default). The small and big
blinds (10 and 20 by default) are posted before the cards are dealt.

**Texas Hold'em.** You get two cards face down. There are four betting
rounds: before the flop, and after the flop (three shared cards), the turn
(a fourth) and the river (a fifth). Make the best five-card hand from your
two cards and the five on the board. Betting is **no-limit**: any bet may be
for any amount up to your whole stack.

**Pot-Limit Omaha.** You get four cards face down, and the betting rounds are
the same as Hold'em. Your hand must use **exactly two** of your four cards
and **exactly three** from the board. Betting is **pot-limit**: a raise can
be at most the size of the pot after you call.

**Five-Card Draw.** You get five cards face down and there's no board. There
are two betting rounds: one on the cards as dealt, and one after the draw. In
the draw, each player still in the hand, starting left of the dealer, throws
away one to three cards and is dealt as many new ones, or stands pat and
keeps all five. Everyone is told how many cards each player drew, but never
which ones.

### Betting

- You may **check** only when nothing is owed. Otherwise **call**, **raise**
  or **fold**.
- A raise has to be at least as big as the last one.
- You can never put in more than your stack, but going all in is always
  allowed, even for less than a full raise (in Omaha, within the pot limit).
- Once you fold, you're out until the next hand.
- To raise, choose the amount with the step buttons or type it, then press
  the control. If you try something that isn't allowed, the **why** sheet
  says what is ("Raise to at least 120.").

### Showdown

Players still in the hand reveal their cards, and the best five-card hand
wins the pot. Which hands are turned face up depends on **Show at showdown**.
You may always turn your own hand face up until you agree to go on. Play
stops after every hand so everyone can see the cards.

## Tournament or cash table

- **Tournament** (default): played to the last chip, and nobody leaves with
  theirs. A player who is away is sat out, and their blinds are still posted.
  The match plays until one seat holds every chip, or until the **Hands**
  limit, when the most chips wins.
- **Cash table**: you may get up whenever you like, and your chips go with
  you as your result. You may sit down between hands with the starting
  chips. A player whose connection has dropped for longer than
  **Leave if away for** leaves the table with their chips.

## Options

| Option | Choices | Default | What it does |
| --- | --- | --- | --- |
| **Table** | Tournament / Cash table | Tournament | See above. |
| **Show at showdown** | Everyone who was called / The winner only | Everyone who was called | Whose cards are turned face up when a hand is called. A player may always show their own. |
| **Starting chips** | 200 (short) / 1000 / 5000 | 1000 | Chips each seat begins with. |
| **Big blind** | 10 / 20 / 50 | 20 | The small blind is half of it. |
| **Hands** | Until one seat is left / 5 / 10 / 30 | Until one seat is left | How many hands to play. |
| **Leave if away for** | 2 / 5 / 10 minutes / Never | 5 minutes | At a cash table, how long a disconnected player is sat out before leaving with their chips. |

Also available: Opponents, Hints.

## Tips

- Position matters: acting last gives you more information.
- In Omaha, remember the exactly-two rule. Four hearts in your hand don't make
  a flush unless the board has three hearts.
- A short starting stack (200) makes for a fast, all-in-heavy game.
