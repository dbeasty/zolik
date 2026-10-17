import { createContext, useCallback, useContext, useMemo, useSyncExternalStore, type ReactNode } from 'react';

import type { WaitingPlayer } from '@/src/api/types';
import { useSession } from '@/src/context/SessionContext';
import { useLobbySocket, type LobbyConnectionStatus } from '@/src/hooks/useLobbySocket';
import { useInvites } from '@/src/notify/InviteProvider';

/**
 * Whether this player is waiting to be picked up, and for which game.
 *
 * Being available is an open socket (`useLobbySocket`). It used to belong to
 * a card on the main menu, so it lasted exactly as long as that card was
 * mounted. Now the button lives on each game's own page, and a player who
 * makes themselves available for Hold'em and then goes back to the list is
 * still waiting — so the socket is held here, above the screens, and the
 * pages only read and flip it.
 *
 * One game at a time. Making yourself available for another game moves you:
 * the socket reconnects under the new game, which is what the server keys the
 * pool by.
 */
type Availability = {
  /** The game this player is waiting for, or null when they are not. */
  availableFor: string | null;
  setAvailableFor: (moduleId: string | null) => void;
  /** The live pool as the socket last reported it — the players this one
   *  could share a table with, themselves included. */
  players: WaitingPlayer[];
  status: LobbyConnectionStatus;
  attempts: number;
  retryNow: () => void;
};

const AvailabilityContext = createContext<Availability | null>(null);

/**
 * The choice itself lives here, outside React, because this provider does not
 * always outlive a browser's Back button: on the web, expo-router answers
 * Back by resetting the root to a recorded state whose keys are stale, which
 * remounts the root layout and every provider in it. Held in component state,
 * a player who pressed Back on a game's page stopped waiting without being
 * told. It is tied to the player who chose it, so signing in as somebody
 * else does not inherit it.
 */
let chosen: { userId: string; moduleId: string } | null = null;
const listeners = new Set<() => void>();
function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}
const snapshot = () => chosen;
function choose(next: typeof chosen) {
  chosen = next;
  listeners.forEach((cb) => cb());
}

export function AvailabilityProvider({ children }: { children: ReactNode }) {
  const { session, offline } = useSession();
  const userId = session?.userId;
  const stored = useSyncExternalStore(subscribe, snapshot, snapshot);
  const availableFor = stored && stored.userId === userId ? stored.moduleId : null;
  const setAvailableFor = useCallback(
    (moduleId: string | null) => choose(moduleId && userId ? { userId, moduleId } : null),
    [userId],
  );

  // The seat is already taken by the time this arrives, so the invite queue
  // walks the player to it — the same thing it does with the copy of this
  // invite that comes over the personal socket, whichever lands first.
  const { receiveSeated } = useInvites();
  const onInvited = useCallback(
    (matchId: string, joinCode: string) => {
      // Picked up: the server has already taken them out of the pool.
      choose(null);
      receiveSeated(matchId, joinCode);
    },
    [receiveSeated],
  );

  // The waiting room is the online server's, and nobody online can pick up
  // a player at a table on this phone — and signing out ends it too.
  const live = !!availableFor && !!session && !offline;
  const { players, status, attempts, retryNow } = useLobbySocket(
    live,
    onInvited,
    availableFor ?? undefined,
  );

  const value = useMemo<Availability>(
    () => ({
      availableFor: live ? availableFor : null,
      setAvailableFor,
      players: live ? players : [],
      status,
      attempts,
      retryNow,
    }),
    [live, availableFor, setAvailableFor, players, status, attempts, retryNow],
  );

  return <AvailabilityContext.Provider value={value}>{children}</AvailabilityContext.Provider>;
}

export function useAvailability(): Availability {
  const ctx = useContext(AvailabilityContext);
  if (!ctx) throw new Error('useAvailability must be used inside AvailabilityProvider');
  return ctx;
}
