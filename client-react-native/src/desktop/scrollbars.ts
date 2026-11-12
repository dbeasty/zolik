/**
 * Scrollbars that stay on screen in the desktop app.
 *
 * The Mac app draws the page in a WKWebView, and WebKit there uses the
 * system's overlay scrollbars: invisible until the content is already moving.
 * A board or a results table taller than its window gave no sign there was
 * anything below it, and a player with a trackpad set to "show scroll bars
 * automatically" never saw one at all. Styling `::-webkit-scrollbar` makes
 * WebKit (and WebView2, on Windows) switch to a classic scrollbar, which is
 * drawn whenever a box overflows and only then, and takes its own strip
 * rather than covering the rightmost or bottom row of what it scrolls.
 *
 * The desktop app only. In a browser the visitor's own scrollbar setting is
 * theirs, and the site does not overrule it.
 */
const CSS = `
::-webkit-scrollbar { width: 11px; height: 11px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb {
  background-color: rgba(128, 128, 128, 0.55);
  border-radius: 6px;
  border: 2px solid transparent;
  background-clip: padding-box;
}
::-webkit-scrollbar-thumb:hover { background-color: rgba(128, 128, 128, 0.8); }
::-webkit-scrollbar-corner { background: transparent; }
`;

const ID = 'zolik-desktop-scrollbars';

export function installDesktopScrollbars(): void {
  if (typeof document === 'undefined' || document.getElementById(ID)) return;
  const style = document.createElement('style');
  style.id = ID;
  style.textContent = CSS;
  document.head.appendChild(style);
}
