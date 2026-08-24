# One architecture — finishing the job

Companion to [`extensibility-plan.md`](./extensibility-plan.md), which took the runtime through
Phase 4, and to [`canasta-plan.md`](./canasta-plan.md), which added the third module. This is
Phases 5–8: making *every* game actually run on the module runtime, and making that runtime
able to host a game that is not a card-matching game at all.

- **Baseline:** `worktree-canasta` @ `b991b50`
- **Goal:** one write path, one state shape, one client shell, one way to add a game — and a
  module interface that has been falsified against poker before poker is written on top of it.

---

## 0. What is actually wrong today

Phase 3 shipped the module runtime **alongside** the Žolíky path rather than replacing it, and
Phase 4's own "what is left" table says so. The result is that the sentence "all games use the
same architecture" is false in a specific, measurable way:

| | Žolíky | Prší | Canasta |
|---|---|---|---|
| Engine entry point | `internal/game.Manager` | `match.Manager` | `match.Manager` |
| Persisted as | `models.Game` (30 typed columns) | `models.Match` (opaque blob) | `models.Match` |
| Routes | `/games/*`, `/ws/games/*` | `/matches/*` | `/matches/*` |
| Wire shape | `GameStateMsg` | `match_state` | `match_state` |
| Bots | real, and they play | seat only, never moves | seat only, never moves |
| Disconnect handling | suspend/resume | none | none |
| Stats recorded | yes | no | no |
| Client | a 1,756-line rummy screen | none | none |

`internal/zolikmod` — the adapter that makes Žolíky a module — is exercised only by tests. The
real game does not go through it. So there are two Žolíky engines' worth of surface, and the
one the abstraction was proved against is not the one anybody plays.

The three rows at the bottom are the more interesting ones. Bots, disconnect handling and stats
are **runtime** concerns that were built into the Žolíky path and never lifted out, which is why
a new module gets a lobby and a socket but not a playable opponent. Fixing that is most of what
makes the module seam worth having.

---

## 1. The rule this plan enforces

> **A game is a module. Everything else — persistence, sockets, turns, bots, reconnection,
> statistics, the screen — is the runtime's, and is written exactly once.**

The measure of progress is the table above collapsing into one column.

---

## Phase 5 — Falsify the interface against poker

**Thesis.** `extensibility-plan.md` §3's own warning: *an interface designed against one game
fits exactly one game.* It was designed against Žolíky and Prší. Canasta then found the first
place it is simply the wrong shape (`Finished` names one winner; Canasta is won by a
partnership). Poker is the game that has been named as coming next, and it is the first
candidate that is not about matching cards at all — so it is the honest test.

**It gets implemented, not sketched.** Prší was implemented for exactly this reason, and it is
what turned up `ParamSpec`. A sketch would let every awkwardness be waved through.

### 5.1 `internal/holdem`

No-limit Texas Hold'em: blinds, four betting streets, a 7-card hand evaluator, side pots,
elimination, button rotation. Two to nine seats.

### 5.2 What poker bends, and the fix

| What poker needs | Why nothing so far asked | Fix |
|---|---|---|
| A **number** as an action input — raise to *how much* | Every existing input is a card or a fixed choice from a list. `ParamSpec` can only enumerate. | `ParamSpec.Kind` + `Min`/`Max`/`Step` |
| **More than one winner** — a split pot | Žolíky and Prší are won by one player. Canasta already showed the seam; poker makes it unavoidable. | `Finished` returns `winners []string` |
| **Numbers attached to a seat** — a stack, a bet in front of you, folded/all-in | Cards are all any game has needed on the board. Chips are not cards and are not secret. | `ViewModel.Seats []Seat` |
| **Whose turn, as a pushed fact** | The runtime infers it by asking who has an enabled offer — which works, and is a strange way to learn something the module knows. | `Seat.Active` |

`Seats` is the one worth arguing for beyond poker: it gives Canasta a place to put partnership
standings, Žolíky a place to put whose turn it is, and the generic client shell (Phase 7) the
one thing it cannot currently render — a table with people at it.

### 5.3 Every module moves to the new shapes

`zolikmod`, `prsi` and `canasta` adopt `Finished`'s new signature and emit `Seats`. No module
keeps a private way of saying who won.

### Exit criterion

Four modules, one interface, all four driven to completion by `module.PlayWithOffers` with no
game-specific branch in the driver.

---

## Phase 6 — The runtime grows the things only Žolíky had

**Thesis.** A module that gets a lobby and a socket but no opponent, no reconnection and no
recorded result is not hosted, it is parked. These are runtime properties and belong in
`match.Manager` where every module gets them.

### 6.1 A bot for every game, from the offer list

`module.PlayWithOffers` already contains a policy that plays any module from offers alone. It is
promoted out of the test harness into `module.ChooseAction`, and the runtime drives AI seats with
it. Every module — including ones not yet written — gets a playable opponent the day it is
registered, with no AI code of its own.

### 6.2 A module may do better

```go
type Agented interface{ Agent() Agent }
type Agent interface{ Act(s State, playerID string, seed int64) (Action, error) }
```

Optional. `zolikmod` implements it and returns the existing `internal/ai` heuristic player, so
Žolíky's bots stay as strong as they are today rather than regressing to a shape-matcher.

### 6.3 Reconnection and statistics

`SuspendOnDisconnect` / `ResumeIfReturning` and the stats recorder move from `game.Manager` to
`match.Manager`, generically. Suspension is an envelope property — it is about a *seat*, not
about melds.

### Exit criterion

`POST /matches/{id}/add-bot` produces an opponent that plays, in all four games. A dropped
socket suspends and a returning one resumes, in all four. A finished match is recorded.

---

## Phase 7 — One screen for every game

**Thesis.** `architecture.md` §7.7's last untested claim: a shell that renders `ViewModel` zones
and offer buttons should play any module with no new screen.

- `app/match/[matchId].tsx` — renders seats, zones, groups and prompts from the view, and one
  control per offer, with `whyNot` as the disabled reason. Card inputs come from the offer's own
  selector; a numeric parameter renders a stepper; a choice parameter renders a picker.
- The lobby's game picker and new-match form come from `/modules`, as the descriptor always
  intended.
- **The acceptance test is that it contains no game's vocabulary.** Not "no rummy logic" — no
  mention of melds, suits, canastas, blinds or pots anywhere in the file.

### Exit criterion

The same screen, unchanged, plays Prší, Canasta and Hold'em in a browser, proven by e2e.

---

## Phase 8 — Retire the second engine

**Thesis.** Only once the above exists is it safe to delete anything.

1. One-shot migration of `games` documents into `matches`, with `zolikmod` reading the migrated
   state.
2. `/games/*` and `/ws/games/*` become thin adapters over `match.Manager`, so the existing RN
   and TUI clients keep working while `GameStateMsg` is produced *from* the module's view rather
   than from a second engine.
3. Delete `game.Manager`, `game.Repository`, `state_builder.go`, `takeback.go` and `models.Game`.
   `game.Hub` stays — it is the runtime's socket fan-out and is already shared.

### Exit criterion

`internal/rules` is reachable only through `internal/zolikmod`. The table in §0 has one column.

---

## Sequencing, and where the risk is

Phases 5 and 6 are additive: nothing that works today stops working. Phase 7 adds a screen
without touching the existing one. **Phase 8 is the only destructive phase**, and it is last on
purpose — it deletes a working engine, and the only responsible time to do that is after its
replacement has been driven to completion in a browser.

Each phase is committed and verified on its own, so stopping after any of them leaves the
codebase better than it found it.

---

## Outcome

Phases 5–7 shipped as planned. Phase 8 shipped up to its destructive step, which is left as a
decision rather than taken as a default — see below.

### The finding that reframed Phase 8

Reading the two paths closely turned up something §0's table got wrong: **there are not two
rummy engines.** `game.Manager` and `zolikmod` both apply moves with `rules.ApplyAction` on a
`rules.GameState`. What is duplicated between the Žolíky path and the module path is
*persistence and transport* — two documents, two wire shapes, two route sets — not two
implementations of the rules.

That matters, because the dangerous kind of duplication, where two copies of a rule drift
apart, cannot happen here. Retiring the legacy path is cleanup with a real payoff, but it is
not a correctness fix and it does not need rushing.

### What each phase cost the interface

Poker bent it in four places, plus a fifth that fell out of writing the shared contract test:

| Change | Why nothing before it asked |
|---|---|
| `ParamSpec.Kind` + range | Every prior input was a card or a short list; a no-limit raise is a number |
| `Finished` → `winners []string` | A split pot has no single winner; Canasta's partnership had already shown the seam |
| `ViewModel.Seats` | Chips, bets, folded, all-in — none are cards, so none fit a Zone |
| `ActionOffer.Facts` | "Call 40" is a button whose whole meaning is its number |
| `ActionOffer.Composite` | A Žolíky meld shape and a Canasta meld are both `lay_meld`; only one is pressable |

What did *not* bend is as informative: zones, groups, hidden-information filtering, the
descriptor and the option vocabulary all expressed a betting game without changing.

### Measured

| | |
|---|---|
| Games on one runtime | 4 — Žolíky, Prší, Canasta, Hold'em |
| Shared contract tests, run against every game | 8 |
| Bot moves accepted / refused, all four games | 730 / 0 |
| Client tests | 101, including the shell's "knows no game" grep |
| e2e | 74, including one screen playing four games in a browser |

### Two bugs the new tests caught that the old ones could not

**Hold'em skipped the first player of every betting street.** The offer-driven driver could not
see it — a driver plays whoever it is offered a move, so a wrong turn order looks like a
different but valid game. `Seat.Active` made the module state its own claim, and the shared
contract test checked it against the offers.

**The shell's e2e was passing without pressing anything.** Its selector matched the scrolling
container as well as the buttons inside it, and it counted clicks rather than progress.
Asserting that the *server's* board had moved turned three green tests red, correctly.

## What is left, and the decision inside it

`cmd/migrate-games` moves the documents, and `internal/game/module_equivalence_test.go` proves a
migrated game presents the same offers, the same legality and the same result from the same
move. So the remaining step is small and evidenced:

> Serve `/games/*` and `/ws/games/*` from `match.Manager`, with `GameStateMsg` produced from the
> module's state, then delete `game.Manager`, `game.Repository`, `state_builder.go`,
> `takeback.go` and `models.Game`.

It stops here because it carries a product question this refactor should not answer by itself:

- The shipped RN client has a **bespoke 1,756-line Žolíky screen** — drag and drop, staging,
  joker swaps. The generic shell plays Žolíky, but it is a generic shell. Retiring
  `GameStateMsg` means either keeping a compatibility adapter for that screen indefinitely, or
  accepting the generic one in its place.
- The TUI reads the same shape.
- Sixty e2e tests seed through `POST /games/{id}/debug-state`, which writes rummy columns
  directly and would need a module-state equivalent.

None of that is hard. All of it is a decision about what the Žolíky *product* should look like
afterwards, which is why it stops with the evidence in place rather than choosing for the owner.
