import { useCallback, useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { GameState, WSAction, WSEnvelope } from '@/src/api/types';
import { logger } from '@/src/lib/logger';

type UseGameSocketOptions = {
  gameId: string;
  enabled?: boolean;
  onRoundEnd?: (data: WSEnvelope, state: GameState) => void;
  onGameEnd?: (data: WSEnvelope, state: GameState) => void;
};

export function useGameSocket({
  gameId,
  enabled = true,
  onRoundEnd,
  onGameEnd,
}: UseGameSocketOptions) {
  const [state, setState] = useState<GameState | null>(null);
  const [status, setStatus] = useState('Connecting…');
  // Distinguishes rule-violation/connection errors (rendered as a prominent
  // banner) from benign connection status ("Connected", "Deck recycled")
  // which shares the same `status` string channel but shouldn't look alarming.
  const [statusIsError, setStatusIsError] = useState(false);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const stateRef = useRef<GameState | null>(null);
  const onRoundEndRef = useRef(onRoundEnd);
  const onGameEndRef = useRef(onGameEnd);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const stableTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Set on unmount / manual reconnect so a delayed auto-reconnect from a
  // superseded connect() call doesn't fire after the hook is done with it.
  const closingRef = useRef(false);
  // The server broadcasts deal_ended/game_ended before the game_state carrying
  // the post-deal totals, so the callbacks fire off the *next* game_state
  // (which has the updated totalScores/game) rather than the stale one
  // captured at the moment the event arrived.
  const pendingRoundEndRef = useRef<WSEnvelope | null>(null);
  const pendingGameEndRef = useRef<WSEnvelope | null>(null);

  onRoundEndRef.current = onRoundEnd;
  onGameEndRef.current = onGameEnd;

  const connect = useCallback(() => {
    if (!gameId || !enabled) return;
    closingRef.current = false;
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    wsRef.current?.close();
    pendingRoundEndRef.current = null;
    pendingGameEndRef.current = null;
    setStatus('Connecting…');
    setStatusIsError(false);
    logger.setContext({ gameId, userId: apiClient.userId });
    logger.info('ws', 'connecting', { attempt: reconnectAttemptsRef.current });
    const url = apiClient.wsUrl(gameId);
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      logger.info('ws', 'open');
      // Only reset the backoff once the connection has held up for a few
      // seconds — if something keeps killing the socket right after it
      // opens (a flapping connection), resetting on every open would keep
      // retrying every ~1s forever instead of backing off.
      if (stableTimerRef.current) clearTimeout(stableTimerRef.current);
      stableTimerRef.current = setTimeout(() => {
        stableTimerRef.current = null;
        reconnectAttemptsRef.current = 0;
      }, 3000);
      setConnected(true);
      setStatus('Connected');
      setStatusIsError(false);
    };

    ws.onmessage = (ev) => {
      try {
        const envelope = JSON.parse(String(ev.data)) as WSEnvelope;
        const t = envelope.type;
        if (t === 'game_state') {
          const st = envelope as unknown as GameState;
          logger.debug('ws', 'game_state', {
            phase: st.phase,
            turn: st.currentTurn,
            deal: st.game,
            round: st.round,
          });
          stateRef.current = st;
          setState(st);
          setStatus('');
          setStatusIsError(false);
          if (pendingGameEndRef.current) {
            const data = pendingGameEndRef.current;
            pendingGameEndRef.current = null;
            pendingRoundEndRef.current = null;
            onGameEndRef.current?.(data, st);
          } else if (pendingRoundEndRef.current) {
            const data = pendingRoundEndRef.current;
            pendingRoundEndRef.current = null;
            onRoundEndRef.current?.(data, st);
          }
        } else if (t === 'error') {
          logger.warn('ws', 'server_error', { message: envelope.message });
          setStatus(String(envelope.message ?? 'Something went wrong'));
          setStatusIsError(true);
        } else if (t === 'deal_ended') {
          logger.info('ws', 'deal_ended');
          pendingRoundEndRef.current = envelope;
        } else if (t === 'game_ended') {
          logger.info('ws', 'game_ended');
          pendingGameEndRef.current = envelope;
        } else if (t === 'reshuffle') {
          logger.info('ws', 'reshuffle');
          setStatus('Deck recycled');
          setStatusIsError(false);
        } else if (t === 'game_suspended') {
          logger.info('ws', 'game_suspended');
          setStatus('Game suspended');
          setStatusIsError(false);
        } else {
          logger.debug('ws', 'event', { type: t });
        }
      } catch (err) {
        logger.error('ws', 'parse_failed', {
          raw: String(ev.data).slice(0, 200),
          err: String(err),
        });
        setStatus('Bad message from server');
        setStatusIsError(true);
      }
    };

    ws.onerror = () => {
      logger.error('ws', 'socket_error');
      setStatus('Connection error');
      setStatusIsError(true);
      setConnected(false);
    };

    ws.onclose = () => {
      logger.warn('ws', 'closed', {
        deliberate: closingRef.current,
        nextAttempt: closingRef.current ? undefined : reconnectAttemptsRef.current + 1,
      });
      setConnected(false);
      if (stableTimerRef.current) {
        clearTimeout(stableTimerRef.current);
        stableTimerRef.current = null;
      }
      if (wsRef.current !== ws) return;
      setStatus('Disconnected');
      setStatusIsError(false);
      if (closingRef.current) return;
      // Auto-recover from dropped connections (server restart, network
      // blip, tab backgrounding) so opponent/AI turns that happen while
      // disconnected show up as soon as the socket comes back, instead of
      // leaving the player stuck until they notice and hit Reconnect.
      const attempt = reconnectAttemptsRef.current;
      reconnectAttemptsRef.current = attempt + 1;
      const delayMs = Math.min(1000 * 2 ** attempt, 10000);
      reconnectTimerRef.current = setTimeout(() => {
        reconnectTimerRef.current = null;
        connect();
      }, delayMs);
    };
  }, [gameId, enabled]);

  useEffect(() => {
    connect();
    return () => {
      closingRef.current = true;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      if (stableTimerRef.current) {
        clearTimeout(stableTimerRef.current);
        stableTimerRef.current = null;
      }
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [connect]);

  const send = useCallback((action: WSAction) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      logger.warn('move', 'blocked', { type: action.type, reason: 'not_connected' });
      setStatus('Not connected');
      setStatusIsError(true);
      return;
    }
    logger.info('move', 'send', {
      type: action.type,
      card: action.card,
      cards: action.cards,
      cardIndex: action.cardIndex,
      meldId: action.meldId,
      from: action.from,
      position: action.position,
    });
    apiClient.sendWS(ws, action);
  }, []);

  const reconnect = useCallback(() => {
    connect();
  }, [connect]);

  return { state, status, statusIsError, connected, send, reconnect, setState };
}
