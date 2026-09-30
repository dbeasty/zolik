import type { PlayerSession, SeatClaim } from '@/src/api/types';
import { storage } from '@/src/context/SessionContext';

/**
 * Seats taken through a seat link, one per table.
 *
 * Kept apart from the device's own session on purpose. A seat link's token
 * plays one seat at one table and is refused everywhere else, so it must never
 * become "who this device is": the lobby, the circle and the player's own
 * games keep using the device's session, and only that table's screen uses
 * the seat.
 */
export type SeatSession = PlayerSession & { matchId: string; expiresAt: number };

const key = (matchId: string) => `zolik_seat_${matchId}`;

export async function loadSeatSession(matchId: string, now = Date.now()): Promise<SeatSession | null> {
  try {
    const raw = await storage.getItem(key(matchId));
    if (!raw) return null;
    const s = JSON.parse(raw) as SeatSession;
    if (!s.accessToken || s.expiresAt <= now) {
      await storage.deleteItem(key(matchId));
      return null;
    }
    return s;
  } catch {
    return null;
  }
}

export async function saveSeatSession(claim: SeatClaim, now = Date.now()): Promise<SeatSession | null> {
  if (!claim.accessToken || !claim.userId) return null;
  const s: SeatSession = {
    matchId: claim.matchId,
    accessToken: claim.accessToken,
    // Nothing to refresh with: when it runs out, the link is opened again.
    refreshToken: '',
    userId: claim.userId,
    username: claim.username ?? '',
    isGuest: true,
    expiresAt: now + (claim.expiresIn ?? 0) * 1000,
  };
  await storage.setItem(key(claim.matchId), JSON.stringify(s));
  return s;
}

export async function clearSeatSession(matchId: string): Promise<void> {
  await storage.deleteItem(key(matchId));
}
