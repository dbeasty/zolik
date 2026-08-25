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
  /** The last refusal from the server, as a stable code. */
  error: { code: string; message?: string } | null;
  connected: boolean;
  send: (action: MatchAction) => void;
  clearError: () => void;
};

export function useMatchSocket(url: string | null): MatchSocketState {
  const [state, setState] = useState<MatchState | null>(null);
  const [error, setError] = useState<{ code: string; message?: string } | null>(null);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  // Reconnect attempts, reset on every successful open. Kept in a ref so the
  // backoff survives re-renders without causing them.
  const attemptRef = useRef(0);
  const closedRef = useRef(false);

  useEffect(() => {
    if (!url) return;
    closedRef.current = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const open = () => {
      if (closedRef.current) return;
      const ws = new WebSocket(url);
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
        const m = msg as { type?: string; code?: string; message?: string };
        if (m.type === 'match_state') {
          setState(msg as MatchState);
          return;
        }
        if (m.type === 'error') {
          setError({ code: m.code ?? 'ERROR', message: m.message });
        }
        // Anything else is an event the board already reflects: the server
        // sends the whole state after every action, so events are for flavour
        // and never for correctness.
      };
      ws.onclose = () => {
        setConnected(false);
        if (closedRef.current) return;
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
      closedRef.current = true;
      if (timer) clearTimeout(timer);
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [url]);

  const send = useCallback((action: MatchAction) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    setError(null);
    ws.send(JSON.stringify(action));
  }, []);

  const clearError = useCallback(() => setError(null), []);

  return { state, error, connected, send, clearError };
}
