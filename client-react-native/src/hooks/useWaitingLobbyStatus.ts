import { useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { WaitingPlayer } from '@/src/api/types';

const pollIntervalMs = 5000;

/**
 * A read-only, poll-based view of who's currently waiting — for the main
 * menu, which shows "N players waiting" without joining the pool itself.
 *
 * Deliberately not the WebSocket connection useLobbySocket opens: connecting
 * to /ws/lobby *is* the request to be visible to hosts (see that hook's own
 * doc comment), and the main menu should not silently make every signed-in
 * visitor inviteable just because the app is open. This polls the plain
 * snapshot endpoint instead, the same one a host's create-lobby screen uses
 * to browse whom to invite, so looking at the count costs nothing more than
 * looking at the list would.
 */
export function useWaitingLobbyStatus(enabled: boolean) {
  const [players, setPlayers] = useState<WaitingPlayer[]>([]);
  const [loaded, setLoaded] = useState(false);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!enabled) {
      setPlayers([]);
      setLoaded(false);
      return undefined;
    }
    let cancelled = false;

    async function poll() {
      try {
        const list = await apiClient.getWaitingLobby();
        if (!cancelled) {
          setPlayers(list);
          setLoaded(true);
        }
      } catch {
        // A signed-out-mid-poll or offline moment shouldn't blank out the
        // last-known count with an error state on the main menu — it just
        // tries again next tick.
      }
    }

    poll();
    timerRef.current = setInterval(poll, pollIntervalMs);
    return () => {
      cancelled = true;
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [enabled]);

  return { players, loaded };
}
