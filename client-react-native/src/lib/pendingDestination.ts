import { storage } from '@/src/context/SessionContext';

/**
 * The table somebody was invited to, held while they sign in.
 *
 * A shared link almost never arrives at a signed-in app. The recipient taps it
 * in a chat, the client opens on a device that has never been used to play,
 * and the very first thing that has to happen is picking a name — by which
 * point the code from the URL is two screens ago and the sign-in screens all
 * finish by navigating somewhere fixed. Without this, following an invite
 * dropped the invitee on the main menu having forgotten what they clicked,
 * which is the one outcome the feature exists to prevent.
 *
 * It is a note about a navigation in progress, not a preference: written on
 * the way into sign-in and consumed exactly once on the way out. Consuming it
 * is what keeps a stale code from hijacking the menu days later — see
 * `consumePendingInvite`.
 */

const KEY = 'zolik_pending_invite';

/** How long an unconsumed note stays interesting, in milliseconds. */
const TTL_MS = 30 * 60 * 1000;

type Note = { code: string; at: number };

/** Remember the table to return to once there is a session. */
export async function savePendingInvite(code: string): Promise<void> {
  const trimmed = code.trim();
  if (!trimmed) return;
  try {
    await storage.setItem(KEY, JSON.stringify({ code: trimmed, at: Date.now() } satisfies Note));
  } catch {
    // Private windows refuse storage outright. The invitee then lands on the
    // menu after signing in and can follow the link again, which is a worse
    // experience than the one intended but not a broken one.
  }
}

/** Look without consuming — for a screen that wants to say where it is going. */
export async function peekPendingInvite(): Promise<string> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return '';
    const note = JSON.parse(raw) as Note;
    if (!note?.code || typeof note.at !== 'number') return '';
    // An expired note is discarded rather than followed. Half an hour is well
    // past the length of a sign-in and well short of "I opened the app the
    // next morning and it dragged me into a table that finished last night".
    if (Date.now() - note.at > TTL_MS) {
      await clearPendingInvite();
      return '';
    }
    return note.code;
  } catch {
    return '';
  }
}

/**
 * Take the note and forget it.
 *
 * Cleared before the navigation it triggers rather than after, so a table that
 * turns out to be full or already dealt leaves the invitee on a screen they
 * can escape from instead of being sent back to it on every visit to the menu.
 */
export async function consumePendingInvite(): Promise<string> {
  const code = await peekPendingInvite();
  if (code) await clearPendingInvite();
  return code;
}

export async function clearPendingInvite(): Promise<void> {
  try {
    await storage.deleteItem(KEY);
  } catch {
    // Nothing to do about it, and nothing worth telling a player.
  }
}
