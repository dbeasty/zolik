# Mariáš — implementation plan (Bridge parked)

- **Deliverable:** `server/internal/marias`, implementing `module.GameModule` like the seven
  games before it. It ships with two variations, **Volený** and **Licitovaný**, for three players.
  It needs three game-agnostic pieces around it:
  - a small `internal/tricks` package;
  - a trick area in the shell that places each card toward the seat that played it;
  - German-suited card faces ("mariášky"). Prší gets these too.
- **Later, separately:** Contract Bridge (Part II). It is a sibling module that reuses the trick
  package and the trick area. It is not a variation of Mariáš.
- **Non-goals, this plan:**
  - Křížový mariáš (four players in partnerships);
  - "hra v barvě" house scoring variants beyond the tariff option;
  - tournaments and ELO;
  - offline play.

  Each is noted in §9.

---

## 0. Is Mariáš close to Bridge?

**No. They are cousins, not siblings.** Both are trick-taking games with trumps, and both have an
auction, which is where the resemblance usually gets noticed. The games underneath are
different:

| | Mariáš | Bridge |
|---|---|---|
| Family | Ace–Ten point game (like Skat or Schnapsen) | Plain-trick game (like Whist) |
| Players | 3. Each deal is **one declarer against two** defenders, and the sides change every deal | 4 in **fixed partnerships** |
| Deck | 32 cards, German suits | 52 cards, French suits |
| What wins | Card points (A and 10 are worth 10 each, the last trick 10) plus marriages (20, or 40 in trumps); or a special contract | The number of tricks against the contract |
| Play obligation | Follow suit **and beat the card if you can**; if void, **must trump** | Follow suit; otherwise play anything |
| Hidden stock | Talon: the declarer discards 2 | None |
| Special contracts | Sedma, Sto (kilo), Betl, Durch, and the defenders' "proti" versions | None; levels 1–7 × 5 strains |
| Doubling | Flek chain (flek, re, tutti, boty, kalhoty…) on each part of the contract | Double and redouble only |
| Distinctive feature | Marriages announced during play | **Dummy**: a hand laid face up and played by the declarer |
| Bot difficulty | Moderate: 30 cards, 10 tricks | Hard: bidding needs a *system*, and play needs a real double-dummy solver |

What they genuinely share, and what this plan builds once:

1. **Trick mechanics:** the led suit, the trump, the winner of a trick, and legal cards under a
   policy. In Mariáš the policy is "follow, overtake, trump"; in Bridge it is "follow". This
   becomes `internal/tricks` (§3.1).
2. **A trick area on the table** with one card per seat position (§3.2). No game has needed this
   so far.
3. **An auction made of offers.** This already works: Hold'em's buttons, `Facts` and `ParamSpec`
   (`module/protocol.go:449-576`, `OfferBar.tsx:188-300`).
4. **A card-play bot by sampling:** deal the unseen cards consistently with what is known, then
   solve each deal open-handed. Mariáš's small tree is the right place to build it.

**Recommendation:** build Mariáš first. Build these as their own packages and components from
day one:
- the trick package;
- the trick area;
- the sampling bot.

Plan Bridge as its own module once Mariáš has shipped. Bridge adds work that Mariáš never touches:
- choosing cards from a hand the viewer does not own (dummy);
- a display for the auction history;
- a bidding bot;
- the question of how a partnership win is recorded in stats.

That puts Bridge at roughly Canasta's size, not "Mariáš plus a bit" (§8).

## 1. Rules implemented

**Pinned in [marias-rules.md](marias-rules.md) (step 0).** That document is the authority: every
rule by id, its English wording, its source, the options, the deviations and the open questions.
It follows the Český svaz mariáše's written rules for volený mariáš (8.5.2007), with Pagat for
what that sheet takes for granted. This section is only the summary.

**Deck.** There are 32 cards in four suits:
- červené (hearts);
- kule (bells);
- žaludy (acorns);
- zelené (leaves).

Ranks run 7, 8, 9, 10, spodek (unter), svršek (ober), král (king), eso (ace). Inside the server
these use the codes the rest of the code already uses (`prsi/state.go:38-42`):

| Code | Card | Code | Suit |
|---|---|---|---|
| `T` | 10 | `H` | červené |
| `J` | spodek | `D` | kule |
| `Q` | svršek | `C` | žaludy |
| `K` | král | `S` | zelené |

This is the standard French↔German correspondence. Keeping the codes means `isCardCode`, the
flight animation and the TUI parser keep working. Only the **faces** change (§3.4).

**In short:**
- **Deal:** the chooser gets 7 cards, names trumps (or takes them *z lidu*), then gets 5 more
  and discards two to the talon. There is no eso or 10 in the talon in trump games.
- **Games:** hra 1, sedma 2, sto 4, betl 15, durch 30 units (the association's table; the pub
  table is betl 5, durch 10). Sedma is settled separately from the game.
- **Takeover:** the others may say *špatná* and take over with betl or durch. This is part of
  Volený too, not only Licitovaný (ČSM B/14).
- **Doubling:** each part can be doubled separately (flek, re, tutti, boty, kalhoty…), and
  defenders may announce sedma or sto *proti*.
- **Play:** follow suit and beat; if you can't follow, trump and overtrump (`tricks.FollowBeatTrump`).
- **Hundreds:** a quiet hundred doubles hra, then doubles again for every ten past it. A quiet
  seven is worth half an announced one. Hearts double every payment.
- **Match:** a fixed number of deals. Standings are cumulative units, shown on the scoreboard
  with `Standing.Shown`.

### Options

These are declared in `server/internal/marias/descriptor.go`; the reasons for each default are
in marias-rules.md.

| Option | Choices | Default |
|---|---|---|
| `deals` | 9, 12, 18, 24 | 12 |
| `tariff` | Association (betl 15, durch 30), Pub (betl 5, durch 10) | Association |
| `redDoubles` | on, off | on |
| `flekLimit` | no limit, flek only, up to re, up to boty | no limit |
| `zLidu` | allowed, not allowed | allowed |
| `showCardPoints` | hidden, shown | hidden |
| `botSkill`, pause | stock | medium, pause |

### Variations

- **Volený** is the only variation until step 4. It is as pinned above.
- **Licitovaný** (step 4) is the association's auction game. Players bid for the right to
  declare, with the talon, and its own tariff. It gets its own pinned rules before it is built.

## 2. What the runtime already gives us

This is from an audit of `main` at `7305d18`.

| Need | Answer today |
|---|---|
| A 32-card French-coded deck | **Yes:** Prší (`prsi/engine.go:18-26`). |
| Three different declarers across the deal | **Yes.** Sides are per deal. `Seat.Side` is optional, and `Finished` returns a list (`module/protocol.go:774-788`). |
| Buttons for Hra / Betl / Durch / Flek / Dobrý / Špatný | **Yes.** Offers with `LabelKey` and `Facts`, as in Hold'em (`holdem/offers.go:82-118`). They must stay distinguishable (`allmodules_test.go:243`). |
| Picking one card as the trump indicator; picking two for the talon | **Yes.** `Source.Cards` with `MinCards`/`MaxCards` (`protocol.go:369-421`). |
| Several seats deciding at once | **Yes**, as blackjack does. It is not needed here: the auction is sequential. |
| A trick area with each card toward its player | **No.** `Group` and `CardView` carry no seat (`protocol.go:147-176`). The board is a single column (`BoardLayout.tsx:141-192`). §3.2. |
| German-suited faces | **No.** `cards.ts:23-44` and every face set (`VectorFace`, `DeluxeFace`) are French. So is the TUI (`client-tui/internal/render/cards.go:163-180`). §3.4. |
| A bot better than first-offer | Module-local only. `internal/ai` is Žolíky-specific (`ai/agent.go:3`), and there is no sampling or solver framework. §4. |
| Rules screen, refusals, message keys | **Yes, generically:** `app/rules.tsx`, `ExplainRefusal`, and `serverKeys.json`. Every key needs wording in all 24 locales (`serverKeys.test.ts:48-60`). |
| TUI | Generic (`client-tui/ui/match.go:17-28`). It needs only suit glyphs. |

## 3. Design

### 3.1 `internal/tricks` (game-agnostic)

This package is small and pure, and it has no knowledge of modules.

```go
type Order string                          // ranks high → low: "ATKQJ987", "AKQJT987"
type Policy int                            // FollowOnly | FollowBeatTrump (Mariáš)
type Trick struct { Plays []Play }         // Play{Seat int; Card string}; Plays[0] is the lead

func (t Trick) Winning(trump byte, o Order) (Play, bool)
func Legal(hand []string, t Trick, trump byte, o Order, p Policy) []string
```

The Mariáš overtaking rule has corner cases, and `Legal` owns them. The main one: a follower who
cannot beat a card that has already been trumped need not play a higher card of the led suit.
Table-driven tests cover every corner case. Bridge later uses `FollowOnly`.

### 3.2 A trick area in the shell (protocol and client)

This is the one protocol addition. Both fields are optional, so the other seven modules are
unaffected:

```go
type CardView struct { …; By string `json:"by,omitempty"` }       // seat (player id) that played it
type Zone     struct { …; Arrange string `json:"arrange,omitempty"` } // "bySeat"
```

The shell draws a shared zone with `arrange: "bySeat"` as a compass:
- the viewer's card at the bottom;
- the others at their seat's angle: with three players, left and right; with four, left, top and
  right.

The shell never learns what game it is drawing. `isTableZone` already puts a shared zone on the
table (`lib/board.ts:78-96`). Existing pieces this uses:
- Flights from seat to trick come from `lib/flights.ts` and `SeatStrip` `registerSpot`.
- A **"last trick"** peek is a second, collapsed `bySeat` zone.
- In the TUI the zone renders as `Anna: A♥  Petr: T♥  you: 7♥`.

Mariáš also shows:
- the talon: a zone with `Count` only, revealed after the deal;
- each player's won tricks: a stack with `Count` only;
- announced marriages and contract parts as Seat `Facts`.

### 3.3 Offers

| Offer ID | Verb | When | Input |
|---|---|---|---|
| `trump` | `choose_trump` | Forhont, 7 cards | `Source.Cards` = the 7, Min/Max 1 |
| `trumpZLidu` | `choose_trump` | Forhont (option) | none |
| `talon` | `discard` | Declarer, 12 cards | Min/Max 2. A and 10 are disabled in trump games (`WhyNot` gives the rule) |
| `announce.hra`, `.sedma`, `.sto`, `.betl`, `.durch` | `announce` | Declarer | One offer per legal contract, with a distinct `LabelKey` |
| `good` / `bad.betl` / `bad.durch` | `bid` | Others in turn, clockwise | none |
| `flek.<part>` | `flek` | Alternating sides | `Facts` shows the next multiplier ("×4") |
| `protiSedma`, `protiSto` | `announce` | Defenders, in the doubling round | none |
| `play` | `play_card` | The seat to play | `Source.Cards` = `tricks.Legal(...)` |
| `continue` | from `Intermission` | Between deals | stock (`module/intermission.go:39-163`) |

The files follow Prší's layout (`prsi/*.go`):
- `state.go`
- `engine.go`
- `auction.go`
- `offers.go`
- `view.go` (descriptor, view, standings, open view)
- `rounds.go`
- `rules.go`
- `ruleindex.go`
- `remedy.go`
- `bot.go`
- `bot_search.go`
- a test file for each

The module is registered in:
- `app/app.go:584`;
- `cmd/dump-keys/main.go:33-36`;
- `module/allmodules_test.go` `allModules()`;
- `gameOrder.ts`.

### 3.4 German-suited faces

- **Where the deck is named:** `ModuleDescriptor.Deck` is `"german32"`. It is a **deck** hint,
  not a skin, because the skin system may not change what a card *is*. It is honoured by every
  skin's face style.
- **Client:** a `GermanFace` component and a German `SUIT_SYMBOLS` / colour map (red = H only).
  The TUI uses a letter-based fallback (`Č K Ž Z`).
- **Art (decided, §7):** hand-drawn vector faces in the Tell pattern, done in-house. The 1837
  design is public domain, but modern scans are not, so nothing is traced from one.
- Prší opts in to the same hint.

## 4. The bot

**Easy and medium: rules of thumb.** The bot declares from hand strength:
- trump length;
- A and 10 count;
- a marriage held;
- the trump 7 for Sedma;
- low singletons for Betl.

In play it follows the textbook defaults: lead a long trump suit as declarer, smear points to
the partner's winning trick as a defender, hold the trump 7 for the last trick. Medium adds
counting of cards played.

**Hard: sampling.** The bot deals the unseen cards many times, consistent with:
- every void shown;
- the talon, which only the declarer knows;
- announced marriages.

It solves each sample open-handed with alpha-beta over the remaining tricks, using
`tricks.Legal` for move generation, and plays the card with the best average. Thirty cards and
ten tricks keep each solve small. The budget is capped per move so bot think time stays within
the runtime's defaults.

**Guard rail:** the bot receives the whole state, so it must not peek. A `TestBotDoesNotPeek`
test follows `holdem/bot.go:34-36` and `ai/nopeek_test.go`. The sampler and solver live in
`internal/tricks/search` so Bridge can reuse them.

## 5. Tests

| Level | What it proves | Where |
|---|---|---|
| Unit | `Legal` and `Winner` under both policies and both orders; the overtake corner cases | `tricks/*_test.go` |
| Unit | Talon rules; contract success for each part; flek arithmetic; red doubling; early stop in Betl/Durch; last-trick 10 | `marias/engine_test.go` |
| Agreement | Offers ≡ what `Apply` accepts (probe pattern, `prsi/offers.go:112-124`) | `marias/conformance_test.go` |
| Hidden info | No viewer sees another hand, the talon (except the declarer), or won-trick contents | `allmodules_test.go` + module test |
| Contract | Every shared check in `allmodules_test.go:143-1448`, including rule-index and rounds that name no card (`:1169`) | automatic once registered |
| Bot | Completes 1 000 seeded matches; beats the fallback bot; does not peek | `marias/bot_test.go` |
| Keys | `serverKeys.json` regenerated; every locale words every key | `dump-keys`, `serverKeys.test.ts` |
| e2e | Play a full deal against bots; the trick area places cards by seat; a Betl take-over in Licitovaný | `e2e/tests/marias.spec.ts` |
| Visual | The trick area and German faces at phone and desktop widths, checked in the browser, not inferred from code | manual, with screenshots in the PR |

## 6. Order of work (each step is one PR)

0. **Pin the rules.** Write `rules.go` sections and the option table first. The user signs off on
   them; the tariff and flek names come from here. *Done: [marias-rules.md](marias-rules.md) and
   `server/internal/marias`, signed off 2026-09-29.*
1. **`internal/tricks`**, with tests. No client change. *Done on `claude/marias-tricks`.*
2. **Trick area:** `CardView.By` and `Zone.Arrange`, plus the React Native compass and the TUI
   row. *Done on `claude/marias-tricks`: `SeatArrangedZone` and `lib/seatArrangement.ts` (named
   for the shape, since the shell may not use a game's words), a flight that lands on the
   player's own spot, and a contract test in `allmodules_test`. It was checked in the browser at
   3, 4-5 seats and phone width using a temporary, uncommitted Prší patch.*
3. **`marias` Volený**, with the rule-of-thumb bot and English and Czech wording. The other 22
   locales are filled in the same PR, because the key test requires it. *Done on
   `claude/marias-tricks`: the engine (auction, take-over, doubling round, play, settlement), the
   view, the scoreboard and round log, the bot (medium averages +5.6 units a match against two
   easy seats), the shared contract suite, `e2e/tests/marias.spec.ts`, and all 24 locales. It
   was played through the real UI against two bots in the browser.*
4. **Licitovaný:** the association's auction game, pinned first like step 0. *Done on `claude/marias-tricks`
   to [marias-licitovany-rules.md](marias-licitovany-rules.md), signed off 2026-09-29: the auction,
   the 12-rung ladder, dvě sedmy, omyl and the forhont's lead, a second row in the contract suite,
   and the bot's bidding. It also moved Volený's sto to the association's linear scoring.*
5. **German faces** (§3.4), and Prší opting in. *Done on `claude/marias-tricks`: `Deck`
   on the descriptor and on every state and replay message; `src/lib/deck.ts` (context, plus
   German names for a card in a sentence); `GermanSuit`, `GermanCourt` and `GermanFace` in all
   four skin styles; the TUI's four-colour German rendering. Prší names the German suits and its
   wild card is the svršek, in all 24 locales. Checked with headless screenshots of real tables
   and a gallery of all 32 cards.*
6. **Hard bot** (sampling), in `internal/tricks/search`. *Done on `claude/marias-tricks`:*
   - An open-hand alpha-beta solver, checked against brute force on 300 random positions. It
     has a transposition table, pruning of equivalent cards, held-back sevens, and marriages
     scored as they are played.
   - A sampler that deals the unseen cards consistently with what the table has shown: voids,
     cards a player failed to beat, promised králs and sevens, a sharp-free talon, and the
     declarer's trump length.
   - The hard seat averages 20 samples a card, capped by a node budget; that is about 260 ms at
     the first trick and under 1 ms by the seventh.
   - Paired against medium on the same deals: volený +5.9 ±1.0 and licitovaný +12.4 ±1.8 units a
     match (75 pairs each).

Steps 1–3 give a playable game. Steps 4–6 each stand on their own.

## 7. Decisions (settled 2026-09-29)

1. **Default variation:** Volený.
2. **Tariff and flek names:** the common pub values in §1, pinned in step 0.
3. **Card art:** new in-house German (Tell-pattern) vector faces, used by Prší as well. No
   interim relabelled French faces.
4. **Bridge:** parked. Part II stays as a sketch for when it is picked up.

## 8. Part II — Bridge, as a sibling module (sketch)

`server/internal/bridge` would be **Chicago** (four deals per match with the vulnerability
schedule). Rubber bridge has no natural end for an app match, and duplicate needs many tables.
Its needs beyond Mariáš:

| Need | Why Mariáš does not cover it | Work |
|---|---|---|
| **Dummy.** The declarer plays from a hand they do not own | Only the viewer's own hand is selectable (`app/match/[matchId].tsx:145-151`, `OfferBar.tsx:163-173`, `client-tui/ui/match.go:278,334`) | The shell lets the viewer select from **any zone an enabled offer's `Source` names**. No protocol change is needed; the server side already allows it (`match/manager.go:600-616`). |
| **Auction history** | No view element shows a table of bids by seat | A small generic ledger view (rows × seats). Buttons use one `bid` offer with choice params for level and strain, plus Pass, X and XX |
| **Partnership results in stats** | Two winners are recorded as a **draw** (`stats/standings.go:117,146`). The test says this is deliberate for Canasta (`standings_test.go:104-106`), but the field's own comment says otherwise (`:58`) | Decide it once for Canasta and Bridge. A `Seat.Side` win is probably a win for both partners |
| **Bidding bot** | Nothing comparable exists | A simplified natural system (SAYC-lite: HCP, suit length, a few conventions). This is the largest single piece |
| **Card-play bot** | 52 cards is far past a naive alpha-beta | Reuse the sampler in `tricks/search` with a solver that has transposition tables and equivalent-card pruning |
| **Scoring** | | The contract, overtrick, undertrick, vulnerability, double and slam tables. Chicago has no honours |

**Size estimate:** Bridge is about 6–9k lines of Go with tests, plus the client dummy and ledger
work. That is between Hold'em and Canasta. Mariáš is about 5–7k, plus the trick area and faces.

A closer relative worth noting is **Křížový mariáš**. It has four players in fixed partnerships
and Mariáš's card rules, and is a variation-sized step once Mariáš exists. It also forces the
partnership-stats decision early, which is a cheap rehearsal for Bridge.

## 9. Follow-ups (out of scope)

- Křížový mariáš (four players, partnerships).
- ~~Four at a three-player table, with the dealer sitting out each deal.~~ Done: a Mariáš table
  seats three or four in both variations (marias-rules.md, difference 4).
- ~~Showing the declarer's hand in Betl or Durch after the first trick.~~ Done: the
  `openBetlDurch` table option (marias-rules.md, difference 5).
- Mariáš leaderboards by units won.
- Offline play.
