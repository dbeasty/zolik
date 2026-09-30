import { storage } from '@/src/context/SessionContext';
import { metricsFor } from '@/src/lib/layout';
import { casino } from '@/src/skins/casino';
import { classic } from '@/src/skins/classic';
import { heirloom } from '@/src/skins/heirloom';
import type { Skin } from '@/src/skins/types';

/**
 * Every skin the client ships, in the order a switcher cycles them. Adding a
 * skin is adding a file next to `casino.ts` and a line here — nothing else
 * knows how many there are.
 */
export const SKINS: readonly Skin[] = [heirloom, casino, classic];

/**
 * What the board wears for somebody who has never chosen.
 *
 * `classic` on a phone, and the fallback anywhere a skin is read outside a
 * provider — see `defaultSkinFor` for the screen this is not the answer for.
 */
export const DEFAULT_SKIN = classic;

/**
 * The default for a screen of a given width.
 *
 * Heirloom is the better board and the one to lead with, but only where
 * there is room for it: its deck is a real engraved one, and an engraving is
 * a set of decisions about line weight that were made for a card 63mm across.
 * At the 52×72 a narrow screen draws, a king's robe is ten dark pixels.
 *
 * The phone gets `classic`, whose face is one large index and nothing else.
 * It was `casino` — two small corners, a centre pip, a medallion on the
 * courts — and that is the same mistake one size down: three marks sharing a
 * card 36 pixels wide leaves each of them too small to read, and a fanned
 * hand hides two of the three anyway. What a phone can show is one mark, as
 * big as the card will take, which is exactly what `classic` draws now.
 *
 * "Narrow" is `metricsFor`'s own — the 768 line the layout already turns on —
 * rather than a second breakpoint that could drift from it.
 *
 * Note what this is *not*: a skin that changes with the window. The choice is
 * made once, from the width at startup, and a saved preference beats it
 * outright. A board that restyled itself mid-resize would be a look changing
 * under somebody's hands while they played.
 */
export function defaultSkinFor(width: number): Skin {
  return metricsFor(width).narrow ? classic : heirloom;
}

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
