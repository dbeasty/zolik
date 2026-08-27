import { useCallback, useEffect, useRef, useState } from 'react';

import type { MatchAction, MatchState } from '@/src/api/matchTypes';

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

export function useMatchSocket(url: string | null): MatchSocketState {
  const [state, setState] = useState<MatchState | null>(null);
  const [error, setError] = useState<{ code: string; message?: string; ruleIds?: string[] } | null>(
    null,
  );
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  // Reconnect attempts, reset on every successful open. Kept in a ref so the
  // backoff survives re-renders without causing them.
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
    let timer: ReturnType<typeof setTimeout> | null = null;
    // This run's own socket, so cleanup closes the one it opened rather than
    // whichever one happens to be in the shared ref.
    let socket: WebSocket | null = null;

    const open = () => {
      if (torn) return;
      const ws = new WebSocket(url);
      socket = ws;
      wsRef.current = ws;

      ws.onopen = () => {
        attemptRef.current = 0;
        setConnected(true);
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
          setError({ code: m.code ?? 'ERROR', message: m.message, ruleIds: m.ruleIds });
        }
        // Anything else is an event the board already reflects: the server
        // sends the whole state after every action, so events are for flavour
        // and never for correctness.
      };
      ws.onclose = () => {
        if (torn) return;
        setConnected(false);
        // Reconnecting matters more here than it does for a local game: the
        // server suspends a match whose active player drops, and resumes it
        // when they return. Coming back is what un-pauses the table.
        const delay = Math.min(1000 * 2 ** attemptRef.current, 10_000);
        attemptRef.current += 1;
        timer = setTimeout(open, delay);
      };
      ws.onerror = () => {
        // onclose always follows; reconnection is handled there.
      };
    };

    open();
    return () => {
      torn = true;
      if (timer) clearTimeout(timer);
      socket?.close();
      // Only if it is still this run's: a later run may already have put its
      // own socket there, and clearing that one would leave `send` with
      // nothing to write to.
      if (wsRef.current === socket) wsRef.current = null;
      setConnected(false);
    };
  }, [url]);

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
