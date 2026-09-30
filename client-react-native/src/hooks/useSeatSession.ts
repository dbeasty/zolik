import { useEffect, useMemo, useState } from 'react';

import { ZolikClient } from '@/src/api/client';
import type { PlayerSession } from '@/src/api/types';
import { clearSeatSession, loadSeatSession, type SeatSession } from '@/src/lib/seatSessions';

/**
 * The seat this device took at `matchId` through a seat link, if any, with a
 * client that speaks as it — see seatSessions.ts for why it is not the
 * device's own session.
 *
 * Ignored when the device's own session already is that seat: somebody who
 * signed back in as themselves plays as themselves.
 */
export function useSeatSession(matchId: string | undefined, own: PlayerSession | null) {
  const [seat, setSeat] = useState<SeatSession | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let live = true;
    setLoaded(false);
    if (!matchId) {
      setSeat(null);
      setLoaded(true);
      return;
    }
    void loadSeatSession(matchId).then((s) => {
      if (!live) return;
      setSeat(s);
      setLoaded(true);
    });
    return () => {
      live = false;
    };
  }, [matchId]);

  const active = seat && seat.userId !== own?.userId ? seat : null;

  const client = useMemo(() => {
    if (!active) return null;
    const c = new ZolikClient();
    // Refused or run out: forget it, so the screen falls back to the device's
    // own session rather than retrying a token that will not work again.
    c.bindSession(active, undefined, () => {
      void clearSeatSession(active.matchId);
      setSeat(null);
    });
    return c;
  }, [active]);

  return { seat: active, client, loaded };
}
