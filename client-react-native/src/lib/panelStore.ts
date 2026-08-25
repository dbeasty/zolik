import { storage } from '@/src/context/SessionContext';

/**
 * Remembering which panels a player put away.
 *
 * The same shape and the same reasons as `handOrderStore.ts`: a preference
 * about how the board looks, not a fact about the game, so it never leaves
 * the device and no module knows it exists. Keyed per match because a panel
 * put away in one deal means nothing in the next one.
 */

const KEY = 'zolik_panels';

/** How many matches are remembered at once — see `handOrderStore.ts` for why this is bounded. */
const KEEP = 5;

type Entry = { matchId: string; minimized: string[] };

async function read(): Promise<Entry[]> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as Entry[]) : [];
  } catch {
    // Unreadable or from an older shape. A closed panel is not worth an
    // error on a screen, so every panel just starts open again.
    return [];
  }
}

export async function loadMinimized(matchId: string): Promise<string[]> {
  const entries = await read();
  return entries.find((e) => e.matchId === matchId)?.minimized ?? [];
}

export async function saveMinimized(matchId: string, minimized: string[]): Promise<void> {
  try {
    const entries = await read();
    const next = [{ matchId, minimized }, ...entries.filter((e) => e.matchId !== matchId)].slice(
      0,
      KEEP,
    );
    await storage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Storage can be full, or refused outright in a private window. Losing a
    // preference is a small thing; interrupting a game over it is not.
  }
}
