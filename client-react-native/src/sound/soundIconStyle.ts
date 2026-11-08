/**
 * How "muted" is drawn. Apple and Android put a slash through the speaker;
 * Windows, and most of the web, swap the waves for an X. The X is also the
 * fallback for anything we cannot name.
 */
export type MutedStyle = 'slash' | 'x';

/**
 * The convention of the system the player is on. A browser reports its host in
 * the user agent, so the web build (and the Mac and Windows shells around it)
 * follows the machine rather than always looking like one OS.
 */
export function mutedStyleFor(os: string, userAgent = ''): MutedStyle {
  if (os === 'ios' || os === 'android' || os === 'macos') return 'slash';
  // iPadOS asks for desktop sites and says "Macintosh", which lands the same way.
  return /Macintosh|Mac OS X|iPhone|iPad|iPod|Android/.test(userAgent) ? 'slash' : 'x';
}
