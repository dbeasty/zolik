import { useCallback, useEffect, useRef, useState } from 'react';
import { AppState } from 'react-native';

import type { MatchAction, MatchState } from '@/src/api/matchTypes';
import { apiClient } from '@/src/api/client';
import type { SocketLike } from '@/src/net/transport';
import { busyBackoff, jitteredBackoff } from '@/src/lib/reconnectBackoff';

/**
 * The socket for a module-hosted match.
 *
 * Deliberately small, and the reason is the protocol rather than the effort:
 * there is one message type to handle
 * (`match_state`) and nothing to merge, because the server sends the whole
 * board every time already filtered for this viewer. There is no local
 * projection to keep in step, so there is no way for one to drift.
 */

export type MatchSocketState = {
  state: MatchState | null;
  /**
   * The last refusal from the server, as a stable code — plus, when the
   * module has written rules, the ids of the ones that justify it. A
   * submission composed by a person (a rummy meld) has no greyed-out control
   * of its own to have been explained in advance, so the refusal frame is
   * where its explanation has to travel.
   */
  error: { code: string; message?: string; ruleIds?: string[] } | null;
  connected: boolean;
  send: (action: MatchAction) => void;
  clearError: () => void;
};

/**
 * Refusals no amount of reconnecting can fix.
 *
 * Everything else this socket reports is transient by nature — a dropped
 * connection, a busy server, an illegal move — and retrying is the right
 * answer. A table that does not exist is not going to start existing, so
 * retrying it is a hot loop against a certainty: the server answers the same
 * way every time, and the player watches a spinner that will never resolve
 * behind an error that was already final.
 */
const TERMINAL_CODES = new Set(['MATCH_NOT_FOUND', 'MATCH_DELETED']);

/**
 * `client` is the server the socket belongs to. After a drop, it is asked
 * whether that server is full. An offline table is hosted on this phone, so
 * asking the online server about it would report someone else's capacity.
 */
export function useMatchSocket(
  url: string | null,
  client: Pick<typeof apiClient, 'getCapacity' | 'openSocket'> = apiClient,
): MatchSocketState {
  const [state, setState] = useState<MatchState | null>(null);
  const [error, setError] = useState<{ code: string; message?: string; ruleIds?: string[] } | null>(
    null,
  );
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<SocketLike | null>(null);
  // Reconnect attempts, reset once an open socket has stayed open for a
  // while (see STABLE_MS below) rather than the instant it opens. Kept in a
  // ref so the backoff survives re-renders without causing them.
  const attemptRef = useRef(0);

  useEffect(() => {
    if (!url) return;
    // Whether *this* run of the effect has been torn down.
    //
    // Deliberately a plain local rather than a ref shared with the next run,
    // which is what it used to be. A ref cannot express "this chain is over"
    // to a socket the previous run opened: cleanup set it true, the next run
    // set it straight back to false, and the old socket's `onclose` — which
    // always lands after both, being a task rather than part of the commit —
    // read false and scheduled its own reconnect, on its own `timer`, which no
    // cleanup could ever reach. That orphan chain then reconnected forever
    // beside the live one, each socket displacing the other on the server,
    // each displacement pausing the table, and `send` firing into whichever
    // one had just been closed. One re-run of this effect was enough to start
    // it; nothing but a page reload stopped it.
    let torn = false;
    // Whether the server has said something no reconnect can improve on. Scoped
    // to this run of the effect like `torn`, so pointing the screen at a
    // different table starts over rather than inheriting the last one's verdict.
    let terminal = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    // This run's own socket, so cleanup closes the one it opened rather than
    // whichever one happens to be in the shared ref.
    let socket: SocketLike | null = null;
    // Armed on every `onopen`, cleared on `onclose` or teardown.
    //
    // The backoff used to reset the moment a socket opened at all. That is
    // right for a socket that stays up, and wrong for one that is about to be
    // displaced straight back out — a second live connection for the same
    // player (another tab, a stuck reconnect elsewhere, the server's own
    // "newest wins" rule) closes the older one on arrival, `onopen` still
    // fires on the way in, and resetting the counter there meant two such
    // sockets could trade the seat back and forth every ~1s forever, neither
    // backoff ever growing enough to let the other settle. Waiting for the
    // connection to prove itself stable first turns that into ordinary
    // exponential backoff instead.
    let stableTimer: ReturnType<typeof setTimeout> | null = null;
    const STABLE_MS = 2000;

    const open = () => {
      if (torn) return;
      const ws = client.openSocket(url);
      socket = ws;
      wsRef.current = ws;

      ws.onopen = () => {
        setConnected(true);
        if (stableTimer) clearTimeout(stableTimer);
        stableTimer = setTimeout(() => {
          attemptRef.current = 0;
        }, STABLE_MS);
      };
      ws.onmessage = (ev) => {
        let msg: unknown;
        try {
          msg = JSON.parse(String(ev.data));
        } catch {
          return;
        }
        const m = msg as { type?: string; code?: string; message?: string; ruleIds?: string[] };
        if (m.type === 'match_state') {
          setState(msg as MatchState);
          return;
        }
        if (m.type === 'error') {
          const code = m.code ?? 'ERROR';
          if (TERMINAL_CODES.has(code)) terminal = true;
          setError({ code, message: m.message, ruleIds: m.ruleIds });
        }
        // Anything else is an event the board already reflects: the server
        // sends the whole state after every action, so events are for flavour
        // and never for correctness.
      };
      ws.onclose = () => {
        if (stableTimer) clearTimeout(stableTimer);
        if (torn) return;
        setConnected(false);
        // The refusal is already on screen and is the last word; reconnecting
        // would only fetch it again, for ever.
        if (terminal) return;
        void (async () => {
          if (torn || socket !== ws) return;
          let delay: number;
          try {
            const cap = await client.getCapacity();
            if (!cap.accepting) {
              setError({ code: 'SERVER_BUSY' });
              delay = busyBackoff(attemptRef.current);
              attemptRef.current += 1;
              // A newer socket may have been opened while the probe was out
              // (the app coming back to the foreground does that). Scheduling
              // another would run two chains side by side.
              if (!torn && socket === ws) timer = setTimeout(open, delay);
              return;
            }
          } catch {
            /* probe failed — treat as an ordinary disconnect */
          }
          delay = jitteredBackoff(attemptRef.current);
          attemptRef.current += 1;
          if (!torn && socket === ws) timer = setTimeout(open, delay);
        })();
      };
      ws.onerror = () => {
        // onclose always follows; reconnection is handled there.
      };
    };

    open();

    // Back in the foreground, reconnect now rather than when the backoff
    // says. A phone hosting an offline table is suspended while its player
    // looks at something else, and every guest's socket drops. The backoff
    // has usually grown by the time the host is back, and a guest waiting
    // it out sees a table stuck for no reason they can see.
    const foreground = AppState.addEventListener('change', (next) => {
      if (next !== 'active' || torn || terminal) return;
      if (socket && socket.readyState !== WebSocket.CLOSED) return;
      if (timer) clearTimeout(timer);
      timer = null;
      attemptRef.current = 0;
      open();
    });

    return () => {
      torn = true;
      foreground.remove();
      if (timer) clearTimeout(timer);
      if (stableTimer) clearTimeout(stableTimer);
      socket?.close();
      // Only if it is still this run's: a later run may already have put its
      // own socket there, and clearing that one would leave `send` with
      // nothing to write to.
      if (wsRef.current === socket) wsRef.current = null;
      setConnected(false);
    };
  }, [url, client]);

  const send = useCallback((action: MatchAction) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      // Said out loud rather than dropped.
      //
      // Returning silently here is what made a connection fault read as a
      // rules fault: a player pressed Discard on a perfectly legal card, the
      // action went nowhere, and the board gave back nothing at all — no
      // refusal, no spinner, no hint that the socket was the problem. The
      // whole board is still on screen during a reconnect, so there is
      // nothing else to tell them from.
      setError({ code: 'NOT_CONNECTED' });
      return;
    }
    setError(null);
    ws.send(JSON.stringify(action));
  }, []);

  const clearError = useCallback(() => setError(null), []);

  return { state, error, connected, send, clearError };
}
