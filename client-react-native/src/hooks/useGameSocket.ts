import { useCallback, useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { GameState, MeldPreview, WSAction, WSEnvelope } from '@/src/api/types';
import { logger } from '@/src/lib/logger';

/**
 * What the server did with one action sent via sendAwait: `ok` means the
 * action was accepted and `state` is the game_state it produced; otherwise
 * `message` is the server's own rejection text (or a connection failure).
 */
export type SendResult = { ok: true; state: GameState } | { ok: false; message: string };

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
  // The server's verdict on the cards currently staged. Read-only and
  // per-connection: a preview is never persisted or broadcast, so it is
  // cleared whenever fresh state arrives and the staged selection is gone.
  const [preview, setPreview] = useState<MeldPreview | null>(null);
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
  // Set while a sendAwait() is in flight: the next game_state (the server
  // accepted it) or error (it did not) settles it. Only ever one at a time —
  // sendAwait's callers are sequential, and the server processes one action
  // per connection at a time.
  const waiterRef = useRef<{
    resolve: (r: SendResult) => void;
    timer: ReturnType<typeof setTimeout>;
  } | null>(null);

  const settle = useCallback((result: SendResult) => {
    const waiter = waiterRef.current;
    if (!waiter) return;
    waiterRef.current = null;
    clearTimeout(waiter.timer);
    waiter.resolve(result);
  }, []);

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
          setPreview(null);
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
          settle({ ok: true, state: st });
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
        } else if (t === 'meld_preview') {
          setPreview(envelope as unknown as MeldPreview);
        } else if (t === 'error') {
          logger.warn('ws', 'server_error', { message: envelope.message });
          setStatus(String(envelope.message ?? 'Something went wrong'));
          setStatusIsError(true);
          settle({ ok: false, message: String(envelope.message ?? 'Something went wrong') });
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
      // A superseded socket (one connect() already replaced) must not report
      // on behalf of the live one: its failure says nothing about the
      // connection the player actually has, and flipping `connected` here
      // would strand the UI on "reconnecting…" over a healthy socket, since
      // only a fresh onopen ever sets it back.
      if (wsRef.current !== ws) return;
      // Not fatal on its own — onclose fires right after and drives the
      // auto-reconnect, so logging at 'error' here would pop a disruptive
      // LogBox red screen on every routine reconnect attempt.
      logger.warn('ws', 'socket_error');
      setStatus('Connection error');
      setStatusIsError(true);
      setConnected(false);
    };

    ws.onclose = () => {
      logger.warn('ws', 'closed', {
        deliberate: closingRef.current,
        stale: wsRef.current !== ws,
        nextAttempt: closingRef.current ? undefined : reconnectAttemptsRef.current + 1,
      });
      // Everything below belongs to the *live* socket. A superseded one
      // closing is expected bookkeeping from a connect() that already moved
      // on, so it must not clear `connected` (nothing would set it back
      // true short of another reconnect, pinning the UI on "reconnecting…"
      // over a working socket) and must not clear the stable-connection
      // timer out from under its replacement (which would stop the backoff
      // from ever resetting).
      if (wsRef.current !== ws) return;
      // Placed below the staleness guard on purpose: a superseded socket
      // closing is routine bookkeeping and must not resolve a wait that
      // belongs to the socket that replaced it.
      settle({ ok: false, message: 'Lost the connection before the server answered' });
      setConnected(false);
      if (stableTimerRef.current) {
        clearTimeout(stableTimerRef.current);
        stableTimerRef.current = null;
      }
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
  }, [gameId, enabled, settle]);

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

  /**
   * Sends an action and waits for the server's verdict on it — the next
   * game_state (accepted) or error (rejected).
   *
   * The socket is otherwise fire-and-forget, which is fine for a move whose
   * only failure mode is a banner. It is not fine when one move's outcome
   * decides whether a *second* one should be sent at all: laying a staged
   * meld before discarding (see layPendingMeldsThenDiscard in the game
   * screen) must not throw the discard after a meld the server refused.
   *
   * Safe because the server only broadcasts in response to an action, and
   * during your own turn every action is yours — nothing else can slip a
   * game_state in between. A timeout resolves as a failure rather than
   * hanging, so a caller never ends up stuck mid-sequence.
   */
  const sendAwait = useCallback(
    (action: WSAction, timeoutMs = 5000): Promise<SendResult> => {
      const ws = wsRef.current;
      if (!ws || ws.readyState !== WebSocket.OPEN) {
        logger.warn('move', 'blocked', { type: action.type, reason: 'not_connected' });
        setStatus('Not connected');
        setStatusIsError(true);
        return Promise.resolve({ ok: false, message: 'Not connected' });
      }
      // A previous wait that never settled must not swallow this one's answer.
      settle({ ok: false, message: 'Superseded by a newer action' });
      return new Promise<SendResult>((resolve) => {
        waiterRef.current = {
          resolve,
          timer: setTimeout(
            () => settle({ ok: false, message: 'The server did not answer in time' }),
            timeoutMs,
          ),
        };
        send(action);
      });
    },
    [send, settle],
  );

  /** Raises a message in the same banner the server's own errors use. */
  const notify = useCallback((message: string, isError = true) => {
    setStatus(message);
    setStatusIsError(isError);
  }, []);

  const reconnect = useCallback(() => {
    connect();
  }, [connect]);

  return {
    state,
    status,
    statusIsError,
    connected,
    send,
    sendAwait,
    notify,
    reconnect,
    setState,
    preview,
    setPreview,
  };
}
