# The amount belongs on the button

## The problem

On a Hold'em raise, the button read **Raise / in the pot 130** whatever the
slider, stepper, typed field or a quick choice (½ Pot, Pot, All-in) had set.
The figure the press would send (483 in the report) appeared only in the field
under the button, so the player had to trust that the button used it. Blackjack's
**Bet** button had the same gap.

The cause is on purpose. `OfferBar` shows the offer's label and the facts the
server sends with it, and never reads a rule. Nothing in the protocol said
"this parameter is what the button does", so the shell had no rule-free way to
know that 483 belonged on the button and a Rummy Tiles split position did not.

## The fix (done on `feat/raise-button-shows-amount`)

1. **Protocol.** `ParamSpec.Headline bool` (`headline` in JSON). It means: this
   parameter's current value is what the press sends, so the offer's own
   control names it. It is opt-in. `ParamKindInt` is also used for things like
   a tile's place in a run, and that is not a button title.
   - `server/internal/module/protocol.go`
2. **Modules.** Hold'em raise `amount` and Blackjack bet `amount` set
   `Headline: true`. Rummy Tiles `position` does not.
   - `server/internal/holdem/offers.go`, `server/internal/blackjack/offers.go`
3. **Client.**
   - `offerHeadline(offer, inProgress)` in `src/api/matchTypes.ts` is a pure
     function. It returns the headline parameter's prompt key and the value the
     press would send right now: the in-progress value if the player moved it,
     otherwise the server default. It clamps the same way `ParamControl` does.
   - `OfferBar` uses that as the button title: *prompt + value*, for example
     **Raise to 483** or **Stake 50**. The title is re-rendered on every
     slider, stepper, keystroke or quick-choice change. Offers without a
     headline keep their verb.
   - No new wording. The title reuses the parameter's existing prompt
     (`holdem.prompt.raiseTo`, `blackjack.prompt.betAmount`), which already has
     a translation in every locale. `serverKeys.json` is unchanged.

## Tests

| Layer | What it pins |
|---|---|
| Go `TestRaiseOfferQuickChoices` / `TestBettingOfferQuickChoices` | Both amount params are marked `Headline`. |
| Jest `src/api/headline.test.ts` | The default is named before any input; the title follows the slider/stepper and quick-choice values; out-of-range values are clamped; **the title matches the value `submissionFor` sends** for the same input; a non-headline int param and a param-less offer keep their verb. |
| Playwright `generic-shell.spec.ts`, "a numeric control…" | Checks the button's **rendered text** (`offer-raise-title`) after dealing, a stepper press, a quick choice, max, dragging the slider to the minimum, and typing past the range. |

Checked against the old behaviour: with the title change reverted, the e2e test
fails with `Received string: "Raise"`. With the fix, it passes on repeated runs.
The Blackjack e2e suite still passes. The full jest suite (919 tests) and `tsc`
are clean.

## The collapsed pill (done, second commit)

The collapsed-rail pill (`OfferGlance`) read "Raise" and sent the server
default, whatever the full bar had set. It was worse than that: collapsing the
controls panel unmounts `OfferBar`, and the in-progress amount lived inside it,
so collapsing also forgot the amount.

- The match screen now holds the in-progress values (`OfferParams`) and passes
  them to both `OfferBar` and `OfferGlance`. Both components still fall back to
  their own local state when rendered alone.
- The pill uses the same `offerHeadline` title and sends `params[offer.id]`.
- `submissionFor` now sends numbers through `paramValue`, which clamps them into
  the range currently on offer. An amount set on an earlier street and not sent
  is sent at the figure the button shows, instead of one the engine refuses.
- The e2e test "the collapsed controls pill names and sends the amount dialled
  in the bar" types an amount, collapses the panel, checks the pill's text,
  reopens the panel to check the amount survived, then presses the pill and
  reads the websocket frame to confirm `raise` was sent with that amount. With
  the shared store disconnected, it fails with `Received string: "Raise to 40"`.

## Follow-ups (not in this change)

1. **"in the pot 130".** This is the pot *if you call*, which is not the
   same as the pot after this raise, and it now sits under "Raise to 483".
   Choose one: reword it (for example "pot after call 130"), or have the client
   add the figure it now knows, "pot becomes N". That second option is pot
   arithmetic, which is a rule, so it would need a server-side fact the client
   can fill in. Get a product decision before building either.
2. **Accessibility.** The title text is now the accessible name. Check that
   VoiceOver or TalkBack reads "Raise to 483" once, and not the prompt twice
   (the parameter label above the slider also says "Raise to").
3. **Other modules.** Any future module that declares a stake or amount should
   set `Headline`. Add one sentence about it to the module-authoring notes in
   `docs/extensibility-plan.md`.
