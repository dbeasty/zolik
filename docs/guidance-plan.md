# Guidance plan: what changed, why not, what next

Players still get stuck on "what do I do now?". Four asks from play-testing:

1. Show when a meld on the table has changed.
2. Let a player tap *any* button, disabled ones included, and be told why it is disabled.
3. Help with the next move.
4. An AI guide that explains how to play and what the rules are.

Much of the groundwork already exists. The phases below go from cheapest to most expensive.

## Status (2026-09-29, branch `claude/guidance`)

Phases 0–4 are implemented, one commit each. Phase 5 (the AI guide) is deferred until an external model or an in-house AI is chosen.

Where the build differs from the plan below:

- **Hints are a REST call** (`POST /matches/{id}/hint`), not a WebSocket verb. The socket carries raw actions only, and a hint changes nothing. Hints are on by default at every table, human-only ones included, and the host can turn them off. Hint use is not recorded in the round results yet.
- **Recent moves** are narrated only by Žolíky so far. Canasta, Gin Rummy and Rummy Tiles need their own `NarrateEvent`.
- **Change markers** do not scroll to an off-screen zone. The owner's panel header carries a "n changed" chip instead, which stays visible when the panel is minimised.
- **Selection help** in the explanation sheet says what to pick, but does not yet highlight the cards that would fit.
- **Bot audit:** no bot reads hidden cards. Blackjack, Gin Rummy and Rummy Tiles still have no no-peek test of their own.

## What already exists

| Piece | Where | Relevance |
|---|---|---|
| Every offer is sent to the client, disabled ones included, with `whyNot`, `ruleIds`, `remedy` and `remedyOfferId` | `server/internal/module/protocol.go:482-576`, per-module `remedy.go` / `ruleindex.go` | Every disabled button already knows why it is disabled. |
| `WhySheet` resolves `ruleIds` against written rules, offers a working remedy button, and deep-links to `/rules?highlight=` | `client-react-native/src/components/match/WhySheet.tsx` | The explanation UI is already built. |
| The small `ReasonLine` under a disabled offer can be pressed and opens `WhySheet` | `OfferBar.tsx:269-285, 1063-1088` | This is the only tappable part of a disabled offer today. |
| A disabled `Pressable` swallows the tap (`disabled={!offer.enabled \|\| !ready}`) | `OfferBar.tsx:229`, `FoldedOffer` 523, `OfferGlance` 429 | This is what blocks ask 2. |
| Client-side "unready" reasons such as `sel.needMore`: no rule behind them, and they cannot be pressed | `OfferBar.tsx:286-291, 910-918` | These need to be explainable too. |
| Previous and next boards are compared to plan animations (`planFlights`); `SettleIn` gives a new card a short entrance animation | `src/lib/flights.ts`, `SettleIn.tsx` | The same comparison can drive change markers. |
| Server `Apply` events (`meld_played`, `card_laid_off`, `joker_swapped`, …) are published to every seat, and the client ignores them | `match/manager.go:706-719`, `useMatchSocket.ts:135` | This is where "who did what" would come from. They are **not filtered per viewer** (see Phase 0). |
| Bots see only what their seat can see (`ai.VisibleFor`, `nopeek_test.go`) and pick an action through `module.BotFor(mod).Act` | `module/bot.go`, `zolikmod/bot.go`, `match/bots.go:143` | A hint can reuse the bot without letting it cheat. |
| `Rules(cfg)` is resolved per variation and option, served at `GET /modules/{id}/rules` | `module/rules.go`, `handlers.go:181-225` | This is the grounding text for the AI guide. |
| No hint, coach, tutorial, activity feed or LLM integration exists | — | Everything in Phases 3-5 is new. |

---

## Phase 0: stop leaking the drawn card (prerequisite, security)

`publishEvents` sends each event unchanged to every seated player. Žolíky's `draw_deck` event carries the drawn card (`rules/engine.go:67-70`, marked "private; redact"). So any opponent's socket receives each card drawn from the deck. The official client drops the frame, but a modified client could read it.

- Add per-viewer event projection: an optional `module.EventProjector` with `ProjectEvent(ev, viewerID) (Event, bool)`. For modules that don't implement it, the default drops any event that has a `card`/`cards` field and was not caused by the viewer. Where a card should be public (for example `draw_discard`, `meld_played`), the module says so explicitly.
- Test: for every module, run a seeded match and check that no seat ever receives a card that was never visible in its own `MatchStateMsg`. This should reuse the replay redaction rules.
- This has to land before Phase 3, which starts reading events.

## Phase 1: every button explains itself (ask 2)

**Behaviour.** A disabled offer looks disabled: dimmed, and `accessibilityState.disabled`. Tapping it opens `WhySheet` instead of doing nothing.

- In `OfferBar`, `FoldedOffer` and `OfferGlance`, replace `disabled={…}` with `aria-disabled`/`accessibilityState` for the look, and route `onPress` like this:
  - `offer.enabled && ready`: submit, as today.
  - `!offer.enabled`: open `onExplain({code: whyNot, ruleIds, remedy, remedyOfferId})`, the same payload `ReasonLine` sends.
  - `offer.enabled && !ready` (a client-side selection problem): open `WhySheet` in a new "how to use this" mode. It shows the `unreadyReason` text, what to select (e.g. "Select 3+ cards of one suit in sequence, then press Meld"), and highlights the cards that could fit, using `fits`/`readyWith` from `src/lib/drops.ts`.
- `OfferGlance` (collapsed rail) currently hides disabled offers. Keep hiding them, since the rail is meant to stay short, but add a "why can't I…?" affordance that expands the full bar.
- `NOT_YOUR_TURN` should name the player: "It's Eva's turn". Add a `whose` param to that reason, filled in from `SeatStrip`'s active seat.
- **Coverage guard.** Add a server test over every module and a sweep of seeded states: every offer with `Enabled=false` has a non-empty `WhyNot` **and** non-empty `RuleIDs`, or is on an explicit allow-list with a comment. This keeps every refusal pointing at a written rule.
- **e2e.** Tap a disabled Meld button, assert that the sheet is visible and shows rule text, then press the remedy and assert that the board changed. The test must check what the player sees, not just the code path.

Size: small. Client-only apart from the `whose` param and the coverage test.

## Phase 2: table change markers + "your turn" step line (asks 1 and 3, cheap part)

**Change markers**, derived on the client by comparing boards, as `planFlights` already does:

- New pure function `diffGroups(prev, next): Map<groupId, {kind: 'new' | 'grown' | 'changed', addedCardIds}>`, unit-tested next to `flights.ts`.
- `ZoneView` shows an accent outline and a small "NEW" or "+1" chip on changed groups, and highlights the added cards within a stacked meld. A stacked meld hides the card that was laid off, so the chip is the only visible cue.
- The zone title (owner name) gets a dot when any of its groups changed, because the spreads row wraps and changed melds may be off-screen. Tapping the dot scrolls to the zone (see memory: `measureLayout` + `getInnerViewNode()`).
- **Lifetime:** markers stay until the viewer's own next action is accepted, not for a fixed time. That way a player who looks away still sees everything that changed since their last move. Changes piled up over several opponent turns merge.
- The skin system may only change colour and decoration: the chip is placed as an overlay and must not resize the group.

**Turn step line**: one sentence above the offer bar, built from the enabled offers:

- The phase comes from the offers: draw phase gives "Draw from the deck or take the discard". Meld phase gives "Meld, lay off, or discard to end your turn". A mandatory remedy (e.g. `zolik.remedy.meldThePickup`) takes priority: "You took the discard — you must meld it this turn".
- Use keys with params only: add `step.*` keys to all 24 locales and regenerate `serverKeys.json` if any key originates on the server.
- If a player makes no move for N seconds on their turn, pulse (`Attention.tsx`) the most likely next action. This is the first thing the Phase 4 hint will improve.

Size: medium, client only. Works offline.

## Phase 3: recent moves strip (ask 1, "who did what")

Comparing boards shows *what* changed but not *who* did it. A lay-off onto Bob's meld by Alice looks exactly like one by Bob. Once Phase 0 has made events safe:

- Server: add `recentMoves []Fact` to `MatchStateMsg`, projected per viewer. This covers the last full round of turns, one fact per event the module chooses to show, for example `move.laidOff {player, card, meldOwner}` and `move.melded {player, cards}`. Build it from the redacted events so it cannot leak.
- Putting it in the state message (rather than consuming loose event frames) means it survives reconnects and is idempotent, and replay can reuse the same facts to improve `captionFor`.
- Client: a one-line strip under `SeatStrip` shows the latest opponent move. Tapping it expands to the round, and tapping an entry pulses the meld it refers to, linking to the Phase 2 markers by group id.
- Message keys must be static literals. Regenerate `serverKeys.json`.

Size: medium. Server, then client. Do Žolíky first, then Canasta, Gin, Rummy Tiles.

## Phase 4: "Hint" button (ask 3)

- New WebSocket verb `hint` (not REST, so it works against the live state and seat). The server runs `module.BotFor(mod).Act(state, BotSeat{PlayerID: caller, Skill: high, Seed}, offers)` for the **caller's** seat. It returns `{offerId, cards, params, reasonKey}` and does not apply the action.
  - Heuristic bots already restrict themselves to `ai.VisibleFor`. Extend `nopeek_test.go` to cover the hint path, so a hint can never tell a player something only the server knows.
  - Several modules use `OfferBot`, which chooses by alphabetical sort order. Their hints would be poor, so enable hints only for modules with a real strategy (Žolíky and Canasta first) and hide the button elsewhere.
- Client: a "Hint" button in the header. It highlights the suggested cards in the hand and the suggested button, and shows one short `reasonKey` sentence, e.g. "Lay off 7♥ on Eva's run to shed points". It never auto-plays.
- **House rule:** a declared table option `hints: on | off`, per the house-rules memory. Default **on** for tables with bots and **off** for all-human tables; the host can change it. Record the hint count per seat in round results so hints aren't hidden.
- Adding an explanation to `ai.Agent` (why it picked this move) is optional. v1 can map the chosen offer's label to a generic reason.

Size: medium. Mainly server work.

## Phase 5: AI guide (ask 4)

An "Ask the guide" panel on the rules screen and in the match screen (a sheet). The player asks in plain language, for example "why can't I lay off here?", "what's a clean run?" or "how does scoring work?", and the answer comes in their locale.

**Architecture**

- Server-side proxy only. The API key never reaches the client. New package `server/internal/guide`, env `ANTHROPIC_API_KEY` (plus `_FILE`), feature flag `FEATURE_FLAG_AI_GUIDE`, configured through `config.go` like the other outbound services.
- The endpoint `POST /guide/ask {moduleId, variation, options, matchId?, question, locale}` requires a signed-in account.
- Grounding sent with every request, built entirely on the server:
  1. `Rules(cfg)` for that variation and those options, rendered to English text from the en bundle, with rule ids so the answer can cite them and the client can link `/rules?highlight=`.
  2. Only when `matchId` is given and the caller is seated: **that seat's own** `MatchStateMsg` projection (their hand, the public table, the current offers with `whyNot`/`remedy`). Nothing else, so the guide can't reveal hidden cards.
  3. The `hints` table option. If hints are off, the system prompt forbids move advice and restricts the guide to explaining rules. This is enforced by withholding the offers and the hand, not only by the prompt.
- Model: `claude-haiku-4-5` by default, for latency and cost; `claude-sonnet-5-5` is available behind config for harder questions. Stream the answer.
- The answer must cite rule ids and never invent rules. Verify it after it comes back: strip any cited id that is not in `StatedRuleIDs`.
- Guardrails:
  - Per-user rate limit and daily token budget.
  - Log question, answer and token counts, but not the grounding (it contains hands).
  - Reply "guide unavailable" when offline or when the flag is off. Phases 1-4 keep working without it.
- Legal: add an AI-disclosure line and a privacy-policy entry (questions go to a third-party model provider). Tie this to `docs/legal-plan.md`.

**Evaluation before launch**

- A fixed set of about 40 rules questions per module, including variation-specific ones such as Samba versus classic Canasta. Each is checked by assertion ("mentions rule X", "doesn't claim Y") and run in CI against the real model behind a flag.
- Hidden-information red team: seeded states where the answer would require an opponent's hand. The guide must refuse or stay generic.

Size: large. Needs a decision on provider and cost (below).

---

## Order and sizing

| Phase | Asks | Size | Depends on |
|---|---|---|---|
| 0 Event redaction | security | S | — |
| 1 Explain every button | 2 | S | — |
| 2 Change markers + step line | 1, 3 | M | — |
| 3 Recent moves strip | 1 | M | 0 |
| 4 Hint | 3 | M | 1 (reuses sheet/highlight) |
| 5 AI guide | 4 | L | 1, 4 (hint option) |

Phases 0, 1 and 2 can run in parallel worktrees.

## Decisions needed

1. **Hints in all-human games:** default off, as proposed, or on everywhere?
2. **AI guide provider and budget:** OK to call the Claude API from production with a server-held key and a per-user daily cap? Is a rules-only guide (no hand or table context) an acceptable first version, since it avoids the hidden-information risk?
3. **Marker lifetime:** keep markers until your next move (proposed), or fade them after one full round?
