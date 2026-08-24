import { useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { LobbyWSMessage, WaitingPlayer } from '@/src/api/types';
import { logger } from '@/src/lib/logger';

/**
 * The waiting-room connection: being open *is* "I'm available to be picked
 * up". There is nothing to send — this only ever receives the current pool
 * and, personally, an invite.
 *
 * Reconnects on a short fixed delay rather than useGameSocket's full
 * backoff/stable-timer machinery: missing a beat here costs a stale "N
 * waiting" count, not a lost game action, so the extra complexity that
 * machinery earns its keep for isn't worth carrying twice.
 */
export function useLobbySocket(enabled: boolean, onInvited: (gameId: string, joinCode: string) => void) {
  const [players, setPlayers] = useState<WaitingPlayer[]>([]);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const closingRef = useRef(false);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onInvitedRef = useRef(onInvited);
  onInvitedRef.current = onInvited;

  useEffect(() => {
    if (!enabled) return undefined;
    closingRef.current = false;

    function connect() {
      if (closingRef.current) return;
      const ws = new WebSocket(apiClient.lobbyWsUrl());
      wsRef.current = ws;

      ws.onopen = () => setConnected(true);

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
        setConnected(false);
        if (closingRef.current) return;
        reconnectTimerRef.current = setTimeout(connect, 2000);
      };

      ws.onerror = () => {
        // onclose always follows onerror for a WebSocket; the reconnect is
        // scheduled there, so nothing further is needed here.
      };
    }

    connect();
    return () => {
      closingRef.current = true;
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [enabled]);

  return { players, connected };
}
