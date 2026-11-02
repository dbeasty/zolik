import { storage } from '@/src/context/SessionContext';

/**
 * Remembering how a player arranged the board.
 *
 * Keyed per game rather than per match, unlike the panels a player put away
 * (`panelStore.ts`): a panel put away is about this deal, but where the hand
 * sits on a wide screen is how somebody likes to play the game, and is the
 * same answer next deal. Per game and not once for all, because the games
 * have different regions — a layout that suits rummy's melds says nothing
 * about blackjack, which has no hand.
 *
 * Stored raw and made whole on the way out (`normalize`), so a stored layout
 * from an older shell still draws every region this one has.
 */

const KEY = 'zolik_board_arrangement';

type Entries = Record<string, unknown>;

async function read(): Promise<Entries> {
  try {
    const raw = await storage.getItem(KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? (parsed as Entries) : {};
  } catch {
    return {};
  }
}

export async function loadArrangement(moduleId: string): Promise<unknown> {
  return (await read())[moduleId] ?? null;
}

/** `null` forgets the game's arrangement, which is what reset means. */
export async function saveArrangement(moduleId: string, arrangement: unknown): Promise<void> {
  try {
    const entries = await read();
    if (arrangement == null) delete entries[moduleId];
    else entries[moduleId] = arrangement;
    await storage.setItem(KEY, JSON.stringify(entries));
  } catch {
    // Storage can be full, or refused outright in a private window. The board
    // still draws the way it was asked to until the screen goes.
  }
}
