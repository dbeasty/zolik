import { useCallback, useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { LobbyWSMessage, WaitingPlayer } from '@/src/api/types';
import { logger } from '@/src/lib/logger';
import { busyBackoff, jitteredBackoff } from '@/src/lib/reconnectBackoff';

/** What the waiting-room screen actually needs to show a person: not just
 *  "connected or not" but *which kind* of not-connected this is, since
 *  "still trying the first time" and "lost the connection and retrying"
 *  read very differently and a silent spinner conflates them. */
export type LobbyConnectionStatus = 'connecting' | 'open' | 'reconnecting' | 'busy';

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
export function useLobbySocket(
  enabled: boolean,
  onInvited: (matchId: string, joinCode: string) => void,
  moduleId?: string,
) {
  const [players, setPlayers] = useState<WaitingPlayer[]>([]);
  const [status, setStatus] = useState<LobbyConnectionStatus>('connecting');
  const [attempts, setAttempts] = useState(0);
  const wsRef = useRef<WebSocket | null>(null);
  const connectRef = useRef<() => void>(() => {});
  const onInvitedRef = useRef(onInvited);
  onInvitedRef.current = onInvited;

  useEffect(() => {
    if (!enabled) return undefined;
    // Whether *this* run of the effect has been torn down, and this run's own
    // pending reconnect. Both are plain locals rather than refs shared with
    // the next run, mirroring useMatchSocket/useUserSocket. A shared ref could
    // not tell the previous run's socket that its chain was over: cleanup set
    // it true, the next run (a moduleId change, or Stop then Make available)
    // set it straight back to false, and the old socket's onclose — which
    // lands after both — read false, probed capacity and reopened a socket for
    // the OLD module. The server keeps only a player's newest lobby
    // connection, so the two chains then displaced each other forever.
    let closing = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let attemptCount = 0;

    function clearTimer() {
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
    }

    function scheduleReconnect(busy = false) {
      if (closing) return;
      clearTimer();
      attemptCount += 1;
      setAttempts(attemptCount);
      setStatus(busy ? 'busy' : 'reconnecting');
      const delay = busy
        ? busyBackoff(attemptCount - 1)
        : jitteredBackoff(attemptCount - 1, initialRetryDelayMs, maxRetryDelayMs);
      timer = setTimeout(connect, delay);
    }

    function connect() {
      if (closing) return;
      clearTimer();
      const ws = new WebSocket(apiClient.lobbyWsUrl(moduleId));
      wsRef.current = ws;
      // Events from a socket this run has since replaced (retryNow) or torn
      // down are ignored, so only the current socket can drive state or
      // schedule a reconnect.
      const current = () => !closing && wsRef.current === ws;

      ws.onopen = () => {
        if (!current()) return;
        attemptCount = 0;
        setAttempts(0);
        setStatus('open');
      };

      ws.onmessage = (event) => {
        if (!current()) return;
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
          onInvitedRef.current(msg.matchId, msg.joinCode);
        }
      };

      ws.onclose = () => {
        if (!current()) return;
        // The list is deliberately left as-is rather than cleared: it was
        // true a moment ago and is more useful than a blank screen while a
        // reconnect is in flight, which is usually seconds.
        void (async () => {
          let busy = false;
          try {
            const cap = await apiClient.getCapacity();
            busy = !cap.waitingRoomOpen;
          } catch {
            /* probe failed — ordinary reconnect */
          }
          // Re-checked after the probe: a teardown or a retryNow may have
          // happened while it was in flight, and either already owns the
          // next socket.
          if (!current()) return;
          scheduleReconnect(busy);
        })();
      };

      ws.onerror = () => {
        // onclose always follows onerror for a WebSocket; the reconnect is
        // scheduled there, so nothing further is needed here.
      };
    }

    connectRef.current = () => {
      if (closing) return;
      attemptCount = 0;
      setAttempts(0);
      setStatus('connecting');
      // Close the socket being replaced first, so a retry never leaves two
      // live connections (the server would displace one with the other).
      // Clearing the ref before close() makes its handlers stale.
      const old = wsRef.current;
      wsRef.current = null;
      old?.close();
      connect();
    };

    connect();
    return () => {
      closing = true;
      clearTimer();
      const ws = wsRef.current;
      wsRef.current = null;
      ws?.close();
    };
  }, [enabled, moduleId]);

  const retryNow = useCallback(() => connectRef.current(), []);

  return { players, status, attempts, retryNow };
}
