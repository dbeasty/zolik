import { storage } from '@/src/context/SessionContext';
import { CLOUD_BASE_URL } from '@/src/config';

/**
 * Seats a player agreed to add to their account, waiting until they can be.
 *
 * A table with no internet hands every guest a receipt its host signed: this
 * guest id sat here. On its own it does nothing. When the player says yes to
 * keeping a game, the receipt is put aside here, and the next time this device
 * is signed in to an account with a connection it is redeemed
 * (`/auth/claim-offline`), which is what credits that game - and any later one
 * played as the same guest at the same table - to the account.
 *
 * Nothing is redeemed without that yes: a receipt the player never agreed to
 * keep is never stored.
 */

const KEY = 'zolik_seat_receipts';

async function read(): Promise<string[]> {
  try {
    const parsed = JSON.parse((await storage.getItem(KEY)) ?? '[]');
    return Array.isArray(parsed) ? parsed.filter((r): r is string => typeof r === 'string' && r !== '') : [];
  } catch {
    return [];
  }
}

/** Puts a receipt aside for the next signed-in moment. */
export async function rememberReceipt(receipt: string): Promise<void> {
  const r = receipt.trim();
  if (!r) return;
  const list = await read();
  if (list.includes(r)) return;
  await storage.setItem(KEY, JSON.stringify([...list, r]));
}

/** What is waiting to be redeemed. */
export async function pendingReceipts(): Promise<string[]> {
  return read();
}

/** Forgets receipts once the cloud has answered for them. */
export async function forgetReceipts(done: string[]): Promise<void> {
  const list = await read();
  const left = list.filter((r) => !done.includes(r));
  if (left.length) await storage.setItem(KEY, JSON.stringify(left));
  else await storage.deleteItem(KEY);
}

/**
 * The link a browser guest takes away from a table on somebody's phone.
 *
 * A page served by that phone cannot keep the receipt for later: its address
 * is the phone's, on this network, and next time it is a different one. So
 * the receipt leaves as a link to the cloud, carried in the fragment so that
 * it never reaches anybody's server log - the claim page reads it in the
 * browser and redeems it once the player has signed in.
 */
export function claimLinkFor(receipt: string, base: string = CLOUD_BASE_URL): string {
  return `${base.replace(/\/$/, '')}/claim#r=${encodeURIComponent(receipt)}`;
}

/** The receipt in a claim link's fragment, or ''. */
export function receiptFromHash(hash: string): string {
  const h = hash.replace(/^#/, '');
  for (const part of h.split('&')) {
    const [k, v] = part.split('=');
    if (k === 'r' && v) {
      try {
        return decodeURIComponent(v);
      } catch {
        return '';
      }
    }
  }
  return '';
}
