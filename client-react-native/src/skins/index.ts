import { storage } from '@/src/context/SessionContext';
import { casino } from '@/src/skins/casino';
import { classic } from '@/src/skins/classic';
import type { Skin } from '@/src/skins/types';

/**
 * Every skin the client ships, in the order a switcher cycles them. Adding a
 * skin is adding a file next to `casino.ts` and a line here — nothing else
 * knows how many there are.
 */
export const SKINS: readonly Skin[] = [casino, classic];

export const DEFAULT_SKIN = casino;

export function skinById(id: string | null | undefined): Skin | undefined {
  return SKINS.find((s) => s.id === id);
}

/**
 * Which skin the player chose. A preference about how the board looks, same
 * as a minimized panel — it never leaves the device — but global rather than
 * per match: a look is chosen once, not per deal.
 */
const KEY = 'zolik_skin';

export async function loadSkinId(): Promise<string | null> {
  try {
    return await storage.getItem(KEY);
  } catch {
    return null;
  }
}

export async function saveSkinId(id: string): Promise<void> {
  try {
    await storage.setItem(KEY, id);
  } catch {
    // Losing a preference is a small thing; interrupting a game over it is not.
  }
}
