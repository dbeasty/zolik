# Interactive demo guide (app tour + per-game lessons)

## As built

The design below shipped with these concrete choices (differences from the first sketch are marked):

- **Server.** `module.LessonProvider` (`internal/module/lesson.go`): `Lessons(cfg)` returns keyed steps (`say` / `act`, verbs, count, strict, anchors, rule ids). `GET /modules/{id}/lessons?variation=&opt.*=` serves them; `POST /matches` takes an optional `lessonId`, which deals the lesson's seed and marks the match `Tutorial`. Tutorial tables are kept out of statistics, `GET /users/me/tables` and the offline hand-up. *Difference: no scripted bots and no server-side step tracking.* Opponents are ordinary Easy bots on a fixed seed and progress lives on the client, so the match runtime is untouched.
- **Safety net.** `internal/lessontest` plays every lesson through the real engine for every variation and for every option choice, and checks each cited rule id exists in that table's `Rules()`. `TestEveryGameShipsLessons` fails a game that has none. `cmd/dump-lessons` locks the key list (`client-react-native/src/guide/lessonKeys.json`).
- **Client.** `src/guide/`: pure `progress.ts` (tested), `useGuide` (counts a move once the table accepts it; holds moves a step does not want), `GuideCard` (above the board, foldable, Exit always visible), `GuideTarget` (Attention ring), `GuideExitSheet` (Play a real game / Keep practising / Leave), `LearnButton` on the game page and on setup (so the lesson follows the selected variation and options), and the app tour (`tour.ts`, `TourBanner` drawn by `Screen`, entry on the intro and home screens).
- **Words.** `src/lib/locales/guide/<topic>.json`, one file per game, each holding all 24 languages; merged into the bundles in `i18n.ts`. `scripts/guide-merge.js` folds translations in; `scripts/guide-check.js` compares the English with the lesson keys.
- **Tests.** Go: lessontest, per-module `lessons_test.go`, match-level lesson table test. Client: `progress.test.ts`, `keys.test.ts`. E2E: `e2e/tests/guide.spec.ts`.

Adding a game's guide: write `internal/<game>/lessons.go` and `lessons_test.go` (copy prsi), fill `guide/<id>.json` (`en`), regenerate `lessonKeys.json`, translate, merge.

## Goal
A guided, hands-on session that teaches (a) how to use the app and (b) how to play each game, **using the variation and options the player has chosen**. It is separate from hints: hints answer "what should I play now?" inside a real match; the guide is a whole scripted session. The player can leave at any moment and either stop or play for real.

## Findings that shape the design (from the survey)
- Nothing like this exists. `docs/guidance-plan.md` Phases 0-4 (WhySheet, change markers, hint) are done; Phase 5 is deferred. `app/intro.tsx` is a one-time marketing screen.
- Engines are pure and seeded (`NewMatch(cfg, players, seed)`, `Apply`), so a scripted lesson is replayable and testable.
- Rules are already variation- and option-accurate: each module's `Rules(cfg)` resolves config through the same `resolveConfig` as `NewMatch`. Lessons should hook into the same resolution so they cannot drift.
- Seeds are not client-settable on `POST /matches`. A tutorial needs its own server-side entry point.
- The client has no spotlight/coach-mark component, but has `Attention.tsx` (ring), `Tip`, `Sheet`, and stable testIDs. The match socket always sends the whole viewer-filtered state, so a coach layer can sit on top without touching match state.
- The offline core (gomobile) exists on native only; web must use the online server.
- Copy rules: 24 locales, `serverKeys.json`, and the gamemcp bundle must all stay in sync.

## Design

### 1. Lesson = data, owned by the game module
Add an optional interface next to `RulesProvider` (`server/internal/module/lesson.go`):

```go
type LessonProvider interface {
    Lessons(cfg MatchConfig) ([]Lesson, error)   // ordered; cfg already resolved
}
type Lesson struct {
    ID, TitleKey string
    Seed int64                  // fixed, chosen/validated per config
    Seats []LessonSeat          // you + scripted opponents
    Steps []LessonStep
}
type LessonStep struct {
    ID string
    NarrationKey string; Params map[string]any   // keys, never prose
    Focus []Anchor                // zones/offers/testID-style anchors to spotlight
    Expect Expect                 // one of: Action(match), AnyOf(set), Acknowledge, Observe
    OnWrong string                // key; reuses ExplainRefusal ruleIds where the engine refuses
    OpponentScript []Action       // scripted replies played after the step
    RuleIDs []string              // "Read the rule" deep link into /rules?highlight=
    When Cond                     // include only for certain variation/options
}
```
- **Variation sensitivity** comes from `When` plus param-bearing narration, built by the same `resolveConfig` as `Rules()`. Examples: Canasta Samba adds the sequence step; Ginrummy Oklahoma adds the knock-card step; Blackjack vegas vs single changes dealer/shoe narration; Rummy Tiles first-lay requirement quotes the configured value; discard-pickup options swap a whole step.
- Steps reference rule ids so the tour can say "this is rule X" and link to the rules screen (already supports `highlight`).
- Shared building blocks (`module/lessonkit.go`) for common steps: "your turn", "draw", "discard", "scoring", "end of round", "rematch".

### 2. Tutorial matches on the server
- New endpoint `POST /tutorials {moduleId, variation, options, lessonId}` creates a **tutorial match**: a normal `Match` flagged `tutorial`, with the lesson's seed and scripted opponents (a `ScriptedBot` that plays `OpponentScript`, then falls back to the Easy bot). No new engine path, so the real UI renders it.
- `GET /tutorials/{id}/script` (or the first `match_state` frame) returns the current step. The server owns progress: it validates the player's action against `Expect` before applying. A wrong move gets `OnWrong` + rule ids and is **not applied**, so the lesson cannot be derailed.
- Tutorial matches are excluded from stats, history, "my games", notifications and rematch. They are garbage-collected on exit or after idle.
- Free-play steps (`Expect: AnyOf`) let the player choose among legal moves while narration still reacts, so late lessons feel like play rather than a script.

### 3. Exit at any time
A persistent **Exit guide** control on every guide screen and on the match header while a tutorial is live. Choices:
1. **Play for real** with the same variation and options (creates a normal match via the existing setup flow; setup choices persist already).
2. **Resume later** (progress stored per user + module + config hash).
3. **Leave**.
Hardware back and the leave sheet map to the same dialog. Because the guide is an overlay on a normal match, leaving never loses anything.

### 4. Client coach layer
New `src/components/coach/`:
- `CoachProvider`: reads the step from the tutorial socket frame, holds visible/minimised state.
- `useCoachTarget(anchorId)` registry: components register measured rects (built on `measureLayout`/`getInnerViewNode`; never `findNodeHandle` on web). Anchor ids map to existing testIDs (`hint-button`, `action-bar`, `score-line`, `explain-*`, hand cards, piles).
- `CoachCard`: bottom-anchored card with narration, Next/Got it, Back, "Read the rule", **Exit**. It must not cover the target, status line, or hint button (see the announcement-corner fix). Spotlight reuses `Attention` and respects reduced motion; the card is a live region for screen readers. Layout must be size-neutral (no typography changes from skins).
- Press-through: when the step expects a move, the spotlighted control stays tappable and everything else is inert; verify with `elementFromPoint` (overlays can hide under inert content).
- Resumes correctly after reload via the socket frame; stack screens stay mounted, so use `useIsFocused`.

### 5. Entry points
- **First run**: `intro.tsx` ends with "Take a 2-minute tour" or "Skip".
- **Home**: "Guide" entry; **Game page** (`lobby/games.tsx`): "Learn this game" beside Rules.
- **Setup** (`lobby/setup.tsx`): "Learn with these settings" uses the variation/options currently selected, so the lesson matches what they are about to play.
- **Match screen menu**: "Practice / guide" while idle in a lobby.

### 6. App-usage tour (no engine needed)
A non-match tour over real screens, scripted client-side with the same coach layer: Home (pick a game, join by code), game page (solo / friends / bots), setup (variations and options, bot strength), table lobby (seats, add bot, start), match basics (status line, action bar, hints vs. guide, score sheet, history/log, leave and resume), My games, Settings (language, accessibility, skins), offline/nearby on native. Hidden or irrelevant steps (native only, signed-in only) are skipped by capability checks.

### 7. Content and i18n
- Narration keys live in `en.ts`; keys the server emits go through `serverKeys.json` (regenerate), then `go generate ./internal/gamemcp`, then all 23 translations with matching `{placeholders}`. Keys must be static literals so they appear in the manifest.
- Each narration step is written against the **written rules** (reuse rule keys/params rather than restating numbers) to avoid divergence.

## Guarantees (tests = the variation safety net)
1. **Lesson replay test per module**: for every variation x relevant option combination from the descriptor, build the lessons, drive them through the real `Apply` with the scripted answers, and assert every expected action is legal, every goal is reached, and the lesson ends in a valid state. A new variation/option that breaks a lesson fails CI.
2. **Coverage test** (like `ruleindexcheck`): each module must either provide lessons or be listed in an explicit allowlist; each option that changes `Rules()` must be either referenced by a `When`/params or declared "lesson-neutral".
3. **Seed finder tool** (`cmd/lessonseed`): searches seeds that exhibit the teachable scenario (e.g. a meld available on turn 2) under a config, so authors do not hand-hunt deals. Chosen seeds are committed per config class.
4. Unit tests for wrong-move handling (not applied, rule ids returned), exit/resume, and stats exclusion.
5. E2E (Playwright, default bot think time): run each module's first lesson to the end; exit mid-way and "Play for real"; assert what the user actually sees (spotlight present, card text, not covering the status line), not just that the socket answered.
6. Check the iOS sim and mobile width (plain custom width, not the mobile preset).

## Phasing
- **P0 Spike (small)**: `LessonProvider`, `ScriptedBot`, `/tutorials`, `CoachProvider/Card`, exit flow, one module (Klondike or Prsi: simple, solo/low-bot) end to end, with replay test.
- **P1 App tour**: client-side tour, entry points, first-run hook, per-user progress store (same storage pattern as `introStore`).
- **P2 Core lessons**: Žolíky, Prsi, Last Card, Sedma, Snaps, Blackjack (3 variations), each with a basics lesson, including variation `When` branches and tests.
- **P3 Complex games**: Canasta (3 variations), Gin Rummy (2), Rummy Tiles, Hold'em, Mariáš, Lora, Ferbl, Okobere; multi-lesson (basics, scoring, advanced) where warranted.
- **P4 Polish**: translations for all copy, accessibility pass (screen reader order, reduced motion), progress/replay of lessons from the game page, optional hint-linking ("try this in a real game with hints on").
- **Later**: AI guide (Phase 5 of `guidance-plan.md`), if free-form questions are wanted; web can use the online server, offline native uses the embedded core unchanged.

## Open decisions
1. **Scripted bots vs. real bots with fixed seed.** Recommendation: scripted for the teaching beats, then real Easy bot for free play, so lessons stay deterministic.
2. **Lesson length.** Recommendation: a ~3-minute "basics" lesson per game first; scoring/advanced as optional follow-ups.
3. **Progress sync.** Per-device first; account sync later if wanted.
4. **Where lessons ship.** Server-side `Lessons(cfg)` (recommended, so variations stay in one place and the replay test can run in Go) vs. client-side JSON (faster to author, but cannot be validated against the engine).
