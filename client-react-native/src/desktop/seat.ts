import { IS_DESKTOP } from '@/src/config';
import { postToApp } from '@/src/desktop/windows';

/**
 * Which offline table the player sits at, in the Mac app, is the app's to
 * remember rather than any one window's: every window sees the same seat
 * (client-macos SeatStore). This is what crosses between the app and a page;
 * the page keeps the connection and the sign-in itself.
 */
export type SeatTable = {
  instanceId: string;
  baseUrl: string;
  role: 'host' | 'guest';
  via: 'self' | 'wifi' | 'bluetooth' | 'internet';
  serverName?: string;
};

type SeatBridge = {
  state(): SeatTable | null;
  subscribe(cb: (seat: SeatTable | null) => void): () => void;
};

function bridge(): SeatBridge | null {
  if (!IS_DESKTOP) return null;
  return (globalThis as { ZolikDesktopSeat?: SeatBridge }).ZolikDesktopSeat ?? null;
}

/** The seat as the app held it when this page loaded. */
export function bootSeat(): SeatTable | null {
  return bridge()?.state() ?? null;
}

/** Hears the seat change in any window. */
export function subscribeSeat(cb: (seat: SeatTable | null) => void): () => void {
  return bridge()?.subscribe(cb) ?? (() => {});
}

/** Tells the app (and through it every window) where this player sits. */
export function postSeat(table: SeatTable | null) {
  postToApp({ op: 'seat', table });
}

/** The part of a seat that identifies it; what changes it is a different table. */
export function seatKey(t: SeatTable | null): string {
  return t ? `${t.instanceId}|${t.baseUrl}|${t.role}|${t.via}` : 'none';
}

/**
 * Whether a window other than the one that sat down can reach this table by
 * its address alone. A table reached through a tunnel that only the sitting
 * page holds (Bluetooth, the internet relay) cannot be shared yet.
 */
export function seatIsShareable(t: Pick<SeatTable, 'baseUrl'> | null): boolean {
  return !!t && /^https?:\/\//.test(t.baseUrl);
}
