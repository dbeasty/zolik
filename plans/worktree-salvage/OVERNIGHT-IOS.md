# Overnight iOS log (branch claude/overnight-ios, worktree ../zolik-overnight-ios)

## Setup
- Core: scripts/build-mobile-core.sh ios OK (34s). prebuild + `expo run:ios` on iPhone 17 Pro (iOS 26.5) sim 6FC613FE.
- GOTCHA: first `expo run:ios` in a worktree failed in "[CP-User] Build ExpoModulesJSI xcframework" ("hermes/hermes.h not found") because
  shared node_modules/expo-modules-jsi/apple/.DerivedData held a clang module cache from another worktree's Pods path. `rm -rf` that dir fixed it.
- Local cloud: /tmp/ovn-ios/zolik-server on :18290 (kdb, no redis). Metro dev-client :18214 with TMPDIR=/tmp/ovn-ios/metro-tmp/.
- Screenshot px -> device points: divide displayed coords by 2.29.

## Findings / fixes
1. FIXED  double top safe-area inset under header: 62pt dead band on every screen (Screen.tsx, match, replay). commit: see git log "no second top inset".
2. FIXED  match header shows raw ids "holdem · holdem" -> localised names (e2e now asserts data-module).
3. FIXED  only 6 of 15 games had module.<id> names: match header, My games, invites showed ids ("ginrummy"). Added 9 keys x 24 locales + gamemcp regenerate.
4. FIXED  long first turn prompt (Mariáš "Say dobrá, or take it over...") ran under the log box corner buttons (MoveAnnouncements firstOpenRow on prompt-0 when no move row).
5. FIXED  Solitaire tableau squeezed beside Foundations on native (flex-basis:100% didn't wrap in Yoga); 7 columns ran off screen. besideRows() gives wide spreads their own row.
Surveyed (iPhone 17 Pro, iOS 26.5, 402pt): all 15 modules deal & render vs bots; Resume survives app reload.
Not fixed / notes:
- 'sel.needMore' = "Select {n} card(s)" lazy plural (en + 23 locales) - cosmetic.
- Rummy Tiles hand tiles show "?" faces (known B1 work, memory gin-rummy-phase-a-shipped).
- Gin Rummy/Rummy Tiles seat card shows a bare "0" fact line under Cards (ginrummy.seat.score = "{value}") - label-less number, duplicates header score; server data design.
- Dev client floating gear button covers the avatar in header (dev build only).
- iPad: MCP sim tool refused access to iPad Pro 11 (M5) (user not at keyboard); not tested natively.
6. FIXED  status word now localised ("In progress"); offline (phone-hosted) match bar printed "· Server: this phone" under arrows/avatar on a 402pt phone -> only shown at width >= 480 (explainer still says it).
7. FIXED  offline screens + NSLocalNetwork/NSBluetooth prompts said "Zolik"; now "Jokerless" (96 locale lines + plugin).
Note: native header title ignores maxWidth/flexShrink (UIKit measures unbounded), hence hiding the label.
Not fixed: home tile "In progress" quiet badge wraps to 2 lines beside the Resume button at 402pt (cosmetic).
8. FIXED  cloud refused handed-up blackjack tables where everyone had left ("replaying its moves does not finish the game"): importer demanded non-empty Standings; now asks mod.Finished. Test: TestABlackjackTableEveryoneLeftReplaysToItsEnd. phone-table.spec made to actually play (it sorted 'leave' first) - passes 1.4m.
Other test notes: Playwright (generic-shell, every-game-ui, klondike, board-layout, adaptive-layout, abandoned-table, untranslated-sweep, nav-arrows, rules, replay, group-indices, a11y-board, tap-to-play, hand-selection, announcements, mobile-board-controls) pass; controls-below-hand holdem flakes ~1/6 (controls y=311 vs hand 1366) - pre-existing/not investigated. go test -short ./... all ok; jest all pass.
Not fixed: "Your side needs 1 more canastas" (prompt.canastasNeeded has no singular; server key + 23 locales).
Not fixed: folded log line "Steady Stanisl…" truncated by the standing-fact in the box corner (Prší with suit in play) - design tradeoff.
9. FIXED  Hints sheet (RN Modal) stayed over About/Stats/... after navigating away from the match (screens stay mounted): gated on useIsFocused.

## Second pass
- Matches played to results on the sim (driver plays the human seat over ws; app opened on the finished table): Prší (+Play again rematch OK), Canasta (3 deals), Sedma, Blackjack, Hold'em, Solitaire (gave up), Gin Rummy, Mariáš started (800-move run too slow, aborted). Results tables, Your record card, Back to games, replay screen all render. iPhone 16e pass: match/results/settings no clipping.
- FIXED singular wording via t() `<key>.1` convention (canastasNeeded, sel.needMore), 24 locales.
- FIXED 8 variation ids shown raw (holdem, omaha, draw, atlantic, vegas, oklahoma, zolik_classic, continental).
- FIXED gin results caption raw "knock" -> ginrummy.kind.* keys (undercut word is English in languages I was unsure of: bg el et ga hu lt lv mt); seat score line labelled "Score N" (Gin, Rummy Tiles).
- FIXED +not-found white screen in dark app; Home "In progress" badge single line.
- FIXED controls-below-hand holdem flake: test bug (bot folds first hand -> intermission layout); spec now deals on.
- Not fixed: results table uses ~2/3 of width; empty bordered log box right after a fresh deal; transient "table nearby"/"table waiting" banners cover the match header; one controls-below-hand holdem run timed out under 3 workers (passes 20/20 at 2 workers).
- Not done: language switch mid-match (switched to French earlier on Home only), hosting an offline table and bg/relaunch resume beyond the earlier Prší offline match, iPad.
