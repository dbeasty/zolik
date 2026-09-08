import { Platform, Share } from 'react-native';

import { ZOLIK_BASE_URL } from '@/src/config';

/**
 * The link a host hands to the people they want at their table.
 *
 * The whole point of the feature is that a host stops dictating six characters
 * down a phone line and instead sends something clickable, so the one thing
 * this module must never do is produce a URL that works on the host's machine
 * and nowhere else. Three facts decide which base a link is built on, in this
 * order:
 *
 *  1. **The browser's own origin, on web.** This is not a guess: the page is
 *     being served from there right now, so it is by definition a place the
 *     app answers. It is also the only base that is right in development,
 *     where Expo serves the client on :8114 while the server's configured
 *     public base names :8090 — a bundle that trusted the server there would
 *     hand out links to a port with no client behind it.
 *  2. **The server's own answer**, everywhere else. A phone talking to a LAN
 *     address has nothing better to offer; the server knows its public name
 *     and puts it in `inviteUrl`.
 *  3. **The API base**, if the server is old enough not to send one. Correct
 *     in production, where the API and the client are one origin by
 *     construction (the bundle is compiled into the binary).
 *
 * Kept as pure functions taking their inputs, rather than reaching for
 * `window` and the session themselves, because the ordering above is the part
 * worth testing and a test should not have to fake a browser to do it.
 */

/** The client route an invite link points at. Must match `match.InvitePath`. */
export const INVITE_PATH = '/join/';

/** Where the running client is reachable, or '' when that cannot be known. */
export function currentOrigin(): string {
  // Guarded rather than assumed: this same module runs under jest-expo's
  // node environment and inside a native bundle, and in both `window` is
  // either absent or has no meaningful location.
  if (Platform.OS !== 'web') return '';
  if (typeof window === 'undefined' || !window.location?.origin) return '';
  return window.location.origin;
}

/**
 * The link for a table, given what the server said about it.
 *
 * Returns '' when there is nothing honest to build one from — the caller shows
 * the join code alone, which is all a host ever had before links existed. A
 * "Copy link" button that copies `undefined/join/` is worse than no button.
 */
export function inviteUrlFor(
  table: { joinCode?: string; inviteUrl?: string },
  origin: string = currentOrigin(),
): string {
  const code = (table.joinCode ?? '').trim();
  const fromServer = (table.inviteUrl ?? '').trim();

  // On web the origin wins, but the *path* still comes from the server when it
  // sent one — the route is the server's to name (see match.InvitePath), and
  // this keeps a client built before a route change from minting dead links.
  if (origin) {
    if (fromServer) return swapOrigin(fromServer, origin);
    if (code) return origin + INVITE_PATH + encodeURIComponent(code);
    return '';
  }

  if (fromServer) return fromServer;
  if (code && ZOLIK_BASE_URL) {
    return ZOLIK_BASE_URL.replace(/\/$/, '') + INVITE_PATH + encodeURIComponent(code);
  }
  return '';
}

/**
 * Replace the scheme+host of an absolute URL, keeping everything after it.
 *
 * Deliberately string surgery over `new URL()`: React Native's URL polyfill is
 * partial, and this runs on native too by way of the tests. A URL that does
 * not parse is returned untouched, which is the safe direction — a link
 * straight from the server is a worse guess than a rewritten one, never a
 * broken one.
 */
function swapOrigin(url: string, origin: string): string {
  const match = /^[a-z][a-z0-9+.-]*:\/\/[^/?#]*/i.exec(url);
  if (!match) return url;
  return origin.replace(/\/$/, '') + url.slice(match[0].length);
}

/**
 * Pull the join code back out of a pasted link.
 *
 * The join box has always accepted a code or a match id, and now that links
 * exist people will paste one in there too — pasting the thing you were sent
 * is the obvious move, and answering it with "no such table" would be the
 * client being pedantic about a format it minted itself. Anything that is not
 * a link is handed back trimmed, so a typed code still behaves exactly as it
 * did.
 */
export function codeFromInviteInput(input: string): string {
  const trimmed = input.trim();
  const at = trimmed.toLowerCase().lastIndexOf(INVITE_PATH);
  if (at === -1) return trimmed;
  const tail = trimmed.slice(at + INVITE_PATH.length);
  // Stop at whatever a chat client may have stuck on the end — a query string,
  // a fragment, a trailing slash, or the space before the next word.
  const code = tail.split(/[/?#\s]/)[0] ?? '';
  try {
    return decodeURIComponent(code);
  } catch {
    // A stray '%' in a pasted link is not worth an exception; the raw text is
    // a better answer than none, and the server will refuse it clearly.
    return code;
  }
}

/**
 * Put the link somewhere the host can paste it from.
 *
 * Two different gestures wearing one name, because they are the same intent on
 * platforms that express it differently: on web, copying to the clipboard is
 * what "share this" means, and on a phone it is the system share sheet — the
 * sheet is the point on mobile, since the recipient is one tap away in
 * whichever messenger the host actually uses.
 *
 * Reports whether it got as far as handing the link over, so the caller can
 * say "Copied" only when it is true and fall back to showing the URL as
 * selectable text when it is not. Clipboard access is refused often enough —
 * an insecure origin, a browser that wants a fresher user gesture — that a
 * silent failure here would look like a dead button.
 */
export async function shareInviteLink(url: string, message?: string): Promise<boolean> {
  if (!url) return false;

  if (Platform.OS !== 'web') {
    try {
      // React Native's own Share, rather than a new dependency: the sheet is
      // the platform's, and this is the whole of what we need from it.
      const result = await Share.share({ message: message ? `${message}\n${url}` : url, url });
      return result.action !== Share.dismissedAction;
    } catch {
      return false;
    }
  }

  return copyToClipboard(url);
}

/** Clipboard write, with the pre-secure-context fallback still in place. */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (!text) return false;
  try {
    const clipboard = (globalThis as { navigator?: Navigator }).navigator?.clipboard;
    if (clipboard?.writeText) {
      await clipboard.writeText(text);
      return true;
    }
  } catch {
    // Denied, or not a secure context. Fall through and try the old way.
  }

  // navigator.clipboard needs HTTPS or localhost. A host running the dev stack
  // on their LAN address is on neither, and that is precisely the person most
  // likely to be sharing a link right now.
  try {
    const doc = (globalThis as { document?: Document }).document;
    if (!doc?.body) return false;
    const area = doc.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'fixed';
    area.style.opacity = '0';
    doc.body.appendChild(area);
    area.select();
    const ok = doc.execCommand?.('copy') ?? false;
    doc.body.removeChild(area);
    return ok;
  } catch {
    return false;
  }
}
