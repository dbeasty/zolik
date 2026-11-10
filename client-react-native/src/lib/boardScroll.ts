/**
 * Where the board's scroll view is, for the things a drag draws inside it.
 *
 * A card being carried is drawn relative to content the board scrolls — a card
 * out of the hand is pinned to the hand's row, a card off a meld moves by how
 * far the pointer has travelled from where it was picked up. Neither notices
 * the board scrolling under it: the pointer stays put, the content moves, and
 * the card moves with the content. With a board taller than the window the
 * edge autoscroll moved a carried card hundreds of pixels off the pointer, and
 * the drop is judged at the pointer, not at the card the player is watching.
 *
 * So the match screen reports every scroll here — from `onScroll`, and also
 * straight after each autoscroll step, because on the web `onScroll` arrives a
 * frame later and the card would visibly lag the pointer for it — and whatever
 * is carrying a card listens.
 *
 * One board at a time is on screen, so one module-level value is enough.
 */
type Listener = (y: number) => void;

let current = 0;
const listeners = new Set<Listener>();

/** The board's vertical scroll offset, as last reported. */
export function boardScrollY(): number {
  return current;
}

/** The board is now scrolled to `y`. Repeated values are ignored. */
export function reportBoardScroll(y: number): void {
  if (y === current) return;
  current = y;
  for (const l of [...listeners]) l(y);
}

/** Called with the new offset whenever the board scrolls. Returns the unsubscribe. */
export function onBoardScroll(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
