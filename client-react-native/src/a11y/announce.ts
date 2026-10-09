import { AccessibilityInfo, Platform } from 'react-native';

import { getA11yPrefs } from '@/src/a11y/prefs';

/**
 * Telling a screen reader something happened that nobody pressed.
 *
 * A card a bot drew, a turn coming round, a move refused: a sighted player
 * sees each of them happen on the board, and a player listening to it hears
 * nothing unless the app says so. One function for all of it, so every
 * platform says it the same way:
 *
 * - native: `AccessibilityInfo.announceForAccessibility`, which VoiceOver and
 *   TalkBack speak over whatever has focus;
 * - web: one pair of visually hidden `aria-live` regions mounted at the root
 *   (see `LiveRegion`), written here. Two, because `assertive` interrupts and
 *   `polite` waits, and a region cannot be both.
 *
 * Messages are queued and spaced rather than fired at once. Three bots moving
 * in a second would otherwise each cut off the last, and the player would
 * hear the third move only. A message identical to the one just spoken is
 * dropped — a re-render is not news.
 */
export type Politeness = 'polite' | 'assertive';

type Sink = (message: string, politeness: Politeness) => void;

let webSink: Sink | null = null;

/** Called by `LiveRegion` when it mounts; null when it unmounts. */
export function setWebAnnouncer(sink: Sink | null) {
  webSink = sink;
}

const GAP_MS = 700;
const queue: { message: string; politeness: Politeness }[] = [];
let draining = false;
let last = '';
let lastAt = 0;

function speak(message: string, politeness: Politeness) {
  if (Platform.OS === 'web') {
    webSink?.(message, politeness);
    return;
  }
  AccessibilityInfo.announceForAccessibility(message);
}

function drain() {
  const next = queue.shift();
  if (!next) {
    draining = false;
    return;
  }
  draining = true;
  speak(next.message, next.politeness);
  setTimeout(drain, GAP_MS);
}

export function announce(message: string, politeness: Politeness = 'polite') {
  const text = message.trim();
  if (!text) return;
  const now = Date.now();
  if (text === last && now - lastAt < 2000) return;
  last = text;
  lastAt = now;
  if (politeness === 'assertive') {
    // An interruption jumps the queue: "your turn" must not wait behind
    // three sentences about what the bots did.
    queue.unshift({ message: text, politeness });
  } else {
    // A long burst is summarised by dropping its oldest, not by growing a
    // backlog the player would still be hearing two turns later.
    if (queue.length >= 4) queue.shift();
    queue.push({ message: text, politeness });
  }
  if (!draining) drain();
}

/** For tests: forget what was said and what is waiting. */
export function resetAnnouncer() {
  queue.length = 0;
  draining = false;
  last = '';
  lastAt = 0;
}

/**
 * What kind of game news a message is, which decides whether the player's
 * verbosity choice lets it through: `move` is somebody else's move, `turn` is
 * the game coming round to you, `result` is a hand or match ending, `refusal`
 * is your own move being turned down.
 */
export type GameNews = 'move' | 'turn' | 'result' | 'refusal';

/** `announce`, filtered by Settings → Accessibility → Announcements. */
export function announceGame(message: string, kind: GameNews) {
  const level = getA11yPrefs().announce;
  if (level === 'off') return;
  if (level === 'mine' && kind === 'move') return;
  announce(message, kind === 'turn' || kind === 'refusal' ? 'assertive' : 'polite');
}
