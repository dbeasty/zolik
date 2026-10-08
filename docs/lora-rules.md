# Lóra — the pinned rules

The Kladno penalty-point game for four players.
- **Code:** `server/internal/lora`.
- **Names:** Game names, card names and *ťuk* stay Czech in every locale, as in Mariáš and Sedma.

## Sources

- **[Pravidla]** The unified tournament rules of the Lóra national championship, as published by
  [lora.game](https://www.lora.game/pravidla-hry-lora/) and
  [lora.cz](https://www.lora.cz/pravidla-karetni-hry-lora/), and summarised on
  [Czech Wikipedia](https://cs.wikipedia.org/wiki/L%C3%B3ra). The Kladno club Petasites keeps the
  official text.
- Pagat's [Lora](https://pagat.com/compendium/lora.html) is the Serbian and Croatian game of the
  same name, with different contracts. It is **not** the source.

## The match

| Id | Rule |
|---|---|
| `deck` | 32 German-suited cards ranked eso, král, svršek, spodek, 10, 9, 8, 7. There are no trumps. |
| `deal` | Four players, 8 cards each, dealt from the leader round. |
| `talie` | A tálie is seven games in a fixed order, all led by the same player: červené, filky, první–poslední, všechny, červený král, kvarty, desítky. Each later tálie is led by the next seat. The dealer is the seat before the leader. |
| `length` 🔧 | 1, 2 or 4 tálie. Four is the tournament *kolo*, in which everyone leads once. |
| `end` | The fewest penalty points wins. |

## The trick games

| Id | Rule |
|---|---|
| `tricks.follow` | Follow suit if able, otherwise play any card. The highest card of the led suit wins, and the winner leads next. |
| `tricks.redLead` | In červené and červený král, a red card (hearts) may be led only by a player holding no other suit. |
| `tricks.stop` | Červené, filky and červený král end as soon as their penalty cards are all taken. |
| `game.cervene` | 1 for each red card taken. |
| `game.filky` | 2 for each svršek (filek) taken. |
| `game.prpo` | 4 for the first trick and 4 for the last trick. |
| `game.vsechny` | 1 for each trick taken. |
| `game.kral` | 8 for the red král. |

## Kvarty

| Id | Rule |
|---|---|
| `kvarty.lay` | The leader starts a quart with any card. The quart is that card and the three ranks above it in its suit, or fewer when it reaches the eso. |
| `kvarty.add` | Whoever holds the next card of the quart must lay it. This continues until the quart is complete or its next card has already been laid. Whoever laid the quart's highest card leads the next one. |
| `kvarty.end` | The game ends when someone has no cards left. Each card left in hand costs 1. |

The written rules describe this as the leader laying a run and the others adding to it. Because
adding is compulsory and there is only one card to add at each step, the outcome is the same
however the cards are split between turns. The app therefore lays the whole quart in one move,
and the leader's only decision is which card to start from.

## Desítky

| Id | Rule |
|---|---|
| `desitky.lay` | On your turn, lay as many cards as you like, one at a time. A 10 opens its suit's row. A row grows one card at a time down to the 7 and up to the eso. |
| `desitky.tuk` | If you can lay a card, you must lay at least one. If you cannot, you say *ťuk* and take 1 point. The turn passes on its own once you have nothing more you could lay. |
| `desitky.end` | The game ends when someone has no cards left. Each card left in hand costs 1, plus 1 for each ťuk. |

## Maturita 🔧

| Id | Rule |
|---|---|
| `maturita` | After the seven games, the tálie's leader plays a maturita: one more game that they must finish without a single penalty point. |
| `maturita.choose` | The maturant chooses the game after seeing their cards. Any game is allowed except první–poslední, which is open only on the third attempt. Only the maturant is scored. |
| `maturita.fail` | The first penalty point ends the attempt and costs the maturant 8. The maturant then tries again with a new deal. A third failure ends the match, and the maturant finishes last whatever their total. |

Two cases count as failure in the laying-out games:
- a ťuk;
- another player going out first.

The `maturita` option turns the whole maturita off, for a shorter game of seven deals per tálie.

## What the app leaves out

1. **Renonc.** The app refuses illegal plays instead of penalising them, so there is nothing to
   punish.
2. **Čistá stovka and harakiri.** These are tournament side rules. Players level on points share
   their place.
3. **The kvarty wild card and royal quart.** These are special cases for a leader with nothing
   below a svršek, and the written rules don't settle them. In the app, every quart follows the
   same rule.
4. **Direction of play** is the app's seat order, as in every other game here.
5. **Showing the cards after the red král falls in the first trick** (the "first trick"
   agreement) is not played.

## Options

| Option | Choices | Default |
|---|---|---|
| `talie` | 1, 2, 4 | 4 |
| `maturita` | Not played, Played | Played |
| `botSkill`, `pauseBetweenRounds` | stock | medium, pause |

## The bot

**Easy** plays a random legal card. In desítky it stops laying cards at random.

**Medium** plays by rules of thumb:
- **Trick games:** it ducks with the highest card that still loses. When it can't follow suit,
  it throws its most dangerous card. It leads low from short suits. As a defender in a maturita
  it feeds penalty cards to a maturant who is taking the trick.
- **Kvarty:** it starts the quart that lays the most of its own cards and leaves it on lead.
- **Desítky:** it lays the cards that free its own next card, and holds back the ones that would
  free someone else's.

**Hard** sees only what is public. It re-deals the cards it cannot see in a way that respects
every void the play has shown, sorting the pool before the shuffle so the real deal cannot leak
in. It plays each choice out to the end of the deal with medium in every seat, then takes the
best choice.

**The maturita choice:** both medium (8 samples) and hard (24 samples) choose their game the same
way. They play each candidate game out over sampled deals and pick the one they came through
clean most often.

Measured with `gamebench` (duplicate pairs, one tálie with its maturita):
- medium beats easy by +14.2 points a match over 200 seeds;
- hard beats medium by +7.5 over 100 seeds.

`botcost` puts hard at about 3 ms a decision (1.1 ms of CPU), with a botgov entry.
