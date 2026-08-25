import { storage } from '@/src/context/SessionContext';

/**
 * Remembering how a player left their hand.
 *
 * Arranging a hand is only worth doing if it stays done, and until now it
 * lasted exactly as long as the screen did — a reload, or walking away and
 * coming back, and the cards were in the module's order again.
 *
 * What is written down is card *names*, not slots: slot identity is minted at
 * runtime to tell two identical cards apart within a session, and means
 * nothing to the next one. Names are enough to put the hand back, because the
 * hand it is being applied to is the same hand.
 *
 * This never leaves the device. Arrangement is a view preference and the
 * server has no idea the feature exists, which is what lets every game have it
 * without any of them knowing.
 */

const KEY = 'zolik_hand_order';

/**
 * How many matches are remembered at once.
 *
 * Bounded on purpose. Keyed per match, because an arrangement means nothing in
 * a different deal — but that means one entry per match ever played, growing
 * forever, on a device whose storage is not ours to fill. A handful covers
 * every real case: the match in front of you, and the one or two you might
 * have going elsewhere.
 */
const KEEP = 5;

type Entry = { matchId: string; zones: Record<string, string[]> };

async function read(): Promise<Entry[]> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as Entry[]) : [];
  } catch {
    // Unreadable or from an older shape. An arrangement is not worth an error
    // on a screen, so it starts again from the order the server sent.
    return [];
  }
}

export async function loadHandOrder(matchId: string): Promise<Record<string, string[]> | null> {
  const entries = await read();
  return entries.find((e) => e.matchId === matchId)?.zones ?? null;
}

export async function saveHandOrder(
  matchId: string,
  zones: Record<string, string[]>,
): Promise<void> {
  try {
    const entries = await read();
    // Newest first, this match moved to the front, so the least recently
    // touched is the one that falls off the end.
    const next = [{ matchId, zones }, ...entries.filter((e) => e.matchId !== matchId)].slice(
      0,
      KEEP,
    );
    await storage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Storage can be full, or refused outright in a private window. Losing an
    // arrangement is a small thing; interrupting a game over it is not.
  }
}
