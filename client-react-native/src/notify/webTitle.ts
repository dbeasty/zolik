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
