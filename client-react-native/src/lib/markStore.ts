import { storage } from '@/src/context/SessionContext';

/**
 * Marks a reader dropped while watching a replay.
 *
 * The same shape and the same reasons as `panelStore.ts` and
 * `handOrderStore.ts`: a note about where somebody was looking, not a fact
 * about the game, so it never leaves the device and no module knows it
 * exists. Keyed per match because a mark on move 40 of one game means nothing
 * in another.
 *
 * Deliberately not called a restore point. Nothing here restores anything —
 * these move the view and only the view — and a name that promised otherwise
 * would be the one place in this app where a word meant two things.
 */

const KEY = 'zolik_marks';

/** How many matches are remembered at once — see `handOrderStore.ts` for why this is bounded. */
const KEEP = 5;

type Entry = { matchId: string; frames: number[] };

async function read(): Promise<Entry[]> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as Entry[]) : [];
  } catch {
    // Unreadable or from an older shape. A lost mark is not worth an error on
    // a screen, so the replay just opens with none.
    return [];
  }
}

export async function loadMarks(matchId: string): Promise<number[]> {
  const entries = await read();
  return entries.find((e) => e.matchId === matchId)?.frames ?? [];
}

export async function saveMarks(matchId: string, frames: number[]): Promise<void> {
  try {
    const entries = await read();
    const next = [{ matchId, frames }, ...entries.filter((e) => e.matchId !== matchId)].slice(
      0,
      KEEP,
    );
    await storage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Storage can be full, or refused outright in a private window. Losing a
    // mark is a small thing; interrupting the replay over it is not.
  }
}
