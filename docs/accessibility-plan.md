# Accessibility and tooltips plan

Make the whole app usable by people with disabilities, on the web build and on
the iOS/Android apps, and give every control, badge and card a tooltip that
says what it is and why it matters. One React Native codebase serves all three
platforms. Nearly every item here is done once in `client-react-native/` and
then checked three times: browser, VoiceOver and TalkBack.

**Target:** WCAG 2.2 AA on the web. On native, the equivalent Apple and Android
guidance: everything works with VoiceOver, TalkBack, Switch Control, a hardware
keyboard, Dynamic Type / font scale and Reduce Motion.

## Where we are (survey of `main` @ 77e818b)

Some groundwork exists, but it is patchy:

| Area | State |
|---|---|
| Roles / labels | `accessibilityRole` in 35 files, `accessibilityLabel` in 16. Most screens have none. |
| Cards | Labelled with the **raw server code**: `HandZone.tsx:991`, `ZoneView.tsx:597`, `ZoneView.tsx:864` all pass `accessibilityLabel={card}`, so VoiceOver reads "K, H". `cardText()` (`src/lib/cards.ts:105`) gives "K♥", which is better but still not speech. |
| Hand reorder | Already has `accessibilityActions` moveLeft/moveRight (`HandZone.tsx:990`). |
| Drag-only actions | Not an issue: tap-to-play covers every drop (`onPressDrop`). |
| Disabled-move explanations | `ExplainOnPress` (`OfferBar.tsx:1217`) is `tabIndex={-1} accessible={false}`, so keyboard and screen-reader users never hear *why* a move is off. |
| Long-press | Badge explanations only open on a 400 ms long-press (`HandZone.tsx:891`), with no alternative. |
| Announcements | One `announceForAccessibility` (results headline, `ResultsFlash.tsx:112`) and one live region (`lobby/setup.tsx:262`). Opponents' moves, your turn and refusals are never announced. |
| Hardcoded English | `Panel.tsx:158` ("Show/Minimize …"), `ZoneView.tsx:415` ("Hide the rest of …"), `SeatStrip.tsx:155` ("AI agent"). |
| Tooltips | None. No hover handling, no `title`, no tooltip component. |
| Keyboard (web) | No key handling anywhere. Focus order is DOM order, and focus rings are the browser defaults or missing. |
| Text size | 273 literal `fontSize`s. `useMetrics` scales by window width only and ignores the OS font scale. |
| Motion | `FlightLayer`, `ResultsFlash`, arrivals, etc. use animation in 14 files. Reduce Motion is read in one place. |
| Colour | Suits always carry a glyph, and Last Card colours carry a shape on the fancy face. There is no four-colour deck and no high-contrast skin. |
| Testing | No axe checks, no a11y lint, and no tests that query by role or name. |

## Principles

1. **One source of truth per meaning.** The tooltip text, the screen-reader
   label and the hint for a control come from the same i18n key. A tooltip is
   the visible form of the accessible description, not a second copy that can
   drift.
2. **No behaviour change by default.** Visual changes (four-colour deck, larger
   cards, high contrast, reduced motion overrides) are player settings that
   default to today's look, in line with the "house rules are options" rule.
   Respecting OS settings (Reduce Motion, font scale) is the exception and
   applies automatically.
3. **Skins stay colour-only.** Larger cards and text are a *metrics* setting,
   not a skin, because skins must not change sizes.
4. **Server ships keys, client renders words.** Any new text the server
   produces (card hints, move announcements) travels as message keys, with
   `serverKeys.json` regenerated, keys as static literals, and en.ts changes
   followed by `go generate ./internal/gamemcp`.
5. **Assert what the user hears.** Tests query by role and accessible name
   (`getByRole('button', { name: 'Draw' })`), which tests the labels for free.

## Phase 0: Foundations (shared, about 1 week)

Build the pieces every later phase uses, with no visible change yet.

- **`src/a11y/` module**
  - `cardSpokenName(code, deck)`: "King of Hearts", "Ober of Leaves",
    "Joker", "Wild Draw Four", rummy tiles ("Red 7"). Uses i18n keys
    (`card.rank.K`, `suit.H`, `card.spoken`: `"{rank} of {suit}"`) in all 25
    locales, because word order differs (e.g. Czech "srdcový král").
  - `announce(message, { politeness })`: on native it wraps
    `AccessibilityInfo.announceForAccessibility`; on web it writes to a single
    visually hidden `aria-live` region mounted in `app/_layout.tsx`. It also
    queues and de-duplicates, so a burst of bot moves doesn't talk over itself.
  - `useReducedMotion()`: OS setting (`AccessibilityInfo.isReduceMotionEnabled`
    plus `prefers-reduced-motion` on web) OR'd with the in-app setting.
  - `useFontScale()`: `PixelRatio.getFontScale()` / browser zoom, clamped.
  - `<VisuallyHidden>` for web-only off-screen text.
- **`<Tip>` component** (the tooltip system, see Phase 4 for content)
  - Web: shows on hover after 500 ms and **on keyboard focus** immediately. It
    is hoverable (moving the pointer onto the bubble keeps it open), stays
    until hover or focus leaves, and closes on Esc (WCAG 1.4.13). It is linked
    by `aria-describedby`.
  - Native: shows on long-press as a bubble anchored to the element, closes on
    tap-outside or after 4 s. The same text becomes `accessibilityHint`, so
    VoiceOver/TalkBack read it after the label without needing the gesture.
  - Coarse pointers (touch on web) get the native behaviour.
  - Never the only place information lives. Anything a player *needs* stays
    on screen, and the tooltip explains it.
- **Focus ring.** One skinned `focusRing` token (2 px, ≥3:1 against the
  surface) applied via `:focus-visible` on web and through `Pressable`'s focus
  state for Android TV, iPad keyboards and Full Keyboard Access.
- **Tooling**
  - `eslint-plugin-react-native-a11y` on `app/` and `src/`, starting in warn
    mode and switched to error per phase as each area is cleaned.
  - `@axe-core/playwright` in `e2e/`: a new `a11y.spec.ts` that loads home,
    lobby, setup, rules, stats, account and a live match of each game, and
    fails on serious or critical violations. It starts report-only, and the
    baseline is committed.
  - `@testing-library/react-native` (add if missing) for role/name queries in
    jest.

## Phase 1: App shell, the screens outside a match (about 1–1.5 weeks)

Home, intro, lobby (list, setup, table), join, guest, auth/oauth, account,
settings, more, stats, rules, about, legal, offline, circle, replay list.

- Every `Pressable` gets a role, a label and a state (`selected`, `disabled`,
  `expanded`, `checked`). Icon-only buttons (menu, seat up/down, close,
  share, QR) get a label **and** a `<Tip>`.
- Headings: titles use `accessibilityRole="header"` (`<h1>`/`<h2>` on web) so
  rotor/heading navigation works. Rules sections become real headings.
- Forms (auth, join code, setup options): labels tied to inputs, errors
  announced and linked (`aria-invalid`, `aria-errormessage`), and no
  placeholder-as-label.
- Option pickers in setup (variation, house rules, bot strength) become proper
  radio groups and switches, with the rule's own description as the tooltip.
- Sheets and modals (`BotStrengthSheet`, `InviteBackSheet`, `WhySheet`,
  account menu): `accessibilityViewIsModal` on iOS, focus trapped and returned
  to the opener on web, and Esc / Android back closes them.
- Text scaling: drop fixed heights on text containers, let rows wrap, and keep
  `allowFontScaling` on for prose. Test at 200% (web zoom, iOS largest
  accessibility size, Android 2.0×).
- Contrast: verify every `classic`/`casino`/`heirloom` text/background pair
  hits 4.5:1 (3:1 for large text and UI). Fix the token, not the call site. Add
  a jest test that walks each skin's palette and fails on any pair below the
  threshold.
- Replace the hardcoded English labels listed above with i18n keys.
- Document language: `<html lang>` follows the chosen locale (`+html.tsx`).

## Phase 2: The match board for screen readers (about 2–3 weeks; the core)

The board is a spatial, real-time UI. The aim is that a blind player can play a
full game against bots and against humans.

**Structure.** Each region is a named group in a fixed order: seats → table
(piles, melds) → your hand → actions → recent moves. On web these are
`role="region"` landmarks with `aria-label`; on native they are
`accessibilityRole="summary"` / grouped containers, so a swipe moves between
regions.

**Cards**
- `accessibilityLabel = cardSpokenName(...)` everywhere a card is drawn:
  hand, melds, piles, flights and results.
- State goes in the label or state, not only in colour or lift: "selected",
  "face down", "wild", "just drawn", "can be played".
- Hand: "Your hand, 11 cards". Each card says its position ("3 of 11"), and
  the existing moveLeft/moveRight actions stay. Add actions **Select**,
  **Play to …** (one per legal drop spot from `dropSpotsFor`) and **Discard**
  where an offer allows it, so a card can be played without ever finding the
  drop zone.
- Piles: "Discard pile, top card Seven of Clubs, 14 cards", with the take /
  draw action as the pile's activation.
- Melds: "Run of Hearts, Five to Nine, Anna's", with lay-off as an action when
  legal.
- Face-down and other players' hands: count only, never codes.

**Seats.** "Anna, bot (medium), 7 cards, 42 points, dealer, their turn". The
badges (bot strength, agent, dealer, simplified) are in the label and also
have tooltips.

**Actions (`OfferBar`)**
- Every offer button has a spoken label including the amount ("Raise to 40").
- Disabled offers stay focusable (`aria-disabled`, not removed from focus
  order) and expose their refusal reason as the hint. Rework `ExplainOnPress`
  so the reason is reachable without a pointer, keeping the
  `Pressable disabled` contract RNW needs.
- Badge long-press explanations get a tap or activate alternative.

**Announcements** (via `announce()`, polite unless noted)
- "Your turn" (assertive) together with what you can do ("Draw or take the
  discard").
- Each opponent move as one sentence: "Anna drew from the stock", "Ben laid
  off Seven of Clubs on Run of Hearts". Built from the same fact/message keys
  `RecentMoves` already renders, so no new server text is needed for most
  games. Where a game's move log lacks a sentence, add a server key.
- Refusals ("That run needs three cards") and timeouts (assertive).
- Round and match results (already partly there).
- A **verbosity setting**: all moves / only my turn and results / off. Bots move
  quickly, so "all" rate-limits and summarises a burst ("3 moves: …").
- The recent-moves panel is reachable as a list for re-reading, with a
  "Repeat last" action.

**Timing.** Any turn timer on a table gets a "players need more time" table
option, off by default (as a house-rule option), so a screen-reader player
can host a table with 3× time or none. Bots never time out the human.

**On-demand summary.** A "Read table" action (web key `?`; native rotor or
magic-tap on iOS / an action on Android) speaks the whole state: scores, whose
turn, top discard, your melds, and the size of your hand. The zolik-games MCP
`observe` output already has the text shape this needs, so reuse its wording
approach.

## Phase 3: Keyboard, switch and external input (about 1–1.5 weeks)

The web comes first. iPad and Android hardware keyboards and Switch Control get
the same model through focusable elements.

- **Logical Tab order:** regions in the Phase 2 order, with one Tab stop per
  region (roving tabindex) and arrow keys inside it.
- **In the hand:** ←/→ move focus, Space selects or deselects, Shift+←/→ moves
  the card in the hand (the existing reorder), and Enter opens "play where?"
  listing the legal drop spots.
- **Global shortcuts (web), shown in a `?` help sheet and as tooltip
  suffixes:** `D` draw, `T` take discard, `Enter` confirm or primary offer,
  `U` undo, `R` read table, `M` recent moves, `Esc` cancel selection or close
  sheet. They never fire while typing in an input. Single-key shortcuts can be
  turned off in settings (WCAG 2.1.4).
- **Visible focus** at all times, with focus never trapped behind a panel and
  restored after the board re-renders on a new state (keyed by card code, not
  by index).
- **Targets:** at least 44×44 pt / 48 dp hit areas. The fanned hand overlaps
  by design, so add `hitSlop` where safe and rely on the hand's actions and
  keyboard path for precise picking. Verify with `elementFromPoint`, because
  fan overlap and inert overlays have bitten us before.
- **Motor:** no action requires drag, multi-touch, a long-press without an
  alternative, or precise timing.

## Phase 4: Tooltips everywhere (about 1 week, partly in parallel with 2–3)

Content work on top of the `<Tip>` component. Every tip is an i18n key in all
25 locales.

| Element | Tooltip |
|---|---|
| Icon buttons | What it does + shortcut ("Undo (U)") |
| Offer buttons | What happens; when disabled, why not |
| Cards in hand | Name, point value, "wild", and where it can go now ("Fits your run of Hearts; can lay off on Ben's set") from `fits()` / `dropSpotsFor()` |
| Cards on table | Name, who played it, meld it belongs to |
| Piles | What drawing or taking does in this game ("Taking the pile requires a matching pair" in Canasta) |
| Seat badges | Bot strength, agent, dealer, simplified mode, offline/host |
| Scores | How the number was made, linking to the scoring screen |
| Setup options | The rule's own `Rules()` text, the same source the rules screen renders |
| Stats / charts | The exact value |

The card point values and "wild" status come from the module. Prefer
the facts already on the wire. If a game doesn't expose them, add an optional
per-card `hintKeys` to the observation (server message keys, so `serverKeys.json`
is regenerated). Without them the tooltip falls back to the card name alone.
Cards also show a tooltip on web hover, so sighted mouse users get the benefit
too.

A setting to turn tooltips off for people who find them noisy (default on,
web hover delay adjustable).

## Phase 5: Vision, colour, motion and cognition (about 1–1.5 weeks)

- **Four-colour deck** (♠ black, ♥ red, ♦ blue, ♣ green) as a player setting,
  and the German deck's equivalent. Colours only, so it fits within skins.
- **High-contrast skin**: a fourth skin that meets 7:1 for text, uses solid
  borders on cards and melds, and drops texture and gradients. Selectable
  automatically when the OS "Increase contrast" / `prefers-contrast: more` /
  Android high-text-contrast is on.
- **Larger cards** setting (Normal / Large / Extra large), wired into
  `useMetrics` so layout stays measured and consistent. It is also the
  answer to OS font scale on the board, where free text scaling would break
  the fan maths. The board text uses `maxFontSizeMultiplier` together with
  this setting, and prose screens scale freely.
- **Last Card:** confirm the `plain` face also carries the colour's shape, not
  colour alone, and add the colour name to the spoken label ("Blue 7").
- **Reduced motion:** with `useReducedMotion()` on, card flights become
  instant moves or fades, `ResultsFlash` stays static, there are no
  arrivals or bounces, and auto-scroll is not animated. Nothing flashes more
  than three times per second.
- **Cognitive:** consistent placement of actions; plain-language refusal
  reasons (already a principle); "Hint" and the rules screen one tap away
  from every match; a confirm step (setting) before irreversible moves like
  going out or discarding your last card.
- **Sound** (if and when added): never the only cue; captions or visual
  equivalents.

## Phase 6: Verification and release (ongoing, final pass about 1 week)

- **Automated in CI:** axe spec switched from report-only to gating; a11y lint
  as errors; the skin contrast test; jest role/name tests for CardView, HandZone,
  OfferBar, SeatStrip and the sheets.
- **e2e:** a "screen-reader path" spec per game that plays a match using only
  role/name locators and keyboard (no coordinates, no testIDs). That proves
  the board is operable without sight or a mouse. Offer-dependent situations
  need several matches, as with the offer-presence e2e.
- **Manual scripts** (checklists in `docs/`, run before each release that
  touches the board):
  - VoiceOver on iOS (iPhone and iPad with keyboard), TalkBack on Android,
    VoiceOver + Safari and NVDA + Firefox/Chrome on web
  - Switch Control on iOS
  - 200% zoom / largest Dynamic Type
  - Reduce Motion, Increase Contrast, grayscale
  - Accessibility Inspector (Xcode) and Accessibility Scanner (Android) for
    targets and contrast
- **User testing:** at least one session each with a blind screen-reader user,
  a low-vision user and a motor-impaired (switch/keyboard) user before calling
  it done. Real use always finds what checklists miss.
- **Accessibility statement** page under legal (conformance level, known gaps,
  how to report a problem), linked from About. Relevant under the European
  Accessibility Act for EU distribution.
- **Store metadata:** fill in Apple's Accessibility Nutrition Labels and the
  Play Store accessibility details once the native pass is done.

## Ordering and rough size

| Phase | Web | iOS / Android | Est. |
|---|---|---|---|
| 0 Foundations | ✓ | ✓ | 1 wk |
| 1 App shell | ✓ | ✓ | 1–1.5 wk |
| 2 Board for screen readers | ✓ | ✓ (most effort on native verification) | 2–3 wk |
| 3 Keyboard / switch | primary | iPad/Android keyboard, Switch Control | 1–1.5 wk |
| 4 Tooltips content | hover + focus | long-press + hint | 1 wk (parallel) |
| 5 Vision / motion / cognition | ✓ | ✓ | 1–1.5 wk |
| 6 Verification | axe + e2e | manual VO/TalkBack | ongoing + 1 wk |

Total ≈ 8–10 weeks of focused work. Each phase ships on its own PR(s). Phase 2
can split per game (Žolíky first, since it has the richest board, then
Canasta, Gin Rummy, Hold'em, Blackjack, Mariáš, Last Card, Klondike, Rummy
Tiles), using the generic shell so most games come along for free.

## Open questions

1. **Verbosity default** for move announcements: "all" (most informative,
   chatty with three bots) or "my turn + results"?
2. **Turn timers**: are any tables timed today in a way a slow screen-reader
   player would hit? If not, Phase 2's timing item is just a guard.
3. **Card hints from the server** (Phase 4): acceptable to add optional
   `hintKeys` to the observation protocol, or keep tooltips to client-derived
   info only?
4. **Conformance claim**: do we want to publicly state WCAG 2.2 AA, or "aims to
   meet" until user testing is done?
