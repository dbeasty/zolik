import { useCallback, useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { LobbyWSMessage, WaitingPlayer } from '@/src/api/types';
import { logger } from '@/src/lib/logger';

/** What the waiting-room screen actually needs to show a person: not just
 *  "connected or not" but *which kind* of not-connected this is, since
 *  "still trying the first time" and "lost the connection and retrying"
 *  read very differently and a silent spinner conflates them. */
export type LobbyConnectionStatus = 'connecting' | 'open' | 'reconnecting';

const initialRetryDelayMs = 1500;
const maxRetryDelayMs = 10000;

/**
 * The waiting-room connection: being open *is* "I'm available to be picked
 * up". There is nothing to send — this only ever receives the current pool
 * and, personally, an invite.
 *
 * Retries with a capped exponential backoff rather than useGameSocket's full
 * backoff/stable-timer machinery: missing a beat here costs a stale "N
 * waiting" count, not a lost game action, so the extra complexity that
 * machinery earns its keep for isn't worth carrying twice. It does still
 * retry forever rather than giving up — a person leaves this screen when
 * they're done waiting, not because the hook decided to stop for them — but
 * every attempt is now visible (`status`, `attempts`) instead of an
 * indefinite, unexplained spinner, and `retryNow` lets a person force one
 * immediately rather than watching a countdown after fixing whatever was
 * wrong (a firewall, a wrong server address).
 */
export function useLobbySocket(enabled: boolean, onInvited: (gameId: string, joinCode: string) => void) {
  const [players, setPlayers] = useState<WaitingPlayer[]>([]);
  const [status, setStatus] = useState<LobbyConnectionStatus>('connecting');
  const [attempts, setAttempts] = useState(0);
  const wsRef = useRef<WebSocket | null>(null);
  const closingRef = useRef(false);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const connectRef = useRef<() => void>(() => {});
  const onInvitedRef = useRef(onInvited);
  onInvitedRef.current = onInvited;

  useEffect(() => {
    if (!enabled) return undefined;
    closingRef.current = false;
    let attemptCount = 0;

    function scheduleReconnect() {
      attemptCount += 1;
      setAttempts(attemptCount);
      setStatus('reconnecting');
      const delay = Math.min(initialRetryDelayMs * 2 ** (attemptCount - 1), maxRetryDelayMs);
      reconnectTimerRef.current = setTimeout(connect, delay);
    }

    function connect() {
      if (closingRef.current) return;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      const ws = new WebSocket(apiClient.lobbyWsUrl());
      wsRef.current = ws;

      ws.onopen = () => {
        attemptCount = 0;
        setAttempts(0);
        setStatus('open');
      };

      ws.onmessage = (event) => {
        let msg: LobbyWSMessage;
        try {
          msg = JSON.parse(String(event.data));
        } catch (e) {
          logger.warn('lobby', 'bad json', { error: e instanceof Error ? e.message : String(e) });
          return;
        }
        if (msg.type === 'lobby_waiting') {
          setPlayers(msg.players);
        } else if (msg.type === 'lobby_invited') {
          onInvitedRef.current(msg.gameId, msg.joinCode);
        }
      };

      ws.onclose = () => {
        if (closingRef.current) return;
        // The list is deliberately left as-is rather than cleared: it was
        // true a moment ago and is more useful than a blank screen while a
        // reconnect is in flight, which is usually seconds.
        scheduleReconnect();
      };

      ws.onerror = () => {
        // onclose always follows onerror for a WebSocket; the reconnect is
        // scheduled there, so nothing further is needed here.
      };
    }

    connectRef.current = () => {
      attemptCount = 0;
      setAttempts(0);
      setStatus('connecting');
      connect();
    };

    connect();
    return () => {
      closingRef.current = true;
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [enabled]);

  const retryNow = useCallback(() => connectRef.current(), []);

  return { players, status, attempts, retryNow };
}
