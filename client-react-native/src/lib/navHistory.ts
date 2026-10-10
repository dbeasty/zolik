/**
 * Forward, for the header's arrows on the website and the phones.
 *
 * A stack navigator only goes back: a screen it leaves is gone. So the
 * screens the ← arrow left are kept here, newest last, and → returns to them
 * in turn. Going anywhere else clears the list, the way a browser's forward
 * history is cleared by following a link. (The Mac app walks its window's own
 * history instead; see src/desktop/NavArrows.tsx.)
 */

let ahead: string[] = [];
/** Set by back() and forward() for the route change they cause, so that one change does not clear the list. */
let expecting: 'back' | 'forward' | null = null;
const listeners = new Set<() => void>();

function changed() {
  for (const l of listeners) l();
}

export function subscribeAhead(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** How many screens → can return to. */
export function aheadCount(): number {
  return ahead.length;
}

/** The ← arrow is about to leave `current`: → will come back to it. */
export function leavingBack(current: string) {
  ahead = [...ahead, current];
  expecting = 'back';
  changed();
}

/** The → arrow: the screen to go to, taken off the list, or null. */
export function takeForward(): string | null {
  const next = ahead[ahead.length - 1];
  if (next === undefined) return null;
  ahead = ahead.slice(0, -1);
  expecting = 'forward';
  changed();
  return next;
}

/**
 * Every route change. One the arrows caused keeps the list; any other —
 * a tap on a game, a link, a redirect — clears it.
 */
export function routeChanged() {
  if (expecting) {
    expecting = null;
    return;
  }
  if (ahead.length) {
    ahead = [];
    changed();
  }
}

/** For tests. */
export function resetNavHistory() {
  ahead = [];
  expecting = null;
  changed();
}
