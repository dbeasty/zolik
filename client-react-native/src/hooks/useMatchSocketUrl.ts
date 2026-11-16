import { useMemo } from 'react';

import type { PlayerSession } from '@/src/api/types';

/**
 * The address the match screen's socket is opened at, or null while there is
 * nothing to open yet.
 *
 * Keyed on *whether* there is a token and *whose* it is, never on the token
 * itself. Access tokens are renewed every fifteen minutes, and a new value
 * here is a new socket: the screen dropped its connection — and showed
 * "Connecting…" — on every renewal, though the socket it had was fine. The
 * socket hook already puts a fresh token in the address when the server
 * turns a stale one away. A different person signing in is a different
 * viewer, though, and that does get a new socket.
 */
export function useMatchSocketUrl(
  client: { matchSocketUrl(matchId: string): string },
  matchId: string | undefined,
  session: Pick<PlayerSession, 'accessToken' | 'userId'> | null | undefined,
  settling: boolean,
): string | null {
  const hasToken = !!session?.accessToken;
  const viewer = session?.userId ?? '';
  return useMemo(() => {
    // Not while the session is still being settled: a game window finds the
    // player's seat at an offline table first, and a socket opened at the
    // cloud in the meantime would ask it for a table it has never heard of.
    if (!matchId || !hasToken || settling) return null;
    return client.matchSocketUrl(String(matchId));
    // `viewer` is a dependency on purpose: see above.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, matchId, hasToken, viewer, settling]);
}
