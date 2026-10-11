# Sound effects and voice announcer — plan

## Goal
Short sound effects as a game is played — a card dropped, a pile taken, your
turn, a round or a game ending — plus an optional voice that calls out plays
("Take 2", "Skip", "Last card!"). Players can switch it off in Settings and with
a quick speaker button in the match header.

## Design
- **Cues come from the board, not from games.** Like `flights.ts` and
  `changes.ts`, `src/sound/cues.ts` compares the previous and next board:
  cards landing somewhere → `card`; a face-up pile losing cards to another zone
  → `take`; deals and reshuffles (bulk moves) → silence. No module is named, so
  a game added tomorrow is covered.
- **Endings** reuse `useResultsFlash` (already "an ending that is news", silent
  on arrival): round → `round`, match → `win` / `lose`.
- **Your turn** → a soft `turn` bell, on the false→true edge only.
- **Voice** is keyed on the move's own label key (`sayFor`): draw penalties,
  skip, reverse, last card, caught, knock/gin; plus "You win!" / "Game over".
  Unrecognised moves are not spoken. Phrases are translated message keys
  (`sound.say.*`) spoken in the app language through `expo-speech`.
- **Assets** are synthesised by `scripts/gen-sounds.js` into
  `client-react-native/assets/sounds/*.wav` (a few KB each, no licensing).
  Played with `expo-audio`, one code path for web and native.
- **Preferences** (`src/sound/prefs.ts`, same shape as the accessibility
  prefs): `enabled` (default on) and `voice` (default off — the announcer is
  the part a table may least expect). The header button flips `enabled`;
  Settings → Sound has both.

## Files
`src/sound/{prefs,cues,player,useBoardSounds}.ts`, `SoundToggle.tsx`,
`SoundSettings.tsx`, wiring in `app/match/[matchId].tsx` and `app/settings.tsx`,
18 `sound.*` keys in all 23 locales.

## Native build
`expo-audio` and `expo-speech` are new native modules: dev-client and store
builds need a rebuild (`expo run:ios` / `expo run:android`); web needs nothing.

## Follow-ups (not in this change)
- Recorded or licensed sound sets, and a volume slider.
- More voice lines per game (Canasta take-pile, Rummy tiles).
- Sound for other players' turns in the lobby, and a per-game override.
