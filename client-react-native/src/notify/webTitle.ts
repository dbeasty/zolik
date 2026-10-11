/**
 * "(2) Jokerless" in the browser tab while tables are waiting.
 *
 * The fallback for a browser that shows no notifications — refused, or
 * Safari in a plain tab — so a player who switched tabs can still see that
 * something arrived. The router rewrites the title on every navigation, so
 * the count is re-applied whenever the title changes rather than set once.
 *
 * Returns the cleanup that puts the plain title back.
 */
const PREFIX = /^\(\d+\) /;

export function showUnreadInTitle(count: number): () => void {
  if (typeof document === 'undefined') return () => {};
  const apply = () => {
    const plain = document.title.replace(PREFIX, '');
    const next = count > 0 ? `(${count}) ${plain}` : plain;
    if (document.title !== next) document.title = next;
  };
  apply();
  const titleEl = document.querySelector('title');
  if (!titleEl || typeof MutationObserver === 'undefined') {
    return () => {
      document.title = document.title.replace(PREFIX, '');
    };
  }
  const observer = new MutationObserver(apply);
  observer.observe(titleEl, { childList: true, characterData: true, subtree: true });
  return () => {
    observer.disconnect();
    document.title = document.title.replace(PREFIX, '');
  };
}

/**
 * Blinks a message in the tab's title until the player looks at the tab.
 *
 * For news that happened while the tab was hidden - somebody sat down at the
 * table this player is waiting at - in a browser that cannot show a system
 * notification: a page served over plain http by a phone in the room never
 * can. A later message replaces an earlier one. Nothing happens to a tab that
 * is in front, where the banner says it.
 */
let stopFlash: (() => void) | null = null;

export function flashTitle(message: string): void {
  if (typeof document === 'undefined' || !document.hidden) return;
  stopFlash?.();
  let showing = false;
  const plain = () => document.title.replace(/^● .*? · /, '');
  const timer = setInterval(() => {
    showing = !showing;
    const base = plain();
    document.title = showing ? `● ${message} · ${base}` : base;
  }, 1000);
  const stop = () => {
    clearInterval(timer);
    document.title = plain();
    document.removeEventListener('visibilitychange', onVisible);
    stopFlash = null;
  };
  const onVisible = () => {
    if (!document.hidden) stop();
  };
  document.addEventListener('visibilitychange', onVisible);
  stopFlash = stop;
}
