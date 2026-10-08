# Mariáš (volený) — the pinned rules

This is step 0 of [marias-plan.md](marias-plan.md). It is the ruleset the engine will play and
the rules screen will print, **written down before any engine code**, for sign-off.

- **In code:** `server/internal/marias`:
  - `rules.go` states these sentences by id.
  - `descriptor.go` declares the options.
  - `settle.go` holds the payment table.
  - `marias_test.go` checks the table against the source.
- **Wording:** the English column below becomes the `en.ts` wording in step 3. The Czech terms
  stay Czech in every locale, as "Betl" and "Durch" already do in German and Austrian games.

## Sources

- **[ČSM]** Český svaz mariáše, *Pravidla dvacetihaléřového bodovaného voleného mariáše*, in
  force from 8.5.2007 ([talon.cz PDF](https://www.talon.cz/pravidla/mari%C3%A1%C5%A1_pravidla_volen%C3%BD.pdf)).
  This is the association's own written rules. It is primary wherever it speaks.
- **[Pagat]** John McLeod, [Mariáš](https://www.pagat.com/marriage/marias.html). It covers what
  the ČSM sheet assumes every player already knows: the rank orders, what must be played, how
  marriages work, and the doubling ladder of a hundred.

## The rules, by id

Ids are `marias.rules.<id>`. A 🔧 marks a sentence that changes or disappears with an option.

### Goal and cards

| Id | Rule (English wording) | Source |
|---|---|---|
| `goal` | Each deal, one player (the declarer) plays a game alone against the other two. Win units from the others; after the last deal, the most units wins. | ČSM A, Pagat |
| `deck` | 32 German-suited cards: červené (hearts), kule (bells), žaludy (acorns), zelené (leaves); 7, 8, 9, 10, spodek, svršek, král, eso. | Pagat |
| `order.trump` | In hra, sedma and sto: eso, 10, král, svršek, spodek, 9, 8, 7. | Pagat |
| `order.plain` | In betl and durch: eso, král, svršek, spodek, 10, 9, 8, 7. | Pagat |
| `points` | Each eso and 10 is worth {sharp}, and the last trick {last}: {total} in all. | Pagat |

### The deal

| Id | Rule | Source |
|---|---|---|
| `deal.roles` | The player left of the dealer chooses. They get 7 cards, then 5 more after choosing trumps (12 in all). The others get 10 each. The chooser changes every deal. | ČSM B/4, Pagat |
| `deal.turns` | Everything goes clockwise: choosing, answering, doubling and play. | ČSM B/11 |
| `deal.trump` | The chooser names trumps by choosing one of their first seven cards. | ČSM B/6 |
| `deal.zLidu` 🔧 | …or *z lidu*: by taking a card unseen from the five they have not looked at. | ČSM B/6 |
| `deal.talon` | The declarer discards two cards to the talon. In hra, sedma and sto neither can be an eso or a 10. The talon's cards count as the declarer's. | ČSM B/7-8, C/13 |

### The games

| Id | Rule | Source |
|---|---|---|
| `game.hra` | **Hra:** take more points than both defenders together, counting marriages. A tie is a loss. | Pagat |
| `game.sedma` | **Sedma:** win the last trick with the 7 of trumps. Announced with hra or sto, and settled separately from it. | ČSM A, B/13 |
| `game.sto` | **Sto:** reach {n} with card points and **the first marriage announced**. It replaces hra, and may be announced with sedma (*sto a sedm*). | ČSM A, ČSM general VII/1 |
| `game.betl` | **Betl:** take no trick. No trumps. | ČSM A, Pagat |
| `game.durch` | **Durch:** take every trick. No trumps. | ČSM A, Pagat |

### Announcing and doubling

| Id | Rule | Source |
|---|---|---|
| `bid.announce` | The chooser picks up their 12 cards, discards two, and announces a game. They may not pass. | Pagat |
| `bid.takeover` | In clockwise turn, each other player says *dobrá* (good) or *špatná* (bad) and announces a higher game: betl over hra, sedma or sto; durch over any of those or over betl. They take the talon, discard two, and become the declarer. Nothing goes over durch. | Pagat, ČSM B/14 |
| `bid.proti` | A defender may announce *sedma proti* (they hold the 7 of trumps and will win the last trick with it) or *sto proti* (the defenders will reach a hundred). | ČSM B/20 |
| `bid.flek` | Each part can be doubled on its own. A defender says flek, the declarer re, and so on alternately: {names}. The turn goes clockwise until a full round passes without a doubling. | ČSM B/11, Pagat |
| `bid.flekLimit` 🔧 | A part can be doubled at most {n} times. | option |

### Play

| Id | Rule | Source |
|---|---|---|
| `play.lead` | The declarer leads to the first trick. Whoever wins a trick leads to the next. | Pagat |
| `play.follow` | Follow suit if you can, and beat the best card of that suit in the trick if you can. Once the trick has been trumped, any card of the led suit will do. | Pagat, ČSM C/1-2 |
| `play.trump` | If you cannot follow, play a trump, and beat any trump already in the trick if you can. With neither, play anything. | Pagat |
| `play.seven` | Whoever announced a sedma keeps the 7 of trumps for the last trick unless no other card is legal. | ČSM B/13, Pagat |
| `play.marriage` | Play the svršek while holding the král of the same suit to announce a marriage: {plain}, or {trump} in trumps. No marriages in betl or durch. | Pagat |
| `play.earlyEnd` | Betl ends at the declarer's first trick, and durch at their first trick lost. | our simplification (see below) |

### Scoring

| Id | Rule | Source |
|---|---|---|
| `score.parts` | The game and each sedma are settled separately, between the declarer and each defender. | ČSM, Pagat |
| `score.tariff` 🔧 | Hra {hra}, sedma {sedma}, sto {sto}, betl {betl}, durch {durch} units. | ČSM A / pub |
| `score.sto` | An announced sto made pays the sto tariff, and the tariff again for every ten points past a hundred. Failed, it pays the tariff for every ten points short and for every ten points of the other side's marriages. | ČSM general V/6 |
| `score.quietSto` | A side that reaches a hundred in hra without announcing it (*tiché sto*), counting every marriage, doubles the game, and is paid the doubled game again for every ten points past. | ČSM A, ČSM general V/7 |
| `score.quietSeven` | An unannounced 7 of trumps that wins the last trick (*tichá sedma*) is worth {n}. So is one beaten in the last trick (*zabitá*), which is paid to the other side. | ČSM A, Pagat |
| `score.flek` | Each doubling doubles that part. | ČSM, Pagat |
| `score.red` 🔧 | With hearts as trumps, every payment is doubled. | ČSM A |
| `score.limit` | No deal pays more than {n} between the declarer and any one defender. | ČSM general V/9 |
| `four.sitOut` | With four players, the dealer sits each deal out and neither pays nor is paid; the player on their left chooses. | pub custom (čtyřhranný) |
| `four.deals` | With four players, the match is rounded up to a multiple of four deals, so everyone sits out equally often. | ours |
| `end` 🔧 | The match is {n} deals. | option |

## Options

| Option | Choices | Default | Why that default |
|---|---|---|---|
| `deals` | 9, 12, 18, 24 | 12 | Four turns each as chooser |
| `tariff` | Association (betl 15, durch 30), Pub (betl 5, durch 10) | Association | ČSM A. Pagat's table is the pub one |
| `redDoubles` | on, off | on | ČSM A |
| `flekLimit` | no limit, flek only, up to re, up to boty | no limit | Pagat: "in theory without limit". ČSM caps payment, not doublings |
| `zLidu` | allowed, not allowed | allowed | ČSM B/6 |
| `showCardPoints` | hidden, shown | hidden | At a real table nobody counts aloud |
| `botSkill`, `pauseBetweenRounds` | stock | medium, pause | |

## Where this differs from the association's sheet (for sign-off)

1. **No renonc.** The engine refuses an illegal card instead of penalising it afterwards, so
   section C (renonc) and the fixed penalties (paušál) do not exist. Nor does the 500× payment
   limit, which exists to cap renonc settlements.
2. **One doubling counts for both defenders.** This is the pub reading (Pagat). ČSM B/15
   settles a doubling only with the defender who said it, which is tournament bookkeeping. It
   could become an option later.
3. **"Flekovaná hra se bez re nehraje" (B/19)** is left out. In a tournament an un-re'd double
   ends the game; online it would just be a trap.
4. **Four at the table (čtyřhranný) is played with the dealer sitting out.** The dealer is dealt
   nothing, is never on turn, and takes no part in that deal's payments. The chooser still moves
   one seat clockwise every deal, so the sitter rotates with it. The deals option is rounded up to
   a multiple of four at a table of four (9→12, 18→20). The sitter sees only what a spectator
   sees.
5. **No open (ložené) betl or durch**, and no laid-down game (B/16-18). Play simply runs out.
6. **Betl and durch end early** once they are decided (`play.earlyEnd`). The result is the same,
   and nobody has to play out nine dead tricks.
7. **Marriages are announced automatically** when a svršek is played while holding its král.
   Playing the král first still forgoes the marriage, as at a real table.

## Settled questions

1. **What a failed announced sto pays.** First settled (2026-09-29) as the sto tariff flat. The
   same day this was **superseded** by the association's general rules (*Obecná pravidla*,
   V/6-7), found while drafting Licitovaný. Sto now scales linearly:
   - Made: the tariff, plus the tariff again for every 10 points past 100.
   - Failed: the tariff for every 10 points short, plus the tariff for every 10 points of the
     other side's marriages.

   A quiet hundred doubles hra and adds the doubled hra for every 10 points past. See
   [marias-licitovany-rules.md](marias-licitovany-rules.md) §8.
2. **The defenders' marriages count in hra** whether or not the defenders take a trick.
3. **An announced sto counts the first marriage its side announced**, not the best one (ČSM).
   Every marriage counts toward a quiet hundred.
4. **No deal pays more than 500 units** between the declarer and any one defender (ČSM general
   V/9).
