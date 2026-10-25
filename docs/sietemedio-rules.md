# Siete y Medio — the rules this module plays

*Siete y medio* ("seven and a half") is the Spanish ancestor of blackjack, and
it is still the card game played round a Mexican kitchen table at Christmas and
at a *cantina* the rest of the year. It is played with the forty-card Spanish
pack, the target is 7½ rather than 21, and — the part that makes it a different
game rather than blackjack with smaller numbers — **the bank is a seat at the
table, not the house.** One of the players deals, covers everybody's stake, and
plays last; the bank is won and lost during the match.

Sources: the Spanish Wikipedia article [Siete y
media](https://es.wikipedia.org/wiki/Siete_y_media),
[ludoteka.com](https://www.ludoteka.com/clasika/siete-y-medio.html),
[asisejuega.com](https://asisejuega.com/juegos-de-cartas/7-y-medio/) and
[casinofun88mx.com](https://www.casinofun88mx.com/como-se-juega-7-y-medio-en-cartas/).
They agree on the pack, the card values, the target, the bank playing last and
a busted player losing at once. They disagree on exactly three things — who
takes a tie, whether 7½ pays a bonus, and how the bank moves — so those three
are table options here rather than constants, which is this codebase's standing
rule for a house rule.

## The pack

Forty cards: the ace to the seven, and the three figures, in each of the four
Spanish suits — oros (coins), copas (cups), espadas (swords) and bastos
(clubs). The eights, nines and tens are not in the pack, which is why a
Mexican table that only owns a French deck plays by throwing those three ranks
out.

Card codes are the ordinary French ones this server uses everywhere, and the
suits line up with the pack the Spanish suits became: `D` → oros, `H` → copas,
`S` → espadas, `C` → bastos; `J` → sota, `Q` → caballo, `K` → rey. So "KD" is
the rey de oros, and nothing in the engine or on the wire had to learn a new
notation. The descriptor says `Deck: spanish` and the client draws Spanish
faces for it, exactly as it already does for the German-suited pack.

**Values.** The ace counts one, the two to the seven count their number, and
each of the three figures counts **half a point**. Every total in this module
is therefore held in *halves* — 7½ is fifteen — because half-points in integer
arithmetic is the one rounding error that would silently mis-settle a round.

Not implemented, on purpose: the *rey de oros* as a wild card. It turns up in
plenty of searches, but it belongs to the Italian *sette e mezzo*, where the
king of coins is *la matta*; the Spanish and Mexican game has no wild and the
sources that discuss comodines in the Spanish pack say so explicitly.

## A round

1. **Stakes.** Every seat except the banker puts a stake up, all at once. It
   must be at least the table minimum, and no more than the bank can answer
   for — *la banca responde hasta donde cubre*. A seat the bank cannot cover a
   minimum stake for sits the round out rather than being shown a control it
   cannot use.
2. **One card each**, the banker included, dealt face down.
3. **The players**, in turn, starting to the banker's left: ask for another
   card (*pedir*) or stand on what you have (*plantarse*). Cards after the
   first are dealt face up, so the table can see what everyone is drawing to
   but not what they started from. Going over 7½ (*pasarse*) loses the stake
   at once and the hand is turned over — it has lost even if the bank goes
   over too afterwards, which is the bank's whole edge. Making exactly 7½
   stands automatically: there is no card left that does not bust it.
4. **The banker** turns their own card and plays last, against the standings
   they can see.
5. **Settlement.** A seat that went over has already lost. Otherwise, if the
   bank went over it pays everybody still standing; if not, the higher total
   wins and a tie goes to whichever side this table's option says. Exactly 7½
   pays double where the table pays a bonus for it.
6. **The bank moves**, or does not, by the table's option.

A match is a fixed number of rounds and the most chips at the end wins it — the
same way every chip game on this server ends.

## The table options

| Option | Choices | Why it is an option |
|---|---|---|
| `ties` | the bank, or a push | Sources split almost evenly. Ludoteka and tiendadelpoker give ties to the bank; a house ruleset gives them to the player. |
| `sevenAndAHalfPays` | double, or even money | Wikipedia and Diario Popular both say a 7½ collects double. Other write-ups do not mention a bonus at all. |
| `bank` | passes on 7½, goes round the table, or stays put | The classic rule is that a player who makes 7½ takes the bank. Plenty of tables simply pass the deal along each round instead. |
| `firstCardDown` | down, or face up | A real variation, and the only hidden information in the game: with it off, every card is on show and the game is pure arithmetic. |
| `minStake`, `startingStack`, `rounds` | | The usual table furniture. |

Three shipped variations set them to tables somebody actually deals:

- **Mexicana** — the standard game. Opening card down, ties to the bank, 7½
  pays double and takes the bank.
- **Cantina** — a friendly table: the deal goes round every round and a tie is
  a push, so the bank's edge is shared out rather than owned.
- **Abierta** — everything face up and the deal going round, ties to the bank.
  No hidden information at all, which makes it the table to learn the game at.

## Where it sits next to the games already here

**Oko bere** (`server/internal/okobere`) is the close relative, not blackjack:
the Czech pub twenty-one also banks round the table, also limits a stake to
what the bank can cover, and also gives ties to the bank. Anyone changing this
module should read that one. The two were kept apart rather than made
variations of each other for the reason Prší and Sedma are kept apart — they
are cousins, not one game under two names — and the differences are not
cosmetic:

| | Oko bere | Siete y Medio |
|---|---|---|
| Pack | 32 German-suited | 40 Spanish-suited |
| Target | 21 | 7½ |
| Card values | whole numbers, eso 11 | halves; the three figures are ½ |
| Hidden | the bank's hand until it plays | every seat's own first card |
| Bank moves | after a set number of rounds, or when broken | to whoever makes exactly 7½, or round the table |
| The special hand | two aces (*oko*), which wins at once | 7½, which pays double and takes the bank |

**Blackjack** is the one it is most often compared to and the one it is least
like in code: there the opponent is the house, a zone nobody owns played by a
rule with no choices in it. Here:

- The banker is on the board like anybody else, with a stack, a hand and a
  standing. There is no `Zone.Dealer` in this module at all.
- The bank is **transferable**, so "who is the opponent" is per-round state
  rather than a property of the match.
- Hidden information runs the ordinary way round — each seat's own first card
  is theirs, hidden from the rest — rather than blackjack's single secret the
  whole table is kept out of. The banker's card is hidden from everyone until
  the banker's turn, which is the one piece of ceremony in the game.
- Every stake is answered out of another player's pocket, so the stake range a
  seat is offered depends on what the bank can still cover. That is the only
  rule in this engine where one seat's legal moves depend on another's stack.

## Verification

| Level | What it proves | Where |
|---|---|---|
| Unit | card values and the halves arithmetic, the forty-card pack, 7½ standing itself, a bust losing to a bank that then busts, ties both ways, the double bonus, the bank passing all three ways, the bank's cover limiting a stake | `sietemedio/engine_test.go` |
| Unit | offers never disagree with `Apply`, awaited seats equal the enabled offers, no offer names a card, every refusal's rule id resolves | `sietemedio/agreement_test.go`, `ruleindex_test.go` |
| Unit | whole matches played from the offer list alone, across all three variations; chips conserved against the round table; no seat ever left with nothing to press | `sietemedio/conformance_test.go` |
| Unit | the banker's hole card is not in a player's view, and a player's own card is not in another's | `sietemedio/view_test.go` |
| Unit | the three bot skills ordered by what they actually win, over a shared deal | `sietemedio/bot_test.go` |
| Contract | the terms every hosted game is held to | `module/allmodules_test.go` |
| E2E | the unchanged generic shell renders and plays it in a browser, and the lobby lists it | `e2e/tests/sietemedio.spec.ts` |
