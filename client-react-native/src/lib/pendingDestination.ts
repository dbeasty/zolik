import { storage } from '@/src/context/SessionContext';

/**
 * Where somebody was going, held while they sign in.
 *
 * A shared link almost never arrives at a signed-in app. The recipient taps it
 * in a chat, the client opens on a device that has never been used to play,
 * and the very first thing that has to happen is picking a name — by which
 * point the destination from the URL is two screens ago and the sign-in
 * screens all finish by navigating somewhere fixed. Without this, following a
 * link dropped the visitor on the main menu having forgotten what they
 * clicked, which is the one outcome the feature exists to prevent.
 *
 * It stores a *route*, not an invite code, and that is a deliberate widening.
 * The code version served `/join/[code]` alone, so `/match/[matchId]` — the
 * other link people actually share, and the one a player follows back to their
 * own table — had nothing to hand and simply rendered "Connecting…" for ever
 * against a socket it had no token to open. Two screens wanting the same
 * handoff is what makes the route the right thing to hold: the far end then
 * navigates rather than reassembling a URL it has to know the shape of.
 *
 * It is a note about a navigation in progress, not a preference: written on
 * the way into sign-in and consumed exactly once on the way out. Consuming it
 * is what keeps a stale route from hijacking the menu days later — see
 * `consumePendingDestination`.
 */

const KEY = 'zolik_pending_destination';

/** How long an unconsumed note stays interesting, in milliseconds. */
const TTL_MS = 30 * 60 * 1000;

type Note = { path: string; at: number };

/**
 * Remember where to go once there is a session.
 *
 * Only an in-app route: a note is replayed into `router.replace` by a screen
 * that cannot judge it, so anything that is not a path rooted at `/` is
 * refused here rather than trusted there. Storage is not a trusted input —
 * on web it is localStorage, which any script that reaches the origin can
 * write — and "navigate wherever this string says" is not a power worth
 * leaving lying around for the sake of two callers that only ever pass a
 * literal.
 */
export async function savePendingDestination(path: string): Promise<void> {
  const trimmed = path.trim();
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) return;
  try {
    await storage.setItem(KEY, JSON.stringify({ path: trimmed, at: Date.now() } satisfies Note));
  } catch {
    // Private windows refuse storage outright. The visitor then lands on the
    // menu after signing in and can follow the link again, which is a worse
    // experience than the one intended but not a broken one.
  }
}

/** Look without consuming — for a screen that wants to say where it is going. */
export async function peekPendingDestination(): Promise<string> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return '';
    const note = JSON.parse(raw) as Note;
    if (typeof note?.path !== 'string' || typeof note.at !== 'number') return '';
    // Re-checked on the way out as well as on the way in, because the note
    // outlives the build that wrote it: a rule tightened here would otherwise
    // not apply to whatever is already sitting in storage.
    if (!note.path.startsWith('/') || note.path.startsWith('//')) return '';
    // An expired note is discarded rather than followed. Half an hour is well
    // past the length of a sign-in and well short of "I opened the app the
    // next morning and it dragged me into a table that finished last night".
    if (Date.now() - note.at > TTL_MS) {
      await clearPendingDestination();
      return '';
    }
    return note.path;
  } catch {
    return '';
  }
}

/**
 * Take the note and forget it.
 *
 * Cleared before the navigation it triggers rather than after, so a table that
 * turns out to be full or already dealt leaves the visitor on a screen they
 * can escape from instead of being sent back to it on every visit to the menu.
 */
export async function consumePendingDestination(): Promise<string> {
  const path = await peekPendingDestination();
  if (path) await clearPendingDestination();
  return path;
}

export async function clearPendingDestination(): Promise<void> {
  try {
    await storage.deleteItem(KEY);
  } catch {
    // Nothing to do about it, and nothing worth telling a player.
  }
}
