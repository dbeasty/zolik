import { storage } from '@/src/context/SessionContext';

/**
 * Whether this device has already been shown the first-run intro screen.
 *
 * A device flag, not an account one: the point is to spare a *phone* the
 * screen twice, and a guest who never signs in still gets that. Storage
 * failure (private windows refuse it outright) degrades to "show it again" —
 * a repeat is a worse experience than the one intended, not a broken one, so
 * every read/write here is non-fatal by design, same as `localeStore.ts`.
 */

const KEY = 'zolik_seen_intro';

/** True once `markIntroSeen` has run on this device. */
export async function hasSeenIntro(): Promise<boolean> {
  try {
    return (await storage.getItem(KEY)) !== null;
  } catch {
    return false;
  }
}

/** Records that the intro has been shown, so it will not be shown again. */
export async function markIntroSeen(): Promise<void> {
  try {
    await storage.setItem(KEY, '1');
  } catch {
    // Nothing to do about it, and nothing worth telling a player — worst
    // case they see the intro again next time.
  }
}
